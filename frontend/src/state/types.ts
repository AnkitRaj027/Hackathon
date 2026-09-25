// Vault State Types
// Canonical type definitions strictly corresponding to real backend data.
// Never add fabricated fields here.

// ─── Cluster & Node ────────────────────────────────────────────────────────

export interface ClusterStatus {
  status: string;
  coordinator: string;
  total_nodes: number;
  healthy_nodes: number;
  active_objects: number;
  replication_factor: number;
  write_quorum: number;
  read_quorum: number;
  erasure_coding: boolean;
  timestamp: string;
}

export type NodeStatus = 'HEALTHY' | 'DEGRADED' | 'SUSPECT' | 'DEAD' | 'REPAIRING' | 'PARTITIONED';

export interface DriveInfo {
  bay_index: number;
  slot: string;
  status: 'HEALTHY' | 'WARNING' | 'FAILED';
  model: string;
  capacity_gb: number;
  used_gb: number;
  temperature_c: number;
  wear_pct: number;
  chunks_count: number;
  chunk_ids: string[];
}

export interface StorageNode {
  id: string;
  address: string;
  status: NodeStatus;
  is_partitioned: boolean;
  rtt_ms: number;
  role: string;
  rack: string;
  zone: string;
  drives?: DriveInfo[];
}

// ─── Object / Chunk / Replica ──────────────────────────────────────────────

export interface ChunkDTO {
  chunk_id: string;
  index: number;
  size: number;
  sha256: string;
  replicas: string[];
  is_parity: boolean;
}

export interface ObjectDTO {
  key: string;
  size: number;
  chunks: ChunkDTO[];
  created_at: string;
  checksum: string;
  scheme: string;
}

// ─── Ring / Topology ──────────────────────────────────────────────────────

export interface RingPoint {
  token: number;
  node: string;
}

export interface TopologyDTO {
  ring_type: string;
  vnodes_per_node: number;
  physical_nodes: string[];
  ring_points: RingPoint[];
  total_points: number;
}

// ─── Repair Queue ─────────────────────────────────────────────────────────

export interface RepairTask {
  id: string;
  chunk_id: string;
  object_key: string;
  source: string;
  target: string;
  status: 'QUEUED' | 'REPAIRING' | 'COMPLETED' | 'FAILED';
  rf_current: number;
  rf_target: number;
  started_at?: string;
  completed_at?: string;
}

// ─── Metrics ──────────────────────────────────────────────────────────────

export interface MetricsPoint {
  timestamp: string;
  write_mbps: number;
  read_mbps: number;
  iops: number;
  avg_latency_ms: number;
}

// ─── Canonical Event Model ─────────────────────────────────────────────────
// Every event must come from the backend. No synthetic events.

export type VaultEventType =
  | 'NODE_JOINED'
  | 'NODE_HEARTBEAT'
  | 'NODE_STATE_CHANGED'
  | 'NODE_SUSPECTED'
  | 'NODE_DEAD'
  | 'NODE_RECOVERED'
  | 'OBJECT_CREATED'
  | 'OBJECT_STORED'
  | 'OBJECT_DELETED'
  | 'CHUNK_WRITTEN'
  | 'CHUNK_READ'
  | 'CHUNK_REPLICATED'
  | 'CHECKSUM_FAILURE'
  | 'REPLICATION_STARTED'
  | 'REPLICATION_PROGRESS'
  | 'REPLICATION_COMPLETED'
  | 'REPAIR_QUEUED'
  | 'REPAIR_STARTED'
  | 'REPAIR_PROGRESS'
  | 'REPAIR_COMPLETED'
  | 'REPAIR_FAILED'
  | 'TOPOLOGY_CHANGED'
  | 'NETWORK_PARTITION'
  | 'NETWORK_HEALED'
  | 'SYSTEM_LOG';

export interface StorageEvent {
  // id is optional for backwards compat — backend may not yet include it
  id?: string;
  type: VaultEventType | string;
  timestamp: string;
  node_id?: string;
  object_id?: string;
  chunk_id?: string;
  source?: string;
  target?: string;
  payload: Record<string, any> | any;
}

// ─── UI State ─────────────────────────────────────────────────────────────

export interface SelectionState {
  type: 'none' | 'node' | 'chunk' | 'object' | 'event';
  nodeId?: string;
  chunkId?: string;
  objectKey?: string;
  eventId?: string;
}

// View modes — visual emphasis over the same underlying state
export type ViewMode = 'OVERVIEW' | 'TOPOLOGY' | 'DATA' | 'REPAIRS' | 'FAILURES';

// Active replication/repair animation — driven only by real backend events
export interface ActiveTransfer {
  id: string;
  source: string;
  target: string;
  chunkId: string;
  isRepair: boolean;
  startedAt: number; // performance.now() timestamp
}
