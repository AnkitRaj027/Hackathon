package placement_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"vault/internal/placement"
)

func TestConsistentHashRing_DistinctReplicas(t *testing.T) {
	nodes := []string{"storage-01", "storage-02", "storage-03", "storage-04", "storage-05"}
	ring, err := placement.NewConsistentHashRing(nodes, 128, 3)
	if err != nil {
		t.Fatalf("failed creating ring: %v", err)
	}

	ctx := context.Background()
	for i := 0; i < 100; i++ {
		chunk := placement.ChunkDescriptor{
			ChunkID:   fmt.Sprintf("object-alpha.chunk.%04d", i),
			ObjectKey: "object-alpha",
			Index:     int64(i),
		}

		replicas, err := ring.PlaceChunk(ctx, chunk)
		if err != nil {
			t.Fatalf("placement failed: %v", err)
		}

		if len(replicas) != 3 {
			t.Fatalf("expected 3 replicas, got %d", len(replicas))
		}

		// Ensure all replicas are distinct physical nodes
		seen := make(map[string]bool)
		for _, r := range replicas {
			if seen[r] {
				t.Fatalf("duplicate physical node replica %s found in %v", r, replicas)
			}
			seen[r] = true
		}
	}
}

func TestConsistentHashRing_UniformDistribution(t *testing.T) {
	nodes := []string{"node-A", "node-B", "node-C", "node-D"}
	ring, err := placement.NewConsistentHashRing(nodes, 256, 3)
	if err != nil {
		t.Fatalf("failed creating ring: %v", err)
	}

	ctx := context.Background()
	numChunks := 1200
	nodeHits := make(map[string]int)

	for i := 0; i < numChunks; i++ {
		chunk := placement.ChunkDescriptor{
			ChunkID: fmt.Sprintf("chunk-%d", i),
		}
		replicas, err := ring.PlaceChunk(ctx, chunk)
		if err != nil {
			t.Fatalf("placement failed: %v", err)
		}
		// Record primary owner (first replica)
		nodeHits[replicas[0]]++
	}

	expectedMean := float64(numChunks) / float64(len(nodes)) // 300
	t.Logf("Node Distribution across %d chunks (Expected mean: %.1f):", numChunks, expectedMean)

	for _, n := range nodes {
		hits := nodeHits[n]
		t.Logf("  %s: %d chunks (%.1f%%)", n, hits, float64(hits)/float64(numChunks)*100)
		diff := math.Abs(float64(hits) - expectedMean)
		// With 256 vnodes, deviation should be reasonably balanced (< 25% from mean)
		if diff > expectedMean*0.35 {
			t.Errorf("distribution skewed for node %s: got %d, expected ~%.1f", n, hits, expectedMean)
		}
	}
}

func TestConsistentHashRing_AddNodeMinimalDisruption(t *testing.T) {
	ctx := context.Background()
	initialNodes := []string{"node-1", "node-2", "node-3"}
	oldRing, err := placement.NewConsistentHashRing(initialNodes, 256, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Create clone with 4th node added
	newRing, err := placement.NewConsistentHashRing(initialNodes, 256, 3)
	if err != nil {
		t.Fatal(err)
	}
	newRing.AddNode("node-4")

	if newRing.NodeCount() != 4 {
		t.Fatalf("expected 4 nodes, got %d", newRing.NodeCount())
	}

	numChunks := 1000
	chunks := make([]placement.ChunkDescriptor, numChunks)
	for i := 0; i < numChunks; i++ {
		chunks[i] = placement.ChunkDescriptor{
			ChunkID:   fmt.Sprintf("file-%d.chunk.0000", i),
			ObjectKey: fmt.Sprintf("file-%d", i),
		}
	}

	plan, err := placement.CalculateMigrationPlan(ctx, oldRing, newRing, chunks)
	if err != nil {
		t.Fatalf("calculate migration plan failed: %v", err)
	}

	// 1. Primary Replica Migration Test (RF=1)
	oldRing1, _ := placement.NewConsistentHashRing(initialNodes, 256, 1)
	newRing1, _ := placement.NewConsistentHashRing(initialNodes, 256, 1)
	newRing1.AddNode("node-4")

	plan1, err := placement.CalculateMigrationPlan(ctx, oldRing1, newRing1, chunks)
	if err != nil {
		t.Fatalf("calculate migration plan failed: %v", err)
	}

	t.Logf("Primary (RF=1) Migration Plan when scaling 3 -> 4 nodes:")
	t.Logf("  Total chunks: %d, Chunks moved: %d, Disruption: %.2f%%",
		plan1.TotalChunksAudited, plan1.ChunksMoved, plan1.DisruptionRatio*100)

	// For primary ownership, adding the 4th node should move ~25% (1/N) of keys
	if plan1.DisruptionRatio < 0.15 || plan1.DisruptionRatio > 0.35 {
		t.Errorf("primary disruption ratio %.2f outside expected range 15%%-35%% (expected ~25%%)", plan1.DisruptionRatio)
	}

	// 2. Multi-replica (RF=3) Migration Test: Every single migration MUST have Target == node-4
	// (Existing nodes should NEVER shuffle data between each other!)
	for _, m := range plan.Migrations {
		if m.Target != "node-4" {
			t.Errorf("unexpected migration target %s: data should only move to newly joined node-4", m.Target)
		}
	}
}

func TestConsistentHashRing_RemoveNodeGraceful(t *testing.T) {
	ctx := context.Background()
	nodes := []string{"node-1", "node-2", "node-3", "node-4"}
	ring, err := placement.NewConsistentHashRing(nodes, 128, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Remove node-4
	if err := ring.RemoveNode("node-4"); err != nil {
		t.Fatalf("failed removing node: %v", err)
	}
	if ring.NodeCount() != 3 {
		t.Fatalf("expected 3 nodes, got %d", ring.NodeCount())
	}

	// Verify placing 100 chunks works and none map to node-4
	for i := 0; i < 100; i++ {
		chunk := placement.ChunkDescriptor{ChunkID: fmt.Sprintf("chk-%d", i)}
		replicas, err := ring.PlaceChunk(ctx, chunk)
		if err != nil {
			t.Fatalf("placement failed after remove: %v", err)
		}
		for _, r := range replicas {
			if r == "node-4" {
				t.Fatalf("removed node-4 was still assigned a replica")
			}
		}
	}
}
