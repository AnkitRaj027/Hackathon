# Vault: Fault-Tolerant Distributed Object Storage System

Vault is an extreme-scope, fault-tolerant distributed object storage system engineered in Go 1.24+. It demonstrates real distributed storage mechanics, including stream chunking, cryptographic checksum verification, Raft-consensus metadata management via etcd, crash-safe local disk persistence, and quorum replication with automatic replica failover.

Vault is built as a **real, production-grade distributed system**—no mocks, no synthetic heartbeats, no fake repairs, and no simulated states.

---

## Architecture Overview (Phase 1)

```text
                           ┌─────────────────────────┐
                           │   vaultctl (CLI Client) │
                           └────────────┬────────────┘
                                        │
                                        │ gRPC Streaming
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

### Core Services
1. **Coordinator Gateway (`cmd/coordinator`)**:
   - Primary client entrypoint for all data and metadata operations.
   - Splits object streams into configurable fixed-size chunks (default: 4 MiB) using `StreamChunker`.
   - Computes SHA-256 digests per chunk and for whole objects.
   - Places chunks across nodes using `PlacementStrategy` (`FixedReplicationPlacement` with `RF=3` in Phase 1).
   - Coordinates parallel writes and enforces write quorum (`W=3`).
   - **Critical Invariant**: Commits metadata to the Metadata Service *only after* all storage replicas acknowledge successful disk writes.
   - Streams downloads with on-the-fly SHA-256 verification and automatic replica failover.

2. **Metadata Service (`cmd/metadata`)**:
   - Manages linearizable object metadata backed by etcd (`go.etcd.io/etcd/client/v3`).
   - Stores versioned Protobuf records under structured namespaces (`/vault/objects/<base64-key>`, `/vault/chunks/<id>`, `/vault/nodes/<id>`).
   - Validates object keys to prevent path traversal attacks.

3. **Storage Nodes (`cmd/storage-node`)**:
   - Three independent nodes (`storage-01`, `storage-02`, `storage-03`) with completely isolated disk volumes.
   - **Crash-Safe Write Pipeline**:
     - Chunk writes stream into temporary files (`<chunk-id>.tmp.<uuid>`).
     - Executes `fsync` to guarantee persistence to disk media.
     - Validates SHA-256; aborts and removes temp file on mismatch.
     - Atomically renames temporary file to canonical `<chunk-id>.chunk`.
     - Atomically writes SHA-256 checksum sidecar `<chunk-id>.sha256`.

4. **vaultctl CLI (`cmd/vaultctl`)**:
   - Operator CLI for uploading, downloading, inspecting, deleting objects, and monitoring cluster storage nodes.

---

## Storage Layout

Each storage node maintains an isolated data directory:

```text
data/storage-01/
├── photos_cat.jpg.chunk.0000.chunk
├── photos_cat.jpg.chunk.0000.sha256
├── photos_cat.jpg.chunk.0001.chunk
└── photos_cat.jpg.chunk.0001.sha256
```

- `<chunk-id>.chunk`: Binary payload of the chunk.
- `<chunk-id>.sha256`: Hexadecimal SHA-256 digest sidecar.

---

## Getting Started

### Prerequisites
- Go 1.24+
- Protocol Buffers compiler (`protoc`)
- Docker & Docker Compose (for containerized deployments)

### 1. Build
```bash
make proto   # Compile protocol buffers
make build   # Compile all binaries into ./bin/
```

### 2. Run with Docker Compose
```bash
make up      # Starts etcd, metadata, storage-01, storage-02, storage-03, coordinator
```

To view live cluster logs:
```bash
make logs
```

To stop:
```bash
make down
```

---

## CLI Usage

Configure client target (default `localhost:50055`):
```bash
export VAULT_COORDINATOR_ADDR=localhost:50055
```

### Check Cluster Health
```bash
vaultctl nodes
```
Output:
```text
Cluster Storage Nodes:
NODE ID         ADDRESS                   STATUS    
-------------------------------------------------------
storage-01      127.0.0.1:50051           HEALTHY   
storage-02      127.0.0.1:50052           HEALTHY   
storage-03      127.0.0.1:50053           HEALTHY   
```

### Upload an Object (Put)
```bash
vaultctl put ./sample.bin sample-key
```
Output:
```text
Uploaded:
  Object:   sample-key
  Size:     8388608 B
  Chunks:   2
  Replicas: 3
  Checksum: 19d2297338c5a496486583a778753dedf2f4f6882d73b0ff65ac3ad9bc50532d
  Status:   SUCCESS
