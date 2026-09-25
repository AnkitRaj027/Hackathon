// Vault State Types
// Sections 81 & 82: Type definitions strictly corresponding to real backend data.

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

export interface StorageNode {
  id: string;
  address: string;
  status: 'HEALTHY' | 'DEGRADED' | 'SUSPECT' | 'DEAD' | 'REPAIRING';
  rtt_ms: number;
  role: string;
  rack: string;
  zone: string;
}

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

export interface StorageEvent {
  type: string;
  timestamp: string;
  payload: any;
}

export interface SelectionState {
  type: 'none' | 'node' | 'chunk' | 'object';
  nodeId?: string;
  chunkId?: string;
  objectKey?: string;
}
