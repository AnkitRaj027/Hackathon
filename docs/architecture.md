# Vault: System Architecture (Phase 1)

Vault is a fault-tolerant distributed object storage system engineered in Go, designed with real distributed consensus, verifiable data integrity, crash-safe storage primitives, and strictly observable backend states.

---

## 1. System Topology

```text
                           ┌─────────────────────────┐
                           │   vaultctl (CLI Client) │
                           └────────────┬────────────┘
                                        │
                                        │ gRPC (streaming)
                                        ▼
                           ┌─────────────────────────┐
                           │   Coordinator Gateway   │
                           │       (:50055)          │
                           └───────┬─────────┬───────┘
                                   │         │
                 Metadata Lookup   │         │ Replicated Chunks
                 & 2-Phase Commit  │         │ (RF=3, W=3, R=1)
                                   ▼         ▼
                      ┌────────────────┐   ┌────────────┬────────────┬────────────┐
                      │ Metadata Svc   │   │ storage-01 │ storage-02 │ storage-03 │
                      │    (:50050)    │   │  (:50051)  │  (:50052)  │  (:50053)  │
                      └───────┬────────┘   └────────────┴────────────┴────────────┘
                              │
                      Consensus State
                              ▼
                      ┌────────────────┐
                      │  etcd cluster  │
                      │    (:2379)     │
                      └────────────────┘
```

---

## 2. Component Roles & Responsibilities

### 2.1 Coordinator Gateway (`cmd/coordinator`)
- Serves as the primary gRPC entrypoint for all clients.
- Splits object input streams into fixed-size chunks (default: 4 MiB) using `StreamChunker`.
- Computes SHA-256 checksums per chunk and for the whole object.
- Queries `PlacementStrategy` (fixed replication `RF=3` in Phase 1) to determine target storage nodes.
- Dispatches parallel chunk writes to replica storage nodes.
- Enforces write quorum (`W=3` in Phase 1).
- **Critical Ordering**: Commits object metadata to the Metadata Service *only after* all replica writes confirm persistence.
- Handles object retrieval by reading from replicas, calculating on-the-fly checksums, comparing against metadata, and failing over to secondary replicas if corruption or node unavailability is detected.

### 2.2 Metadata Service (`cmd/metadata`)
- Backed by an etcd consensus cluster (`go.etcd.io/etcd/client/v3`) using versioned protobuf payloads.
- Maintains hierarchical namespace:
  - `/vault/objects/<encoded-key>`: Stores complete `ObjectMetadata` (sizes, timestamps, and chunk descriptors).
  - `/vault/chunks/<chunk-id>`: Chunk references.
  - `/vault/nodes/<node-id>`: Storage node registry.
- Provides atomic transactions and prefix-based scanning.

### 2.3 Storage Node (`cmd/storage-node`)
- Independent, isolated process managing its own dedicated local disk volume.
- Implements gRPC `StorageService` (`PutChunk`, `GetChunk`, `DeleteChunk`, `VerifyChecksum`).
- **Crash-Safe Write Pipeline**:
  1. Writes chunk payload to temporary file `<chunk-id>.tmp.<uuid>`.
  2. Executes `fsync` to flush dirty buffers to physical media.
  3. Verifies SHA-256 against caller expectation; rejects immediately if mismatched.
  4. Atomically renames temporary file to `<chunk-id>.chunk`.
  5. Atomically writes SHA-256 checksum sidecar `<chunk-id>.sha256`.
  6. Confirms write to coordinator.

### 2.4 vaultctl CLI (`cmd/vaultctl`)
- Direct operator CLI providing `put`, `get`, `inspect`, `delete`, and `nodes`.

---

## 3. Technology Decisions

| Choice | Rationale |
|---|---|
| **Go 1.24+** | Exceptional concurrency primitives, memory efficiency, static single-binary deployments, and native gRPC/protobuf support. |
| **gRPC & Protobuf** | Strongly typed, contract-first interfaces with bidirectional streaming support, low latency, and zero ambiguity across service boundaries. |
| **etcd (v3.5+)** | Industry-standard Raft consensus engine guaranteeing strong consistency (linearizable reads/writes) for metadata state. |
| **SHA-256 Checksums** | Collision-resistant cryptographic hashing at chunk and object boundaries to detect bit rot, network degradation, or tampered disk blocks. |
| **Isolated Disk Directories** | Strict separation of storage node volumes prevents cross-node state leaks. |
