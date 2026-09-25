package erasure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"vault/internal/checksum"
	"vault/internal/placement"
	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

// StorageClientProvider provides connections to cluster storage nodes and metadata service.
type StorageClientProvider interface {
	GetStorageClient(nodeID string) (pbStorage.StorageServiceClient, error)
	GetMetadataClient() (pbMeta.MetadataServiceClient, error)
}

// Pipeline orchestrates distributed Reed-Solomon erasure encoding, striped placement, and reconstruction.
type Pipeline struct {
	codec *Codec
	pool  StorageClientProvider
	ring  *placement.ConsistentHashRing
}

// NewPipeline creates a distributed erasure coding pipeline.
func NewPipeline(dataShards, parityShards int, pool StorageClientProvider, ring *placement.ConsistentHashRing) (*Pipeline, error) {
	codec, err := NewCodec(dataShards, parityShards)
	if err != nil {
		return nil, err
	}
	return &Pipeline{
		codec: codec,
		pool:  pool,
		ring:  ring,
	}, nil
}

// PutObjectEC encodes an object into k+m shards, distributes them across distinct nodes, and persists metadata.
func (p *Pipeline) PutObjectEC(ctx context.Context, key string, data []byte) (*pbMeta.ObjectMetadata, error) {
	start := time.Now()
	shards, err := p.codec.Encode(data)
	if err != nil {
		return nil, fmt.Errorf("erasure encoding failed: %w", err)
	}

	// Placement: select k+m distinct physical storage nodes from the consistent hash ring
	shardNodes, err := p.ring.PlaceChunk(ctx, placement.ChunkDescriptor{
		ChunkID:   key,
		ObjectKey: key,
	})
	if err != nil || len(shardNodes) < p.codec.TotalShards() {
		// Fallback: collect all registered ring nodes up to totalShards
		allNodes := p.ring.AllNodes()
		if len(allNodes) < p.codec.TotalShards() {
			return nil, fmt.Errorf("insufficient cluster nodes for %d+%d erasure coding (have %d, need %d)",
				p.codec.DataShards(), p.codec.ParityShards(), len(allNodes), p.codec.TotalShards())
		}
		shardNodes = allNodes[:p.codec.TotalShards()]
	}

	type shardWriteResult struct {
		index  int
		nodeID string
		err    error
	}

	resChan := make(chan shardWriteResult, len(shards))
	var wg sync.WaitGroup

	// Write shards in parallel across distinct nodes
	for i, sh := range shards {
		targetNode := shardNodes[i%len(shardNodes)]
		wg.Add(1)
		go func(idx int, shard Shard, nodeID string) {
			defer wg.Done()
			client, cErr := p.pool.GetStorageClient(nodeID)
			if cErr != nil {
				resChan <- shardWriteResult{index: idx, nodeID: nodeID, err: cErr}
				return
			}

			shardID := fmt.Sprintf("%s.ec.shard.%02d", key, idx)
			writeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			resp, putErr := client.PutChunk(writeCtx, &pbStorage.PutChunkRequest{
				ChunkId:   shardID,
				Data:      shard.Data,
				Checksum:  shard.Checksum,
				ObjectKey: key,
				Index:     int64(idx),
			})
			if putErr != nil {
				resChan <- shardWriteResult{index: idx, nodeID: nodeID, err: putErr}
				return
			}
			if !resp.GetSuccess() {
				resChan <- shardWriteResult{index: idx, nodeID: nodeID, err: errors.New(resp.GetErrorMessage())}
				return
			}

			resChan <- shardWriteResult{index: idx, nodeID: nodeID, err: nil}
		}(i, sh, targetNode)
	}

	wg.Wait()
	close(resChan)

	var chunkMetaList []*pbMeta.ChunkMetadata
	successfulShards := 0

	for res := range resChan {
		if res.err == nil {
			successfulShards++
		} else {
			slog.Warn("erasure_shard_write_failed",
				"key", key,
				"shard_index", res.index,
				"node", res.nodeID,
				"error", res.err,
			)
		}
	}

	// Verify write quorum: must successfully write at least k data shards to be durable
	if successfulShards < p.codec.DataShards() {
		return nil, fmt.Errorf("erasure write failed: only %d of %d shards succeeded (minimum k=%d)",
			successfulShards, p.codec.TotalShards(), p.codec.DataShards())
	}

	for i, sh := range shards {
		targetNode := shardNodes[i%len(shardNodes)]
		chunkMetaList = append(chunkMetaList, &pbMeta.ChunkMetadata{
			ChunkId:   fmt.Sprintf("%s.ec.shard.%02d", key, i),
			ObjectKey: key,
			Index:     int64(i),
			Size:      sh.Size,
			Sha256:    sh.Checksum,
			Replicas:  []string{targetNode},
		})
	}

	objMeta := &pbMeta.ObjectMetadata{
		Key:       key,
		Size:      int64(len(data)),
		CreatedAt: time.Now().Unix(),
		Chunks:    chunkMetaList,
		CustomMetadata: map[string]string{
			"encoding":   "erasure_coding",
			"ec_k":       strconv.Itoa(p.codec.DataShards()),
			"ec_m":       strconv.Itoa(p.codec.ParityShards()),
			"ec_total":   strconv.Itoa(p.codec.TotalShards()),
			"orig_size":  strconv.FormatInt(int64(len(data)), 10),
			"sha256":     checksum.ComputeBytes(data),
		},
	}

	// Commit metadata to etcd
	metaClient, err := p.pool.GetMetadataClient()
	if err != nil {
		return nil, fmt.Errorf("metadata client unavailable: %w", err)
	}

	putMetaResp, err := metaClient.PutObject(ctx, &pbMeta.PutObjectMetadataRequest{Metadata: objMeta})
	if err != nil || !putMetaResp.GetSuccess() {
		return nil, fmt.Errorf("failed committing erasure object metadata: %v", err)
	}

	slog.Info("erasure_object_stored",
		"key", key,
		"size", len(data),
		"shards", len(shards),
		"data_shards", p.codec.DataShards(),
		"parity_shards", p.codec.ParityShards(),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return objMeta, nil
}

// GetObjectEC fetches shards from storage nodes and mathematically reconstructs the original object.
func (p *Pipeline) GetObjectEC(ctx context.Context, objMeta *pbMeta.ObjectMetadata) ([]byte, error) {
	totalShards := p.codec.TotalShards()
	shardsData := make([][]byte, totalShards)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, chunk := range objMeta.GetChunks() {
		idx := int(chunk.GetIndex())
		if idx >= totalShards {
			continue
		}
		if len(chunk.GetReplicas()) == 0 {
			continue
		}

		nodeID := chunk.GetReplicas()[0]
		wg.Add(1)

		go func(shardIdx int, cMeta *pbMeta.ChunkMetadata, nid string) {
			defer wg.Done()
			client, err := p.pool.GetStorageClient(nid)
			if err != nil {
				return
			}

			readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			stream, err := client.GetChunk(readCtx, &pbStorage.GetChunkRequest{
				ChunkId: cMeta.GetChunkId(),
			})
			if err != nil {
				return
			}

			var buf bytes.Buffer
			for {
				msg, sErr := stream.Recv()
				if errors.Is(sErr, io.EOF) {
					break
				}
				if sErr != nil {
					return
				}
				buf.Write(msg.GetData())
			}

			bytesRead := buf.Bytes()
			if checksum.Verify(cMeta.GetSha256(), checksum.ComputeBytes(bytesRead)) {
				mu.Lock()
				shardsData[shardIdx] = bytesRead
				mu.Unlock()
			} else {
				slog.Warn("erasure_shard_checksum_mismatch",
					"shard_id", cMeta.GetChunkId(),
					"node", nid,
				)
			}
		}(idx, chunk, nodeID)
	}

	wg.Wait()

	// Mathematically reconstruct the payload from available surviving shards
	reconstructed, err := p.codec.Reconstruct(shardsData)
	if err != nil {
		return nil, fmt.Errorf("erasure reconstruction failed: %w", err)
	}

	// Verify end-to-end checksum if recorded in metadata
	if expectedSHA, ok := objMeta.GetCustomMetadata()["sha256"]; ok && expectedSHA != "" {
		if !checksum.Verify(expectedSHA, checksum.ComputeBytes(reconstructed)) {
			return nil, errors.New("reconstructed erasure object checksum verification failed")
		}
	}

	return reconstructed, nil
}
