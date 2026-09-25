package coordinator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"vault/internal/checksum"
	"vault/internal/chunking"
	"vault/internal/config"
	"vault/internal/detector"
	"vault/internal/metadata"
	"vault/internal/placement"
	pbCoord "vault/proto/coordinator"
	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

// Service implements pbCoord.CoordinatorServiceServer.
type Service struct {
	pbCoord.UnimplementedCoordinatorServiceServer
	cfg       *config.CoordinatorConfig
	pool      *ClientPool
	placement placement.PlacementStrategy
	detector  *detector.Detector
}

// NewService creates a new Coordinator service.
func NewService(cfg *config.CoordinatorConfig, pool *ClientPool, placementStrategy placement.PlacementStrategy) *Service {
	return &Service{
		cfg:       cfg,
		pool:      pool,
		placement: placementStrategy,
	}
}

// SetDetector configures the node failure detector for liveness tracking.
func (s *Service) SetDetector(d *detector.Detector) {
	s.detector = d
}

// PutObject processes an object upload stream, chunks it, persists replicas, and commits metadata.
func (s *Service) PutObject(stream pbCoord.CoordinatorService_PutObjectServer) error {
	ctx := stream.Context()
	start := time.Now()

	// 1. Receive initial header
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed receiving object header: %v", err)
	}
	header := firstMsg.GetHeader()
	if header == nil {
		return status.Error(codes.InvalidArgument, "first message must be an ObjectHeader")
	}

	key := header.GetKey()
	if err := metadata.ValidateKey(key); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid object key: %v", err)
	}

	// Read remainder of stream into buffer or chunk stream
	var dataBuf bytes.Buffer
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return status.Errorf(codes.Canceled, "failed receiving object data stream: %v", err)
		}
		dataBuf.Write(msg.GetChunkData())
	}

	totalBytes := int64(dataBuf.Len())
	wholeObjectSHA := checksum.ComputeBytes(dataBuf.Bytes())

	// 2. Chunk object
	chunker := chunking.NewStreamChunker(&dataBuf, key, s.cfg.ChunkSize)
	var storedChunks []*pbMeta.ChunkMetadata
	chunkIndex := int64(0)

	for {
		chunk, err := chunker.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "chunking failed: %v", err)
		}

		// 3. Placement
		replicas, err := s.placement.PlaceChunk(ctx, placement.ChunkDescriptor{
			ChunkID:   chunk.ID,
			ObjectKey: key,
			Index:     chunk.Index,
			Size:      chunk.Size,
		})
		if err != nil {
			return status.Errorf(codes.Internal, "placement failed for chunk %s: %v", chunk.ID, err)
		}

		// 4. Replicated writes across nodes
		type writeResult struct {
			nodeID string
			err    error
		}
		resChan := make(chan writeResult, len(replicas))
		var wg sync.WaitGroup

		for _, nodeID := range replicas {
			wg.Add(1)
			go func(nid string) {
				defer wg.Done()
				storageClient, clientErr := s.pool.GetStorageClient(nid)
				if clientErr != nil {
					resChan <- writeResult{nodeID: nid, err: clientErr}
					return
				}

				writeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				_, putErr := storageClient.PutChunk(writeCtx, &pbStorage.PutChunkRequest{
					ChunkId:   chunk.ID,
					Data:      chunk.Data,
					Checksum:  chunk.Checksum,
					ObjectKey: key,
					Index:     chunk.Index,
				})
				resChan <- writeResult{nodeID: nid, err: putErr}
			}(nodeID)
		}

		wg.Wait()
		close(resChan)

		// 5. Verify Quorum
		successfulWrites := 0
		var successfulNodes []string
		var writeErrors []error

		for res := range resChan {
			if res.err == nil {
				successfulWrites++
				successfulNodes = append(successfulNodes, res.nodeID)
			} else {
				writeErrors = append(writeErrors, fmt.Errorf("node %s write failed: %w", res.nodeID, res.err))
			}
		}

		if successfulWrites < s.cfg.WriteQuorum {
			slog.Error("quorum not reached for chunk write",
				"key", key,
				"chunk_id", chunk.ID,
				"successful", successfulWrites,
				"required_quorum", s.cfg.WriteQuorum,
				"errors", fmt.Sprintf("%v", writeErrors),
			)
			return status.Errorf(codes.Unavailable,
				"write quorum not reached for chunk %s (got %d, required %d): %v",
				chunk.ID, successfulWrites, s.cfg.WriteQuorum, writeErrors,
			)
		}

		storedChunks = append(storedChunks, &pbMeta.ChunkMetadata{
			ChunkId:   chunk.ID,
			ObjectKey: key,
			Index:     chunk.Index,
			Size:      chunk.Size,
			Sha256:    chunk.Checksum,
			Replicas:  successfulNodes,
		})

		chunkIndex++
	}

	// Special case: empty object (0 chunks)
	if len(storedChunks) == 0 {
		// Put an empty chunk record so metadata reflects the empty object
		safeKey := strings.ReplaceAll(strings.ReplaceAll(key, "/", "_"), "\\", "_")
		chunkID := fmt.Sprintf("%s.chunk.0000", safeKey)
		emptySHA := checksum.ComputeBytes([]byte{})

		replicas, err := s.placement.PlaceChunk(ctx, placement.ChunkDescriptor{
			ChunkID:   chunkID,
			ObjectKey: key,
			Index:     0,
			Size:      0,
		})
		if err == nil {
			var successfulNodes []string
			for _, nid := range replicas {
				if sc, cerr := s.pool.GetStorageClient(nid); cerr == nil {
					_, _ = sc.PutChunk(ctx, &pbStorage.PutChunkRequest{
						ChunkId:  chunkID,
						Data:     []byte{},
						Checksum: emptySHA,
					})
					successfulNodes = append(successfulNodes, nid)
				}
			}
			storedChunks = append(storedChunks, &pbMeta.ChunkMetadata{
				ChunkId:   chunkID,
				ObjectKey: key,
				Index:     0,
				Size:      0,
				Sha256:    emptySHA,
				Replicas:  successfulNodes,
			})
		}
	}

	// 6. Commit Metadata ONLY AFTER storage operations succeed
	metaClient, err := s.pool.GetMetadataClient()
	if err != nil {
		return status.Errorf(codes.Internal, "failed getting metadata client: %v", err)
	}

	objMeta := &pbMeta.ObjectMetadata{
		Key:            key,
		Size:           totalBytes,
		ContentType:    header.GetContentType(),
		CreatedAt:      time.Now().Unix(),
		Chunks:         storedChunks,
		CustomMetadata: header.GetMetadata(),
	}

	commitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err = metaClient.PutObject(commitCtx, &pbMeta.PutObjectMetadataRequest{
		Metadata: objMeta,
	})
	if err != nil {
		slog.Error("CRITICAL: metadata commit failed after chunk storage",
			"key", key,
			"chunks", len(storedChunks),
			"error", err.Error(),
		)
		return status.Errorf(codes.Internal, "metadata commit failed: %v", err)
	}

	duration := time.Since(start)
	slog.Info("put_object_complete",
		"key", key,
		"size", totalBytes,
		"chunks", len(storedChunks),
		"rf", s.cfg.ReplicationFactor,
		"duration_ms", duration.Milliseconds(),
	)

	return stream.SendAndClose(&pbCoord.PutObjectResponse{
		Key:               key,
		Size:              totalBytes,
		ChunkCount:        int64(len(storedChunks)),
		ReplicationFactor: int32(s.cfg.ReplicationFactor),
		Checksum:          wholeObjectSHA,
		Success:           true,
	})
}

