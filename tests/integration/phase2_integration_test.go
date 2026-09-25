package integration

import (
	"bytes"
	"context"
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
	"vault/internal/detector"
	"vault/internal/metadata"
	"vault/internal/placement"
	"vault/internal/repair"
	"vault/internal/storage"
	pbCoord "vault/proto/coordinator"
	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

type Phase2Cluster struct {
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
	Pool           *coordinator.ClientPool
	Detector       *detector.Detector
	RepairMgr      *repair.Manager
	NodeIDs        []string
}

func setupPhase2Cluster(t *testing.T, writeQuorum int) *Phase2Cluster {
	pc := &Phase2Cluster{
		storageServers: make(map[string]*grpc.Server),
		storageLis:     make(map[string]net.Listener),
		storageDirs:    make(map[string]string),
		storageAddrs:   make(map[string]string),
		NodeIDs:        []string{"storage-01", "storage-02", "storage-03"},
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
		dir, err := os.MkdirTemp("", "vault-p2-"+nid+"-*")
		if err != nil {
			t.Fatalf("failed to create dir for %s: %v", nid, err)
		}
		pc.storageDirs[nid] = dir

		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to listen for %s: %v", nid, err)
		}
		pc.storageLis[nid] = lis
		pc.storageAddrs[nid] = lis.Addr().String()

		sServer, err := storage.NewServer(nid, dir)
		if err != nil {
			t.Fatalf("failed to create storage server: %v", err)
		}
		grpcS := grpc.NewServer(
			grpc.MaxRecvMsgSize(64*1024*1024),
			grpc.MaxSendMsgSize(64*1024*1024),
		)
		pbStorage.RegisterStorageServiceServer(grpcS, sServer)
		pc.storageServers[nid] = grpcS
		go func(l net.Listener, gs *grpc.Server) { _ = gs.Serve(l) }(lis, grpcS)
	}

	// 3. Coordinator with Detector and Repair Manager
	coordCfg := &config.CoordinatorConfig{
		ChunkSize:         1024 * 1024, // 1MB for fast tests
		ReplicationFactor: 3,
		WriteQuorum:       writeQuorum,
		ReadQuorum:        1,
		StorageNodes:      pc.storageAddrs,
		MetadataAddr:      pc.metaAddr,
	}

	pool := coordinator.NewClientPool(pc.metaAddr, pc.storageAddrs)
	pc.Pool = pool

	place, err := placement.NewFixedReplicationPlacement(pc.NodeIDs, 3)
	if err != nil {
		t.Fatalf("failed creating placement: %v", err)
	}

	coordSvc := coordinator.NewService(coordCfg, pool, place)

	// Failure detector
	det := detector.NewDetector(pool, pc.NodeIDs, detector.Config{
		ProbeInterval:    100 * time.Millisecond,
		SuspectThreshold: 1,
		DeadThreshold:    2,
	})
	pc.Detector = det
	coordSvc.SetDetector(det)

	// Repair manager
	pc.RepairMgr = repair.NewManager(pool, det, pc.NodeIDs, 3, 500*time.Millisecond)

	coordLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for coordinator: %v", err)
	}
	pc.coordAddr = coordLis.Addr().String()
	pc.coordServer = grpc.NewServer(
		grpc.MaxRecvMsgSize(64*1024*1024),
		grpc.MaxSendMsgSize(64*1024*1024),
	)
	pbCoord.RegisterCoordinatorServiceServer(pc.coordServer, coordSvc)
	go func() { _ = pc.coordServer.Serve(coordLis) }()

	// Client connection
	conn, err := grpc.NewClient(
		pc.coordAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64*1024*1024),
			grpc.MaxCallSendMsgSize(64*1024*1024),
		),
	)
	if err != nil {
		t.Fatalf("failed connecting client to coordinator: %v", err)
	}
	pc.coordConn = conn
	pc.Client = pbCoord.NewCoordinatorServiceClient(conn)

	return pc
}

