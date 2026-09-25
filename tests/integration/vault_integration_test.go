package integration

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
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"vault/internal/checksum"
	"vault/internal/config"
	"vault/internal/coordinator"
	"vault/internal/metadata"
	"vault/internal/placement"
	"vault/internal/storage"
	pbCoord "vault/proto/coordinator"
	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

type IntegrationCluster struct {
	metaServer     *grpc.Server
	metaAddr       string
	storageServers map[string]*grpc.Server
	storageLis     map[string]net.Listener
	storageDirs    map[string]string
	storageAddrs   map[string]string
	coordServer    *grpc.Server
	coordAddr      string
	coordConn      *grpc.ClientConn
	Client         pbCoord.CoordinatorServiceClient
}

func setupIntegrationCluster(t *testing.T, chunkSize int64) *IntegrationCluster {
	ic := &IntegrationCluster{
		storageServers: make(map[string]*grpc.Server),
		storageLis:     make(map[string]net.Listener),
		storageDirs:    make(map[string]string),
		storageAddrs:   make(map[string]string),
	}

	// 1. Metadata Server
	metaLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for metadata: %v", err)
	}
	ic.metaAddr = metaLis.Addr().String()
	ic.metaServer = grpc.NewServer()
	metaStore := metadata.NewMemoryStore()
	pbMeta.RegisterMetadataServiceServer(ic.metaServer, metadata.NewServer(metaStore))
	go func() { _ = ic.metaServer.Serve(metaLis) }()

	// 2. 3 Storage Nodes
	nodeIDs := []string{"storage-01", "storage-02", "storage-03"}
	for _, nid := range nodeIDs {
		dir, err := os.MkdirTemp("", "vault-integ-"+nid+"-*")
		if err != nil {
			t.Fatalf("failed to create dir for %s: %v", nid, err)
		}
		ic.storageDirs[nid] = dir

		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to listen for %s: %v", nid, err)
		}
		ic.storageLis[nid] = lis
		ic.storageAddrs[nid] = lis.Addr().String()

		sServer, err := storage.NewServer(nid, dir)
		if err != nil {
			t.Fatalf("failed to create storage server: %v", err)
		}
		grpcS := grpc.NewServer()
		pbStorage.RegisterStorageServiceServer(grpcS, sServer)
		ic.storageServers[nid] = grpcS
		go func(l net.Listener, gs *grpc.Server) { _ = gs.Serve(l) }(lis, grpcS)
	}

	// 3. Coordinator
	coordCfg := &config.CoordinatorConfig{
		ChunkSize:         chunkSize,
		ReplicationFactor: 3,
		WriteQuorum:       3,
		ReadQuorum:        1,
		StorageNodes:      ic.storageAddrs,
		MetadataAddr:      ic.metaAddr,
	}

	pool := coordinator.NewClientPool(ic.metaAddr, ic.storageAddrs)
	place, err := placement.NewFixedReplicationPlacement(nodeIDs, 3)
	if err != nil {
		t.Fatalf("failed creating placement: %v", err)
	}

	coordSvc := coordinator.NewService(coordCfg, pool, place)
	coordLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for coordinator: %v", err)
	}
	ic.coordAddr = coordLis.Addr().String()
	ic.coordServer = grpc.NewServer()
	pbCoord.RegisterCoordinatorServiceServer(ic.coordServer, coordSvc)
	go func() { _ = ic.coordServer.Serve(coordLis) }()

	// 4. Client Connection
	conn, err := grpc.NewClient(ic.coordAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed connecting client to coordinator: %v", err)
	}
	ic.coordConn = conn
	ic.Client = pbCoord.NewCoordinatorServiceClient(conn)

	return ic
}

func (ic *IntegrationCluster) Teardown() {
	if ic.coordConn != nil {
		_ = ic.coordConn.Close()
	}
	if ic.coordServer != nil {
		ic.coordServer.GracefulStop()
	}
	for _, gs := range ic.storageServers {
		gs.GracefulStop()
	}
	if ic.metaServer != nil {
		ic.metaServer.GracefulStop()
	}
	for _, dir := range ic.storageDirs {
		_ = os.RemoveAll(dir)
	}
}

// uploadHelper uploads data using coordinator streaming PutObject.
func uploadHelper(ctx context.Context, client pbCoord.CoordinatorServiceClient, key string, data []byte) (*pbCoord.PutObjectResponse, error) {
	stream, err := client.PutObject(ctx)
	if err != nil {
		return nil, err
	}

	if err := stream.Send(&pbCoord.PutObjectRequest{
		Payload: &pbCoord.PutObjectRequest_Header{
			Header: &pbCoord.ObjectHeader{
				Key:  key,
				Size: int64(len(data)),
			},
		},
	}); err != nil {
		return nil, err
	}

	// Stream chunks in 16KB batches
	batchSize := 16 * 1024
	r := bytes.NewReader(data)
	buf := make([]byte, batchSize)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if sErr := stream.Send(&pbCoord.PutObjectRequest{
				Payload: &pbCoord.PutObjectRequest_ChunkData{
					ChunkData: buf[:n],
				},
			}); sErr != nil {
				return nil, sErr
			}
		}
		if err != nil {
			break
		}
	}

	return stream.CloseAndRecv()
}

