# Vault: Phase 4 Specification & Architecture

## 1. Phase 4 Overview: Reed-Solomon Erasure Coding ($k+m$)

In Phase 4, **Vault** adds enterprise-grade **Reed-Solomon Erasure Coding**, replacing brute-force $3\times$ replication with mathematically guaranteed fault tolerance and a **50% reduction in raw disk storage consumption**.

### Primary Objectives:
1. **Dramatic Storage Efficiency**: Reduce cluster storage overhead from **$3.00\times$** ($RF=3$) down to **$1.50\times$** ($2+1$ or $4+2$).
2. **Deterministic Mathematical Recovery**: Reconstruct lost data chunks using Cauchy/Vandermonde matrix operations over Galois Field $GF(2^8)$ from any $k$ surviving shards.
3. **Catastrophic Hardware Fault Tolerance**: Tolerate the complete wipe or crash of up to $m$ storage nodes without data loss.

---

## 2. Technical Architecture

### A. Reed-Solomon Galois-Field Codec ([`internal/erasure/codec.go`](file:///c:/Users/ankit/OneDrive/Desktop/hackathon/internal/erasure/codec.go))
- **Parameters**:
  - $k$: Number of data shards.
  - $m$: Number of parity shards.
  - Storage Overhead: $\frac{k+m}{k}$ (e.g. $\frac{2+1}{2} = 1.5\times$).
- **Encoding Pipeline**:
  1. Splits input data stream into $k$ equal-sized blocks.
  2. Multiplies data vector by the generator matrix in $GF(2^8)$ to derive $m$ parity blocks.
  3. Prefixes an 8-byte big-endian original payload length header onto each shard.
  4. Computes independent cryptographic SHA-256 digests for each shard.

### B. Distributed Striping Pipeline ([`internal/erasure/service.go`](file:///c:/Users/ankit/OneDrive/Desktop/hackathon/internal/erasure/service.go))
- **Parallel Shard Distribution**: Writes all $k+m$ shards concurrently across distinct storage nodes identified via the Consistent Hash Ring.
- **Write Quorum**: Requires at least $k$ successful shard writes to ensure linearizable durability before committing metadata.
- **Self-Describing Metadata**: Records `encoding=erasure_coding`, `ec_k`, `ec_m`, and shard digests in etcd.

### C. On-the-Fly Mathematical Reconstruction
- **Reconstruction Trigger**: When `GetObjectEC` reads shards and encounters missing or corrupted chunks:
  1. Validates checksum of each retrieved shard.
  2. Replaces corrupted/offline shards with `nil`.
  3. Verifies $\ge k$ valid shards exist.
  4. Inverts the sub-matrix of surviving rows and multiplies by the available shard vector in $GF(2^8)$ to restore the exact missing data shards.
  5. Assembles and trims the final byte stream to the verified original length.

---

## 3. Storage Efficiency Comparison

| Storage Mode | Data Shards ($k$) | Parity Shards ($m$) | Node Fault Tolerance ($m$) | Storage Overhead | Disk Space for 1 TB Data |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **Full Replication ($RF=3$)** | 1 | 2 | 2 nodes | **$3.00\times$** | **3.00 TB** |
| **Reed-Solomon ($2+1$)** | 2 | 1 | 1 node | **$1.50\times$** | **1.50 TB (50% savings!)** |
| **Reed-Solomon ($4+2$)** | 4 | 2 | 2 nodes | **$1.50\times$** | **1.50 TB (50% savings!)** |

---

## 4. Verification & Reproducibility Matrix

| Component | Test File | Verification Result |
| :--- | :--- | :--- |
| **Codec Round-Trip** | `internal/erasure/codec_test.go` | Clean encode/reconstruct with zero loss |
| **Single Data Shard Loss ($2+1$)** | `internal/erasure/codec_test.go` | Reconstructed with 1 data shard lost |
| **Parity Shard Loss ($2+1$)** | `internal/erasure/codec_test.go` | Reconstructed with parity shard lost |
| **Double Shard Loss ($4+2$)** | `internal/erasure/codec_test.go` | Reconstructed with 2 shards lost |
| **Exceeded Fault Tolerance** | `internal/erasure/codec_test.go` | Clean `ErrInsufficientShards` when loss $> m$ |
| **Distributed Storage Overhead** | `tests/integration/phase4_integration_test.go` | Confirmed exact **$1.50\times$** physical disk overhead |
| **Total Node Loss Recovery** | `tests/integration/phase4_integration_test.go` | 100% bit-for-bit reconstruction after hard node disk wipe |
