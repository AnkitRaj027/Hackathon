package repair

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"vault/internal/checksum"
	pbMeta "vault/proto/metadata"
	pbStorage "vault/proto/storage"
)

// StorageClientProvider gets connected StorageServiceClient for a node.
type StorageClientProvider interface {
	GetStorageClient(nodeID string) (pbStorage.StorageServiceClient, error)
	GetMetadataClient() (pbMeta.MetadataServiceClient, error)
}

// NodeLivenessChecker reports whether a storage node is currently alive.
type NodeLivenessChecker interface {
	IsNodeAlive(nodeID string) bool
}

// RepairStats tracks summary counts from repair cycles.
type RepairStats struct {
	TotalObjectsChecked int
	TotalChunksChecked  int
	UnderReplicated     int
	RepairsAttempted    int
	RepairsSuccessful   int
	RepairsFailed       int
	Duration            time.Duration
}

// RepairEventCallback receives real-time repair lifecycle notifications.
type RepairEventCallback func(event string, chunkID string, srcNode string, targetNode string)

// Manager orchestrates background chunk audit sweeps and automatic self-healing replication.
type Manager struct {
	clientPool        StorageClientProvider
	liveness          NodeLivenessChecker
	allNodes          []string
	replicationFactor int
	repairInterval    time.Duration

	mu            sync.Mutex
	cancel        context.CancelFunc
	done          chan struct{}
	inRepair      sync.Map // prevents concurrent repairs for the same chunk_id
	eventCallback RepairEventCallback
}

// SetEventCallback registers a callback for repair lifecycle events.
func (m *Manager) SetEventCallback(cb RepairEventCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallback = cb
}

// NewManager creates a self-healing Repair Manager.
func NewManager(
	clientPool StorageClientProvider,
	liveness NodeLivenessChecker,
	allNodes []string,
	rf int,
	repairInterval time.Duration,
) *Manager {
	if rf <= 0 {
		rf = 3
	}
	if repairInterval <= 0 {
		repairInterval = 5 * time.Second
	}
	return &Manager{
		clientPool:        clientPool,
		liveness:          liveness,
		allNodes:          allNodes,
		replicationFactor: rf,
		repairInterval:    repairInterval,
		done:              make(chan struct{}),
	}
}

// Start launches the continuous background repair loop.
func (m *Manager) Start(parentCtx context.Context) {
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parentCtx)
	m.cancel = cancel
	m.mu.Unlock()

	go func() {
		defer close(m.done)
		ticker := time.NewTicker(m.repairInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = m.RunRepairCycle(ctx)
			}
		}
	}()
}

// Stop halts the background repair loop.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.mu.Unlock()
	<-m.done
}

// RunRepairCycle audits all objects in metadata, finding and repairing under-replicated chunks.
func (m *Manager) RunRepairCycle(ctx context.Context) (*RepairStats, error) {
	start := time.Now()
	stats := &RepairStats{
		Duration: 0,
	}

	metaClient, err := m.clientPool.GetMetadataClient()
	if err != nil {
		return stats, fmt.Errorf("metadata client unavailable: %w", err)
	}

	listResp, err := metaClient.ListObjects(ctx, &pbMeta.ListObjectsRequest{})
	if err != nil {
		return stats, fmt.Errorf("failed listing objects for repair audit: %w", err)
	}

	objects := listResp.GetObjects()
	stats.TotalObjectsChecked = len(objects)

	for _, obj := range objects {
		select {
		case <-ctx.Done():
			stats.Duration = time.Since(start)
			return stats, ctx.Err()
		default:
		}

		objectModified := false
		for _, chunk := range obj.GetChunks() {
			stats.TotalChunksChecked++

			// 1. Audit active replicas
			var aliveReplicas []string
			var deadReplicas []string
			replicaSet := make(map[string]bool)

			for _, replicaID := range chunk.GetReplicas() {
				replicaSet[replicaID] = true
				if m.liveness.IsNodeAlive(replicaID) {
					aliveReplicas = append(aliveReplicas, replicaID)
				} else {
					deadReplicas = append(deadReplicas, replicaID)
				}
			}

			// Under-replicated if alive replicas < target RF
			if len(aliveReplicas) < m.replicationFactor {
				stats.UnderReplicated++

				// Single-flight check: ensure another worker isn't already repairing this chunk
				if _, loaded := m.inRepair.LoadOrStore(chunk.GetChunkId(), true); loaded {
					continue
				}

				stats.RepairsAttempted++
				repairedNode, repairErr := m.repairChunk(ctx, chunk, aliveReplicas, replicaSet)
				m.inRepair.Delete(chunk.GetChunkId())

				if repairErr != nil {
					stats.RepairsFailed++
					slog.Error("SELF_HEALING_REPAIR_FAILED",
						"object", obj.GetKey(),
						"chunk_id", chunk.GetChunkId(),
						"alive_replicas", aliveReplicas,
						"dead_replicas", deadReplicas,
						"error", repairErr,
					)
				} else {
					stats.RepairsSuccessful++
					// Update chunk's replica list in-memory
					chunk.Replicas = append(chunk.Replicas, repairedNode)
					objectModified = true
					slog.Info("SELF_HEALING_REPAIR_SUCCESS",
						"object", obj.GetKey(),
						"chunk_id", chunk.GetChunkId(),
						"new_replica", repairedNode,
						"total_replicas", len(chunk.Replicas),
					)
				}
			}
		}

		// If any chunk in the object was repaired, persist updated metadata to etcd
		if objectModified {
			_, err := metaClient.PutObject(ctx, &pbMeta.PutObjectMetadataRequest{Metadata: obj})
			if err != nil {
				slog.Error("failed persisting repaired object metadata",
					"object", obj.GetKey(),
					"error", err,
				)
			}
		}
	}

	stats.Duration = time.Since(start)
	if stats.UnderReplicated > 0 {
		slog.Info("repair_cycle_summary",
			"objects", stats.TotalObjectsChecked,
			"chunks", stats.TotalChunksChecked,
			"under_replicated", stats.UnderReplicated,
			"repaired", stats.RepairsSuccessful,
			"failed", stats.RepairsFailed,
			"duration_ms", stats.Duration.Milliseconds(),
		)
	}

	return stats, nil
}

