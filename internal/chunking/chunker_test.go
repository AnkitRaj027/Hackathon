package chunking

import (
	"bytes"
	"io"
	"testing"
	"vault/internal/checksum"
)

func TestStreamChunker(t *testing.T) {
	// Create 10 bytes of data, chunk size 3 bytes -> should give 4 chunks (3, 3, 3, 1)
	data := []byte("0123456789")
	chunker := NewStreamChunker(bytes.NewReader(data), "test-obj", 3)

	var chunks []*Chunk
	for {
		chunk, err := chunker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("unexpected chunk error: %v", err)
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}

	expectedSizes := []int64{3, 3, 3, 1}
	expectedData := []string{"012", "345", "678", "9"}

	for i, c := range chunks {
		if c.Index != int64(i) {
			t.Errorf("chunk %d: expected index %d, got %d", i, i, c.Index)
		}
		if c.Size != expectedSizes[i] {
			t.Errorf("chunk %d: expected size %d, got %d", i, expectedSizes[i], c.Size)
		}
		if string(c.Data) != expectedData[i] {
			t.Errorf("chunk %d: expected data %s, got %s", i, expectedData[i], string(c.Data))
		}
		if c.Checksum != checksum.ComputeBytes([]byte(expectedData[i])) {
			t.Errorf("chunk %d: checksum mismatch", i)
		}
	}
}

func TestStreamChunkerEmpty(t *testing.T) {
	chunker := NewStreamChunker(bytes.NewReader([]byte{}), "empty-obj", 1024)
	chunk, err := chunker.Next()
	if err != io.EOF {
		t.Fatalf("expected EOF on empty stream, got chunk: %v, err: %v", chunk, err)
	}
}
