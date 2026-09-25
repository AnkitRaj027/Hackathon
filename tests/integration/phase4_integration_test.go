package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"

	"vault/internal/checksum"
	"vault/internal/coordinator"
	"vault/internal/erasure"
	"vault/internal/metadata"
	"vault/internal/placement"
	"vault/internal/storage"
	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

type Phase4Cluster struct {
	metaServer     *grpc.Server
	metaAddr       string
	storageServers map[string]*grpc.Server
	storageLis     map[string]net.Listener
	storageDirs    map[string]string
	storageAddrs   map[string]string
	Pool           *coordinator.ClientPool
	Ring           *placement.ConsistentHashRing
	Pipeline       *erasure.Pipeline
	NodeIDs        []string
}

func setupPhase4Cluster(t *testing.T, dataShards, parityShards int) *Phase4Cluster {
	totalShards := dataShards + parityShards
	nodeIDs := make([]string, totalShards)
	for i := 0; i < totalShards; i++ {
		nodeIDs[i] = fmt.Sprintf("storage-ec-%02d", i+1)
	}

	pc := &Phase4Cluster{
		storageServers: make(map[string]*grpc.Server),
		storageLis:     make(map[string]net.Listener),
		storageDirs:    make(map[string]string),
		storageAddrs:   make(map[string]string),
		NodeIDs:        nodeIDs,
	}

	// 1. Metadata Server
	metaLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for metadata: %v", err)
	}
	pc.metaAddr = metaLis.Addr().String()
	pc.metaServer = grpc.NewServer()
	metaStore := metadata.NewMemoryStore()
	pbMeta.RegisterMetadataServiceServer(pc.metaServer, metadata.NewServer(metaStore))
	go func() { _ = pc.metaServer.Serve(metaLis) }()

	// 2. Storage Nodes
	for _, nid := range pc.NodeIDs {
		dir, err := os.MkdirTemp("", "vault-p4-"+nid+"-*")
		if err != nil {
			t.Fatalf("failed creating dir for %s: %v", nid, err)
		}
		pc.storageDirs[nid] = dir

		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed listening for %s: %v", nid, err)
		}
		pc.storageLis[nid] = lis
		pc.storageAddrs[nid] = lis.Addr().String()

		sServer, err := storage.NewServer(nid, dir)
		if err != nil {
			t.Fatalf("failed creating storage server: %v", err)
		}
		grpcS := grpc.NewServer(
			grpc.MaxRecvMsgSize(64*1024*1024),
			grpc.MaxSendMsgSize(64*1024*1024),
		)
		pbStorage.RegisterStorageServiceServer(grpcS, sServer)
		pc.storageServers[nid] = grpcS
		go func(l net.Listener, gs *grpc.Server) { _ = gs.Serve(l) }(lis, grpcS)
	}

	// 3. Consistent Hash Ring
	ring, err := placement.NewConsistentHashRing(pc.NodeIDs, 128, totalShards)
	if err != nil {
		t.Fatalf("failed creating ring: %v", err)
	}
	pc.Ring = ring

	// 4. Client Pool & Erasure Pipeline
	pool := coordinator.NewClientPool(pc.metaAddr, pc.storageAddrs)
	pc.Pool = pool

	pipeline, err := erasure.NewPipeline(dataShards, parityShards, pool, ring)
	if err != nil {
		t.Fatalf("failed creating erasure pipeline: %v", err)
	}
	pc.Pipeline = pipeline

	return pc
}

func (pc *Phase4Cluster) Teardown() {
	if pc.Pool != nil {
		pc.Pool.Close()
	}
	for _, gs := range pc.storageServers {
		gs.GracefulStop()
	}
	if pc.metaServer != nil {
		pc.metaServer.GracefulStop()
	}
	for _, dir := range pc.storageDirs {
		_ = os.RemoveAll(dir)
	}
}

