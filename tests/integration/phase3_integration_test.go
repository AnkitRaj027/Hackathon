package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"vault/internal/config"
	"vault/internal/coordinator"
	"vault/internal/metadata"
	"vault/internal/placement"
	"vault/internal/storage"
	pbCoord "vault/proto/coordinator"
	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

type Phase3Cluster struct {
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
	Ring           *placement.ConsistentHashRing
	NodeIDs        []string
}

func setupPhase3Cluster(t *testing.T, initialNodes []string, rf int) *Phase3Cluster {
	pc := &Phase3Cluster{
		storageServers: make(map[string]*grpc.Server),
		storageLis:     make(map[string]net.Listener),
		storageDirs:    make(map[string]string),
		storageAddrs:   make(map[string]string),
		NodeIDs:        initialNodes,
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
		pc.startStorageNode(t, nid)
	}

	// 3. Consistent Hash Ring
	ring, err := placement.NewConsistentHashRing(pc.NodeIDs, 128, rf)
	if err != nil {
		t.Fatalf("failed creating consistent hash ring: %v", err)
	}
	pc.Ring = ring

	// 4. Coordinator
	coordCfg := &config.CoordinatorConfig{
		ChunkSize:         1024 * 1024,
		ReplicationFactor: rf,
		WriteQuorum:       rf,
		ReadQuorum:        1,
		StorageNodes:      pc.storageAddrs,
		MetadataAddr:      pc.metaAddr,
		PlacementStrategy: "consistent_hash",
	}

	pool := coordinator.NewClientPool(pc.metaAddr, pc.storageAddrs)
	coordSvc := coordinator.NewService(coordCfg, pool, ring)

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

func (pc *Phase3Cluster) startStorageNode(t *testing.T, nid string) {
	dir, err := os.MkdirTemp("", "vault-p3-"+nid+"-*")
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

func (pc *Phase3Cluster) Teardown() {
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

// TestPhase3_ConsistentHashRing_MultiNodeDistribution verifies that with 5 nodes and RF=3,
// chunks are placed across distinct subsets of nodes instead of being restricted to fixed node indices.
func TestPhase3_ConsistentHashRing_MultiNodeDistribution(t *testing.T) {
	nodes := []string{"node-01", "node-02", "node-03", "node-04", "node-05"}
	pc := setupPhase3Cluster(t, nodes, 3)
	defer pc.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	distinctSubsets := make(map[string]int)

	// Upload 15 different objects
	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("dataset-object-%d.bin", i)
		data := []byte(fmt.Sprintf("Payload content for object index %d on consistent hash ring", i))

		putStream, err := pc.Client.PutObject(ctx)
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}
		_ = putStream.Send(&pbCoord.PutObjectRequest{
			Payload: &pbCoord.PutObjectRequest_Header{
				Header: &pbCoord.ObjectHeader{Key: key, Size: int64(len(data))},
			},
		})
		_ = putStream.Send(&pbCoord.PutObjectRequest{
			Payload: &pbCoord.PutObjectRequest_ChunkData{ChunkData: data},
		})
		putResp, err := putStream.CloseAndRecv()
		if err != nil || !putResp.GetSuccess() {
			t.Fatalf("put failed for %s: %v", key, err)
		}

		// Inspect replicas
		inspectResp, err := pc.Client.InspectObject(ctx, &pbCoord.InspectObjectRequest{Key: key})
		if err != nil {
			t.Fatalf("inspect failed: %v", err)
		}
		replicas := inspectResp.GetMetadata().GetChunks()[0].GetReplicas()
		if len(replicas) != 3 {
			t.Fatalf("expected 3 replicas, got %d", len(replicas))
		}

		subsetKey := fmt.Sprintf("%s,%s,%s", replicas[0], replicas[1], replicas[2])
		distinctSubsets[subsetKey]++

		// Verify read works seamlessly
		getStream, err := pc.Client.GetObject(ctx, &pbCoord.GetObjectRequest{Key: key})
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		_, _ = getStream.Recv()
		var readBuf bytes.Buffer
		for {
			msg, err := getStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("read stream failed: %v", err)
			}
			readBuf.Write(msg.GetChunkData())
		}
		if readBuf.String() != string(data) {
			t.Fatalf("read content mismatch for %s", key)
		}
	}

	t.Logf("Consistent Hash Placements across 15 objects generated %d distinct node combinations:", len(distinctSubsets))
	for subset, count := range distinctSubsets {
		t.Logf("  [%s] -> %d objects", subset, count)
	}

	// Unlike fixed placement (which always maps to the same fixed 3 nodes),
	// consistent hashing across 5 nodes must utilize multiple distinct replica subsets!
	if len(distinctSubsets) < 3 {
		t.Errorf("expected at least 3 distinct replica subsets across 5 nodes, got %d", len(distinctSubsets))
	}
}