// downloadHelper downloads an object using coordinator streaming GetObject.
func downloadHelper(ctx context.Context, client pbCoord.CoordinatorServiceClient, key string) ([]byte, error) {
	stream, err := client.GetObject(ctx, &pbCoord.GetObjectRequest{Key: key})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if cData := resp.GetChunkData(); len(cData) > 0 {
			buf.Write(cData)
		}
	}
	return buf.Bytes(), nil
}

// Test 1: Single-Node Storage Mechanics
func TestIntegration_SingleNodeStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vault-single-node-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	server, err := storage.NewServer("node-single", tempDir)
	if err != nil {
		t.Fatalf("failed to create storage server: %v", err)
	}

	ctx := context.Background()
	payload := []byte("deterministic single node chunk persistence test")
	chunkID := "sample.chunk.0000"
	expectedHash := checksum.ComputeBytes(payload)

	putResp, err := server.PutChunk(ctx, &pbStorage.PutChunkRequest{
		ChunkId:  chunkID,
		Data:     payload,
		Checksum: expectedHash,
	})
	if err != nil || !putResp.Success {
		t.Fatalf("PutChunk failed: %v", err)
	}

	// Verify disk files
	chunkFile := filepath.Join(tempDir, chunkID+".chunk")
	sidecarFile := filepath.Join(tempDir, chunkID+".sha256")
	if _, err := os.Stat(chunkFile); err != nil {
		t.Fatalf("missing chunk file: %v", err)
	}
	if _, err := os.Stat(sidecarFile); err != nil {
		t.Fatalf("missing sidecar file: %v", err)
	}

	// Read and verify
	sidecarBytes, _ := os.ReadFile(sidecarFile)
	if string(sidecarBytes) != expectedHash {
		t.Fatalf("sidecar content mismatch: expected %s, got %s", expectedHash, string(sidecarBytes))
	}
}

// Test 2: Three-Node Replication (RF=3) and Multi-Chunk Distribution
func TestIntegration_ThreeNodeReplication(t *testing.T) {
	cluster := setupIntegrationCluster(t, 2048) // 2KB chunks
	defer cluster.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 5KB payload -> 3 chunks (2048, 2048, 1024)
	payload := make([]byte, 5120)
	_, _ = rand.Read(payload)
	key := "reports/q3_financials.bin"

	putResp, err := uploadHelper(ctx, cluster.Client, key, payload)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if putResp.ChunkCount != 3 {
		t.Fatalf("expected 3 chunks, got %d", putResp.ChunkCount)
	}
	if putResp.ReplicationFactor != 3 {
		t.Fatalf("expected RF=3, got %d", putResp.ReplicationFactor)
	}

	// Verify every chunk exists on all 3 storage nodes
	safeKey := "reports_q3_financials.bin"
	for nodeID, dir := range cluster.storageDirs {
		for i := 0; i < 3; i++ {
			cName := fmt.Sprintf("%s.chunk.%04d.chunk", safeKey, i)
			sName := fmt.Sprintf("%s.chunk.%04d.sha256", safeKey, i)
			if _, err := os.Stat(filepath.Join(dir, cName)); err != nil {
				t.Fatalf("node %s missing chunk %s", nodeID, cName)
			}
			if _, err := os.Stat(filepath.Join(dir, sName)); err != nil {
				t.Fatalf("node %s missing sidecar %s", nodeID, sName)
			}
		}
	}
}

// Test 3: Checksum Correctness and Corruption Detection
func TestIntegration_ChecksumVerification(t *testing.T) {
	cluster := setupIntegrationCluster(t, 4096)
	defer cluster.Teardown()

	ctx := context.Background()
	payload := []byte("integrity test data sequence")
	key := "security/token.pem"

	_, err := uploadHelper(ctx, cluster.Client, key, payload)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Corrupt chunk on storage-02
	safeKey := "security_token.pem"
	chunkPath := filepath.Join(cluster.storageDirs["storage-02"], safeKey+".chunk.0000.chunk")
	if err := os.WriteFile(chunkPath, []byte("MALICIOUS_TAMPERED_BITS"), 0644); err != nil {
		t.Fatalf("failed corrupting file: %v", err)
	}

	// Connect directly to storage-02 and verify checksum fails
	sc, err := cluster.poolClient(cluster.storageAddrs["storage-02"])
	if err != nil {
		t.Fatalf("failed connecting to storage-02: %v", err)
	}
	verifyResp, err := sc.VerifyChecksum(ctx, &pbStorage.VerifyChecksumRequest{
		ChunkId: safeKey + ".chunk.0000",
	})
	if err != nil {
		t.Fatalf("VerifyChecksum error: %v", err)
	}
	if verifyResp.Valid {
		t.Fatalf("expected chunk on storage-02 to be INVALID, but was reported valid")
	}
}

