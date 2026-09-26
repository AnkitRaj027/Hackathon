// Fallback offline/demo cluster data
// Used when coordinator is offline or during initial startup handshake

import { ClusterStatus, StorageNode, ObjectDTO, TopologyDTO, MetricsPoint } from './types';

export const FALLBACK_STATUS: ClusterStatus = {
  status: 'STANDBY',
  coordinator: 'vault-coordinator-01 (offline)',
  total_nodes: 4,
  healthy_nodes: 4,
  active_objects: 3,
  replication_factor: 3,
  write_quorum: 2,
  read_quorum: 1,
  erasure_coding: true,
  timestamp: new Date().toISOString(),
};

export const FALLBACK_NODES: StorageNode[] = [
  {
    id: 'node-1',
    address: '10.0.1.11:50051',
    status: 'HEALTHY',
    is_partitioned: false,
    rtt_ms: 1.12,
    role: 'PRIMARY_STORAGE',
    rack: 'rack-alpha',
    zone: 'us-east-1a',
    drives: [
      { bay_index: 0, slot: 'bay-0', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1612, temperature_c: 34, wear_pct: 12, chunks_count: 48, chunk_ids: ['chk-weights-00', 'chk-ledger-00'] },
      { bay_index: 1, slot: 'bay-1', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1740, temperature_c: 35, wear_pct: 14, chunks_count: 52, chunk_ids: ['chk-dataset-00'] },
      { bay_index: 2, slot: 'bay-2', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1520, temperature_c: 33, wear_pct: 10, chunks_count: 45, chunk_ids: ['chk-weights-03'] },
      { bay_index: 3, slot: 'bay-3', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1390, temperature_c: 33, wear_pct: 9,  chunks_count: 39, chunk_ids: [] },
    ],
  },
  {
    id: 'node-2',
    address: '10.0.1.12:50052',
    status: 'HEALTHY',
    is_partitioned: false,
    rtt_ms: 1.45,
    role: 'REPLICA_STORAGE',
    rack: 'rack-alpha',
    zone: 'us-east-1a',
    drives: [
      { bay_index: 0, slot: 'bay-0', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1420, temperature_c: 32, wear_pct: 8,  chunks_count: 41, chunk_ids: ['chk-weights-00', 'chk-weights-01'] },
      { bay_index: 1, slot: 'bay-1', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1680, temperature_c: 34, wear_pct: 11, chunks_count: 46, chunk_ids: ['chk-ledger-00', 'chk-weights-03'] },
      { bay_index: 2, slot: 'bay-2', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1810, temperature_c: 35, wear_pct: 15, chunks_count: 50, chunk_ids: ['chk-dataset-01'] },
      { bay_index: 3, slot: 'bay-3', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1350, temperature_c: 32, wear_pct: 7,  chunks_count: 38, chunk_ids: [] },
    ],
  },
  {
    id: 'node-3',
    address: '10.0.1.13:50053',
    status: 'HEALTHY',
    is_partitioned: false,
    rtt_ms: 1.28,
    role: 'REPLICA_STORAGE',
    rack: 'rack-beta',
    zone: 'us-east-1b',
    drives: [
      { bay_index: 0, slot: 'bay-0', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1550, temperature_c: 33, wear_pct: 10, chunks_count: 44, chunk_ids: ['chk-weights-00', 'chk-weights-01', 'chk-weights-02'] },
      { bay_index: 1, slot: 'bay-1', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1620, temperature_c: 34, wear_pct: 12, chunks_count: 47, chunk_ids: ['chk-ledger-01'] },
      { bay_index: 2, slot: 'bay-2', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1480, temperature_c: 33, wear_pct: 9,  chunks_count: 42, chunk_ids: ['chk-dataset-00'] },
      { bay_index: 3, slot: 'bay-3', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1290, temperature_c: 31, wear_pct: 6,  chunks_count: 36, chunk_ids: [] },
    ],
  },
  {
    id: 'node-4',
    address: '10.0.1.14:50054',
    status: 'HEALTHY',
    is_partitioned: false,
    rtt_ms: 1.62,
    role: 'PARITY_STORAGE',
    rack: 'rack-beta',
    zone: 'us-east-1b',
    drives: [
      { bay_index: 0, slot: 'bay-0', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1690, temperature_c: 35, wear_pct: 13, chunks_count: 49, chunk_ids: ['chk-weights-01', 'chk-weights-02', 'chk-weights-03'] },
      { bay_index: 1, slot: 'bay-1', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1730, temperature_c: 36, wear_pct: 14, chunks_count: 51, chunk_ids: ['chk-ledger-00', 'chk-ledger-01'] },
      { bay_index: 2, slot: 'bay-2', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1510, temperature_c: 33, wear_pct: 11, chunks_count: 43, chunk_ids: ['chk-dataset-01'] },
      { bay_index: 3, slot: 'bay-3', status: 'HEALTHY', model: 'NVMe-Enterprise-3.84TB', capacity_gb: 3840, used_gb: 1400, temperature_c: 32, wear_pct: 8,  chunks_count: 39, chunk_ids: [] },
    ],
  },
];

