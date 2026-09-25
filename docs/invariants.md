# Vault: System Invariants

This document establishes the fundamental engineering invariants that govern the Vault distributed storage system. Every code modification, test suite, and architectural evolution must preserve these invariants.

---

## Invariant 1: Metadata Ordering & Durability Guarantee

> **Metadata must never claim that a replica or object exists unless all required storage nodes have acknowledged successful, durable persistence.**

### Implementation Rule:
1. The Coordinator splits objects into chunks.
2. The Coordinator dispatches parallel writes to target storage nodes.
3. Each storage node writes to disk, calls `fsync`, atomically renames the chunk, and writes the sidecar.
4. The Coordinator verifies that the number of successful node confirmations meets or exceeds `WriteQuorum` (in Phase 1, `W = 3`).
5. Only upon reaching write quorum does the Coordinator issue `PutObject` to the Metadata Service.
6. If write quorum is not achieved, the write fails with `codes.Unavailable`, and no metadata entry is committed.

---

## Invariant 2: Checksum Verification & Zero Silent Bit Rot

> **A downloaded chunk must never silently be returned to the client if its checksum does not match the recorded metadata checksum.**

### Implementation Rule:
1. Every chunk write computes a SHA-256 digest stored both in central metadata and in a local disk sidecar `<chunk-id>.sha256`.
2. Upon retrieval (`GetObject`), the Coordinator calculates the SHA-256 digest of the raw byte stream received from the replica.
3. If the computed digest does not match `ChunkMetadata.Sha256`, the Coordinator logs a high-severity `CHECKSUM_MISMATCH` alert, discards the tainted payload, and automatically fails over to the next replica node.
4. If all replicas fail or exhibit corruption, the operation aborts with `codes.DataLoss`.

---

## Invariant 3: Crash-Safe Local Disk Operations

> **A storage node process crash at any point during a write must never leave a partially written or corrupted logical chunk on disk.**

### Implementation Rule:
1. All chunk data is written to a temporary file (`<chunk-id>.tmp.<uuid>`).
2. The node invokes `f.Sync()` (`fsync`) before committing.
3. The temporary file is atomically renamed (`os.Rename`) to the canonical `<chunk-id>.chunk`.
4. The sidecar `<chunk-id>.sha256` is written to a temp file and atomically renamed.
5. Incomplete writes leave only temporary files, which are safely cleaned up.

---

## Invariant 4: True Backend Observability

> **The system state must originate strictly from actual backend state machines and disk reality. No simulated heartbeats, synthetic health flags, or frontend-invented events are permitted.**

### Implementation Rule:
- Health status reflects live socket reachability and filesystem read/write capability.
- Cluster topology and chunk distribution queries inspect physical disk layouts.
- Failure detection in future phases will be driven exclusively by real network probes and consensus heartbeats.

---

## Invariant 5: Idempotency Under Failure & Retries

> **Retrying any operation (`Put`, `Get`, `Delete`) must never leave the cluster in an inconsistent or indeterminate state.**

### Implementation Rule:
- Duplicate `Put` operations overwrite metadata deterministically and replace chunk files.
- `Delete` operations are completely idempotent: repeated deletes return success without error.
- Multiple concurrent or sequential `Get` operations yield identical, verified byte streams.
