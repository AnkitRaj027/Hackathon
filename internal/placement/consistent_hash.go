package placement

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
)

// ConsistentHashRing implements PlacementStrategy using consistent hashing with virtual nodes.
type ConsistentHashRing struct {
	mu                sync.RWMutex
	vnodes            int
	replicationFactor int
	ring              []uint32          // sorted virtual node token positions
	ringMap           map[uint32]string // token position -> physical node ID
	nodes             map[string]bool   // set of unique physical storage nodes
}

// NewConsistentHashRing initializes a consistent hash ring with virtual nodes.
func NewConsistentHashRing(nodeIDs []string, vnodes int, replicationFactor int) (*ConsistentHashRing, error) {
	if len(nodeIDs) == 0 {
		return nil, errors.New("cannot create consistent hash ring with zero nodes")
	}
	if vnodes <= 0 {
		vnodes = 128
	}
	if replicationFactor <= 0 {
		return nil, errors.New("replication factor must be greater than zero")
	}
	if replicationFactor > len(nodeIDs) {
		return nil, fmt.Errorf("replication factor %d exceeds physical node count %d", replicationFactor, len(nodeIDs))
	}

	chr := &ConsistentHashRing{
		vnodes:            vnodes,
		replicationFactor: replicationFactor,
		ringMap:           make(map[uint32]string),
		nodes:             make(map[string]bool),
	}

	for _, nid := range nodeIDs {
		chr.addNodeLocked(nid)
	}

	return chr, nil
}

// hashKey computes a 32-bit FNV-1a hash for a given token string.
func (c *ConsistentHashRing) hashKey(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}

// AddNode dynamically registers a new physical node and its virtual nodes on the ring.
func (c *ConsistentHashRing) AddNode(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addNodeLocked(nodeID)
}

func (c *ConsistentHashRing) addNodeLocked(nodeID string) {
	if c.nodes[nodeID] {
		return
	}
	c.nodes[nodeID] = true

	for i := 0; i < c.vnodes; i++ {
		vnodeKey := nodeID + "#vnode" + strconv.Itoa(i)
		token := c.hashKey(vnodeKey)
		c.ring = append(c.ring, token)
		c.ringMap[token] = nodeID
	}

	sort.Slice(c.ring, func(i, j int) bool {
		return c.ring[i] < c.ring[j]
	})
}

// RemoveNode dynamically unregisters a physical node and its virtual nodes from the ring.
func (c *ConsistentHashRing) RemoveNode(nodeID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.nodes[nodeID] {
		return fmt.Errorf("node %s not present on ring", nodeID)
	}
	if len(c.nodes)-1 < c.replicationFactor {
		return fmt.Errorf("cannot remove node %s: remaining nodes (%d) would be less than replication factor (%d)",
			nodeID, len(c.nodes)-1, c.replicationFactor)
	}

	delete(c.nodes, nodeID)

	var newRing []uint32
	for _, token := range c.ring {
		if c.ringMap[token] == nodeID {
			delete(c.ringMap, token)
		} else {
			newRing = append(newRing, token)
		}
	}
	c.ring = newRing

	return nil
}

// PlaceChunk maps a chunk deterministically to RF distinct physical nodes by walking the ring clockwise.
func (c *ConsistentHashRing) PlaceChunk(ctx context.Context, chunk ChunkDescriptor) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.ring) == 0 {
		return nil, errors.New("consistent hash ring is empty")
	}
	if len(c.nodes) < c.replicationFactor {
		return nil, fmt.Errorf("insufficient nodes: cluster has %d, required RF is %d", len(c.nodes), c.replicationFactor)
	}

	// Key used for consistent ring lookup: ChunkID
	lookupKey := chunk.ChunkID
	if lookupKey == "" {
		lookupKey = fmt.Sprintf("%s.chunk.%04d", chunk.ObjectKey, chunk.Index)
	}
	chunkHash := c.hashKey(lookupKey)

	// Binary search for first virtual node with token >= chunkHash
	idx := sort.Search(len(c.ring), func(i int) bool {
		return c.ring[i] >= chunkHash
	})

	// Wrap around if index reaches the end
	if idx == len(c.ring) {
		idx = 0
	}

	// Walk clockwise, selecting RF distinct physical nodes
	replicas := make([]string, 0, c.replicationFactor)
	selected := make(map[string]bool)

	startIdx := idx
	curr := startIdx
	for len(replicas) < c.replicationFactor {
		token := c.ring[curr]
		physicalNode := c.ringMap[token]

		if !selected[physicalNode] {
			selected[physicalNode] = true
			replicas = append(replicas, physicalNode)
		}

		curr = (curr + 1) % len(c.ring)
		if curr == startIdx && len(replicas) < c.replicationFactor {
			// Looped all the way around ring
			break
		}
	}

	if len(replicas) < c.replicationFactor {
		return nil, fmt.Errorf("unable to find %d distinct physical replicas on ring (got %d)", c.replicationFactor, len(replicas))
	}

	return replicas, nil
}

// AllNodes returns a sorted slice of all currently registered physical nodes.
func (c *ConsistentHashRing) AllNodes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	nodes := make([]string, 0, len(c.nodes))
	for n := range c.nodes {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	return nodes
}

// NodeCount returns the total number of physical nodes on the ring.
func (c *ConsistentHashRing) NodeCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.nodes)
}

// RingPoint represents a virtual node token position and owner on the ring.
type RingPoint struct {
	Token uint32 `json:"token"`
	Node  string `json:"node"`
}

// GetRingSnapshot returns the total vnode count and all virtual node token points.
func (c *ConsistentHashRing) GetRingSnapshot() (int, []RingPoint) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	points := make([]RingPoint, len(c.ring))
	for i, t := range c.ring {
		points[i] = RingPoint{
			Token: t,
			Node:  c.ringMap[t],
		}
	}
	return c.vnodes, points
}