// TestPhase3_DynamicRingScaling verifies that adding a node to the consistent hash ring dynamically
// causes subsequent chunk uploads to incorporate the new node without breaking access to earlier objects.
func TestPhase3_DynamicRingScaling(t *testing.T) {
	initialNodes := []string{"node-01", "node-02", "node-03"}
	pc := setupPhase3Cluster(t, initialNodes, 3)
	defer pc.Teardown()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 1. Upload initial object on 3-node ring
	key1 := "pre-scale-doc.txt"
	data1 := []byte("Created prior to ring scaling event.")
	putStream, err := pc.Client.PutObject(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = putStream.Send(&pbCoord.PutObjectRequest{
		Payload: &pbCoord.PutObjectRequest_Header{
			Header: &pbCoord.ObjectHeader{Key: key1, Size: int64(len(data1))},
		},
	})
	_ = putStream.Send(&pbCoord.PutObjectRequest{
		Payload: &pbCoord.PutObjectRequest_ChunkData{ChunkData: data1},
	})
	putResp, err := putStream.CloseAndRecv()
	if err != nil || !putResp.GetSuccess() {
		t.Fatalf("put failed: %v", err)
	}

	// 2. Start node-04 and dynamically add to consistent hash ring
	pc.startStorageNode(t, "node-04")
	pc.Ring.AddNode("node-04")

	if pc.Ring.NodeCount() != 4 {
		t.Fatalf("expected 4 nodes on ring, got %d", pc.Ring.NodeCount())
	}

	// 3. Upload 10 new objects; verify node-04 is placed for some of the new objects
	node4Hit := false
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("post-scale-item-%d.dat", i)
		data := []byte(fmt.Sprintf("Item data %d after adding node-04", i))

		pStream, _ := pc.Client.PutObject(ctx)
		_ = pStream.Send(&pbCoord.PutObjectRequest{
			Payload: &pbCoord.PutObjectRequest_Header{
				Header: &pbCoord.ObjectHeader{Key: key, Size: int64(len(data))},
			},
		})
		_ = pStream.Send(&pbCoord.PutObjectRequest{
			Payload: &pbCoord.PutObjectRequest_ChunkData{ChunkData: data},
		})
		pResp, err := pStream.CloseAndRecv()
		if err != nil || !pResp.GetSuccess() {
			t.Fatalf("post-scale upload failed: %v", err)
		}

		inspectResp, err := pc.Client.InspectObject(ctx, &pbCoord.InspectObjectRequest{Key: key})
		if err != nil {
			t.Fatal(err)
		}
		for _, replica := range inspectResp.GetMetadata().GetChunks()[0].GetReplicas() {
			if replica == "node-04" {
				node4Hit = true
			}
		}
	}

	if !node4Hit {
		t.Fatalf("node-04 was never selected for any of the 10 post-scale objects")
	}

	// 4. Verify initial object (uploaded before scaling) is still completely readable
	gStream, err := pc.Client.GetObject(ctx, &pbCoord.GetObjectRequest{Key: key1})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = gStream.Recv()
	var readBuf bytes.Buffer
	for {
		msg, err := gStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read pre-scale object failed: %v", err)
		}
		readBuf.Write(msg.GetChunkData())
	}
	if readBuf.String() != string(data1) {
		t.Fatalf("pre-scale object corruption: got %q, expected %q", readBuf.String(), string(data1))
	}
}
