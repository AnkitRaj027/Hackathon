package placement

import (
	"context"
	"fmt"
)

// ChunkMigration describes a chunk that needs to be copied to a new node due to ring topology changes.
type ChunkMigration struct {
	ChunkID   string
	Source    string
	Target    string
	ObjectKey string
}

// MigrationPlan summarizes data movements required to transition between two ring topologies.
type MigrationPlan struct {
	TotalChunksAudited int
	ChunksMoved        int
	DisruptionRatio    float64
	Migrations         []ChunkMigration
}

// CalculateMigrationPlan computes the required replica migrations between an old and new ring topology.
func CalculateMigrationPlan(
	ctx context.Context,
	oldRing *ConsistentHashRing,
	newRing *ConsistentHashRing,
	chunks []ChunkDescriptor,
) (*MigrationPlan, error) {
	plan := &MigrationPlan{
		TotalChunksAudited: len(chunks),
	}

	for _, chunk := range chunks {
		oldReplicas, err := oldRing.PlaceChunk(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("old ring placement failed for chunk %s: %w", chunk.ChunkID, err)
		}

		newReplicas, err := newRing.PlaceChunk(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("new ring placement failed for chunk %s: %w", chunk.ChunkID, err)
		}

		oldSet := make(map[string]bool)
		for _, r := range oldReplicas {
			oldSet[r] = true
		}

		// Find newly assigned replicas that weren't in old set
		moved := false
		for _, nr := range newReplicas {
			if !oldSet[nr] {
				moved = true
				source := oldReplicas[0]
				plan.Migrations = append(plan.Migrations, ChunkMigration{
					ChunkID:   chunk.ChunkID,
					Source:    source,
					Target:    nr,
					ObjectKey: chunk.ObjectKey,
				})
			}
		}

		if moved {
			plan.ChunksMoved++
		}
	}

	if plan.TotalChunksAudited > 0 {
		plan.DisruptionRatio = float64(plan.ChunksMoved) / float64(plan.TotalChunksAudited)
	}

	return plan, nil
}
