// ContextualInspector Component
// Section 64, 65 & 66: Contextual inspection panel for selected node, object, or chunk.

import React from 'react';
import { StorageNode, ObjectDTO, SelectionState } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { X, Server, Package, Download, Trash2, AlertTriangle, FileCode } from 'lucide-react';

interface ContextualInspectorProps {
  selection: SelectionState;
  nodes: StorageNode[];
  objects: ObjectDTO[];
  onClose: () => void;
  onSelectObject: (key: string) => void;
  onDownloadObject: (key: string) => void;
  onDeleteObject: (key: string) => void;
  onTriggerKillNode: (nodeId: string) => void;
  onTriggerCorruptChunk: (chunkId: string) => void;
}

export const ContextualInspector: React.FC<ContextualInspectorProps> = ({
  selection,
  nodes,
  objects,
  onClose,
  onSelectObject,
  onDownloadObject,
  onDeleteObject,
  onTriggerKillNode,
  onTriggerCorruptChunk,
}) => {
  if (selection.type === 'none') {
    return null;
  }

  const selectedNode = selection.type === 'node' ? nodes.find((n) => n.id === selection.nodeId) : null;
  const selectedObject = selection.type === 'object' ? objects.find((o) => o.key === selection.objectKey) : null;

  return (
    <aside
      style={{
        position: 'absolute',
        top: '56px',
        right: '12px',
        bottom: '50px',
        width: '380px',
        background: 'rgba(13, 19, 31, 0.94)',
        backdropFilter: 'blur(12px)',
        border: `1px solid ${BaseColors.border}`,
        borderRadius: '4px',
        display: 'flex',
        flexDirection: 'column',
        zIndex: 50,
        boxShadow: '0 8px 32px rgba(0,0,0,0.5)',
        fontFamily: '"IBM Plex Mono", monospace',
        color: BaseColors.textPrimary,
        overflow: 'hidden',
      }}
    >
      {/* Header */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '12px 14px',
          borderBottom: `1px solid ${BaseColors.border}`,
          background: 'rgba(10, 15, 26, 0.6)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          {selection.type === 'node' && <Server size={15} color="#38bdf8" />}
          {selection.type === 'object' && <Package size={15} color="#38bdf8" />}
          <span style={{ fontSize: '12px', fontWeight: 600, letterSpacing: '0.04em' }}>
            {selection.type === 'node' ? 'NODE INSPECTOR' : 'OBJECT INSPECTOR'}
          </span>
        </div>
        <button
          onClick={onClose}
          style={{
            background: 'transparent',
            border: 'none',
            color: BaseColors.textMuted,
            cursor: 'pointer',
            padding: '4px',
            display: 'flex',
          }}
        >
          <X size={15} />
        </button>
      </div>

      {/* Body Content */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '14px', fontSize: '12px' }}>
        {/* Node Inspection Details */}
        {selectedNode && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: '14px', fontWeight: 700, color: '#f8fafc' }}>
                {selectedNode.id}
              </span>
              <span
                style={{
                  fontSize: '11px',
                  fontWeight: 600,
                  padding: '2px 8px',
                  borderRadius: '3px',
                  background: `${StateColors[selectedNode.status]}20`,
                  color: StateColors[selectedNode.status],
                  border: `1px solid ${StateColors[selectedNode.status]}50`,
                }}
              >
                {selectedNode.status}
              </span>
            </div>

            <div style={{ background: '#0a0f1a', border: `1px solid ${BaseColors.border}`, borderRadius: '3px', padding: '10px' }}>
              <div style={{ display: 'grid', gridTemplateColumns: '90px 1fr', gap: '6px', fontSize: '11px' }}>
                <span style={{ color: BaseColors.textMuted }}>Address:</span>
                <span style={{ color: BaseColors.textPrimary }}>{selectedNode.address}</span>

                <span style={{ color: BaseColors.textMuted }}>RTT Latency:</span>
                <span style={{ color: '#38bdf8', fontVariantNumeric: 'tabular-nums' }}>
                  {selectedNode.rtt_ms.toFixed(2)} ms
                </span>

                <span style={{ color: BaseColors.textMuted }}>Role:</span>
                <span>{selectedNode.role}</span>

                <span style={{ color: BaseColors.textMuted }}>Zone / Rack:</span>
                <span>{selectedNode.zone} / {selectedNode.rack}</span>
              </div>
            </div>

            {/* Hosted Objects breakdown on this node */}
            <div>
              <div style={{ fontSize: '11px', fontWeight: 600, color: BaseColors.textMuted, marginBottom: '6px' }}>
                REPLICATED OBJECTS ON THIS NODE
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                {objects
                  .filter((o) => o.chunks.some((c) => c.replicas.includes(selectedNode.id)))
                  .map((obj) => (
                    <div
                      key={obj.key}
                      onClick={() => onSelectObject(obj.key)}
                      style={{
                        padding: '6px 8px',
                        background: '#090d16',
                        border: `1px solid ${BaseColors.border}`,
                        borderRadius: '2px',
                        cursor: 'pointer',
                        display: 'flex',
                        justifyContent: 'space-between',
                      }}
                    >
                      <span style={{ color: '#38bdf8' }}>{obj.key}</span>
                      <span style={{ color: BaseColors.textMuted }}>
                        {(obj.size / (1024 * 1024)).toFixed(2)} MB
                      </span>
                    </div>
                  ))}
              </div>
            </div>

            {/* Chaos Administration Controls */}
            <div style={{ marginTop: '16px', paddingTop: '12px', borderTop: `1px solid ${BaseColors.border}` }}>
              <div style={{ fontSize: '11px', fontWeight: 600, color: '#f87171', marginBottom: '8px', display: 'flex', alignItems: 'center', gap: '4px' }}>
                <AlertTriangle size={13} />
                <span>CHAOS ADMINISTRATION</span>
              </div>
              <button
                disabled={selectedNode.status === 'DEAD'}
                onClick={() => onTriggerKillNode(selectedNode.id)}
                style={{
                  width: '100%',
                  padding: '7px',
                  background: selectedNode.status === 'DEAD' ? '#1e293b' : '#991b1b',
                  border: 'none',
                  color: '#ffffff',
                  borderRadius: '3px',
                  cursor: selectedNode.status === 'DEAD' ? 'not-allowed' : 'pointer',
                  fontWeight: 600,
                  fontSize: '11px',
                }}
              >
                {selectedNode.status === 'DEAD' ? 'NODE OFFLINE (TERMINATED)' : `KILL NODE (${selectedNode.id})`}
              </button>
            </div>
          </div>
        )}

        {/* Object Inspection Details */}
        {selectedObject && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span style={{ fontSize: '13px', fontWeight: 700, color: '#f8fafc', wordBreak: 'break-all' }}>
                {selectedObject.key}
              </span>
            </div>

            <div style={{ background: '#0a0f1a', border: `1px solid ${BaseColors.border}`, borderRadius: '3px', padding: '10px' }}>
              <div style={{ display: 'grid', gridTemplateColumns: '80px 1fr', gap: '6px', fontSize: '11px' }}>
                <span style={{ color: BaseColors.textMuted }}>Size:</span>
                <span style={{ fontVariantNumeric: 'tabular-nums' }}>
                  {selectedObject.size} bytes ({(selectedObject.size / (1024 * 1024)).toFixed(2)} MiB)
                </span>

                <span style={{ color: BaseColors.textMuted }}>Scheme:</span>
                <span style={{ color: selectedObject.scheme.includes('reed-solomon') ? '#a855f7' : '#38bdf8' }}>
                  {selectedObject.scheme}
                </span>

                <span style={{ color: BaseColors.textMuted }}>Chunks:</span>
                <span>{selectedObject.chunks.length} total</span>

                <span style={{ color: BaseColors.textMuted }}>Checksum:</span>
                <span style={{ color: '#38bdf8', fontSize: '10px', wordBreak: 'break-all' }}>
                  {selectedObject.checksum || 'N/A'}
                </span>
              </div>
            </div>

            {/* Chunk Breakdown with Replica Placement & Corrupt Action */}
            <div>
              <div style={{ fontSize: '11px', fontWeight: 600, color: BaseColors.textMuted, marginBottom: '6px' }}>
                PHYSICAL CHUNKS & REPLICAS
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                {selectedObject.chunks.map((chk) => (
                  <div
                    key={chk.chunk_id}
                    style={{
                      background: '#090d16',
                      border: `1px solid ${BaseColors.border}`,
                      borderRadius: '3px',
                      padding: '8px',
                      fontSize: '11px',
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                      <span style={{ color: chk.is_parity ? '#a855f7' : '#38bdf8', fontWeight: 600 }}>
                        {chk.chunk_id}
                      </span>
                      <button
                        onClick={() => onTriggerCorruptChunk(chk.chunk_id)}
                        title="Inject physical bit-rot into disk payload"
                        style={{
                          background: 'transparent',
                          border: 'none',
                          color: '#f87171',
                          cursor: 'pointer',
                          fontSize: '10px',
                          display: 'flex',
                          alignItems: 'center',
                          gap: '2px',
                        }}
                      >
                        <FileCode size={11} />
                        <span>CORRUPT</span>
                      </button>
                    </div>

                    <div style={{ color: BaseColors.textMuted, fontSize: '10px', marginBottom: '3px' }}>
                      SHA: {chk.sha256.substring(0, 16)}...
                    </div>

                    <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                      {chk.replicas.map((r) => (
                        <span
                          key={r}
                          style={{
                            background: '#0f172a',
                            border: '1px solid #1e293b',
                            padding: '1px 5px',
                            borderRadius: '2px',
                            fontSize: '10px',
                            color: BaseColors.textSecondary,
                          }}
                        >
                          {r}
                        </span>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            {/* Actions */}
            <div style={{ display: 'flex', gap: '8px', marginTop: '12px' }}>
              <button
                onClick={() => onDownloadObject(selectedObject.key)}
                style={{
                  flex: 1,
                  padding: '7px',
                  background: '#0284c7',
                  border: 'none',
                  color: '#ffffff',
                  borderRadius: '3px',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: '6px',
                  fontWeight: 600,
                  fontSize: '11px',
                }}
              >
                <Download size={13} />
                <span>DOWNLOAD</span>
              </button>

              <button
                onClick={() => onDeleteObject(selectedObject.key)}
                style={{
                  padding: '7px 12px',
                  background: '#991b1b',
                  border: 'none',
                  color: '#ffffff',
                  borderRadius: '3px',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  fontSize: '11px',
                }}
              >
                <Trash2 size={13} />
                <span>DELETE</span>
              </button>
            </div>
          </div>
        )}
      </div>
    </aside>
  );
};