// GetObject retrieves object metadata, reads each chunk from replicas with failover & SHA-256 verification, and streams data.
func (s *Service) GetObject(req *pbCoord.GetObjectRequest, stream pbCoord.CoordinatorService_GetObjectServer) error {
	ctx := stream.Context()
	key := req.GetKey()
	if err := metadata.ValidateKey(key); err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid object key: %v", err)
	}

	// 1. Lookup metadata
	metaClient, err := s.pool.GetMetadataClient()
	if err != nil {
		return status.Errorf(codes.Internal, "metadata client unavailable: %v", err)
	}

	metaResp, err := metaClient.GetObject(ctx, &pbMeta.GetObjectMetadataRequest{Key: key})
	if err != nil {
		return status.Errorf(codes.Internal, "metadata lookup failed: %v", err)
	}
	if !metaResp.GetFound() {
		return status.Errorf(codes.NotFound, "object %q not found", key)
	}

	objMeta := metaResp.GetMetadata()

	// 2. Send header first
	if err := stream.Send(&pbCoord.GetObjectResponse{
		Payload: &pbCoord.GetObjectResponse_Header{
			Header: &pbCoord.ObjectHeader{
				Key:         objMeta.GetKey(),
				Size:        objMeta.GetSize(),
				ContentType: objMeta.GetContentType(),
				Metadata:    objMeta.GetCustomMetadata(),
			},
		},
	}); err != nil {
		return status.Errorf(codes.Canceled, "failed sending object header: %v", err)
	}

	// 3. Stream chunks in index order
	for _, chunkMeta := range objMeta.GetChunks() {
		chunkData, err := s.readChunkWithFailover(ctx, chunkMeta)
		if err != nil {
			slog.Error("failed reading chunk from all replicas",
				"key", key,
				"chunk_id", chunkMeta.GetChunkId(),
				"error", err.Error(),
			)
			return status.Errorf(codes.DataLoss, "data loss: failed reading chunk %s: %v", chunkMeta.GetChunkId(), err)
		}

		// Stream chunk data to client
		sendErr := stream.Send(&pbCoord.GetObjectResponse{
			Payload: &pbCoord.GetObjectResponse_ChunkData{
				ChunkData: chunkData,
			},
		})
		if sendErr != nil {
			return status.Errorf(codes.Canceled, "failed streaming chunk to client: %v", sendErr)
		}
	}

	return nil
}

