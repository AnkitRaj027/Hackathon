# Vault: Phase 1 Specification & Implementation Scope

## 1. Phase 1 Goal
The primary objective of Phase 1 is to construct a **provably correct, fault-tolerant distributed object storage core** operating across actual processes, real disk storage, real cryptographic checksums, and consensus-backed metadata.

---

## 2. In-Scope Components (Phase 1)
- **Metadata Service**: Go gRPC service backed by etcd (`go.etcd.io/etcd/client/v3`) with versioned Protobuf schemas.
- **Storage Nodes**: Three independent nodes (`storage-01`, `storage-02`, `storage-03`) with isolated data volumes.
- **Crash-Safe Disk Storage**: Temp file writes, `fsync`, atomic renames, and SHA-256 sidecars (`.sha256`).
- **Coordinator Gateway**: Client entrypoint managing stream chunking, placement, quorum validation, metadata commit, and failover read reconstruction.
- **Fixed Replication (`RF = 3`, `W = 3`, `R = 1`)**: Chunks replicate to all 3 nodes before metadata commits.
- **Integrity Verification**: SHA-256 computed and validated at every hop.
- **vaultctl CLI**: Real command-line utility for operator workflows (`put`, `get`, `inspect`, `delete`, `nodes`).
- **Docker Compose Cluster**: Full containerized topology with isolated volume mounts.
- **Reproducible Test Suite**: Unit, integration, and failure tolerance tests.

---

## 3. Deliberately Deferred to Later Phases
To guarantee correctness before sophistication, the following are strictly deferred:
- **Failure Detector (SWIM / Gossip)**: Scheduled for Phase 2.
- **Repair Manager & Background Scrubbing**: Scheduled for Phase 2.
- **Configurable Quorum Variations (`W=2, R=2`)**: Interface-ready; enabled in Phase 2.
- **Consistent Hashing & Dynamic Ring Topology**: Scheduled for Phase 3.
- **Erasure Coding (Reed-Solomon)**: Scheduled for Phase 4.
- **3D Real-Time Dashboard & WebSocket Stream**: Scheduled for Phase 2+.

---

## 4. Architectural Trade-offs & Rationale

### Why etcd?
etcd implements the Raft consensus algorithm, providing linearizable, strongly consistent key-value storage. By decoupling object metadata consensus from chunk data storage, Vault achieves predictable transaction semantics without running Raft across gigabytes of bulk file chunks.

### Why Fixed Chunking (4 MiB)?
Fixed-size chunking provides deterministic memory boundaries for streaming pipelines, avoids unbounded memory allocation in the coordinator, and enables parallel multi-replica writes without loading complete multi-gigabyte files into RAM.

### Why SHA-256?
Cryptographic collision resistance ensures that silent disk corruption, network packet tampering, or accidental byte flips are caught immediately before corrupted data can reach client applications.

### Why Commit Metadata After Storage Nodes Ack?
Committing metadata before physical storage succeeds leads to the classic distributed systems trap: phantom data references. In Vault, metadata is only durable once disk write quorum is guaranteed.

---

## 5. Evolution to Phase 2
Phase 1 code explicitly designs interfaces to support Phase 2 without architectural rewrites:
- `PlacementStrategy` interface supports pluggable hashing algorithms.
- `ClientPool` and `StorageServiceClient` are prepared for heartbeat sweeps and background repairs.
- Quorum parameters (`VAULT_WRITE_QUORUM`, `VAULT_READ_QUORUM`) are environment-driven.
- The `frontend/` directory is prepared for React + Three.js real-time event streaming.
