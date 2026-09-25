package placement

import (
	"context"
	"errors"
	"fmt"
)

// ChunkDescriptor provides the contextual information necessary for placing a chunk.
type ChunkDescriptor struct {
	ChunkID   string
	ObjectKey string
	Index     int64
	Size      int64
}

// PlacementStrategy abstracts how chunk replicas are assigned across nodes in the cluster.
type PlacementStrategy interface {
	PlaceChunk(ctx context.Context, chunk ChunkDescriptor) ([]string, error)
	AllNodes() []string
}

// FixedReplicationPlacement places every chunk on all available nodes up to replicationFactor.
type FixedReplicationPlacement struct {
	nodeIDs           []string
	replicationFactor int
}

// NewFixedReplicationPlacement creates a new FixedReplicationPlacement strategy.
func NewFixedReplicationPlacement(nodeIDs []string, replicationFactor int) (*FixedReplicationPlacement, error) {
	if len(nodeIDs) == 0 {
		return nil, errors.New("no storage nodes configured for placement")
	}
	if replicationFactor <= 0 {
		return nil, errors.New("replication factor must be greater than zero")
	}
	if replicationFactor > len(nodeIDs) {
		return nil, fmt.Errorf("replication factor %d exceeds available nodes count %d", replicationFactor, len(nodeIDs))
	}
	// Copy nodeIDs to prevent external modification
	nodes := make([]string, len(nodeIDs))
	copy(nodes, nodeIDs)

	return &FixedReplicationPlacement{
		nodeIDs:           nodes,
		replicationFactor: replicationFactor,
	}, nil
}

// PlaceChunk returns the list of target node IDs for the given chunk.
func (f *FixedReplicationPlacement) PlaceChunk(ctx context.Context, chunk ChunkDescriptor) ([]string, error) {
	if f.replicationFactor > len(f.nodeIDs) {
		return nil, fmt.Errorf("insufficient nodes: need %d, have %d", f.replicationFactor, len(f.nodeIDs))
	}
	replicas := make([]string, f.replicationFactor)
	copy(replicas, f.nodeIDs[:f.replicationFactor])
	return replicas, nil
}

// AllNodes returns all registered storage node IDs.
func (f *FixedReplicationPlacement) AllNodes() []string {
	nodes := make([]string, len(f.nodeIDs))
	copy(nodes, f.nodeIDs)
	return nodes
}
