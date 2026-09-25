package chunking

import (
	"fmt"
	"io"
	"strings"
	"vault/internal/checksum"
)

const (
	// DefaultChunkSize is 4 MiB
	DefaultChunkSize int64 = 4 * 1024 * 1024
)

// Chunk represents a discrete portion of an object stream.
type Chunk struct {
	ID        string
	ObjectKey string
	Index     int64
	Size      int64
	Checksum  string
	Data      []byte
}

// Chunker defines the interface for splitting an input stream into chunks.
type Chunker interface {
	Next() (*Chunk, error)
}

// StreamChunker splits an io.Reader into fixed-size chunks sequentially.
type StreamChunker struct {
	reader    io.Reader
	objectKey string
	chunkSize int64
	index     int64
	buffer    []byte
	done      bool
}

// NewStreamChunker creates a new StreamChunker for an object.
func NewStreamChunker(r io.Reader, objectKey string, chunkSize int64) *StreamChunker {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	return &StreamChunker{
		reader:    r,
		objectKey: objectKey,
		chunkSize: chunkSize,
		index:     0,
		buffer:    make([]byte, chunkSize),
		done:      false,
	}
}

// Next reads and returns the next Chunk from the reader. Returns io.EOF when exhausted.
func (c *StreamChunker) Next() (*Chunk, error) {
	if c.done {
		return nil, io.EOF
	}

	totalRead := 0
	for int64(totalRead) < c.chunkSize {
		n, err := c.reader.Read(c.buffer[totalRead:])
		if n > 0 {
			totalRead += n
		}
		if err != nil {
			if err == io.EOF {
				c.done = true
				break
			}
			return nil, fmt.Errorf("chunk read error at index %d: %w", c.index, err)
		}
	}

	if totalRead == 0 {
		c.done = true
		return nil, io.EOF
	}

	// Copy data to ensure independence
	chunkData := make([]byte, totalRead)
	copy(chunkData, c.buffer[:totalRead])

	sha := checksum.ComputeBytes(chunkData)
	safeKey := strings.ReplaceAll(strings.ReplaceAll(c.objectKey, "/", "_"), "\\", "_")
	chunkID := fmt.Sprintf("%s.chunk.%04d", safeKey, c.index)

	chunk := &Chunk{
		ID:        chunkID,
		ObjectKey: c.objectKey,
		Index:     c.index,
		Size:      int64(totalRead),
		Checksum:  sha,
		Data:      chunkData,
	}

	c.index++
	return chunk, nil
}
