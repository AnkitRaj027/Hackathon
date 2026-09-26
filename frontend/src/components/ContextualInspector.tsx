// ContextualInspector Component
// Contextual inspection panel for selected node, object, or chunk.
// Only displays real backend data — N/A for missing fields.

import React from 'react';
import { StorageNode, ObjectDTO, SelectionState } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';
import {
  X, Server, Package, Download, Trash2, AlertTriangle,
  FileCode, HardDrive, CheckCircle2, XCircle,
} from 'lucide-react';

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

// ─── Shared primitives ────────────────────────────────────────────────────

const Row: React.FC<{ label: string; value: React.ReactNode; accent?: string }> = ({
  label, value, accent,
}) => (
  <>
    <span style={{ color: BaseColors.textMuted }}>{label}</span>
    <span style={{ color: accent ?? BaseColors.textPrimary, wordBreak: 'break-all' }}>
      {value}
    </span>
  </>
);

const Section: React.FC<{ title: string; children: React.ReactNode }> = ({ title, children }) => (
  <div>
    <div
      style={{
        fontSize: FontSize.xxs,
        fontWeight: FontWeight.semibold,
        color: BaseColors.textMuted,
        letterSpacing: '0.06em',
        marginBottom: '6px',
      }}
    >
      {title}
    </div>
    {children}
  </div>
);

const MetricGrid: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div
    style={{
      background: BaseColors.bg,
      border: `1px solid ${BaseColors.border}`,
      borderRadius: '2px',
      padding: '8px 10px',
      display: 'grid',
      gridTemplateColumns: '100px 1fr',
      gap: '5px',
      fontSize: FontSize.xs,
    }}
  >
    {children}
  </div>
);

