package coordinator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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
}

// NewClientPool creates a new ClientPool.
func NewClientPool(metaAddr string, storageAddrs map[string]string) *ClientPool {
	return &ClientPool{
		storageConns: make(map[string]*grpc.ClientConn),
		storageAddrs: storageAddrs,
		metaAddr:     metaAddr,
	}
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
	// A NotFound error indicates the node is running and responding to RPCs
	return addr, nil
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
