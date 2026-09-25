package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"vault/internal/checksum"
	pb "vault/proto/storage"
)

const (
	StreamBufferSize = 64 * 1024 // 64 KiB
)

// Server implements pb.StorageServiceServer for a local disk node.
type Server struct {
	pb.UnimplementedStorageServiceServer
	nodeID  string
	dataDir string
	mu      sync.RWMutex
}

// NewServer creates a new Storage Node server.
func NewServer(nodeID string, dataDir string) (*Server, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory %s: %w", dataDir, err)
	}
	return &Server{
		nodeID:  nodeID,
		dataDir: dataDir,
	}, nil
}

// validateChunkID ensures the chunk ID does not contain path traversal elements.
func (s *Server) validateChunkID(chunkID string) error {
	if chunkID == "" {
		return status.Error(codes.InvalidArgument, "chunk_id cannot be empty")
	}
	if strings.Contains(chunkID, "/") || strings.Contains(chunkID, "\\") || strings.Contains(chunkID, "..") {
		return status.Errorf(codes.InvalidArgument, "invalid chunk_id containing path characters: %s", chunkID)
	}
	return nil
}

func (s *Server) chunkPath(chunkID string) string {
	return filepath.Join(s.dataDir, chunkID+".chunk")
}

func (s *Server) sidecarPath(chunkID string) string {
	return filepath.Join(s.dataDir, chunkID+".sha256")
}

// PutChunk persists a chunk crash-safely using a temp file, fsync, checksum check, and atomic rename.
func (s *Server) PutChunk(ctx context.Context, req *pb.PutChunkRequest) (*pb.PutChunkResponse, error) {
	start := time.Now()
	if err := s.validateChunkID(req.GetChunkId()); err != nil {
		return nil, err
	}

	data := req.GetData()
	actualChecksum := checksum.ComputeBytes(data)

	// If coordinator provided an expected checksum, verify it matches
	if req.GetChecksum() != "" && !checksum.Verify(req.GetChecksum(), actualChecksum) {
		slog.Error("checksum mismatch during put_chunk",
			"node", s.nodeID,
			"chunk_id", req.GetChunkId(),
			"expected", req.GetChecksum(),
			"actual", actualChecksum,
		)
		return nil, status.Errorf(codes.InvalidArgument, "checksum mismatch: expected %s, got %s", req.GetChecksum(), actualChecksum)
	}

	tempFile := filepath.Join(s.dataDir, fmt.Sprintf("%s.tmp.%s", req.GetChunkId(), uuid.NewString()))
	f, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create temp file: %v", err)
	}

	// Ensure cleanup on failure
	cleanup := true
	defer func() {
		if cleanup {
			_ = f.Close()
			_ = os.Remove(tempFile)
		}
	}()

	n, err := f.Write(data)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed writing chunk data: %v", err)
	}

	// fsync for durable persistence
	if err := f.Sync(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to sync chunk data: %v", err)
	}
	if err := f.Close(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed closing temp file: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Atomic rename to final chunk file
	finalPath := s.chunkPath(req.GetChunkId())
	if err := os.Rename(tempFile, finalPath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed atomically renaming chunk: %v", err)
	}
	cleanup = false

	// Write sidecar checksum file atomically
	sidecarTemp := filepath.Join(s.dataDir, fmt.Sprintf("%s.sha256.tmp.%s", req.GetChunkId(), uuid.NewString()))
	if err := os.WriteFile(sidecarTemp, []byte(actualChecksum), 0644); err != nil {
		return nil, status.Errorf(codes.Internal, "failed writing checksum sidecar temp: %v", err)
	}
	if err := os.Rename(sidecarTemp, s.sidecarPath(req.GetChunkId())); err != nil {
		_ = os.Remove(sidecarTemp)
		return nil, status.Errorf(codes.Internal, "failed renaming checksum sidecar: %v", err)
	}

	durationMs := time.Since(start).Milliseconds()
	slog.Info("put_chunk",
		"node", s.nodeID,
		"chunk_id", req.GetChunkId(),
		"bytes", n,
		"checksum", actualChecksum,
		"duration_ms", durationMs,
	)

	return &pb.PutChunkResponse{
		ChunkId:      req.GetChunkId(),
		Checksum:     actualChecksum,
		BytesWritten: int64(n),
		Success:      true,
	}, nil
}