// ─── Main Component ────────────────────────────────────────────────────────

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
  if (selection.type === 'none') return null;

  const selectedNode = selection.type === 'node'
    ? nodes.find((n) => n.id === selection.nodeId)
    : null;
  const selectedObject = selection.type === 'object'
    ? objects.find((o) => o.key === selection.objectKey)
    : null;

  const statusColor = selectedNode
    ? (StateColors[selectedNode.status as keyof typeof StateColors] ?? BaseColors.textMuted)
    : BaseColors.textMuted;

  return (
    <aside
      style={{
        position: 'absolute',
        top: '54px',
        right: '12px',
        bottom: '44px',
        width: '400px',
        background: BaseColors.surface,
        border: `1px solid ${BaseColors.border}`,
        borderRadius: '2px',
        display: 'flex',
        flexDirection: 'column',
        zIndex: 50,
        fontFamily: FontFamily.mono,
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
          padding: '10px 14px',
          borderBottom: `1px solid ${BaseColors.border}`,
          background: BaseColors.bg,
          flexShrink: 0,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          {selection.type === 'node' && <Server size={13} color={StateColors.HEALTHY} />}
          {selection.type === 'object' && <Package size={13} color={StateColors.HEALTHY} />}
          <span
            style={{
              fontSize: FontSize.xs,
              fontWeight: FontWeight.semibold,
              letterSpacing: '0.04em',
              color: BaseColors.textSecondary,
            }}
          >
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
            padding: '3px',
            display: 'flex',
          }}
        >
          <X size={13} />
        </button>
      </div>

      {/* Body */}
      <div
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: '14px',
          display: 'flex',
          flexDirection: 'column',
          gap: '14px',
          fontSize: FontSize.sm,
        }}
      >
        {/* ── NODE ──────────────────────────────────────────────────── */}
        {selectedNode && (
          <>
            {/* Node ID + status badge */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
              <span
                style={{
                  fontSize: FontSize.xl,
                  fontWeight: FontWeight.bold,
                  color: BaseColors.textPrimary,
                }}
              >
                {selectedNode.id.toUpperCase()}
              </span>
              <span
                style={{
                  fontSize: FontSize.xxs,
                  fontWeight: FontWeight.semibold,
                  padding: '2px 8px',
                  borderRadius: '2px',
                  background: `${statusColor}20`,
                  color: statusColor,
                  border: `1px solid ${statusColor}50`,
                  letterSpacing: '0.04em',
                }}
              >
                {selectedNode.status}
              </span>
            </div>

            {/* Core metrics */}
            <Section title="TELEMETRY">
              <MetricGrid>
                <Row label="Address" value={selectedNode.address} />
                <Row
                  label="RTT Latency"
                  value={`${selectedNode.rtt_ms.toFixed(2)} ms`}
                  accent={StateColors.HEALTHY}
                />
                <Row label="Role" value={selectedNode.role} />
                <Row label="Zone" value={selectedNode.zone || 'N/A'} />
                <Row label="Rack" value={selectedNode.rack || 'N/A'} />
                <Row
                  label="Partition"
                  value={selectedNode.is_partitioned ? 'YES' : 'NO'}
                  accent={selectedNode.is_partitioned ? StateColors.DEAD : BaseColors.textSecondary}
                />
              </MetricGrid>
            </Section>

            {/* Drive bay telemetry — real from backend */}
            {selectedNode.drives && selectedNode.drives.length > 0 && (
              <Section title="DRIVE BAYS (12-BAY 2U CHASSIS)">
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: 'repeat(4, 1fr)',
                    gap: '3px',
                  }}
                >
                  {selectedNode.drives.map((d) => {
                    const dColor =
                      d.status === 'FAILED'
                        ? StateColors.DEAD
                        : d.status === 'WARNING'
                        ? StateColors.DEGRADED
                        : StateColors.HEALTHY;
                    return (
                      <div
                        key={d.bay_index}
                        title={`Bay ${d.bay_index}: ${d.status} | ${d.chunks_count} chunks | ${d.temperature_c}°C`}
                        style={{
                          background: BaseColors.bg,
                          border: `1px solid ${dColor}40`,
                          borderRadius: '2px',
                          padding: '4px',
                          display: 'flex',
                          flexDirection: 'column',
                          alignItems: 'center',
                          gap: '2px',
                        }}
                      >
                        <HardDrive size={10} color={dColor} />
                        <span style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted }}>
                          B{String(d.bay_index).padStart(2, '0')}
                        </span>
                        <span style={{ fontSize: FontSize.xxs, color: dColor }}>
                          {d.chunks_count}c
                        </span>
                      </div>
                    );
                  })}
                </div>
              </Section>
            )}

            {/* Objects on this node */}
            <Section title="REPLICATED OBJECTS">
              <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
                {objects
                  .filter((o) => o.chunks.some((c) => c.replicas.includes(selectedNode.id)))
                  .map((obj) => (
                    <div
                      key={obj.key}
                      onClick={() => onSelectObject(obj.key)}
                      style={{
                        padding: '5px 8px',
                        background: BaseColors.bg,
                        border: `1px solid ${BaseColors.border}`,
                        borderRadius: '2px',
                        cursor: 'pointer',
                        display: 'flex',
                        justifyContent: 'space-between',
                        transition: 'border-color 150ms ease',
                      }}
                      onMouseEnter={(e) =>
                        ((e.currentTarget as HTMLDivElement).style.borderColor = BaseColors.borderHover)
                      }
                      onMouseLeave={(e) =>
                        ((e.currentTarget as HTMLDivElement).style.borderColor = BaseColors.border)
                      }
                    >
                      <span style={{ color: StateColors.HEALTHY }}>{obj.key}</span>
                      <span style={{ color: BaseColors.textMuted }}>
                        {(obj.size / 1024).toFixed(1)} KB
                      </span>
                    </div>
                  ))}
                {objects.filter((o) =>
                  o.chunks.some((c) => c.replicas.includes(selectedNode.id))
                ).length === 0 && (
                  <span style={{ color: BaseColors.textMuted }}>No objects on this node</span>
                )}
              </div>
            </Section>

            {/* Chaos controls — always call real backend */}
            <Section title="CHAOS ADMINISTRATION">
              <div
                style={{
                  borderTop: `1px solid ${StateColors.DEAD}30`,
                  paddingTop: '10px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '6px',
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '4px',
                    color: StateColors.DEAD,
                    fontSize: FontSize.xxs,
                    marginBottom: '4px',
                  }}
                >
                  <AlertTriangle size={11} />
                  <span>DESTRUCTIVE OPERATIONS — REAL BACKEND ACTIONS</span>
                </div>
                <button
                  disabled={selectedNode.status === 'DEAD'}
                  onClick={() => onTriggerKillNode(selectedNode.id)}
                  style={{
                    width: '100%',
                    padding: '7px',
                    background:
                      selectedNode.status === 'DEAD' ? BaseColors.surfaceElevated : '#7f1d1d',
                    border: `1px solid ${selectedNode.status === 'DEAD' ? BaseColors.border : StateColors.DEAD}`,
                    color:
                      selectedNode.status === 'DEAD' ? BaseColors.textMuted : BaseColors.textPrimary,
                    borderRadius: '2px',
                    cursor: selectedNode.status === 'DEAD' ? 'not-allowed' : 'pointer',
                    fontWeight: FontWeight.semibold,
                    fontSize: FontSize.xs,
                    fontFamily: FontFamily.mono,
                    letterSpacing: '0.04em',
                    transition: 'background 150ms ease',
                  }}
                >
                  {selectedNode.status === 'DEAD'
                    ? 'NODE OFFLINE (TERMINATED)'
                    : `KILL NODE (${selectedNode.id})`}
                </button>
              </div>
            </Section>
          </>
        )}

        {/* ── OBJECT ────────────────────────────────────────────────── */}
        {selectedObject && (
          <>
            <div>
              <span
                style={{
                  fontSize: FontSize.lg,
                  fontWeight: FontWeight.bold,
                  color: BaseColors.textPrimary,
                  wordBreak: 'break-all',
                  display: 'block',
                  marginBottom: '6px',
                }}
              >
                {selectedObject.key}
              </span>
            </div>

            <Section title="OBJECT METADATA">
              <MetricGrid>
                <Row
                  label="Size"
                  value={`${selectedObject.size.toLocaleString()} bytes (${(selectedObject.size / 1024).toFixed(1)} KB)`}
                />
                <Row
                  label="Scheme"
                  value={selectedObject.scheme}
                  accent={selectedObject.scheme.includes('reed') ? '#a78bfa' : StateColors.HEALTHY}
                />
                <Row label="Chunks" value={String(selectedObject.chunks.length)} />
                <Row label="Created" value={new Date(selectedObject.created_at).toLocaleString()} />
                <Row
                  label="Checksum"
                  value={
                    selectedObject.checksum
                      ? `${selectedObject.checksum.substring(0, 24)}…`
                      : 'N/A'
                  }
                  accent={StateColors.HEALTHY}
                />
              </MetricGrid>
            </Section>

            {/* Chunk breakdown */}
            <Section title="PHYSICAL CHUNKS & REPLICAS">
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                {selectedObject.chunks.map((chk) => {
                  const chkColor = chk.is_parity ? '#a78bfa' : StateColors.HEALTHY;
                  return (
                    <div
                      key={chk.chunk_id}
                      style={{
                        background: BaseColors.bg,
                        border: `1px solid ${BaseColors.border}`,
                        borderRadius: '2px',
                        padding: '8px',
                        fontSize: FontSize.xs,
                      }}
                    >
                      {/* Chunk header */}
                      <div
                        style={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          marginBottom: '6px',
                          alignItems: 'center',
                        }}
                      >
                        <span
                          style={{
                            color: chkColor,
                            fontWeight: FontWeight.semibold,
                            fontSize: FontSize.xxs,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                            maxWidth: '200px',
                          }}
                          title={chk.chunk_id}
                        >
                          {chk.chunk_id}
                        </span>
                        <button
                          onClick={() => onTriggerCorruptChunk(chk.chunk_id)}
                          title="Inject physical bit-rot into disk payload"
                          style={{
                            background: 'transparent',
                            border: 'none',
                            color: StateColors.DEAD,
                            cursor: 'pointer',
                            fontSize: FontSize.xxs,
                            display: 'flex',
                            alignItems: 'center',
                            gap: '2px',
                            fontFamily: FontFamily.mono,
                          }}
                        >
                          <FileCode size={10} />
                          <span>CORRUPT</span>
                        </button>
                      </div>

                      {/* Chunk meta */}
                      <div
                        style={{
                          display: 'grid',
                          gridTemplateColumns: '60px 1fr',
                          gap: '3px',
                          fontSize: FontSize.xxs,
                          color: BaseColors.textMuted,
                          marginBottom: '6px',
                        }}
                      >
                        <span>Index</span>
                        <span style={{ color: BaseColors.textSecondary }}>{chk.index}</span>
                        <span>Size</span>
                        <span style={{ color: BaseColors.textSecondary }}>
                          {(chk.size / 1024).toFixed(1)} KB
                        </span>
                        <span>SHA-256</span>
                        <span
                          style={{ color: BaseColors.textSecondary, wordBreak: 'break-all' }}
                        >
                          {chk.sha256 ? `${chk.sha256.substring(0, 20)}…` : 'N/A'}
                        </span>
                        {chk.is_parity && (
                          <>
                            <span>Type</span>
                            <span style={{ color: '#a78bfa' }}>PARITY</span>
                          </>
                        )}
                      </div>

                      {/* Replica placement */}
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '3px' }}>
                        {chk.replicas.map((r) => (
                          <div
                            key={r}
                            style={{
                              display: 'flex',
                              alignItems: 'center',
                              gap: '6px',
                              fontSize: FontSize.xxs,
                            }}
                          >
                            <CheckCircle2 size={10} color={StateColors.HEALTHY} />
                            <span style={{ color: BaseColors.textSecondary }}>{r}</span>
                          </div>
                        ))}
                        {chk.replicas.length === 0 && (
                          <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                            <XCircle size={10} color={StateColors.DEAD} />
                            <span style={{ color: StateColors.DEAD }}>NO REPLICAS</span>
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </Section>

            {/* Actions */}
            <div style={{ display: 'flex', gap: '8px', marginTop: '4px' }}>
              <button
                onClick={() => onDownloadObject(selectedObject.key)}
                style={{
                  flex: 1,
                  padding: '7px',
                  background: '#0369a1',
                  border: 'none',
                  color: BaseColors.textPrimary,
                  borderRadius: '2px',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: '6px',
                  fontWeight: FontWeight.semibold,
                  fontSize: FontSize.xs,
                  fontFamily: FontFamily.mono,
                }}
              >
                <Download size={12} />
                <span>DOWNLOAD</span>
              </button>

              <button
                onClick={() => onDeleteObject(selectedObject.key)}
                style={{
                  padding: '7px 12px',
                  background: '#7f1d1d',
                  border: 'none',
                  color: BaseColors.textPrimary,
                  borderRadius: '2px',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  fontSize: FontSize.xs,
                  fontFamily: FontFamily.mono,
                }}
              >
                <Trash2 size={12} />
                <span>DELETE</span>
              </button>
            </div>
          </>
        )}
      </div>
    </aside>
  );
};