// TestPhase4_ErasureCoding_EfficiencyAndStorageSavings verifies that 2+1 Reed-Solomon encoding
// stores an object across 3 nodes with only 1.5x storage overhead (saving 50% vs 3x replication).
func TestPhase4_ErasureCoding_EfficiencyAndStorageSavings(t *testing.T) {
	pc := setupPhase4Cluster(t, 2, 1) // 2 data + 1 parity = 3 nodes
	defer pc.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	payloadSize := 64 * 1024 // 64 KiB
	payload := make([]byte, payloadSize)
	_, _ = rand.Read(payload)
	originalSHA := checksum.ComputeBytes(payload)

	key := "dataset-archive-ec.iso"

	// 1. Put object via Erasure Coding
	objMeta, err := pc.Pipeline.PutObjectEC(ctx, key, payload)
	if err != nil {
		t.Fatalf("PutObjectEC failed: %v", err)
	}

	if objMeta.GetCustomMetadata()["encoding"] != "erasure_coding" {
		t.Fatalf("expected encoding=erasure_coding, got %s", objMeta.GetCustomMetadata()["encoding"])
	}

	// 2. Measure raw disk storage across nodes
	totalDiskBytes := int64(0)
	for _, nid := range pc.NodeIDs {
		dir := pc.storageDirs[nid]
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".chunk" {
				info, _ := e.Info()
				totalDiskBytes += info.Size()
			}
		}
	}

	// For 64 KiB with 2 data shards: each shard is 32 KiB + 8 bytes header = ~32776 bytes.
	// Total across 3 nodes = ~98328 bytes (~1.5x of 64 KiB).
	// Under 3x replication it would be 3 * 64 KiB = 192 KiB.
	overheadRatio := float64(totalDiskBytes) / float64(payloadSize)
	t.Logf("Erasure Coding Storage Metrics:")
	t.Logf("  Original Payload: %d bytes (%.1f KiB)", payloadSize, float64(payloadSize)/1024)
	t.Logf("  Total Disk Space Across 3 Nodes: %d bytes (%.1f KiB)", totalDiskBytes, float64(totalDiskBytes)/1024)
	t.Logf("  Actual Storage Overhead: %.2fx (vs 3.00x with full replication!)", overheadRatio)

	if overheadRatio > 1.60 {
		t.Errorf("storage overhead %.2fx exceeded expected 1.5x bound", overheadRatio)
	}

	// 3. Normal read: retrieve and verify data
	readBytes, err := pc.Pipeline.GetObjectEC(ctx, objMeta)
	if err != nil {
		t.Fatalf("GetObjectEC failed: %v", err)
	}
	if !bytes.Equal(readBytes, payload) {
		t.Fatalf("retrieved bytes do not match original")
	}
	if checksum.ComputeBytes(readBytes) != originalSHA {
		t.Fatalf("checksum mismatch")
	}
}

// TestPhase4_ErasureCoding_MathematicalReconstruction verifies that after the catastrophic loss of a storage node,
// the original object is mathematically reconstructed from surviving shards.
func TestPhase4_ErasureCoding_MathematicalReconstruction(t *testing.T) {
	pc := setupPhase4Cluster(t, 2, 1) // 2 data + 1 parity = 3 nodes
	defer pc.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	payload := []byte("Catastrophic hardware failure test payload. Mathematical Reed-Solomon reconstruction proof.")
	originalSHA := checksum.ComputeBytes(payload)
	key := "resilience-ec-test.bin"

	// 1. Put object
	objMeta, err := pc.Pipeline.PutObjectEC(ctx, key, payload)
	if err != nil {
		t.Fatalf("PutObjectEC failed: %v", err)
	}

	// 2. Kill storage-ec-01 completely (wipe disk and terminate listener)
	crashedNode := "storage-ec-01"
	pc.storageServers[crashedNode].Stop()
	_ = pc.storageLis[crashedNode].Close()
	_ = os.RemoveAll(pc.storageDirs[crashedNode])

	// 3. Retrieve object: only 2 of 3 nodes are alive (k=2 data shards required)
	// The missing shard from storage-ec-01 must be mathematically reconstructed!
	reconstructed, err := pc.Pipeline.GetObjectEC(ctx, objMeta)
	if err != nil {
		t.Fatalf("GetObjectEC failed after node loss: %v", err)
	}

	if !bytes.Equal(reconstructed, payload) {
		t.Fatalf("reconstruction mismatch: got %q, expected %q", string(reconstructed), string(payload))
	}
	if checksum.ComputeBytes(reconstructed) != originalSHA {
		t.Fatalf("reconstructed SHA mismatch: expected %s, got %s", originalSHA, checksum.ComputeBytes(reconstructed))
	}

	t.Logf("SUCCESS: Object mathematically reconstructed from 2 surviving shards after total loss of %s!", crashedNode)
}