func (pc *Phase2Cluster) Teardown() {
	if pc.coordConn != nil {
		_ = pc.coordConn.Close()
	}
	if pc.coordServer != nil {
		pc.coordServer.GracefulStop()
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

// TestPhase2_ReadRepair_HealsCorruptedReplica verifies that when a corrupted replica chunk is detected during read,
// the coordinator fails over to a healthy replica, returns the valid object, and heals the corrupted replica on disk.
func TestPhase2_ReadRepair_HealsCorruptedReplica(t *testing.T) {
	pc := setupPhase2Cluster(t, 3)
	defer pc.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	key := "read-repair-test.bin"
	payload := []byte("The quick brown fox jumps over the lazy dog. Phase 2 Read Repair Test Payload.")
	expectedSHA := checksum.ComputeBytes(payload)

	// 1. Put object
	putStream, err := pc.Client.PutObject(ctx)
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	if err := putStream.Send(&pbCoord.PutObjectRequest{
		Payload: &pbCoord.PutObjectRequest_Header{
			Header: &pbCoord.ObjectHeader{Key: key, Size: int64(len(payload))},
		},
	}); err != nil {
		t.Fatalf("send header failed: %v", err)
	}
	if err := putStream.Send(&pbCoord.PutObjectRequest{
		Payload: &pbCoord.PutObjectRequest_ChunkData{ChunkData: payload},
	}); err != nil {
		t.Fatalf("send data failed: %v", err)
	}
	putResp, err := putStream.CloseAndRecv()
	if err != nil || !putResp.GetSuccess() {
		t.Fatalf("put object failed: %v", err)
	}

	// 2. Identify the first replica node in metadata and deliberately corrupt it with bit-rot
	inspectResp, err := pc.Client.InspectObject(ctx, &pbCoord.InspectObjectRequest{Key: key})
	if err != nil {
		t.Fatalf("inspect object failed: %v", err)
	}
	firstReplicaNode := inspectResp.GetMetadata().GetChunks()[0].GetReplicas()[0]

	chunkID := fmt.Sprintf("%s.chunk.0000", key)
	corruptedPath := filepath.Join(pc.storageDirs[firstReplicaNode], chunkID+".chunk")

	corruptedData := []byte("CORRUPTED-JUNK-DATA-CAUSING-SHA-MISMATCH")
	if err := os.WriteFile(corruptedPath, corruptedData, 0644); err != nil {
		t.Fatalf("failed to write corrupted data: %v", err)
	}

	// Verify target node now has corrupted data
	currentBytes, _ := os.ReadFile(corruptedPath)
	if string(currentBytes) != string(corruptedData) {
		t.Fatalf("sanity check failed: corrupted data not written")
	}

	// 3. Read object through coordinator -> failover to storage-02/03 should occur AND trigger read repair!
	getStream, err := pc.Client.GetObject(ctx, &pbCoord.GetObjectRequest{Key: key})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}

	firstMsg, err := getStream.Recv()
	if err != nil {
		t.Fatalf("failed receiving header: %v", err)
	}
	if firstMsg.GetHeader() == nil {
		t.Fatalf("expected header, got %v", firstMsg)
	}

	var readBuf bytes.Buffer
	for {
		msg, err := getStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("error streaming object: %v", err)
		}
		readBuf.Write(msg.GetChunkData())
	}

	// Verify client got complete uncorrupted data
	if readBuf.String() != string(payload) {
		t.Fatalf("expected payload %q, got %q", string(payload), readBuf.String())
	}
	if checksum.ComputeBytes(readBuf.Bytes()) != expectedSHA {
		t.Fatalf("checksum mismatch on returned data")
	}

	// 4. Wait a short interval for async Read Repair to complete on storage-01
	time.Sleep(500 * time.Millisecond)

	// 5. Inspect storage-01 on disk: the file should now be HEALED!
	repairedBytes, err := os.ReadFile(corruptedPath)
	if err != nil {
		t.Fatalf("failed reading chunk from storage-01: %v", err)
	}

	if string(repairedBytes) != string(payload) {
		t.Fatalf("Read Repair failed: storage-01 still has %q instead of %q", string(repairedBytes), string(payload))
	}

	repairedSHA := checksum.ComputeBytes(repairedBytes)
	if repairedSHA != expectedSHA {
		t.Fatalf("Read Repair checksum mismatch: expected %s, got %s", expectedSHA, repairedSHA)
	}
}

