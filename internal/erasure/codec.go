package erasure

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/klauspost/reedsolomon"
	"vault/internal/checksum"
)

var (
	ErrInsufficientShards = errors.New("cannot reconstruct: fewer than k valid shards available")
	ErrInvalidShardCount  = errors.New("invalid shard configuration: k and m must be >= 1")
)

// Shard represents an individual data or parity fragment produced by Reed-Solomon encoding.
type Shard struct {
	Index        int    `json:"index"`
	IsParity     bool   `json:"is_parity"`
	Data         []byte `json:"-"`
	Checksum     string `json:"checksum"`
	Size         int64  `json:"size"`
	OriginalSize int64  `json:"original_size"`
}

// Codec handles mathematical Reed-Solomon Galois-field encoding and reconstruction.
type Codec struct {
	dataShards   int
	parityShards int
	totalShards  int
	enc          reedsolomon.Encoder
}

// NewCodec creates a new Reed-Solomon erasure encoder/decoder with k data and m parity shards.
func NewCodec(dataShards, parityShards int) (*Codec, error) {
	if dataShards <= 0 || parityShards <= 0 {
		return nil, ErrInvalidShardCount
	}

	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, fmt.Errorf("failed creating Reed-Solomon encoder: %w", err)
	}

	return &Codec{
		dataShards:   dataShards,
		parityShards: parityShards,
		totalShards:  dataShards + parityShards,
		enc:          enc,
	}, nil
}

// Encode splits raw data into k data shards, computes m parity shards, and embeds metadata headers.
func (c *Codec) Encode(data []byte) ([]Shard, error) {
	origSize := int64(len(data))

	// Split data into k equal-sized shards (reedsolomon handles padding automatically)
	shards, err := c.enc.Split(data)
	if err != nil {
		return nil, fmt.Errorf("failed splitting data for erasure coding: %w", err)
	}

	// Compute parity shards
	if err := c.enc.Encode(shards); err != nil {
		return nil, fmt.Errorf("failed computing Reed-Solomon parity: %w", err)
	}

	result := make([]Shard, c.totalShards)
	for i := 0; i < c.totalShards; i++ {
		// Embed 8-byte original size prefix into shard data for standalone reconstruction
		var shardBuf bytes.Buffer
		_ = binary.Write(&shardBuf, binary.BigEndian, origSize)
		shardBuf.Write(shards[i])

		shardBytes := shardBuf.Bytes()
		result[i] = Shard{
			Index:        i,
			IsParity:     i >= c.dataShards,
			Data:         shardBytes,
			Checksum:     checksum.ComputeBytes(shardBytes),
			Size:         int64(len(shardBytes)),
			OriginalSize: origSize,
		}
	}

	return result, nil
}

// Reconstruct recovers missing or corrupted shards from any k surviving shards and returns the original bytes.
func (c *Codec) Reconstruct(shardsWithHeaders [][]byte) ([]byte, error) {
	if len(shardsWithHeaders) != c.totalShards {
		return nil, fmt.Errorf("expected %d shard slots, got %d", c.totalShards, len(shardsWithHeaders))
	}

	// Count available non-nil shards and extract original size
	validCount := 0
	var origSize int64 = -1
	rawShards := make([][]byte, c.totalShards)

	for i, sh := range shardsWithHeaders {
		if len(sh) >= 8 {
			validCount++
			if origSize == -1 {
				_ = binary.Read(bytes.NewReader(sh[:8]), binary.BigEndian, &origSize)
			}
			// Strip the 8-byte prefix for the Reed-Solomon math engine
			rawShards[i] = sh[8:]
		} else {
			rawShards[i] = nil
		}
	}

	if validCount < c.dataShards {
		return nil, fmt.Errorf("%w: have %d valid, require %d (k)", ErrInsufficientShards, validCount, c.dataShards)
	}

	// Mathematically reconstruct missing shards
	if err := c.enc.Reconstruct(rawShards); err != nil {
		return nil, fmt.Errorf("reedsolomon reconstruction failed: %w", err)
	}

	// Join the k data shards into the original data buffer
	var out bytes.Buffer
	for i := 0; i < c.dataShards; i++ {
		out.Write(rawShards[i])
	}

	// Truncate trailing padding to exact original byte length
	res := out.Bytes()
	if origSize >= 0 && int64(len(res)) > origSize {
		res = res[:origSize]
	}

	return res, nil
}

// DataShards returns k (number of data shards).
func (c *Codec) DataShards() int { return c.dataShards }

// ParityShards returns m (number of parity shards).
func (c *Codec) ParityShards() int { return c.parityShards }

// TotalShards returns k+m.
func (c *Codec) TotalShards() int { return c.totalShards }

// StorageOverhead returns the theoretical storage multiplier (e.g., 1.5 for 2+1 or 4+2).
func (c *Codec) StorageOverhead() float64 {
	return float64(c.totalShards) / float64(c.dataShards)
}
