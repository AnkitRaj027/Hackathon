package metadata

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "vault/proto/metadata"
)

// Server implements pb.MetadataServiceServer.
type Server struct {
	pb.UnimplementedMetadataServiceServer
	store Store
}

// NewServer creates a new gRPC Metadata service server.
func NewServer(store Store) *Server {
	return &Server{store: store}
}

// PutObject stores metadata for an object.
func (s *Server) PutObject(ctx context.Context, req *pb.PutObjectMetadataRequest) (*pb.PutObjectMetadataResponse, error) {
	meta := req.GetMetadata()
	if meta == nil {
		return nil, status.Error(codes.InvalidArgument, "metadata payload cannot be nil")
	}
	if err := ValidateKey(meta.GetKey()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid object key: %v", err)
	}

	start := time.Now()
	if err := s.store.PutObject(ctx, meta); err != nil {
		slog.Error("failed committing object metadata",
			"key", meta.GetKey(),
			"error", err.Error(),
		)
		return nil, status.Errorf(codes.Internal, "metadata commit failed: %v", err)
	}

	slog.Info("metadata_committed",
		"key", meta.GetKey(),
		"size", meta.GetSize(),
		"chunks", len(meta.GetChunks()),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return &pb.PutObjectMetadataResponse{
		Success: true,
	}, nil
}

// GetObject retrieves metadata for an object key.
func (s *Server) GetObject(ctx context.Context, req *pb.GetObjectMetadataRequest) (*pb.GetObjectMetadataResponse, error) {
	key := req.GetKey()
	if err := ValidateKey(key); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid object key: %v", err)
	}

	meta, err := s.store.GetObject(ctx, key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed retrieving metadata: %v", err)
	}

	if meta == nil {
		return &pb.GetObjectMetadataResponse{
			Found:        false,
			ErrorMessage: "object not found",
		}, nil
	}

	return &pb.GetObjectMetadataResponse{
		Found:    true,
		Metadata: meta,
	}, nil
}

// DeleteObject deletes metadata for an object key and returns previous metadata.
func (s *Server) DeleteObject(ctx context.Context, req *pb.DeleteObjectMetadataRequest) (*pb.DeleteObjectMetadataResponse, error) {
	key := req.GetKey()
	if err := ValidateKey(key); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid object key: %v", err)
	}

	deleted, err := s.store.DeleteObject(ctx, key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed deleting metadata: %v", err)
	}

	if deleted == nil {
		return &pb.DeleteObjectMetadataResponse{
			Success:      true,
			ErrorMessage: "object did not exist",
		}, nil
	}

	slog.Info("metadata_deleted", "key", key)

	return &pb.DeleteObjectMetadataResponse{
		Success:         true,
		DeletedMetadata: deleted,
	}, nil
}

// ListObjects lists stored objects.
func (s *Server) ListObjects(ctx context.Context, req *pb.ListObjectsRequest) (*pb.ListObjectsResponse, error) {
	objects, err := s.store.ListObjects(ctx, req.GetPrefix(), req.GetLimit())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed listing metadata: %v", err)
	}

	return &pb.ListObjectsResponse{
		Objects: objects,
	}, nil
}
