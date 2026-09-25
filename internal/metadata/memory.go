package metadata

import (
	"context"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"

	pb "vault/proto/metadata"
)

// MemoryStore is an in-memory implementation of Store for tests and local embedded use.
type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string]*pb.ObjectMetadata
}

// NewMemoryStore creates an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		objects: make(map[string]*pb.ObjectMetadata),
	}
}

func (m *MemoryStore) PutObject(ctx context.Context, meta *pb.ObjectMetadata) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clone to ensure immutability
	cloned := proto.Clone(meta).(*pb.ObjectMetadata)
	m.objects[meta.GetKey()] = cloned
	return nil
}

func (m *MemoryStore) GetObject(ctx context.Context, key string) (*pb.ObjectMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	meta, ok := m.objects[key]
	if !ok {
		return nil, nil
	}
	return proto.Clone(meta).(*pb.ObjectMetadata), nil
}

func (m *MemoryStore) DeleteObject(ctx context.Context, key string) (*pb.ObjectMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta, ok := m.objects[key]
	if !ok {
		return nil, nil
	}
	delete(m.objects, key)
	return proto.Clone(meta).(*pb.ObjectMetadata), nil
}

func (m *MemoryStore) ListObjects(ctx context.Context, prefix string, limit int32) ([]*pb.ObjectMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*pb.ObjectMetadata
	for k, meta := range m.objects {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			result = append(result, proto.Clone(meta).(*pb.ObjectMetadata))
			if limit > 0 && int32(len(result)) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (m *MemoryStore) Close() error {
	return nil
}