// GetChunk streams the chunk content back to the client.
func (s *Server) GetChunk(req *pb.GetChunkRequest, stream pb.StorageService_GetChunkServer) error {
	if err := s.validateChunkID(req.GetChunkId()); err != nil {
		return err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	finalPath := s.chunkPath(req.GetChunkId())
	f, err := os.Open(finalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Errorf(codes.NotFound, "chunk not found: %s", req.GetChunkId())
		}
		return status.Errorf(codes.Internal, "failed to open chunk: %v", err)
	}
	defer f.Close()

	sidecarData, err := os.ReadFile(s.sidecarPath(req.GetChunkId()))
	sidecarChecksum := ""
	if err == nil {
		sidecarChecksum = strings.TrimSpace(string(sidecarData))
	}

	buffer := make([]byte, StreamBufferSize)
	for {
		n, readErr := f.Read(buffer)
		if n > 0 {
			chunkMsg := &pb.ChunkData{
				ChunkId:  req.GetChunkId(),
				Data:     buffer[:n],
				Checksum: sidecarChecksum,
			}
			if sendErr := stream.Send(chunkMsg); sendErr != nil {
				return status.Errorf(codes.Canceled, "failed to send chunk stream: %v", sendErr)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return status.Errorf(codes.Internal, "failed reading chunk: %v", readErr)
		}
	}

	return nil
}

// DeleteChunk removes both the chunk file and the checksum sidecar file. Idempotent.
func (s *Server) DeleteChunk(ctx context.Context, req *pb.DeleteChunkRequest) (*pb.DeleteChunkResponse, error) {
	if err := s.validateChunkID(req.GetChunkId()); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	chunkPath := s.chunkPath(req.GetChunkId())
	sidecarPath := s.sidecarPath(req.GetChunkId())

	_ = os.Remove(chunkPath)
	_ = os.Remove(sidecarPath)

	slog.Info("delete_chunk",
		"node", s.nodeID,
		"chunk_id", req.GetChunkId(),
	)

	return &pb.DeleteChunkResponse{
		ChunkId: req.GetChunkId(),
		Success: true,
		Message: "deleted",
	}, nil
}

// VerifyChecksum calculates the chunk's current SHA-256 and compares it against expected or sidecar.
func (s *Server) VerifyChecksum(ctx context.Context, req *pb.VerifyChecksumRequest) (*pb.VerifyChecksumResponse, error) {
	if err := s.validateChunkID(req.GetChunkId()); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	finalPath := s.chunkPath(req.GetChunkId())
	data, err := os.ReadFile(finalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "chunk not found: %s", req.GetChunkId())
		}
		return nil, status.Errorf(codes.Internal, "failed to read chunk: %v", err)
	}

	actualChecksum := checksum.ComputeBytes(data)

	expected := req.GetExpectedChecksum()
	if expected == "" {
		// Read sidecar as expected
		sidecarData, err := os.ReadFile(s.sidecarPath(req.GetChunkId()))
		if err == nil {
			expected = strings.TrimSpace(string(sidecarData))
		}
	}

	isValid := (expected != "") && checksum.Verify(expected, actualChecksum)

	return &pb.VerifyChecksumResponse{
		ChunkId:        req.GetChunkId(),
		Valid:          isValid,
		ActualChecksum: actualChecksum,
		Message: func() string {
			if isValid {
				return "checksum verified"
			}
			return fmt.Sprintf("checksum mismatch: expected %s, got %s", expected, actualChecksum)
		}(),
	}, nil
}
