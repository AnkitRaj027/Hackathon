# Vault: Phase 2 Specification & Architecture

## 1. Phase 2 Overview: High Availability, Fault Tolerance & Self-Healing

Phase 2 transitions **Vault** from a strict-consensus distributed storage prototype into a resilient, self-healing, fault-tolerant distributed object store. The primary objectives are:
1. **Survivability under node loss**: Allowing writes and reads to succeed when a minority of nodes fail or crash.
2. **Bit-rot & Silent Data Corruption Resilience**: Detecting corrupt bytes via active scrubbing and auto-healing via Read Repair.
3. **Automated Self-Healing Re-replication**: Auditing chunk replica sets and restoring target Replication Factor (`RF=3`) without operator intervention.

---

## 2. Implemented Components

### A. Configurable / Flexible Quorum Engine
- **Write Quorum (`W`)**: Governed by `VAULT_WRITE_QUORUM` (e.g. `W=2` of `RF=3`). An object write commits as durable as long as at least `W` storage nodes fsync and acknowledge chunk writes.
- **Tolerance**: Cluster continues serving write and read traffic even if 1 storage node crashes or network-partitions.
- **Accurate Metadata Tracking**: Metadata only lists storage nodes that successfully acknowledged the write, enabling the self-healing manager to identify under-replicated chunks.

### B. Active Failure Detector (`internal/detector`)
- **Telemetry & Probing**: Concurrently probes storage nodes at configurable intervals (default: `1.5s`).
- **State Machine Transitions**:
  - `HEALTHY`: Node responds promptly to gRPC probes.
  - `SUSPECT`: Node missed 1 probe (temporary transient blip).
  - `DEAD`: Node missed $\ge 3$ consecutive probes.
- **Telemetry Integration**: Integrated with `ListNodes` RPC and `vaultctl nodes` for real-time cluster health inspection.

### C. Automatic Read Repair (`internal/coordinator`)
- **Cryptographic Verification at Read**: Every chunk read streams from replicas and calculates SHA-256 against metadata ground truth.
- **Failover**: If a replica returns an RPC error or a corrupted chunk (mismatched checksum), the coordinator fails over to the next replica.
- **Async Healing**: Upon verifying a good chunk from a surviving replica, the coordinator asynchronously issues a `PutChunk` to heal the corrupted or lagging replica on disk.

### D. Continuous Disk Scrubber (`internal/scrubber`)
- **Bit-Rot Sweep**: Periodic background worker running on storage nodes that iterates through all stored `.chunk` files.
- **Sidecar Comparison**: Recomputes physical SHA-256 and compares it against the `.sha256` sidecar file.
- **Alerting**: Emits structured `BIT_ROT_DETECTED` events when byte degradation is found.

### E. Self-Healing Repair Manager (`internal/repair`)
- **Continuous Audit**: Scans metadata objects to discover chunks with $\text{alive replicas} < \text{ReplicationFactor}$.
- **Orchestrated Re-replication**:
  1. Identifies a healthy source replica with verified SHA-256.
  2. Selects an alive target node that does not currently store the chunk.
  3. Copies the chunk data to the target node.
  4. Commits an updated replica list to etcd atomically.

---

## 3. Verification & Reproducibility Matrix

| Component | Test File | Verification Result |
| :--- | :--- | :--- |
| **Failure Detector Transitions** | `internal/detector/detector_test.go` | `HEALTHY -> SUSPECT -> DEAD -> HEALTHY` verified |
| **Disk Scrubber & Bit Rot** | `internal/scrubber/scrubber_test.go` | Detects physical bit-rot and missing sidecars |
| **Repair Manager Audit** | `internal/repair/repair_manager_test.go` | Re-replicates under-replicated chunk and updates metadata |
| **Read Repair End-to-End** | `tests/integration/phase2_integration_test.go` | Corrupted chunk on disk healed after failover read |
| **Flexible Quorum (`W=2`)** | `tests/integration/phase2_integration_test.go` | Write succeeds with 1 node completely offline |
| **Failure Detector Telemetry** | `tests/integration/phase2_integration_test.go` | Dead node reflected in `ListNodes` |
| **Self-Healing Re-replication** | `tests/integration/phase2_integration_test.go` | Under-replicated object restored to full `RF=3` on node recovery |
