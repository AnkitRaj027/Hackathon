package coordinator

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

	"vault/internal/checksum"
	"vault/internal/config"
	"vault/internal/metadata"
	"vault/internal/placement"
	"vault/internal/storage"
	pbCoord "vault/proto/coordinator"
	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

type testCluster struct {
	metaServer     *grpc.Server
	metaAddr       string
	storageServers map[string]*grpc.Server
	storageDirs    map[string]string
	storageAddrs   map[string]string
	coordServer    *grpc.Server
	coordAddr      string
	pool           *ClientPool
}

func startTestCluster(t *testing.T) *testCluster {
	tc := &testCluster{
		storageServers: make(map[string]*grpc.Server),
		storageDirs:    make(map[string]string),
		storageAddrs:   make(map[string]string),
	}

	// 1. Start Metadata Server
	metaLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for metadata: %v", err)
	}
	tc.metaAddr = metaLis.Addr().String()
	tc.metaServer = grpc.NewServer()
	metaStore := metadata.NewMemoryStore()
	pbMeta.RegisterMetadataServiceServer(tc.metaServer, metadata.NewServer(metaStore))
	go func() { _ = tc.metaServer.Serve(metaLis) }()

	// 2. Start 3 Storage Nodes
	nodeIDs := []string{"storage-01", "storage-02", "storage-03"}
	for _, nid := range nodeIDs {
		dir, err := os.MkdirTemp("", "vault-test-"+nid+"-*")
		if err != nil {
			t.Fatalf("failed creating dir for %s: %v", nid, err)
		}
		tc.storageDirs[nid] = dir

		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed listening for %s: %v", nid, err)
		}
		tc.storageAddrs[nid] = lis.Addr().String()

		sServer, err := storage.NewServer(nid, dir)
		if err != nil {
			t.Fatalf("failed creating storage server: %v", err)
		}
		grpcS := grpc.NewServer()
		pbStorage.RegisterStorageServiceServer(grpcS, sServer)
		tc.storageServers[nid] = grpcS
		go func(l net.Listener, gs *grpc.Server) { _ = gs.Serve(l) }(lis, grpcS)
	}

	// 3. Start Coordinator
	coordCfg := &config.CoordinatorConfig{
		ChunkSize:         1024, // Small 1KB chunks for fast multi-chunk testing
		ReplicationFactor: 3,
		WriteQuorum:       3,
		ReadQuorum:        1,
		StorageNodes:      tc.storageAddrs,
		MetadataAddr:      tc.metaAddr,
	}

	tc.pool = NewClientPool(tc.metaAddr, tc.storageAddrs)
	place, err := placement.NewFixedReplicationPlacement(nodeIDs, 3)
	if err != nil {
		t.Fatalf("failed creating placement: %v", err)
	}

	coordSvc := NewService(coordCfg, tc.pool, place)
	coordLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed listening for coordinator: %v", err)
	}
	tc.coordAddr = coordLis.Addr().String()
	tc.coordServer = grpc.NewServer()
	pbCoord.RegisterCoordinatorServiceServer(tc.coordServer, coordSvc)
	go func() { _ = tc.coordServer.Serve(coordLis) }()

	return tc
}

func (tc *testCluster) Stop() {
	tc.coordServer.GracefulStop()
	for _, gs := range tc.storageServers {
		gs.GracefulStop()
	}
	tc.metaServer.GracefulStop()
	tc.pool.Close()
	for _, dir := range tc.storageDirs {
		_ = os.RemoveAll(dir)
	}
}

type mockPutStream struct {
	grpc.ServerStream
	ctx      context.Context
	messages []*pbCoord.PutObjectRequest
	index    int
	resp     *pbCoord.PutObjectResponse
}

func (m *mockPutStream) Context() context.Context { return m.ctx }
func (m *mockPutStream) Recv() (*pbCoord.PutObjectRequest, error) {
	if m.index >= len(m.messages) {
		return nil, io.EOF
	}
	msg := m.messages[m.index]
	m.index++
	return msg, nil
}
func (m *mockPutStream) SendAndClose(resp *pbCoord.PutObjectResponse) error {
	m.resp = resp
	return nil
}

type mockGetStream struct {
	grpc.ServerStream
	ctx      context.Context
	messages []*pbCoord.GetObjectResponse
}

func (m *mockGetStream) Context() context.Context { return m.ctx }
func (m *mockGetStream) Send(resp *pbCoord.GetObjectResponse) error {
	m.messages = append(m.messages, resp)
	return nil
}

