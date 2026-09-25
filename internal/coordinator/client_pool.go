package coordinator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

// ClientPool manages gRPC connections to storage nodes and metadata service.
type ClientPool struct {
	mu           sync.RWMutex
	storageConns map[string]*grpc.ClientConn
	storageAddrs map[string]string

	metaConn *grpc.ClientConn
	metaAddr string

	partitioned map[string]bool
}

// NewClientPool creates a new ClientPool.
func NewClientPool(metaAddr string, storageAddrs map[string]string) *ClientPool {
	return &ClientPool{
		storageConns: make(map[string]*grpc.ClientConn),
		storageAddrs: storageAddrs,
		metaAddr:     metaAddr,
		partitioned:  make(map[string]bool),
	}
}

// SetPartitioned simulates a network partition for a storage node.
func (p *ClientPool) SetPartitioned(nodeID string, partitioned bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.partitioned == nil {
		p.partitioned = make(map[string]bool)
	}
	p.partitioned[nodeID] = partitioned
	if partitioned {
		if conn, ok := p.storageConns[nodeID]; ok {
			_ = conn.Close()
			delete(p.storageConns, nodeID)
		}
	}
}

// IsPartitioned checks if a storage node is currently partitioned from the network.
func (p *ClientPool) IsPartitioned(nodeID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.partitioned != nil && p.partitioned[nodeID]
}

// AddStorageNode dynamically registers a new storage node.
func (p *ClientPool) AddStorageNode(nodeID string, addr string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.storageAddrs[nodeID] = addr
}

// StorageNodes returns a map of all configured storage node IDs to their addresses.
func (p *ClientPool) StorageNodes() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	res := make(map[string]string, len(p.storageAddrs))
	for k, v := range p.storageAddrs {
		res[k] = v
	}
	return res
}

// GetMetadataClient returns a connected MetadataServiceClient.
func (p *ClientPool) GetMetadataClient() (pbMeta.MetadataServiceClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.metaConn == nil {
		conn, err := grpc.NewClient(
			p.metaAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(64*1024*1024),
				grpc.MaxCallSendMsgSize(64*1024*1024),
			),
		)
		if err != nil {
			return nil, fmt.Errorf("failed connecting to metadata service at %s: %w", p.metaAddr, err)
		}
		p.metaConn = conn
	}

	return pbMeta.NewMetadataServiceClient(p.metaConn), nil
}

// GetStorageClient returns a connected StorageServiceClient for the specified node ID.
func (p *ClientPool) GetStorageClient(nodeID string) (pbStorage.StorageServiceClient, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.partitioned != nil && p.partitioned[nodeID] {
		return nil, status.Error(codes.Unavailable, fmt.Sprintf("network partition: storage node %s is unreachable", nodeID))
	}

	if conn, ok := p.storageConns[nodeID]; ok {
		return pbStorage.NewStorageServiceClient(conn), nil
	}

	addr, ok := p.storageAddrs[nodeID]
	if !ok {
		return nil, fmt.Errorf("node %s not found in cluster configuration", nodeID)
	}

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64*1024*1024),
			grpc.MaxCallSendMsgSize(64*1024*1024),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed connecting to storage node %s at %s: %w", nodeID, addr, err)
	}

	p.storageConns[nodeID] = conn
	return pbStorage.NewStorageServiceClient(conn), nil
}

// CheckStorageNode checks if a storage node is reachable.
func (p *ClientPool) CheckStorageNode(ctx context.Context, nodeID string) (string, error) {
	if p.IsPartitioned(nodeID) {
		p.mu.RLock()
		addr := p.storageAddrs[nodeID]
		p.mu.RUnlock()
		return addr, status.Error(codes.Unavailable, fmt.Sprintf("network partition: storage node %s is unreachable", nodeID))
	}

	addr, ok := p.storageAddrs[nodeID]
	if !ok {
		return "", fmt.Errorf("unknown node: %s", nodeID)
	}

	client, err := p.GetStorageClient(nodeID)
	if err != nil {
		return addr, err
	}

	// Probe with quick timeout
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err = client.VerifyChecksum(probeCtx, &pbStorage.VerifyChecksumRequest{
		ChunkId: "__probe__",
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.NotFound || st.Code() == codes.InvalidArgument) {
			// Node is running and actively responding to gRPC requests
			return addr, nil
		}
		// Connection refused, unavailable, timeout, etc.
		p.InvalidateStorageConn(nodeID)
		return addr, err
	}
	return addr, nil
}

// InvalidateStorageConn discards a cached connection so reconnect attempts dial freshly.
func (p *ClientPool) InvalidateStorageConn(nodeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if conn, ok := p.storageConns[nodeID]; ok {
		_ = conn.Close()
		delete(p.storageConns, nodeID)
	}
}

// Close closes all pooled connections.
func (p *ClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.metaConn != nil {
		_ = p.metaConn.Close()
		p.metaConn = nil
	}
	for id, conn := range p.storageConns {
		_ = conn.Close()
		delete(p.storageConns, id)
	}
}
