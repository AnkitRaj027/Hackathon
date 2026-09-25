package detector_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"vault/internal/detector"
)

type mockProber struct {
	mu     sync.Mutex
	status map[string]error
}

func (m *mockProber) CheckStorageNode(ctx context.Context, nodeID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return "127.0.0.1:50000", m.status[nodeID]
}

func (m *mockProber) setNodeError(nodeID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status[nodeID] = err
}

func TestFailureDetector_Transitions(t *testing.T) {
	prober := &mockProber{
		status: map[string]error{
			"node-1": nil,
			"node-2": nil,
		},
	}

	cfg := detector.Config{
		ProbeInterval:    50 * time.Millisecond,
		SuspectThreshold: 1,
		DeadThreshold:    3,
	}

	var transitions []string
	var mu sync.Mutex

	d := detector.NewDetector(prober, []string{"node-1", "node-2"}, cfg)
	d.OnStatusChange(func(nodeID string, oldStatus, newStatus detector.NodeStatus) {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, string(nodeID)+":"+string(oldStatus)+"->"+string(newStatus))
	})

	ctx := context.Background()

	// Initial probe: both healthy
	d.ProbeAll(ctx)
	if d.GetNodeStatus("node-1") != detector.StatusHealthy {
		t.Fatalf("expected node-1 to be HEALTHY, got %v", d.GetNodeStatus("node-1"))
	}

	// Fail node-1 once -> SUSPECT
	prober.setNodeError("node-1", errors.New("timeout"))
	_ = d.ProbeNode(ctx, "node-1")
	if d.GetNodeStatus("node-1") != detector.StatusSuspect {
		t.Fatalf("expected node-1 to be SUSPECT, got %v", d.GetNodeStatus("node-1"))
	}
	if !d.IsNodeAlive("node-1") {
		t.Fatalf("suspect node should still be considered alive")
	}

	// Fail node-1 second time -> still SUSPECT
	_ = d.ProbeNode(ctx, "node-1")
	if d.GetNodeStatus("node-1") != detector.StatusSuspect {
		t.Fatalf("expected node-1 to still be SUSPECT, got %v", d.GetNodeStatus("node-1"))
	}

	// Fail node-1 third time -> DEAD
	_ = d.ProbeNode(ctx, "node-1")
	if d.GetNodeStatus("node-1") != detector.StatusDead {
		t.Fatalf("expected node-1 to be DEAD, got %v", d.GetNodeStatus("node-1"))
	}
	if d.IsNodeAlive("node-1") {
		t.Fatalf("dead node should NOT be considered alive")
	}

	// Recover node-1 -> HEALTHY
	prober.setNodeError("node-1", nil)
	_ = d.ProbeNode(ctx, "node-1")
	if d.GetNodeStatus("node-1") != detector.StatusHealthy {
		t.Fatalf("expected node-1 to recover to HEALTHY, got %v", d.GetNodeStatus("node-1"))
	}

	mu.Lock()
	defer mu.Unlock()
	expectedTransitions := []string{
		"node-1:HEALTHY->SUSPECT",
		"node-1:SUSPECT->DEAD",
		"node-1:DEAD->HEALTHY",
	}
	if len(transitions) != len(expectedTransitions) {
		t.Fatalf("expected transitions %v, got %v", expectedTransitions, transitions)
	}
}
