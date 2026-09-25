package metadata

import (
	"context"
	"fmt"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	pb "vault/proto/metadata"
)

// EtcdStore implements Store backed by an etcd cluster.
type EtcdStore struct {
	client *clientv3.Client
}

// NewEtcdStore establishes a connection to etcd and returns an EtcdStore.
func NewEtcdStore(endpoints []string) (*EtcdStore, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed connecting to etcd: %w", err)
	}

	// Verify connectivity with a quick timeout
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = client.Status(ctx, endpoints[0])
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("etcd health check failed for %s: %w", endpoints[0], err)
	}

	return &EtcdStore{client: client}, nil
}

// PutObject stores the serialized object metadata in etcd.
func (e *EtcdStore) PutObject(ctx context.Context, meta *pb.ObjectMetadata) error {
	data, err := proto.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed marshaling object metadata: %w", err)
	}

	key := ObjectKey(meta.GetKey())
	_, err = e.client.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("etcd Put failed for %s: %w", key, err)
	}

	return nil
}

// GetObject retrieves and unmarshals object metadata from etcd.
func (e *EtcdStore) GetObject(ctx context.Context, key string) (*pb.ObjectMetadata, error) {
	etcdKey := ObjectKey(key)
	resp, err := e.client.Get(ctx, etcdKey)
	if err != nil {
		return nil, fmt.Errorf("etcd Get failed for %s: %w", etcdKey, err)
	}

	if len(resp.Kvs) == 0 {
		return nil, nil
	}

	meta := &pb.ObjectMetadata{}
	if err := proto.Unmarshal(resp.Kvs[0].Value, meta); err != nil {
		return nil, fmt.Errorf("failed unmarshaling metadata from etcd: %w", err)
	}

	return meta, nil
}

// DeleteObject deletes an object metadata entry and returns the prior metadata.
func (e *EtcdStore) DeleteObject(ctx context.Context, key string) (*pb.ObjectMetadata, error) {
	// First fetch existing
	existing, err := e.GetObject(ctx, key)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	etcdKey := ObjectKey(key)
	_, err = e.client.Delete(ctx, etcdKey)
	if err != nil {
		return nil, fmt.Errorf("etcd Delete failed for %s: %w", etcdKey, err)
	}

	return existing, nil
}

// ListObjects retrieves all objects matching the prefix.
func (e *EtcdStore) ListObjects(ctx context.Context, prefix string, limit int32) ([]*pb.ObjectMetadata, error) {
	var opts []clientv3.OpOption
	opts = append(opts, clientv3.WithPrefix())
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(int64(limit)))
	}

	resp, err := e.client.Get(ctx, PrefixObjects, opts...)
	if err != nil {
		return nil, fmt.Errorf("etcd List failed: %w", err)
	}

	var results []*pb.ObjectMetadata
	for _, kv := range resp.Kvs {
		meta := &pb.ObjectMetadata{}
		if err := proto.Unmarshal(kv.Value, meta); err != nil {
			continue
		}
		if prefix == "" || strings.HasPrefix(meta.GetKey(), prefix) {
			results = append(results, meta)
		}
	}

	return results, nil
}

func (e *EtcdStore) Close() error {
	return e.client.Close()
}
