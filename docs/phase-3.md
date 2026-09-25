# Vault: Phase 3 Specification & Architecture

## 1. Phase 3 Overview: Consistent Hashing & Dynamic Ring Topology

In Phase 3, **Vault** transitions from fixed replication index mapping to an elastically scalable **Consistent Hash Ring** with **Virtual Nodes (vnodes)**.

### Primary Objectives:
1. **Uniform Key Distribution**: Eliminate storage hotspots by distributing chunks statistically evenly across all physical storage nodes.
2. **Minimal Data Disruption**: When scaling the cluster up ($N \rightarrow N+1$) or down ($N \rightarrow N-1$), minimize chunk relocations to the theoretical lower bound ($\approx 1/N$), avoiding massive full-cluster reshuffling.
3. **Deterministic Multi-Replica Placement**: Ensure that any chunk with replication factor $RF$ is assigned to $RF$ distinct physical nodes by walking the ring clockwise.

---

## 2. Technical Architecture

### A. Virtual Nodes (vnodes)
- **Token Density**: Each physical node maps to $V$ virtual node tokens (default: `256` vnodes per node) spread across the 32-bit integer space.
- **Hash Function**: Fast, deterministic 32-bit FNV-1a hashing on `nodeID + "#vnode" + index`.
- **Search Efficiency**: Ring tokens are kept in sorted order in memory. A binary search (`sort.Search`) locates the primary ring owner in $O(\log(N \times V))$ time.

### B. Clockwise Multi-Replica Ring Walk
- To place a chunk with `ReplicationFactor = RF`:
  1. Hash the chunk's unique `ChunkID` to locate its entry point on the ring.
  2. Traverse the ring tokens clockwise.
  3. Map each virtual token to its underlying physical storage node.
  4. Collect distinct physical nodes, skipping redundant virtual nodes that belong to an already-selected physical node.
  5. Conclude once $RF$ distinct physical nodes are selected.

### C. Dynamic Node Scaling (Join & Leave)
- **`AddNode(nodeID string)`**: Generates $V$ virtual node tokens for the joining node, inserts them into the ring, and re-sorts.
- **`RemoveNode(nodeID string)`**: Atomically purges all tokens for the leaving node from the ring.
- **Thread-Safety**: Protected by `sync.RWMutex`, allowing concurrent live reads and writes during topology modifications.

### D. Rebalance & Migration Planner (`internal/placement/rebalance.go`)
- Computes difference sets between two ring topologies:
  $$\text{Migrations} = \{ (c, \text{source}, \text{target}) \mid \text{target} \in \text{NewReplicas}(c) \setminus \text{OldReplicas}(c) \}$$
- Proves minimal disruption: adding an $(N+1)$-th node moves only $\approx \frac{1}{N+1}$ of primary keys.

---

## 3. Verification & Reproducibility Matrix

| Component | Test File | Verification Result |
| :--- | :--- | :--- |
| **Distinct Physical Replicas** | `internal/placement/consistent_hash_test.go` | All $RF$ replicas are distinct physical nodes |
| **Uniform Distribution** | `internal/placement/consistent_hash_test.go` | 1200 chunks balance across 4 nodes within statistical bounds |
| **Minimal Disruption ($1/N$)** | `internal/placement/consistent_hash_test.go` | Scaling 3 $\rightarrow$ 4 nodes moves 26.9% of primary keys (expected ~25%) |
| **Multi-Node Cluster Distribution** | `tests/integration/phase3_integration_test.go` | 5-node cluster distributes chunks across diverse node combinations |
| **Live Dynamic Ring Scaling** | `tests/integration/phase3_integration_test.go` | Dynamically added `node-04` receives chunk replicas while existing data remains accessible |