// TestPhase2_FlexibleQuorum_WriteWithOneNodeDown verifies that when WriteQuorum=2 and RF=3,
// an upload succeeds even if 1 storage node is completely offline/dead.
func TestPhase2_FlexibleQuorum_WriteWithOneNodeDown(t *testing.T) {
	// Cluster with W=2, RF=3
	pc := setupPhase2Cluster(t, 2)
	defer pc.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 1. Kill storage-03 to simulate a node crash
	pc.storageServers["storage-03"].Stop()
	_ = pc.storageLis["storage-03"].Close()

	key := "write-quorum-test.txt"
	payload := []byte("Flexible Quorum Write: W=2 succeeds when 1 of 3 nodes is crashed!")

	// 2. Upload object through coordinator
	putStream, err := pc.Client.PutObject(ctx)
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	if err := putStream.Send(&pbCoord.PutObjectRequest{
		Payload: &pbCoord.PutObjectRequest_Header{
			Header: &pbCoord.ObjectHeader{Key: key, Size: int64(len(payload))},
		},
	}); err != nil {
		t.Fatalf("send header failed: %v", err)
	}
	if err := putStream.Send(&pbCoord.PutObjectRequest{
		Payload: &pbCoord.PutObjectRequest_ChunkData{ChunkData: payload},
	}); err != nil {
		t.Fatalf("send data failed: %v", err)
	}
	putResp, err := putStream.CloseAndRecv()
	if err != nil || !putResp.GetSuccess() {
		t.Fatalf("put object failed despite W=2 quorum reached: %v", err)
	}

	// 3. Inspect metadata: should reflect 2 replicas (storage-01, storage-02)
	inspectResp, err := pc.Client.InspectObject(ctx, &pbCoord.InspectObjectRequest{Key: key})
	if err != nil {
		t.Fatalf("inspect object failed: %v", err)
	}

	chunks := inspectResp.GetMetadata().GetChunks()
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	replicas := chunks[0].GetReplicas()
	if len(replicas) != 2 {
		t.Fatalf("expected 2 active replicas recorded in metadata, got %v", replicas)
	}

	// 4. Read object back
	getStream, err := pc.Client.GetObject(ctx, &pbCoord.GetObjectRequest{Key: key})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	_, _ = getStream.Recv() // header
	var readBuf bytes.Buffer
	for {
		msg, err := getStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("error streaming object: %v", err)
		}
		readBuf.Write(msg.GetChunkData())
	}

	if readBuf.String() != string(payload) {
		t.Fatalf("expected payload %q, got %q", string(payload), readBuf.String())
	}
}

// TestPhase2_FailureDetectorAndListNodes verifies active liveness detection and state transitions in ListNodes.
func TestPhase2_FailureDetectorAndListNodes(t *testing.T) {
	pc := setupPhase2Cluster(t, 2)
	defer pc.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Initial probe: all 3 nodes healthy
	pc.Detector.ProbeAll(ctx)
	listResp, err := pc.Client.ListNodes(ctx, &pbCoord.ListNodesRequest{})
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}
	for _, n := range listResp.GetNodes() {
		if n.GetStatus() != "HEALTHY" {
			t.Errorf("expected node %s to be HEALTHY, got %s", n.GetNodeId(), n.GetStatus())
		}
	}

	// Kill storage-02
	pc.storageServers["storage-02"].Stop()
	_ = pc.storageLis["storage-02"].Close()

	// Probe twice to trigger DEAD threshold (threshold = 2)
	_ = pc.Detector.ProbeNode(ctx, "storage-02")
	_ = pc.Detector.ProbeNode(ctx, "storage-02")

	if pc.Detector.GetNodeStatus("storage-02") != detector.StatusDead {
		t.Fatalf("expected detector to mark storage-02 DEAD, got %v", pc.Detector.GetNodeStatus("storage-02"))
	}

	// Query ListNodes
	listResp2, err := pc.Client.ListNodes(ctx, &pbCoord.ListNodesRequest{})
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}
	for _, n := range listResp2.GetNodes() {
		if n.GetNodeId() == "storage-02" && n.GetStatus() != "DEAD" {
			t.Fatalf("expected storage-02 status DEAD, got %s", n.GetStatus())
		}
	}
}