export const FALLBACK_OBJECTS: ObjectDTO[] = [
  {
    key: 'models/llama-3-8b-instruct.safetensors',
    size: 4194304,
    chunks: [
      { chunk_id: 'chk-weights-00', index: 0, size: 1048576, sha256: '4a7d1ed414474e4033ac29ccb8653d9b048a82d396a0f4ed056c97d7', replicas: ['node-1', 'node-2', 'node-3'], is_parity: false },
      { chunk_id: 'chk-weights-01', index: 1, size: 1048576, sha256: 'b5d7d9a19c676d1e431804c4547bebb07b8b7095c9a6ff336f328f41', replicas: ['node-2', 'node-3', 'node-4'], is_parity: false },
      { chunk_id: 'chk-weights-02', index: 2, size: 1048576, sha256: '8d3e91b5c4f2e71829bb57201c13d7890a56e6d1838cf451a92e1215', replicas: ['node-3', 'node-4', 'node-1'], is_parity: false },
      { chunk_id: 'chk-weights-03', index: 3, size: 1048576, sha256: 'e2f0a1c97b8319dc6e82a3b04c8e71510fa9504e927c32bf28a9b6c0', replicas: ['node-4', 'node-1', 'node-2'], is_parity: true },
    ],
    created_at: '2026-09-25T14:32:00Z',
    checksum: '4a7d1ed414474e4033ac29ccb8653d9b048a82d396a0f4ed056c97d7',
    scheme: 'reed-solomon (2+1)',
  },
  {
    key: 'telemetry/audit-ledger-2026.parquet',
    size: 2097152,
    chunks: [
      { chunk_id: 'chk-ledger-00', index: 0, size: 1048576, sha256: '6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52dd', replicas: ['node-1', 'node-2', 'node-4'], is_parity: false },
      { chunk_id: 'chk-ledger-01', index: 1, size: 1048576, sha256: 'd4735e3a265e16eee03f59718b9b5d03019c07d8b6c51f90da3a666e', replicas: ['node-2', 'node-3', 'node-4'], is_parity: false },
    ],
    created_at: '2026-09-25T18:15:22Z',
    checksum: '6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52dd',
    scheme: '3x-replication',
  },
  {
    key: 'datasets/training-eval-subset.arrow',
    size: 2097152,
    chunks: [
      { chunk_id: 'chk-dataset-00', index: 0, size: 1048576, sha256: 'f1c2b3a4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2', replicas: ['node-1', 'node-3'], is_parity: false },
      { chunk_id: 'chk-dataset-01', index: 1, size: 1048576, sha256: 'a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2', replicas: ['node-2', 'node-4'], is_parity: false },
    ],
    created_at: '2026-09-26T02:10:00Z',
    checksum: 'f1c2b3a4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2',
    scheme: '2x-replication',
  },
];

export const FALLBACK_TOPOLOGY: TopologyDTO = {
  ring_type: 'consistent_hash_vnode',
  vnodes_per_node: 8,
  physical_nodes: ['node-1', 'node-2', 'node-3', 'node-4'],
  ring_points: [
    { token: 268435456, node: 'node-1' },
    { token: 536870912, node: 'node-2' },
    { token: 805306368, node: 'node-3' },
    { token: 1073741824, node: 'node-4' },
    { token: 1342177280, node: 'node-1' },
    { token: 1610612736, node: 'node-2' },
    { token: 1879048192, node: 'node-3' },
    { token: 2147483648, node: 'node-4' },
    { token: 2415919104, node: 'node-1' },
    { token: 2684354560, node: 'node-2' },
    { token: 2952790016, node: 'node-3' },
    { token: 3221225472, node: 'node-4' },
    { token: 3489660928, node: 'node-1' },
    { token: 3758096384, node: 'node-2' },
    { token: 4026531840, node: 'node-3' },
    { token: 4294967295, node: 'node-4' },
  ],
  total_points: 16,
};

export const FALLBACK_METRICS: MetricsPoint[] = [
  { timestamp: '12:00:00', write_mbps: 34.2, read_mbps: 88.5, iops: 3200, avg_latency_ms: 1.4 },
  { timestamp: '12:00:05', write_mbps: 42.1, read_mbps: 94.2, iops: 3650, avg_latency_ms: 1.3 },
  { timestamp: '12:00:10', write_mbps: 48.5, read_mbps: 112.3, iops: 4120, avg_latency_ms: 1.25 },
];
