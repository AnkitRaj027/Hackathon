package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

	"vault/internal/checksum"
	pb "vault/proto/storage"
)

type mockGetChunkServer struct {
	grpc.ServerStream
	ctx        context.Context
	sentChunks []*pb.ChunkData
}

func (m *mockGetChunkServer) Context() context.Context {
	return m.ctx
}

func (m *mockGetChunkServer) Send(c *pb.ChunkData) error {
	m.sentChunks = append(m.sentChunks, c)
	return nil
}

func TestStorageServer(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vault-storage-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	server, err := NewServer("test-node-01", tempDir)
	if err != nil {
		t.Fatalf("failed to create storage server: %v", err)
	}

	ctx := context.Background()
	chunkData := []byte("hello distributed vault storage")
	chunkID := "test-object.chunk.0000"
	expectedSHA := checksum.ComputeBytes(chunkData)

	// 1. Test PutChunk
	putResp, err := server.PutChunk(ctx, &pb.PutChunkRequest{
		ChunkId:   chunkID,
		Data:      chunkData,
		Checksum:  expectedSHA,
		ObjectKey: "test-object",
		Index:     0,
	})
	if err != nil {
		t.Fatalf("PutChunk failed: %v", err)
	}
	if !putResp.Success {
		t.Fatalf("PutChunk reported failure")
	}
	if putResp.Checksum != expectedSHA {
		t.Fatalf("expected checksum %s, got %s", expectedSHA, putResp.Checksum)
	}

	// Verify chunk file and sidecar file exist on disk
	chunkFile := filepath.Join(tempDir, chunkID+".chunk")
	sidecarFile := filepath.Join(tempDir, chunkID+".sha256")
	if _, err := os.Stat(chunkFile); err != nil {
		t.Fatalf("chunk file does not exist on disk: %v", err)
	}
	if _, err := os.Stat(sidecarFile); err != nil {
		t.Fatalf("sidecar file does not exist on disk: %v", err)
	}

	// 2. Test GetChunk
	mockStream := &mockGetChunkServer{ctx: ctx}
	err = server.GetChunk(&pb.GetChunkRequest{ChunkId: chunkID}, mockStream)
	if err != nil {
		t.Fatalf("GetChunk failed: %v", err)
	}
	var receivedBytes []byte
	for _, c := range mockStream.sentChunks {
		receivedBytes = append(receivedBytes, c.Data...)
	}
	if string(receivedBytes) != string(chunkData) {
		t.Fatalf("GetChunk data mismatch: expected %q, got %q", string(chunkData), string(receivedBytes))
	}

	// 3. Test VerifyChecksum valid
	verifyResp, err := server.VerifyChecksum(ctx, &pb.VerifyChecksumRequest{
		ChunkId:          chunkID,
		ExpectedChecksum: expectedSHA,
	})
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}
	if !verifyResp.Valid {
		t.Fatalf("expected checksum to be valid")
	}

	// 4. Test Checksum Corruption Detection
	// Corrupt chunk file directly
	if err := os.WriteFile(chunkFile, []byte("corrupted data!"), 0644); err != nil {
		t.Fatalf("failed to corrupt chunk file: %v", err)
	}
	verifyCorruptResp, err := server.VerifyChecksum(ctx, &pb.VerifyChecksumRequest{
		ChunkId:          chunkID,
		ExpectedChecksum: expectedSHA,
	})
	if err != nil {
		t.Fatalf("VerifyChecksum on corrupted file returned error: %v", err)
	}
	if verifyCorruptResp.Valid {
		t.Fatalf("expected corrupted chunk to be reported as INVALID")
	}

	// 5. Test DeleteChunk
	delResp, err := server.DeleteChunk(ctx, &pb.DeleteChunkRequest{ChunkId: chunkID})
	if err != nil {
		t.Fatalf("DeleteChunk failed: %v", err)
	}
	if !delResp.Success {
		t.Fatalf("DeleteChunk reported failure")
	}
	if _, err := os.Stat(chunkFile); !os.IsNotExist(err) {
		t.Fatalf("chunk file should have been deleted")
	}
	if _, err := os.Stat(sidecarFile); !os.IsNotExist(err) {
		t.Fatalf("sidecar file should have been deleted")
	}

	// 6. Test Path Traversal Protection
	_, err = server.PutChunk(ctx, &pb.PutChunkRequest{
		ChunkId: "../../dangerous",
		Data:    []byte("bad"),
	})
	if err == nil {
		t.Fatalf("expected error on path traversal chunk ID")
	}
}
