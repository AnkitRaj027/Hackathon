package repair_test

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	"vault/internal/checksum"
	"vault/internal/repair"
	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

type mockLiveness map[string]bool

func (m mockLiveness) IsNodeAlive(nodeID string) bool {
	return m[nodeID]
}

type mockStorageClient struct {
	pbStorage.StorageServiceClient
	mu     sync.Mutex
	chunks map[string][]byte
}

func (m *mockStorageClient) PutChunk(ctx context.Context, in *pbStorage.PutChunkRequest, opts ...grpc.CallOption) (*pbStorage.PutChunkResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks[in.ChunkId] = in.Data
	return &pbStorage.PutChunkResponse{Success: true}, nil
}

type mockChunkStream struct {
	grpc.ClientStream
	data   []byte
	served bool
}

func (s *mockChunkStream) Recv() (*pbStorage.ChunkData, error) {
	if s.served {
		return nil, io.EOF
	}
	s.served = true
	return &pbStorage.ChunkData{Data: s.data}, nil
}

func (m *mockStorageClient) GetChunk(ctx context.Context, in *pbStorage.GetChunkRequest, opts ...grpc.CallOption) (pbStorage.StorageService_GetChunkClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &mockChunkStream{data: m.chunks[in.ChunkId]}, nil
}

type mockMetaClient struct {
	pbMeta.MetadataServiceClient
	mu      sync.Mutex
	objects map[string]*pbMeta.ObjectMetadata
}

func (m *mockMetaClient) ListObjects(ctx context.Context, in *pbMeta.ListObjectsRequest, opts ...grpc.CallOption) (*pbMeta.ListObjectsResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var objs []*pbMeta.ObjectMetadata
	for _, o := range m.objects {
		objs = append(objs, o)
	}
	return &pbMeta.ListObjectsResponse{Objects: objs}, nil
}

func (m *mockMetaClient) PutObject(ctx context.Context, in *pbMeta.PutObjectMetadataRequest, opts ...grpc.CallOption) (*pbMeta.PutObjectMetadataResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[in.Metadata.Key] = in.Metadata
	return &pbMeta.PutObjectMetadataResponse{Success: true}, nil
}

type mockPool struct {
	storageClients map[string]*mockStorageClient
	metaClient     *mockMetaClient
}

func (p *mockPool) GetStorageClient(nodeID string) (pbStorage.StorageServiceClient, error) {
	return p.storageClients[nodeID], nil
}

func (p *mockPool) GetMetadataClient() (pbMeta.MetadataServiceClient, error) {
	return p.metaClient, nil
}

func TestRepairManager_HealsUnderReplicatedChunk(t *testing.T) {
	testData := []byte("hello repair world")
	testSHA := checksum.ComputeBytes(testData)
	chunkID := "test.txt.chunk.0000"

	s1 := &mockStorageClient{chunks: map[string][]byte{chunkID: testData}}
	s2 := &mockStorageClient{chunks: make(map[string][]byte)}

	meta := &mockMetaClient{
		objects: map[string]*pbMeta.ObjectMetadata{
			"test.txt": {
				Key:  "test.txt",
				Size: int64(len(testData)),
				Chunks: []*pbMeta.ChunkMetadata{
					{
						ChunkId:   chunkID,
						ObjectKey: "test.txt",
						Index:     0,
						Size:      int64(len(testData)),
						Sha256:    testSHA,
						Replicas:  []string{"storage-01"}, // only 1 replica, target RF is 2!
					},
				},
			},
		},
	}

	pool := &mockPool{
		storageClients: map[string]*mockStorageClient{
			"storage-01": s1,
			"storage-02": s2,
		},
		metaClient: meta,
	}

	liveness := mockLiveness{
		"storage-01": true,
		"storage-02": true,
	}

	mgr := repair.NewManager(pool, liveness, []string{"storage-01", "storage-02"}, 2, 1*time.Second)
	stats, err := mgr.RunRepairCycle(context.Background())
	if err != nil {
		t.Fatalf("repair cycle failed: %v", err)
	}

	if stats.UnderReplicated != 1 {
		t.Errorf("expected 1 under-replicated chunk, got %d", stats.UnderReplicated)
	}
	if stats.RepairsSuccessful != 1 {
		t.Errorf("expected 1 successful repair, got %d", stats.RepairsSuccessful)
	}

	// Verify storage-02 received the chunk
	s2.mu.Lock()
	replicatedData, exists := s2.chunks[chunkID]
	s2.mu.Unlock()
	if !exists || string(replicatedData) != string(testData) {
		t.Fatalf("chunk was not replicated to storage-02")
	}

	// Verify metadata was updated with both replicas
	meta.mu.Lock()
	updatedReplicas := meta.objects["test.txt"].Chunks[0].Replicas
	meta.mu.Unlock()

	if len(updatedReplicas) != 2 {
		t.Fatalf("expected 2 replicas in updated metadata, got %v", updatedReplicas)
	}
}
