// Vault Real-Time State Hook
// Section 81 & 82: Subscribes to backend SSE stream and exposes real operational state.

import { useState, useEffect, useCallback, useRef } from 'react';
import {
  ClusterStatus,
  StorageNode,
  ObjectDTO,
  TopologyDTO,
  StorageEvent,
  SelectionState,
} from './types';

export function useVaultState() {
  const [status, setStatus] = useState<ClusterStatus | null>(null);
  const [nodes, setNodes] = useState<StorageNode[]>([]);
  const [objects, setObjects] = useState<ObjectDTO[]>([]);
  const [topology, setTopology] = useState<TopologyDTO | null>(null);
  const [events, setEvents] = useState<StorageEvent[]>([]);
  const [connected, setConnected] = useState<boolean>(false);
  const [activeReplication, setActiveReplication] = useState<{
    id: string;
    source: string;
    target: string;
    chunkId: string;
  } | null>(null);

  const [selection, setSelection] = useState<SelectionState>({ type: 'none' });
  const eventSourceRef = useRef<EventSource | null>(null);

  // Fetch complete cluster snapshot
  const fetchSnapshot = useCallback(async () => {
    try {
      const [statusRes, nodesRes, objsRes, topoRes] = await Promise.all([
        fetch('/api/status').then((r) => (r.ok ? r.json() : null)),
        fetch('/api/nodes').then((r) => (r.ok ? r.json() : [])),
        fetch('/api/objects').then((r) => (r.ok ? r.json() : [])),
        fetch('/api/topology').then((r) => (r.ok ? r.json() : null)),
      ]);

      if (statusRes) setStatus(statusRes);
      if (nodesRes) setNodes(nodesRes);
      if (objsRes) setObjects(objsRes);
      if (topoRes) setTopology(topoRes);
    } catch (err) {
      console.error('Failed fetching Vault snapshot:', err);
    }
  }, []);

  // Connect to SSE event stream
  useEffect(() => {
    fetchSnapshot();

    const es = new EventSource('/api/events');
    eventSourceRef.current = es;

    es.onopen = () => {
      setConnected(true);
    };

    es.onerror = () => {
      setConnected(false);
    };

    es.onmessage = (msg) => {
      try {
        const ev: StorageEvent = JSON.parse(msg.data);
        setEvents((prev) => [ev, ...prev.slice(0, 99)]);

        // Event-driven state updates
        if (ev.type === 'NODE_STATE_CHANGED' && ev.payload) {
          const { node_id, status: newStatus } = ev.payload;
          setNodes((prev) =>
            prev.map((n) => (n.id === node_id ? { ...n, status: newStatus } : n))
          );
        } else if (ev.type === 'CHUNK_REPLICATED' && ev.payload) {
          const { target, chunk_id } = ev.payload;
          setActiveReplication({
            id: `${Date.now()}-${chunk_id}`,
            source: 'coordinator',
            target,
            chunkId: chunk_id,
          });
          setTimeout(() => setActiveReplication(null), 1800);
          fetchSnapshot();
        } else if (ev.type === 'REPAIR_STARTED' && ev.payload) {
          const { source, target, chunk_id } = ev.payload;
          setActiveReplication({
            id: `${Date.now()}-${chunk_id}`,
            source,
            target,
            chunkId: chunk_id,
          });
        } else if (ev.type === 'REPAIR_COMPLETED') {
          setTimeout(() => setActiveReplication(null), 800);
          fetchSnapshot();
        } else if (ev.type === 'OBJECT_STORED' || ev.type === 'OBJECT_DELETED') {
          fetchSnapshot();
        }
      } catch (err) {
        console.error('Error parsing SSE event:', err);
      }
    };

    // Periodic poll fallback for node latencies (every 10s)
    const pollInterval = setInterval(() => {
      fetch('/api/nodes')
        .then((r) => r.json())
        .then((data) => {
          if (Array.isArray(data)) setNodes(data);
        })
        .catch(() => {});
    }, 10000);

    return () => {
      clearInterval(pollInterval);
      es.close();
    };
  }, [fetchSnapshot]);

  // Chaos controls
  const killNode = async (nodeId: string) => {
    const res = await fetch(`/api/admin/nodes/${nodeId}/kill`, {
      method: 'POST',
    });
    if (!res.ok) {
      throw new Error(`Failed killing node: ${res.statusText}`);
    }
  };

  const corruptChunk = async (chunkId: string) => {
    const res = await fetch(`/api/admin/chunks/${chunkId}/corrupt`, {
      method: 'POST',
    });
    if (!res.ok) {
      throw new Error(`Failed corrupting chunk: ${res.statusText}`);
    }
  };

  const deleteObject = async (key: string) => {
    const res = await fetch(`/api/objects/${encodeURIComponent(key)}`, {
      method: 'DELETE',
    });
    if (!res.ok) {
      throw new Error(`Failed deleting object: ${res.statusText}`);
    }
    setSelection({ type: 'none' });
    fetchSnapshot();
  };

  const uploadObject = async (file: File, key: string, scheme: 'replication' | 'erasure') => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('key', key || file.name);
    formData.append('scheme', scheme);

    const res = await fetch('/api/upload', {
      method: 'POST',
      body: formData,
    });
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
    status,
    nodes,
    objects,
    topology,
    events,
    connected,
    activeReplication,
    selection,
    setSelection,
    killNode,
    corruptChunk,
    deleteObject,
    uploadObject,
    downloadObject,
    refresh: fetchSnapshot,
  };
}