// readChunkWithFailover attempts to read a chunk from its replica nodes in order, validating the checksum.
// If any replica failed (connection, missing chunk, or SHA mismatch), it triggers Read Repair once a valid chunk is found.
func (s *Service) readChunkWithFailover(ctx context.Context, chunkMeta *pbMeta.ChunkMetadata) ([]byte, error) {
	var lastErr error
	var damagedNodes []string

	for _, nodeID := range chunkMeta.GetReplicas() {
		storageClient, err := s.pool.GetStorageClient(nodeID)
		if err != nil {
			lastErr = fmt.Errorf("node %s connection failed: %w", nodeID, err)
			damagedNodes = append(damagedNodes, nodeID)
			continue
		}

		readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		getStream, err := storageClient.GetChunk(readCtx, &pbStorage.GetChunkRequest{
			ChunkId: chunkMeta.GetChunkId(),
		})
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("node %s GetChunk call failed: %w", nodeID, err)
			damagedNodes = append(damagedNodes, nodeID)
			continue
		}

		var chunkBuf bytes.Buffer
		var streamErr error
		for {
			chunkDataMsg, err := getStream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				streamErr = err
				break
			}
			chunkBuf.Write(chunkDataMsg.GetData())
		}
		cancel()

		if streamErr != nil {
			lastErr = fmt.Errorf("node %s streaming chunk failed: %w", nodeID, streamErr)
			damagedNodes = append(damagedNodes, nodeID)
			continue
		}

		data := chunkBuf.Bytes()
		actualSHA := checksum.ComputeBytes(data)

		// Verify SHA-256 against metadata
		if !checksum.Verify(chunkMeta.GetSha256(), actualSHA) {
			slog.Warn("CHECKSUM_MISMATCH: corrupted chunk detected on replica, failing over",
				"node", nodeID,
				"chunk_id", chunkMeta.GetChunkId(),
				"expected", chunkMeta.GetSha256(),
				"actual", actualSHA,
			)
			lastErr = fmt.Errorf("checksum mismatch on node %s: expected %s, got %s", nodeID, chunkMeta.GetSha256(), actualSHA)
			damagedNodes = append(damagedNodes, nodeID)
			continue
		}

		// Valid chunk retrieved!
		if len(damagedNodes) > 0 {
			s.triggerReadRepair(chunkMeta, data, damagedNodes)
		}

		return data, nil
	}

	return nil, fmt.Errorf("all replicas failed for chunk %s: %w", chunkMeta.GetChunkId(), lastErr)
}

