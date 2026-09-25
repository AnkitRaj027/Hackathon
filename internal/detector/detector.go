package detector

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// NodeStatus represents the operational state of a storage node.
type NodeStatus string

const (
	StatusHealthy NodeStatus = "HEALTHY"
	StatusSuspect NodeStatus = "SUSPECT"
	StatusDead    NodeStatus = "DEAD"
)

// NodeProber defines the capability to probe a node for liveness.
type NodeProber interface {
	CheckStorageNode(ctx context.Context, nodeID string) (string, error)
}

// Config holds settings for the failure detector.
type Config struct {
	ProbeInterval    time.Duration
	SuspectThreshold int
	DeadThreshold    int
}

// DefaultConfig provides standard failure detection thresholds.
func DefaultConfig() Config {
	return Config{
		ProbeInterval:    1500 * time.Millisecond,
		SuspectThreshold: 1,
		DeadThreshold:    3,
	}
}

type nodeState struct {
	status              NodeStatus
	address             string
	consecutiveFailures int
	lastSuccess         time.Time
	lastProbe           time.Time
}

// StatusChangeCallback is invoked when a node transitions state.
type StatusChangeCallback func(nodeID string, oldStatus, newStatus NodeStatus)

// Detector continuously monitors storage node liveness and triggers failure alerts.
type Detector struct {
	mu        sync.RWMutex
	prober    NodeProber
	nodes     []string
	cfg       Config
	states    map[string]*nodeState
	callbacks []StatusChangeCallback

	cancel context.CancelFunc
	done   chan struct{}
}

// NewDetector initializes a failure detector for the given nodes.
func NewDetector(prober NodeProber, nodes []string, cfg Config) *Detector {
	if cfg.ProbeInterval <= 0 {
		cfg.ProbeInterval = 1500 * time.Millisecond
	}
	if cfg.SuspectThreshold <= 0 {
		cfg.SuspectThreshold = 1
	}
	if cfg.DeadThreshold <= 0 {
		cfg.DeadThreshold = 3
	}

	states := make(map[string]*nodeState)
	for _, n := range nodes {
		states[n] = &nodeState{
			status:      StatusHealthy,
			lastSuccess: time.Now(),
		}
	}

	return &Detector{
		prober: prober,
		nodes:  nodes,
		cfg:    cfg,
		states: states,
		done:   make(chan struct{}),
	}
}

// OnStatusChange registers a callback function for state transitions.
func (d *Detector) OnStatusChange(cb StatusChangeCallback) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callbacks = append(d.callbacks, cb)
}

// Start begins periodic background heartbeating.
func (d *Detector) Start(parentCtx context.Context) {
	d.mu.Lock()
	if d.cancel != nil {
		d.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parentCtx)
	d.cancel = cancel
	d.mu.Unlock()

	go d.runLoop(ctx)
}

// Stop gracefully shuts down the failure detector.
func (d *Detector) Stop() {
	d.mu.Lock()
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	d.mu.Unlock()
	<-d.done
}

func (d *Detector) runLoop(ctx context.Context) {
	defer close(d.done)
	ticker := time.NewTicker(d.cfg.ProbeInterval)
	defer ticker.Stop()

	// Initial immediate sweep
	d.ProbeAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.ProbeAll(ctx)
		}
	}
}

// ProbeAll runs a probe sweep across all monitored storage nodes in parallel.
func (d *Detector) ProbeAll(ctx context.Context) {
	d.mu.RLock()
	nodes := make([]string, len(d.nodes))
	copy(nodes, d.nodes)
	d.mu.RUnlock()

	var wg sync.WaitGroup
	for _, nodeID := range nodes {
		wg.Add(1)
		go func(nid string) {
			defer wg.Done()
			d.ProbeNode(ctx, nid)
		}(nodeID)
	}
	wg.Wait()
}

// ProbeNode tests liveness of a specific node and updates its state machine.
func (d *Detector) ProbeNode(ctx context.Context, nodeID string) error {
	addr, err := d.prober.CheckStorageNode(ctx, nodeID)

	d.mu.Lock()
	state, exists := d.states[nodeID]
	if !exists {
		state = &nodeState{
			status:  StatusHealthy,
			address: addr,
		}
		d.states[nodeID] = state
	}
	if addr != "" {
		state.address = addr
	}
	state.lastProbe = time.Now()

	oldStatus := state.status
	var newStatus NodeStatus

	if err == nil {
		state.consecutiveFailures = 0
		state.lastSuccess = time.Now()
		newStatus = StatusHealthy
	} else {
		state.consecutiveFailures++
		if state.consecutiveFailures >= d.cfg.DeadThreshold {
			newStatus = StatusDead
		} else if state.consecutiveFailures >= d.cfg.SuspectThreshold {
			newStatus = StatusSuspect
		} else {
			newStatus = state.status
		}
	}

	state.status = newStatus
	cbs := make([]StatusChangeCallback, len(d.callbacks))
	copy(cbs, d.callbacks)
	d.mu.Unlock()

	if oldStatus != newStatus {
		slog.Warn("NODE_STATE_TRANSITION",
			"node_id", nodeID,
			"old_state", oldStatus,
			"new_state", newStatus,
			"failures", state.consecutiveFailures,
			"error", err,
		)
		for _, cb := range cbs {
			cb(nodeID, oldStatus, newStatus)
		}
	}

	return err
}

// GetNodeStatus returns the current evaluated state for a given node.
func (d *Detector) GetNodeStatus(nodeID string) NodeStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if state, ok := d.states[nodeID]; ok {
		return state.status
	}
	return StatusDead
}

// IsNodeAlive returns true if the node is Healthy or Suspect (not Dead).
func (d *Detector) IsNodeAlive(nodeID string) bool {
	status := d.GetNodeStatus(nodeID)
	return status == StatusHealthy || status == StatusSuspect
}

// GetAllStatuses snapshot maps node IDs to their current status.
func (d *Detector) GetAllStatuses() map[string]NodeStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()
	res := make(map[string]NodeStatus, len(d.states))
	for k, v := range d.states {
		res[k] = v.status
	}
	return res
}
