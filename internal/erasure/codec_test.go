package erasure_test

import (
	"bytes"
	"errors"
	"testing"

	"vault/internal/checksum"
	"vault/internal/erasure"
)

func TestCodec_RoundTrip_AllShardsIntact(t *testing.T) {
	codec, err := erasure.NewCodec(2, 1) // 2 data + 1 parity = 3 shards
	if err != nil {
		t.Fatalf("failed creating codec: %v", err)
	}

	if codec.StorageOverhead() != 1.5 {
		t.Errorf("expected 1.5x storage overhead, got %f", codec.StorageOverhead())
	}

	payload := []byte("The quick brown fox jumps over the lazy dog. 1234567890!")
	originalSHA := checksum.ComputeBytes(payload)

	shards, err := codec.Encode(payload)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	if len(shards) != 3 {
		t.Fatalf("expected 3 shards, got %d", len(shards))
	}

	rawShards := make([][]byte, 3)
	for i, s := range shards {
		rawShards[i] = s.Data
	}

	recovered, err := codec.Reconstruct(rawShards)
	if err != nil {
		t.Fatalf("reconstruction failed: %v", err)
	}

	if !bytes.Equal(recovered, payload) {
		t.Fatalf("recovered bytes do not match original: got %q, expected %q", string(recovered), string(payload))
	}
	if checksum.ComputeBytes(recovered) != originalSHA {
		t.Fatalf("recovered SHA mismatch")
	}
}

func TestCodec_Reconstruct_OneDataShardLost_2Plus1(t *testing.T) {
	codec, err := erasure.NewCodec(2, 1)
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte("Fault-tolerant erasure coding test payload for Vault distributed object storage!")
	shards, err := codec.Encode(payload)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate complete loss of shard 0 (first data shard)
	rawShards := make([][]byte, 3)
	rawShards[0] = nil // LOST!
	rawShards[1] = shards[1].Data
	rawShards[2] = shards[2].Data // Parity shard

	recovered, err := codec.Reconstruct(rawShards)
	if err != nil {
		t.Fatalf("reconstruction with missing data shard failed: %v", err)
	}

	if !bytes.Equal(recovered, payload) {
		t.Fatalf("recovered data mismatch: got %q, expected %q", string(recovered), string(payload))
	}
}

func TestCodec_Reconstruct_ParityShardLost_2Plus1(t *testing.T) {
	codec, err := erasure.NewCodec(2, 1)
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte("Testing parity shard loss in 2+1 Reed-Solomon scheme.")
	shards, err := codec.Encode(payload)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate loss of shard 2 (the parity shard)
	rawShards := make([][]byte, 3)
	rawShards[0] = shards[0].Data
	rawShards[1] = shards[1].Data
	rawShards[2] = nil // Parity LOST!

	recovered, err := codec.Reconstruct(rawShards)
	if err != nil {
		t.Fatalf("reconstruction with missing parity shard failed: %v", err)
	}

	if !bytes.Equal(recovered, payload) {
		t.Fatalf("recovered data mismatch")
	}
}

func TestCodec_Reconstruct_4Plus2_TwoShardsLost(t *testing.T) {
	codec, err := erasure.NewCodec(4, 2) // 4 data + 2 parity = 6 shards, tolerates 2 lost
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte("Large enterprise archive chunk encoded using 4+2 Reed-Solomon Galois-field arithmetic.")
	shards, err := codec.Encode(payload)
	if err != nil {
		t.Fatal(err)
	}

	if len(shards) != 6 {
		t.Fatalf("expected 6 shards, got %d", len(shards))
	}

	// Simulate loss of shards 1 and 4 (one data shard and one parity shard)
	rawShards := make([][]byte, 6)
	for i := range shards {
		rawShards[i] = shards[i].Data
	}
	rawShards[1] = nil // Data shard lost
	rawShards[4] = nil // Parity shard lost

	recovered, err := codec.Reconstruct(rawShards)
	if err != nil {
		t.Fatalf("4+2 reconstruction failed with 2 missing shards: %v", err)
	}

	if !bytes.Equal(recovered, payload) {
		t.Fatalf("recovered bytes mismatch")
	}

	// Now simulate loss of a 3rd shard (shards 1, 3, and 4 lost -> only 3 survive, but k=4!)
	rawShards[3] = nil
	_, err = codec.Reconstruct(rawShards)
	if err == nil || !errors.Is(err, erasure.ErrInsufficientShards) {
		t.Fatalf("expected ErrInsufficientShards when 3 shards are lost in 4+2, got %v", err)
	}
}