// triggerReadRepair heals corrupted or lagging replica nodes asynchronously with verified chunk data.
func (s *Service) triggerReadRepair(chunkMeta *pbMeta.ChunkMetadata, validData []byte, damagedNodes []string) {
	for _, nodeID := range damagedNodes {
		go func(nid string) {
			client, err := s.pool.GetStorageClient(nid)
			if err != nil {
				slog.Warn("READ_REPAIR_SKIPPED_UNAVAILABLE",
					"node", nid,
					"chunk_id", chunkMeta.GetChunkId(),
					"error", err,
				)
				return
			}

			repairCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, putErr := client.PutChunk(repairCtx, &pbStorage.PutChunkRequest{
				ChunkId:   chunkMeta.GetChunkId(),
				Data:      validData,
				Checksum:  chunkMeta.GetSha256(),
				ObjectKey: chunkMeta.GetObjectKey(),
				Index:     chunkMeta.GetIndex(),
			})
			if putErr != nil || (resp != nil && !resp.GetSuccess()) {
				slog.Warn("READ_REPAIR_FAILED",
					"node", nid,
					"chunk_id", chunkMeta.GetChunkId(),
					"error", putErr,
				)
			} else {
				slog.Info("READ_REPAIR_SUCCESS",
					"node", nid,
					"chunk_id", chunkMeta.GetChunkId(),
					"bytes_repaired", len(validData),
				)
			}
		}(nodeID)
	}
}

// DeleteObject deletes an object's metadata and chunk data from all storage nodes idempotently.
func (s *Service) DeleteObject(ctx context.Context, req *pbCoord.DeleteObjectRequest) (*pbCoord.DeleteObjectResponse, error) {
	key := req.GetKey()
	if err := metadata.ValidateKey(key); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid object key: %v", err)
	}

	metaClient, err := s.pool.GetMetadataClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "metadata client unavailable: %v", err)
	}

	delResp, err := metaClient.DeleteObject(ctx, &pbMeta.DeleteObjectMetadataRequest{Key: key})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed deleting metadata: %v", err)
	}

	if delResp.GetDeletedMetadata() != nil {
		for _, chunk := range delResp.GetDeletedMetadata().GetChunks() {
			for _, nodeID := range chunk.GetReplicas() {
				if storageClient, err := s.pool.GetStorageClient(nodeID); err == nil {
					delCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					_, _ = storageClient.DeleteChunk(delCtx, &pbStorage.DeleteChunkRequest{
						ChunkId: chunk.GetChunkId(),
					})
					cancel()
				}
			}
		}
	}

	slog.Info("delete_object", "key", key)

	return &pbCoord.DeleteObjectResponse{
		Key:     key,
		Success: true,
		Message: "object deleted",
	}, nil
}

// InspectObject returns the detailed chunk layout and replica locations.
func (s *Service) InspectObject(ctx context.Context, req *pbCoord.InspectObjectRequest) (*pbCoord.InspectObjectResponse, error) {
	key := req.GetKey()
	if err := metadata.ValidateKey(key); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid object key: %v", err)
	}

	metaClient, err := s.pool.GetMetadataClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "metadata client unavailable: %v", err)
	}

	metaResp, err := metaClient.GetObject(ctx, &pbMeta.GetObjectMetadataRequest{Key: key})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "metadata lookup failed: %v", err)
	}
	if !metaResp.GetFound() {
		return nil, status.Errorf(codes.NotFound, "object %q not found", key)
	}

	return &pbCoord.InspectObjectResponse{
		Metadata: metaResp.GetMetadata(),
	}, nil
}

// ListNodes returns the status of storage nodes in the cluster using Failure Detector telemetry.
func (s *Service) ListNodes(ctx context.Context, req *pbCoord.ListNodesRequest) (*pbCoord.ListNodesResponse, error) {
	var nodes []*pbCoord.StorageNodeInfo

	for _, nodeID := range s.placement.AllNodes() {
		addr, err := s.pool.CheckStorageNode(ctx, nodeID)
		nodeStatus := "HEALTHY"
		if s.detector != nil {
			nodeStatus = string(s.detector.GetNodeStatus(nodeID))
		} else if err != nil {
			nodeStatus = "DEAD"
		}
		nodes = append(nodes, &pbCoord.StorageNodeInfo{
			NodeId:  nodeID,
			Address: addr,
			Status:  nodeStatus,
		})
	}

	return &pbCoord.ListNodesResponse{Nodes: nodes}, nil
}