```

### Inspect Object Metadata & Replica Locations
```bash
vaultctl inspect sample-key
```
Output:
```text
Object: sample-key

Size:        8388608
Chunks:      2
Replication: 3

Chunk 0
  ID:       sample-key.chunk.0000
  Size:     4194304
  SHA-256:  2539cc8e3c55a2b59beb8cec23d547b83e2a6b167fb3548e8adcfe7aa38f0de2
  Replicas:
    storage-01
    storage-02
    storage-03

Chunk 1
  ID:       sample-key.chunk.0001
  Size:     4194304
  SHA-256:  88295ba4c73eeebd97a2c4182aa23de2cf94e1eac812b04bdf24876c92bf39d6
  Replicas:
    storage-01
    storage-02
    storage-03
```

### Download an Object (Get)
```bash
vaultctl get sample-key ./downloaded.bin
```
Output:
```text
Downloaded:
  Object:   sample-key
  Size:     8388608 B
  Checksum: 19d2297338c5a496486583a778753dedf2f4f6882d73b0ff65ac3ad9bc50532d
  Verified: true
  Output:   ./downloaded.bin
```

Verify exact match:
```bash
sha256sum ./sample.bin ./downloaded.bin
```

### Delete an Object
```bash
vaultctl delete sample-key
```
Output:
```text
Deleted object: sample-key (object deleted)
```

---

## Failure Behavior & Invariants (Phase 1)

1. **Node Failure During Reads (`R = 1`)**:
   - If one of the three storage nodes goes down or is unreachable, the Coordinator automatically routes reads to one of the remaining healthy replicas (`storage-02` or `storage-03`). The client download succeeds transparently without error.

2. **Bit Rot / Corrupted Data Detection**:
   - If a chunk file is manually corrupted on a storage node disk, the Coordinator detects the SHA-256 mismatch during `GetObject`, logs a warning (`CHECKSUM_MISMATCH: corrupted chunk detected on replica, failing over`), and reads the uncorrupted copy from another replica. Corrupted data is **never** silently served to the client.

3. **Durability Invariant**:
   - Metadata is committed to etcd only after all `WriteQuorum` replicas confirm successful disk persistence. If write quorum fails, metadata is never created, preventing phantom data references.

4. **Phase 1 Boundary**:
   - Automatic background active healing, repair managers, SWIM failure detectors, and dynamic consistent hashing are deliberately scheduled for subsequent phases.

---

## Testing

Run unit tests across all packages:
```bash
make test
```

Run comprehensive end-to-end integration tests (with Go race detector):
```bash
make integration-test
```

### Integration Test Suite Coverage:
- `TestIntegration_SingleNodeStorage`: Isolated crash-safe chunk write and sidecar verification.
- `TestIntegration_ThreeNodeReplication`: Multi-chunk object upload across all 3 nodes (`RF=3`).
- `TestIntegration_ChecksumVerification`: Manual bit-rot injection and verification rejection.
- `TestIntegration_ObjectReconstruction`: Exact byte-for-byte stream reconstruction.
- `TestIntegration_LargeObject`: 5 MiB object split across 10 chunks with complete reconstruction.
- `TestIntegration_IdempotentGet`: Repeated sequential downloads verified identical.
- `TestIntegration_DuplicatePut`: Deterministic overwrite with updated metadata.
- `TestIntegration_NodeFailureReadTolerance`: Abrupt process termination of `storage-01`; download succeeds seamlessly from surviving replicas.
