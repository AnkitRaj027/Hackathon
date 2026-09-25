package metadata

import (
	"context"
	pb "vault/proto/metadata"
)

// Store defines the persistent storage layer for object metadata.
type Store interface {
	PutObject(ctx context.Context, meta *pb.ObjectMetadata) error
	GetObject(ctx context.Context, key string) (*pb.ObjectMetadata, error)
	DeleteObject(ctx context.Context, key string) (*pb.ObjectMetadata, error)
	ListObjects(ctx context.Context, prefix string, limit int32) ([]*pb.ObjectMetadata, error)
	Close() error
}
