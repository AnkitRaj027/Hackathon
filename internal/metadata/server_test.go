package metadata

import (
	"context"
	"testing"

	pb "vault/proto/metadata"
)

func TestMetadataKeys(t *testing.T) {
	key := "photos/vacation/sunset.png"
	encoded := EncodeObjectKey(key)
	decoded, err := DecodeObjectKey(encoded)
	if err != nil {
		t.Fatalf("failed decoding: %v", err)
	}
	if decoded != key {
		t.Fatalf("expected %s, got %s", key, decoded)
	}

	if err := ValidateKey("valid-key/123"); err != nil {
		t.Fatalf("expected valid key, got: %v", err)
	}
	if err := ValidateKey("../escape"); err == nil {
		t.Fatalf("expected error on path traversal key")
	}
	if err := ValidateKey(""); err == nil {
		t.Fatalf("expected error on empty key")
	}
}

func TestMetadataServerWithMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer(store)
	ctx := context.Background()

	objKey := "documents/contract.pdf"
	meta := &pb.ObjectMetadata{
		Key:         objKey,
		Size:        1024,
		ContentType: "application/pdf",
		CreatedAt:   1234567890,
		Chunks: []*pb.ChunkMetadata{
			{
				ChunkId:   objKey + ".chunk.0000",
				ObjectKey: objKey,
				Index:     0,
				Size:      1024,
				Sha256:    "abcd1234ef",
				Replicas:  []string{"storage-01", "storage-02", "storage-03"},
			},
		},
	}

	// 1. Put
	putResp, err := server.PutObject(ctx, &pb.PutObjectMetadataRequest{Metadata: meta})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	if !putResp.Success {
		t.Fatalf("PutObject reported failure")
	}

	// 2. Get
	getResp, err := server.GetObject(ctx, &pb.GetObjectMetadataRequest{Key: objKey})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	if !getResp.Found {
		t.Fatalf("expected object to be found")
	}
	if getResp.Metadata.Size != 1024 {
		t.Fatalf("expected size 1024, got %d", getResp.Metadata.Size)
	}
	if len(getResp.Metadata.Chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(getResp.Metadata.Chunks))
	}

	// 3. List
	listResp, err := server.ListObjects(ctx, &pb.ListObjectsRequest{Prefix: "documents/"})
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(listResp.Objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(listResp.Objects))
	}

	// 4. Delete
	delResp, err := server.DeleteObject(ctx, &pb.DeleteObjectMetadataRequest{Key: objKey})
	if err != nil {
		t.Fatalf("DeleteObject failed: %v", err)
	}
	if !delResp.Success {
		t.Fatalf("DeleteObject reported failure")
	}
	if delResp.DeletedMetadata.Key != objKey {
		t.Fatalf("expected deleted key %s, got %s", objKey, delResp.DeletedMetadata.Key)
	}

	// 5. Verify deleted
	getAgain, err := server.GetObject(ctx, &pb.GetObjectMetadataRequest{Key: objKey})
	if err != nil {
		t.Fatalf("GetObject after delete failed: %v", err)
	}
	if getAgain.Found {
		t.Fatalf("expected object not to be found after delete")
	}
}