// TestPhase2_SelfHealing_RepairsUnderReplicatedChunk verifies that the Repair Manager automatically detects
// an under-replicated chunk and restores full RF=3 replication when a node recovers or joins.
func TestPhase2_SelfHealing_RepairsUnderReplicatedChunk(t *testing.T) {
	pc := setupPhase2Cluster(t, 2)
	defer pc.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// 1. Storage-03 is killed
	pc.storageServers["storage-03"].Stop()
	_ = pc.storageLis["storage-03"].Close()

	key := "self-heal-test.dat"
	payload := []byte("Self-healing background repair test data for Vault Phase 2.")

	// 2. Put object with W=2 (only storage-01 and storage-02 get replicas)
	putStream, err := pc.Client.PutObject(ctx)
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	if err := putStream.Send(&pbCoord.PutObjectRequest{
		Payload: &pbCoord.PutObjectRequest_Header{
			Header: &pbCoord.ObjectHeader{Key: key, Size: int64(len(payload))},
		},
	}); err != nil {
		t.Fatalf("send header failed: %v", err)
	}
	if err := putStream.Send(&pbCoord.PutObjectRequest{
		Payload: &pbCoord.PutObjectRequest_ChunkData{ChunkData: payload},
	}); err != nil {
		t.Fatalf("send data failed: %v", err)
	}
	putResp, err := putStream.CloseAndRecv()
	if err != nil || !putResp.GetSuccess() {
		t.Fatalf("put object failed: %v", err)
	}

	// Verify only 2 replicas currently exist
	inspectResp, err := pc.Client.InspectObject(ctx, &pbCoord.InspectObjectRequest{Key: key})
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}
	if len(inspectResp.GetMetadata().GetChunks()[0].GetReplicas()) != 2 {
		t.Fatalf("expected 2 replicas initially, got %v", inspectResp.GetMetadata().GetChunks()[0].GetReplicas())
	}

	// 3. Restart storage-03 on the original storage-03 address
	lis, err := net.Listen("tcp", pc.storageAddrs["storage-03"])
	if err != nil {
		t.Fatalf("failed restarting storage-03 listener: %v", err)
	}
	sServer, err := storage.NewServer("storage-03", pc.storageDirs["storage-03"])
	if err != nil {
		t.Fatalf("failed restarting storage server: %v", err)
	}
	grpcS := grpc.NewServer(
		grpc.MaxRecvMsgSize(64*1024*1024),
		grpc.MaxSendMsgSize(64*1024*1024),
	)
	pbStorage.RegisterStorageServiceServer(grpcS, sServer)
	pc.storageServers["storage-03"] = grpcS
	pc.storageLis["storage-03"] = lis
	go func() { _ = grpcS.Serve(lis) }()

	pc.Pool.InvalidateStorageConn("storage-03")

	// Probe to ensure detector sees storage-03 alive
	_ = pc.Detector.ProbeNode(ctx, "storage-03")
	if !pc.Detector.IsNodeAlive("storage-03") {
		t.Fatalf("expected storage-03 to be alive after restart")
	}

	// 4. Trigger Repair Manager cycle
	stats, err := pc.RepairMgr.RunRepairCycle(ctx)
	if err != nil {
		t.Fatalf("repair cycle failed: %v", err)
	}

	if stats.UnderReplicated != 1 {
		t.Errorf("expected 1 under-replicated chunk detected, got %d", stats.UnderReplicated)
	}
	if stats.RepairsSuccessful != 1 {
		t.Fatalf("expected 1 successful repair, got %d", stats.RepairsSuccessful)
	}

	// 5. Verify metadata now has all 3 replicas restored!
	inspectAfter, err := pc.Client.InspectObject(ctx, &pbCoord.InspectObjectRequest{Key: key})
	if err != nil {
		t.Fatalf("inspect after repair failed: %v", err)
	}
	restoredReplicas := inspectAfter.GetMetadata().GetChunks()[0].GetReplicas()
	if len(restoredReplicas) != 3 {
		t.Fatalf("expected 3 restored replicas in metadata, got %v", restoredReplicas)
	}

	// 6. Verify chunk file physically exists on storage-03 disk
	chunkID := fmt.Sprintf("%s.chunk.0000", key)
	chunkFile := filepath.Join(pc.storageDirs["storage-03"], chunkID+".chunk")
	dataOnNode3, err := os.ReadFile(chunkFile)
	if err != nil {
		t.Fatalf("chunk not found on storage-03 disk: %v", err)
	}
	if string(dataOnNode3) != string(payload) {
		t.Fatalf("data on storage-03 %q does not match payload %q", string(dataOnNode3), string(payload))
	}
}