// Test 4: Object Reconstruction Exactness
func TestIntegration_ObjectReconstruction(t *testing.T) {
	cluster := setupIntegrationCluster(t, 1024)
	defer cluster.Teardown()

	ctx := context.Background()
	payload := make([]byte, 10000)
	_, _ = rand.Read(payload)
	originalHash := checksum.ComputeBytes(payload)
	key := "archives/dataset.tar"

	_, err := uploadHelper(ctx, cluster.Client, key, payload)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	downloaded, err := downloadHelper(ctx, cluster.Client, key)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	downloadedHash := checksum.ComputeBytes(downloaded)
	if downloadedHash != originalHash {
		t.Fatalf("checksum mismatch: expected %s, got %s", originalHash, downloadedHash)
	}
	if !bytes.Equal(payload, downloaded) {
		t.Fatalf("reconstructed byte sequence does not match original")
	}
}

// Test 5: Large Multi-Megabyte Object
func TestIntegration_LargeObject(t *testing.T) {
	// 5 MiB object with 512 KiB chunks -> 10 chunks
	cluster := setupIntegrationCluster(t, 512*1024)
	defer cluster.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	size := 5 * 1024 * 1024
	payload := make([]byte, size)
	_, _ = rand.Read(payload)
	originalHash := checksum.ComputeBytes(payload)
	key := "large/iso_image.img"

	putResp, err := uploadHelper(ctx, cluster.Client, key, payload)
	if err != nil {
		t.Fatalf("Large upload failed: %v", err)
	}
	if putResp.ChunkCount != 10 {
		t.Fatalf("expected 10 chunks for 5MB object with 512KB chunks, got %d", putResp.ChunkCount)
	}

	downloaded, err := downloadHelper(ctx, cluster.Client, key)
	if err != nil {
		t.Fatalf("Large download failed: %v", err)
	}
	if len(downloaded) != size {
		t.Fatalf("expected %d bytes, got %d", size, len(downloaded))
	}
	if checksum.ComputeBytes(downloaded) != originalHash {
		t.Fatalf("large object hash mismatch")
	}
}

// Test 6: Idempotent GET
func TestIntegration_IdempotentGet(t *testing.T) {
	cluster := setupIntegrationCluster(t, 2048)
	defer cluster.Teardown()

	ctx := context.Background()
	payload := []byte("consistent repeated download validation")
	key := "idempotent/file.txt"

	_, err := uploadHelper(ctx, cluster.Client, key, payload)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		downloaded, err := downloadHelper(ctx, cluster.Client, key)
		if err != nil {
			t.Fatalf("iteration %d download failed: %v", i, err)
		}
		if !bytes.Equal(downloaded, payload) {
			t.Fatalf("iteration %d data mismatch", i)
		}
	}
}

// Test 7: Duplicate PUT Behavior (Deterministic Overwrite)
func TestIntegration_DuplicatePut(t *testing.T) {
	cluster := setupIntegrationCluster(t, 2048)
	defer cluster.Teardown()

	ctx := context.Background()
	v1Data := []byte("version 1 initial content")
	v2Data := []byte("version 2 updated replacement content that is longer")
	key := "config/app.json"

	// PUT v1
	_, err := uploadHelper(ctx, cluster.Client, key, v1Data)
	if err != nil {
		t.Fatalf("Upload v1 failed: %v", err)
	}

	// PUT v2 with same key
	resp2, err := uploadHelper(ctx, cluster.Client, key, v2Data)
	if err != nil {
		t.Fatalf("Upload v2 duplicate failed: %v", err)
	}
	if resp2.Checksum != checksum.ComputeBytes(v2Data) {
		t.Fatalf("expected updated checksum on duplicate put")
	}

	// GET should return v2
	downloaded, err := downloadHelper(ctx, cluster.Client, key)
	if err != nil {
		t.Fatalf("Download after duplicate put failed: %v", err)
	}
	if !bytes.Equal(downloaded, v2Data) {
		t.Fatalf("expected v2 data after overwrite, got: %s", string(downloaded))
	}
}

// Test 8: Failure Tolerance (Node Stop & Read Quorum R=1)
func TestIntegration_NodeFailureReadTolerance(t *testing.T) {
	cluster := setupIntegrationCluster(t, 1024)
	defer cluster.Teardown()

	ctx := context.Background()
	payload := []byte("distributed resiliency across node failure")
	key := "resilience/test.bin"

	_, err := uploadHelper(ctx, cluster.Client, key, payload)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// Stop storage-01 process!
	cluster.storageServers["storage-01"].Stop()
	_ = cluster.storageLis["storage-01"].Close()

	// With R=1 and storage-02 and storage-03 alive, GET must still succeed seamlessly!
	downloaded, err := downloadHelper(ctx, cluster.Client, key)
	if err != nil {
		t.Fatalf("Download failed after node failure: %v", err)
	}
	if !bytes.Equal(downloaded, payload) {
		t.Fatalf("data mismatch after reading from surviving replicas")
	}
}

func (ic *IntegrationCluster) poolClient(addr string) (pbStorage.StorageServiceClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return pbStorage.NewStorageServiceClient(conn), nil
}
