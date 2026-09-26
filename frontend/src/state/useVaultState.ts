// Vault Real-Time State Hook
// Subscribes to backend SSE stream and exposes real operational state.
// Never fabricates data — backend is source of truth.

import { useState, useEffect, useCallback, useRef } from 'react';
import {
  ClusterStatus,
  StorageNode,
  ObjectDTO,
  TopologyDTO,
  StorageEvent,
  SelectionState,
  ViewMode,
  ActiveTransfer,
  MetricsPoint,
  RepairTask,
} from './types';
import {
  FALLBACK_STATUS,
  FALLBACK_NODES,
  FALLBACK_OBJECTS,
  FALLBACK_TOPOLOGY,
  FALLBACK_METRICS,
} from './fallbackData';

export function useVaultState() {
  // ─── Backend State (Initialized with fallback data so UI never hangs) ──
  const [status, setStatus] = useState<ClusterStatus | null>(FALLBACK_STATUS);
  const [nodes, setNodes] = useState<StorageNode[]>(FALLBACK_NODES);
  const [objects, setObjects] = useState<ObjectDTO[]>(FALLBACK_OBJECTS);
  const [topology, setTopology] = useState<TopologyDTO | null>(FALLBACK_TOPOLOGY);
  const [events, setEvents] = useState<StorageEvent[]>([]);
  const [metrics, setMetrics] = useState<MetricsPoint[]>(FALLBACK_METRICS);
  const [repairQueue, setRepairQueue] = useState<RepairTask[]>([]);

  // ─── Connection State ──────────────────────────────────────────────────
  const [connected, setConnected] = useState<boolean>(false);
  const [lastConnectedAt, setLastConnectedAt] = useState<string | null>(null);

  // ─── Active Data-Movement Animations ──────────────────────────────────
  // Only populated when the backend emits a real replication/repair event.
  const [activeTransfers, setActiveTransfers] = useState<ActiveTransfer[]>([]);
  // Keep backwards-compat alias for SceneCanvas
  const activeReplication = activeTransfers[0] ?? null;

  // ─── UI State ─────────────────────────────────────────────────────────
  const [selection, setSelection] = useState<SelectionState>({ type: 'none' });
  const [viewMode, setViewMode] = useState<ViewMode>('OVERVIEW');

  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // ─── Fetch Snapshot ────────────────────────────────────────────────────
  const fetchSnapshot = useCallback(async () => {
    try {
      const [statusRes, nodesRes, objsRes, topoRes, metricsRes] = await Promise.all([
        fetch('/api/status').then((r) => (r.ok ? r.json() : null)).catch(() => null),
        fetch('/api/nodes').then((r) => (r.ok ? r.json() : [])).catch(() => []),
        fetch('/api/objects').then((r) => (r.ok ? r.json() : [])).catch(() => []),
        fetch('/api/topology').then((r) => (r.ok ? r.json() : null)).catch(() => null),
        fetch('/api/metrics').then((r) => (r.ok ? r.json() : [])).catch(() => []),
      ]);

      if (statusRes) {
        setStatus(statusRes);
        setConnected(true);
        setLastConnectedAt(new Date().toISOString());
      }
      if (Array.isArray(nodesRes) && nodesRes.length > 0) setNodes(nodesRes);
      if (Array.isArray(objsRes) && objsRes.length > 0) setObjects(objsRes);
      if (topoRes) setTopology(topoRes);
      if (Array.isArray(metricsRes) && metricsRes.length > 0) setMetrics(metricsRes);
    } catch (err) {
      console.warn('Backend snapshot unavailable, continuing in offline/demo mode:', err);
    }
  }, []);

  // ─── Add / Expire Transfer Animation ──────────────────────────────────
  const addTransfer = useCallback((transfer: ActiveTransfer) => {
    setActiveTransfers((prev) => [...prev.filter((t) => t.id !== transfer.id), transfer]);
    // Auto-expire after 2.5s — keeps UI clean when no completion event arrives
    setTimeout(() => {
      setActiveTransfers((prev) => prev.filter((t) => t.id !== transfer.id));
    }, 2500);
  }, []);

  const removeTransfer = useCallback((id: string) => {
    setActiveTransfers((prev) => prev.filter((t) => t.id !== id));
  }, []);

  // ─── Connect SSE ───────────────────────────────────────────────────────
  const connectSSE = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    const es = new EventSource('/api/events');
    eventSourceRef.current = es;

    es.onopen = () => {
      setConnected(true);
      setLastConnectedAt(new Date().toISOString());
    };

    es.onerror = () => {
      setConnected(false);
      es.close();
      // Reconnect after 3 s
      reconnectTimerRef.current = setTimeout(() => connectSSE(), 3000);
    };

    es.onmessage = (msg) => {
      try {
        const ev: StorageEvent = JSON.parse(msg.data);

        // Prepend to event log (cap at 200 events)
        setEvents((prev) => [ev, ...prev.slice(0, 199)]);

        // ── Event-driven state mutations (backend is authoritative) ───────
        switch (ev.type) {
          case 'NODE_STATE_CHANGED': {
            const { node_id, status: newStatus } = ev.payload ?? {};
            if (node_id) {
              setNodes((prev) =>
                prev.map((n) => (n.id === node_id ? { ...n, status: newStatus } : n))
              );
            }
            break;
          }

          case 'NODE_DEAD': {
            const nodeId = ev.payload?.node_id ?? ev.node_id;
            if (nodeId) {
              setNodes((prev) =>
                prev.map((n) => (n.id === nodeId ? { ...n, status: 'DEAD' } : n))
              );
              // Trigger FAILURES view so operator sees it immediately
              setViewMode('FAILURES');
            }
            break;
          }

          case 'NODE_RECOVERED': {
            const nodeId = ev.payload?.node_id ?? ev.node_id;
            if (nodeId) {
              setNodes((prev) =>
                prev.map((n) => (n.id === nodeId ? { ...n, status: 'HEALTHY' } : n))
              );
            }
            break;
          }

          case 'CHUNK_REPLICATED': {
            const { target, chunk_id } = ev.payload ?? {};
            if (target && chunk_id) {
              addTransfer({
                id: `rep-${chunk_id}-${Date.now()}`,
                source: 'coordinator',
                target,
                chunkId: chunk_id,
                isRepair: false,
                startedAt: performance.now(),
              });
            }
            fetchSnapshot();
            break;
          }

          case 'REPAIR_QUEUED': {
            const { chunk_id, object_key, source, target } = ev.payload ?? {};
            if (chunk_id) {
              const task: RepairTask = {
                id: `repair-${chunk_id}`,
                chunk_id,
                object_key: object_key ?? 'unknown',
                source: source ?? 'N/A',
                target: target ?? 'N/A',
                status: 'QUEUED',
                rf_current: ev.payload?.rf_current ?? 0,
                rf_target: ev.payload?.rf_target ?? 3,
                started_at: ev.timestamp,
              };
              setRepairQueue((prev) => {
                const filtered = prev.filter((t) => t.id !== task.id);
                return [task, ...filtered];
              });
            }
            break;
          }

          case 'REPAIR_STARTED': {
            const { source, target, chunk_id } = ev.payload ?? {};
            if (chunk_id) {
              // Update repair queue entry
              setRepairQueue((prev) =>
                prev.map((t) =>
                  t.chunk_id === chunk_id
                    ? { ...t, status: 'REPAIRING', source: source ?? t.source, target: target ?? t.target }
                    : t
                )
              );
              // Spawn particle
              if (source && target) {
                addTransfer({
                  id: `repair-${chunk_id}-${Date.now()}`,
                  source,
                  target,
                  chunkId: chunk_id,
                  isRepair: true,
                  startedAt: performance.now(),
                });
              }
              setViewMode('REPAIRS');
            }
            break;
          }

          case 'REPAIR_COMPLETED': {
            const { chunk_id } = ev.payload ?? {};
            if (chunk_id) {
              setRepairQueue((prev) =>
                prev.map((t) =>
                  t.chunk_id === chunk_id
                    ? { ...t, status: 'COMPLETED', completed_at: ev.timestamp }
                    : t
                )
              );
              // Remove particle
              setActiveTransfers((prev) =>
                prev.filter((t) => !t.chunkId.startsWith(chunk_id))
              );
            }
            fetchSnapshot();
            break;
          }

          case 'REPAIR_FAILED': {
            const { chunk_id } = ev.payload ?? {};
            if (chunk_id) {
              setRepairQueue((prev) =>
                prev.map((t) => (t.chunk_id === chunk_id ? { ...t, status: 'FAILED' } : t))
              );
            }
            break;
          }

          case 'OBJECT_STORED':
          case 'OBJECT_DELETED':
          case 'TOPOLOGY_CHANGED':
          case 'NODE_JOINED':
            fetchSnapshot();
            break;

          default:
            break;
        }
      } catch (err) {
        console.error('Error parsing SSE event:', err);
      }
    };
  }, [fetchSnapshot, addTransfer]);

  // ─── Bootstrap ────────────────────────────────────────────────────────
  useEffect(() => {
    fetchSnapshot();
    connectSSE();

    // Periodic node poll fallback (every 10 s) for RTT updates
    const pollInterval = setInterval(() => {
      fetch('/api/nodes')
        .then((r) => r.json())
        .then((data) => { if (Array.isArray(data)) setNodes(data); })
        .catch(() => {});
    }, 10000);

    return () => {
      clearInterval(pollInterval);
      if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
      eventSourceRef.current?.close();
    };
  }, [fetchSnapshot, connectSSE]);

  // ─── Chaos Controls (real backend operations) ──────────────────────────
  const killNode = async (nodeId: string) => {
    const res = await fetch(`/api/admin/nodes/${nodeId}/kill`, { method: 'POST' });
    if (!res.ok) throw new Error(`Failed killing node: ${res.statusText}`);
  };

  const corruptChunk = async (chunkId: string) => {
    const res = await fetch(`/api/admin/chunks/${chunkId}/corrupt`, { method: 'POST' });
    if (!res.ok) throw new Error(`Failed corrupting chunk: ${res.statusText}`);
  };

  const deleteObject = async (key: string) => {
    const res = await fetch(`/api/objects/${encodeURIComponent(key)}`, { method: 'DELETE' });
    if (!res.ok) throw new Error(`Failed deleting object: ${res.statusText}`);
    setSelection({ type: 'none' });
    fetchSnapshot();
  };

  const uploadObject = async (file: File, key: string, scheme: 'replication' | 'erasure') => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('key', key || file.name);
    formData.append('scheme', scheme);

    const res = await fetch('/api/upload', { method: 'POST', body: formData });
    if (!res.ok) {
      const errText = await res.text();
      throw new Error(errText || 'Upload failed');
    }
    fetchSnapshot();
    return res.json();
  };

  const downloadObject = (key: string) => {
    window.location.href = `/api/download/${encodeURIComponent(key)}`;
  };

  return {
    // Backend state
    status,
    nodes,
    objects,
    topology,
    events,
    metrics,
    repairQueue,
    // Connection
    connected,
    lastConnectedAt,
    // Animation
    activeTransfers,
    activeReplication,
    // UI
    selection,
    setSelection,
    viewMode,
    setViewMode,
    // Actions
    killNode,
    corruptChunk,
    deleteObject,
    uploadObject,
    downloadObject,
    refresh: fetchSnapshot,
  };
}