func TestCoordinatorEndToEnd(t *testing.T) {
	cluster := startTestCluster(t)
	defer cluster.Stop()

	coordCfg := &config.CoordinatorConfig{
		ChunkSize:         1024,
		ReplicationFactor: 3,
		WriteQuorum:       3,
		ReadQuorum:        1,
	}
	place, _ := placement.NewFixedReplicationPlacement([]string{"storage-01", "storage-02", "storage-03"}, 3)
	svc := NewService(coordCfg, cluster.pool, place)

	ctx := context.Background()

	// 1. Generate 3.5 KB test payload (will produce 4 chunks: 1024, 1024, 1024, 512)
	payload := make([]byte, 3584)
	_, _ = rand.Read(payload)
	originalSHA := checksum.ComputeBytes(payload)
	objKey := "test/multichunk.bin"

	// 2. PUT object
	putStream := &mockPutStream{
		ctx: ctx,
		messages: []*pbCoord.PutObjectRequest{
			{
				Payload: &pbCoord.PutObjectRequest_Header{
					Header: &pbCoord.ObjectHeader{
						Key:  objKey,
						Size: int64(len(payload)),
					},
				},
			},
			{
				Payload: &pbCoord.PutObjectRequest_ChunkData{
					ChunkData: payload,
				},
			},
		},
	}

	err := svc.PutObject(putStream)
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	if putStream.resp == nil || !putStream.resp.Success {
		t.Fatalf("PutObject response indicated failure")
	}
	if putStream.resp.ChunkCount != 4 {
		t.Fatalf("expected 4 chunks, got %d", putStream.resp.ChunkCount)
	}
	if putStream.resp.Checksum != originalSHA {
		t.Fatalf("expected checksum %s, got %s", originalSHA, putStream.resp.Checksum)
	}

	// 3. Verify RF=3 physical placement across all 3 nodes
	safeKey := "test_multichunk.bin"
	for nid, dir := range cluster.storageDirs {
		for i := 0; i < 4; i++ {
			chunkName := fmt.Sprintf("%s.chunk.%04d.chunk", safeKey, i)
			sidecarName := fmt.Sprintf("%s.chunk.%04d.sha256", safeKey, i)
			if _, err := os.Stat(filepath.Join(dir, chunkName)); err != nil {
				t.Errorf("node %s missing chunk %s: %v", nid, chunkName, err)
			}
			if _, err := os.Stat(filepath.Join(dir, sidecarName)); err != nil {
				t.Errorf("node %s missing sidecar %s: %v", nid, sidecarName, err)
			}
		}
	}

	// 4. Test GET with exact reconstruction
	getStream := &mockGetStream{ctx: ctx}
	err = svc.GetObject(&pbCoord.GetObjectRequest{Key: objKey}, getStream)
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}

	var downloaded bytes.Buffer
	for _, m := range getStream.messages {
		if cData := m.GetChunkData(); len(cData) > 0 {
			downloaded.Write(cData)
		}
	}

	if downloaded.Len() != len(payload) {
		t.Fatalf("expected %d bytes, got %d", len(payload), downloaded.Len())
	}
	downloadedSHA := checksum.ComputeBytes(downloaded.Bytes())
	if downloadedSHA != originalSHA {
		t.Fatalf("downloaded SHA mismatch: expected %s, got %s", originalSHA, downloadedSHA)
	}

	// 5. Test Checksum Verification & Failover
	// Corrupt chunk 0 on storage-01
	corruptFile := filepath.Join(cluster.storageDirs["storage-01"], fmt.Sprintf("%s.chunk.0000.chunk", safeKey))
	if err := os.WriteFile(corruptFile, []byte("bad-corrupt-data"), 0644); err != nil {
		t.Fatalf("failed to corrupt file: %v", err)
	}

	// GET again: it should detect corruption on storage-01, failover to storage-02, and succeed!
	failoverGetStream := &mockGetStream{ctx: ctx}
	err = svc.GetObject(&pbCoord.GetObjectRequest{Key: objKey}, failoverGetStream)
	if err != nil {
		t.Fatalf("GetObject failover failed: %v", err)
	}
	var failoverDownloaded bytes.Buffer
	for _, m := range failoverGetStream.messages {
		if cData := m.GetChunkData(); len(cData) > 0 {
			failoverDownloaded.Write(cData)
		}
	}
	if checksum.ComputeBytes(failoverDownloaded.Bytes()) != originalSHA {
		t.Fatalf("failover data corrupted! Did not match original")
	}

	// 6. Test Inspect
	inspectResp, err := svc.InspectObject(ctx, &pbCoord.InspectObjectRequest{Key: objKey})
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if inspectResp.Metadata.Key != objKey {
		t.Fatalf("inspect key mismatch")
	}
	if len(inspectResp.Metadata.Chunks) != 4 {
		t.Fatalf("inspect chunk count mismatch")
	}

	// 7. Test Delete
	delResp, err := svc.DeleteObject(ctx, &pbCoord.DeleteObjectRequest{Key: objKey})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !delResp.Success {
		t.Fatalf("Delete reported failure")
	}

	// Verify chunks deleted on nodes
	for nid, dir := range cluster.storageDirs {
		for i := 1; i < 4; i++ { // skip 0 since we modified it manually earlier
			chunkName := fmt.Sprintf("%s.chunk.%04d.chunk", safeKey, i)
			if _, err := os.Stat(filepath.Join(dir, chunkName)); !os.IsNotExist(err) {
				t.Errorf("node %s did not delete chunk %s", nid, chunkName)
			}
		}
	}
}