// repairChunk copies a chunk from a verified healthy source replica to an available healthy target node.
func (m *Manager) repairChunk(
	ctx context.Context,
	chunk *pbMeta.ChunkMetadata,
	aliveReplicas []string,
	existingReplicas map[string]bool,
) (string, error) {
	if len(aliveReplicas) == 0 {
		return "", errors.New("cannot repair: zero alive source replicas available (critical data loss)")
	}

	// 1. Find healthy target candidate not already holding a replica
	var targetNode string
	for _, nodeID := range m.allNodes {
		if !existingReplicas[nodeID] && m.liveness.IsNodeAlive(nodeID) {
			targetNode = nodeID
			break
		}
	}

	if targetNode == "" {
		return "", fmt.Errorf("no available candidate node for repair (all %d nodes already hold replica or dead)", len(m.allNodes))
	}

	// 2. Fetch chunk from one of the alive source replicas and verify checksum
	var chunkData []byte
	var fetchErr error
	var usedSourceNode string

	for _, sourceNode := range aliveReplicas {
		sourceClient, err := m.clientPool.GetStorageClient(sourceNode)
		if err != nil {
			fetchErr = err
			continue
		}

		readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		stream, err := sourceClient.GetChunk(readCtx, &pbStorage.GetChunkRequest{
			ChunkId: chunk.GetChunkId(),
		})
		if err != nil {
			cancel()
			fetchErr = err
			continue
		}

		var buf bytes.Buffer
		for {
			msg, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				fetchErr = err
				break
			}
			buf.Write(msg.GetData())
		}
		cancel()

		if fetchErr == nil {
			data := buf.Bytes()
			// Cryptographic verification
			if checksum.Verify(chunk.GetSha256(), checksum.ComputeBytes(data)) {
				chunkData = data
				usedSourceNode = sourceNode
				break
			} else {
				fetchErr = fmt.Errorf("source node %s chunk failed checksum verification", sourceNode)
			}
		}
	}

	if chunkData == nil {
		return "", fmt.Errorf("failed fetching verified chunk from any alive replica: %w", fetchErr)
	}

	m.mu.Lock()
	cb := m.eventCallback
	m.mu.Unlock()
	if cb != nil {
		cb("REPAIR_STARTED", chunk.GetChunkId(), usedSourceNode, targetNode)
	}

	// 3. Write chunk to target node
	targetClient, err := m.clientPool.GetStorageClient(targetNode)
	if err != nil {
		return "", fmt.Errorf("failed connecting to target node %s: %w", targetNode, err)
	}

	writeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := targetClient.PutChunk(writeCtx, &pbStorage.PutChunkRequest{
		ChunkId:   chunk.GetChunkId(),
		Data:      chunkData,
		Checksum:  chunk.GetSha256(),
		ObjectKey: chunk.GetObjectKey(),
		Index:     chunk.GetIndex(),
	})
	if err != nil {
		return "", fmt.Errorf("failed writing repaired chunk to target node %s: %w", targetNode, err)
	}
	if !resp.GetSuccess() {
		return "", fmt.Errorf("target node %s rejected repaired chunk: %s", targetNode, resp.GetErrorMessage())
	}

	if cb != nil {
		cb("REPAIR_COMPLETED", chunk.GetChunkId(), usedSourceNode, targetNode)
	}

	return targetNode, nil
}
