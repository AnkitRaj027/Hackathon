// OperationsPanel Component
// Unified left-hand infrastructure operations panel.
// Combines Object Catalog, Topology Inspector, Data Distribution,
// Repair Queue, and Failure Analysis in a cohesive console dock.

import React from 'react';
import {
  ViewMode,
  StorageNode,
  ObjectDTO,
  TopologyDTO,
  MetricsPoint,
  RepairTask,
  SelectionState,
} from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';
import {
  Database,
  Layers,
  Package,
  Wrench,
  AlertTriangle,
  CheckCircle2,
  HardDrive,
  Activity,
  ArrowRight,
  LucideIcon,
} from 'lucide-react';

interface OperationsPanelProps {
  viewMode: ViewMode;
  onViewModeChange: (mode: ViewMode) => void;
  nodes: StorageNode[];
  objects: ObjectDTO[];
  topology: TopologyDTO | null;
  metrics: MetricsPoint[];
  repairQueue: RepairTask[];
  selection: SelectionState;
  onSelectNode: (id: string) => void;
  onSelectObject: (key: string) => void;
}

const TABS: { id: ViewMode; label: string; icon: LucideIcon }[] = [
  { id: 'OVERVIEW', label: 'CATALOG', icon: Database },
  { id: 'TOPOLOGY', label: 'RING', icon: Layers },
  { id: 'DATA', label: 'DATA', icon: Package },
  { id: 'REPAIRS', label: 'REPAIRS', icon: Wrench },
  { id: 'FAILURES', label: 'FAILURES', icon: AlertTriangle },
];

const Row: React.FC<{ label: string; value: React.ReactNode; accent?: string }> = ({
  label,
  value,
  accent,
}) => (
  <div
    style={{
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'center',
      padding: '4px 0',
      borderBottom: `1px solid ${BaseColors.bg}`,
    }}
  >
    <span style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted }}>{label}</span>
    <span
      style={{
        fontSize: FontSize.xxs,
        color: accent ?? BaseColors.textPrimary,
        fontVariantNumeric: 'tabular-nums',
      }}
    >
      {value}
    </span>
  </div>
);

export const OperationsPanel: React.FC<OperationsPanelProps> = ({
  viewMode,
  onViewModeChange,
  nodes,
  objects,
  topology,
  metrics,
  repairQueue,
  selection,
  onSelectNode,
  onSelectObject,
}) => {
  const activeRepairCount = repairQueue.filter(
    (t) => t.status === 'REPAIRING' || t.status === 'QUEUED'
  ).length;
  const failureCount = nodes.filter(
    (n) => n.status === 'DEAD' || n.status === 'SUSPECT' || n.status === 'DEGRADED' || n.status === 'PARTITIONED'
  ).length;

  const totalBytes = objects.reduce((s, o) => s + o.size, 0);
  const totalChunks = objects.reduce((s, o) => s + o.chunks.length, 0);
  const ecObjects = objects.filter((o) => o.scheme.includes('reed'));
  const latestMetric = metrics.length > 0 ? metrics[metrics.length - 1] : null;

  return (
    <div
      style={{
        position: 'absolute',
        top: '50px',
        left: '12px',
        width: '280px',
        bottom: '44px',
        background: BaseColors.surface,
        border: `1px solid ${BaseColors.border}`,
        borderRadius: '3px',
        display: 'flex',
        flexDirection: 'column',
        zIndex: 35,
        fontFamily: FontFamily.mono,
        overflow: 'hidden',
        boxShadow: '0 4px 20px rgba(0, 0, 0, 0.4)',
      }}
    >
      {/* ── Integrated Infrastructure Tab Bar ────────────────────────────── */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(5, 1fr)',
          borderBottom: `1px solid ${BaseColors.border}`,
          background: BaseColors.bg,
          flexShrink: 0,
        }}
      >
        {TABS.map((tab) => {
          const isActive = viewMode === tab.id;
          const Icon = tab.icon;
          const hasBadge =
            (tab.id === 'REPAIRS' && activeRepairCount > 0) ||
            (tab.id === 'FAILURES' && failureCount > 0);
          const badgeColor =
            tab.id === 'FAILURES' ? StateColors.DEAD : StateColors.REPAIRING;

          return (
            <button
              key={tab.id}
              onClick={() => onViewModeChange(tab.id)}
              title={`${tab.label} View`}
              style={{
                background: isActive ? BaseColors.surface : 'transparent',
                border: 'none',
                borderBottom: isActive ? `2px solid ${BaseColors.accent}` : '2px solid transparent',
                color: isActive ? BaseColors.textPrimary : BaseColors.textMuted,
                padding: '8px 2px',
                cursor: 'pointer',
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: '3px',
                fontSize: '9px',
                fontWeight: isActive ? FontWeight.semibold : FontWeight.regular,
                fontFamily: FontFamily.mono,
                letterSpacing: '0.04em',
                position: 'relative',
                transition: 'background 120ms ease, color 120ms ease',
              }}
            >
              <div style={{ position: 'relative' }}>
                <Icon size={12} color={isActive ? BaseColors.accent : BaseColors.textMuted} />
                {hasBadge && (
                  <span
                    style={{
                      position: 'absolute',
                      top: -2,
                      right: -3,
                      width: '5px',
                      height: '5px',
                      borderRadius: '50%',
                      background: badgeColor,
                    }}
                  />
                )}
              </div>
              <span>{tab.label}</span>
            </button>
          );
        })}
      </div>

      {/* ── Content View Area ────────────────────────────────────────────── */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '10px' }}>
        {/* ── 1. CATALOG VIEW ────────────────────────────────────────────── */}
        {viewMode === 'OVERVIEW' && (
          <div>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                alignItems: 'center',
                marginBottom: '8px',
              }}
            >
              <span
                style={{
                  fontSize: FontSize.xxs,
                  fontWeight: FontWeight.semibold,
                  color: BaseColors.textMuted,
                  letterSpacing: '0.06em',
                }}
              >
                OBJECT CATALOG ({objects.length})
              </span>
              <span style={{ fontSize: '9px', color: BaseColors.textMuted }}>
                {(totalBytes / 1024).toFixed(1)} KB TOTAL
              </span>
            </div>

            {objects.length === 0 ? (
              <div
                style={{
                  padding: '24px 8px',
                  fontSize: FontSize.xxs,
                  color: BaseColors.textMuted,
                  textAlign: 'center',
                  lineHeight: '1.6',
                }}
              >
                <Package size={24} color={BaseColors.textMuted} style={{ marginBottom: '8px' }} />
                <div>No objects stored.</div>
                <div style={{ marginTop: '4px', color: BaseColors.accent }}>
                  Click INGEST OBJECT above to store files.
                </div>
              </div>
            ) : (
              objects.map((obj) => {
                const isSelected =
                  selection.type === 'object' && selection.objectKey === obj.key;
                const isEC = obj.scheme.includes('reed');

                return (
                  <div
                    key={obj.key}
                    onClick={() => onSelectObject(obj.key)}
                    style={{
                      padding: '8px 10px',
                      marginBottom: '6px',
                      background: isSelected
                        ? `${BaseColors.accent}15`
                        : BaseColors.surfaceCard,
                      border: `1px solid ${isSelected ? BaseColors.accent : BaseColors.border}`,
                      borderRadius: '2px',
                      cursor: 'pointer',
                      transition: 'background 120ms ease, border-color 120ms ease',
                    }}
                  >
                    <div
                      style={{
                        fontSize: FontSize.xs,
                        fontWeight: FontWeight.semibold,
                        color: isSelected ? BaseColors.accent : BaseColors.textPrimary,
                        wordBreak: 'break-all',
                        marginBottom: '4px',
                      }}
                    >
                      {obj.key}
                    </div>
                    <div
                      style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        fontSize: FontSize.xxs,
                        color: BaseColors.textMuted,
                      }}
                    >
                      <span>{(obj.size / 1024).toFixed(1)} KB</span>
                      <span
                        style={{
                          color: isEC ? '#a78bfa' : StateColors.HEALTHY,
                          fontWeight: 500,
                        }}
                      >
                        {isEC ? 'RS 2+1' : `RF ${obj.chunks[0]?.replicas.length || 3}`}
                      </span>
                      <span>{obj.chunks.length} chunks</span>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        )}

        {/* ── 2. TOPOLOGY / RING VIEW ────────────────────────────────────── */}
        {viewMode === 'TOPOLOGY' && (
          <div>
            <div style={{ marginBottom: '12px' }}>
              <div
                style={{
                  fontSize: FontSize.xxs,
                  color: BaseColors.textMuted,
                  fontWeight: FontWeight.semibold,
                  marginBottom: '6px',
                  letterSpacing: '0.06em',
                }}
              >
                CONSISTENT HASH RING
              </div>
              <Row label="Architecture" value={topology?.ring_type ?? 'ConsistentHash-32b'} />
              <Row
                label="Virtual nodes / node"
                value={topology?.vnodes_per_node ?? 256}
                accent={StateColors.HEALTHY}
              />
              <Row label="Total ring points" value={topology?.total_points ?? 768} />
              <Row label="Physical nodes" value={nodes.length} />
            </div>

            <div style={{ marginBottom: '12px' }}>
              <div
                style={{
                  fontSize: FontSize.xxs,
                  color: BaseColors.textMuted,
                  fontWeight: FontWeight.semibold,
                  marginBottom: '6px',
                  letterSpacing: '0.06em',
                }}
              >
                ZONE / RACK TOPOLOGY
              </div>
              {[...new Set(nodes.map((n) => n.zone || 'us-east-1a'))].map((zone) => (
                <div
                  key={zone}
                  style={{
                    padding: '6px 8px',
                    background: BaseColors.bg,
                    border: `1px solid ${BaseColors.border}`,
                    borderRadius: '2px',
                    marginBottom: '4px',
                  }}
                >
                  <div
                    style={{
                      fontSize: FontSize.xxs,
                      color: BaseColors.accent,
                      fontWeight: FontWeight.semibold,
                      marginBottom: '2px',
                    }}
                  >
                    ZONE: {zone}
                  </div>
                  <div style={{ fontSize: '9px', color: BaseColors.textMuted, paddingLeft: '6px' }}>
                    rack-01: {nodes.map((n) => n.id).join(', ')}
                  </div>
                </div>
              ))}
            </div>

            <div>
              <div
                style={{
                  fontSize: FontSize.xxs,
                  color: BaseColors.textMuted,
                  fontWeight: FontWeight.semibold,
                  marginBottom: '6px',
                  letterSpacing: '0.06em',
                }}
              >
                PHYSICAL NODES ({nodes.length})
              </div>
              {nodes.map((n) => {
                const c =
                  StateColors[n.status as keyof typeof StateColors] ?? BaseColors.textMuted;
                const isSelected = selection.type === 'node' && selection.nodeId === n.id;

                return (
                  <div
                    key={n.id}
                    onClick={() => onSelectNode(n.id)}
                    style={{
                      padding: '6px 8px',
                      background: isSelected ? `${c}15` : BaseColors.bg,
                      border: `1px solid ${isSelected ? c : BaseColors.border}`,
                      borderRadius: '2px',
                      marginBottom: '4px',
                      cursor: 'pointer',
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <span
                        style={{
                          width: '6px',
                          height: '6px',
                          borderRadius: '50%',
                          background: c,
                          display: 'inline-block',
                        }}
                      />
                      <span
                        style={{
                          fontSize: FontSize.xxs,
                          color: isSelected ? BaseColors.textPrimary : BaseColors.textSecondary,
                          fontWeight: 600,
                        }}
                      >
                        {n.id}
                      </span>
                    </div>
                    <span style={{ fontSize: FontSize.xxs, color: StateColors.HEALTHY }}>
                      {n.rtt_ms.toFixed(1)} ms
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* ── 3. DATA DISTRIBUTION VIEW ──────────────────────────────────── */}
        {viewMode === 'DATA' && (
          <div>
            <div style={{ marginBottom: '12px' }}>
              <div
                style={{
                  fontSize: FontSize.xxs,
                  color: BaseColors.textMuted,
                  fontWeight: FontWeight.semibold,
                  marginBottom: '6px',
                  letterSpacing: '0.06em',
                }}
              >
                STORAGE SUMMARY
              </div>
              <Row label="Total objects" value={objects.length} accent={BaseColors.accent} />
              <Row label="Stored volume" value={`${(totalBytes / 1024).toFixed(1)} KB`} />
              <Row label="Total chunk shards" value={totalChunks} />
              <Row label="Replication ratio" value="RF 3 (100%)" />
              {latestMetric && (
                <>
                  <Row label="Live write" value={`${latestMetric.write_mbps.toFixed(2)} MB/s`} />
                  <Row label="Live read" value={`${latestMetric.read_mbps.toFixed(2)} MB/s`} />
                  <Row label="Avg I/O latency" value={`${latestMetric.avg_latency_ms.toFixed(1)} ms`} />
                </>
              )}
            </div>

            <div style={{ marginBottom: '12px' }}>
              <div
                style={{
                  fontSize: FontSize.xxs,
                  color: BaseColors.textMuted,
                  fontWeight: FontWeight.semibold,
                  marginBottom: '6px',
                  letterSpacing: '0.06em',
                }}
              >
                CHUNK DISTRIBUTION
              </div>
              {nodes.map((n) => {
                const count = objects.reduce(
                  (s, o) => s + o.chunks.filter((c) => c.replicas.includes(n.id)).length,
                  0
                );
                const pct = totalChunks > 0 ? (count / totalChunks) * 100 : 0;
                const c =
                  StateColors[n.status as keyof typeof StateColors] ?? BaseColors.textMuted;

                return (
                  <div key={n.id} style={{ marginBottom: '8px' }}>
                    <div
                      style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        fontSize: FontSize.xxs,
                        marginBottom: '3px',
                      }}
                    >
                      <span style={{ color: BaseColors.textPrimary }}>{n.id}</span>
                      <span style={{ color: BaseColors.textMuted }}>
                        {count} chunks ({pct.toFixed(0)}%)
                      </span>
                    </div>
                    <div
                      style={{
                        height: '4px',
                        background: BaseColors.bg,
                        borderRadius: '2px',
                        overflow: 'hidden',
                      }}
                    >
                      <div
                        style={{
                          height: '100%',
                          width: `${pct}%`,
                          background: c,
                          borderRadius: '2px',
                          transition: 'width 300ms ease',
                        }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* ── 4. REPAIR QUEUE VIEW ───────────────────────────────────────── */}
        {viewMode === 'REPAIRS' && (
          <div>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: '1fr 1fr',
                gap: '6px',
                marginBottom: '12px',
              }}
            >
              {[
                { label: 'ACTIVE', count: repairQueue.filter((t) => t.status === 'REPAIRING').length, color: StateColors.REPAIRING },
                { label: 'QUEUED', count: repairQueue.filter((t) => t.status === 'QUEUED').length, color: StateColors.STALE },
                { label: 'FAILED', count: repairQueue.filter((t) => t.status === 'FAILED').length, color: StateColors.DEAD },
                { label: 'DONE', count: repairQueue.filter((t) => t.status === 'COMPLETED').length, color: StateColors.HEALTHY },
              ].map((s) => (
                <div
                  key={s.label}
                  style={{
                    background: BaseColors.bg,
                    border: `1px solid ${BaseColors.border}`,
                    borderRadius: '2px',
                    padding: '6px 8px',
                    textAlign: 'center',
                  }}
                >
                  <div
                    style={{
                      fontSize: '18px',
                      fontWeight: FontWeight.bold,
                      color: s.count > 0 ? s.color : BaseColors.textMuted,
                      lineHeight: 1,
                    }}
                  >
                    {s.count}
                  </div>
                  <div style={{ fontSize: '9px', color: BaseColors.textMuted, marginTop: '2px' }}>
                    {s.label}
                  </div>
                </div>
              ))}
            </div>

            {repairQueue.length === 0 ? (
              <div
                style={{
                  padding: '24px 8px',
                  textAlign: 'center',
                  color: BaseColors.textMuted,
                  fontSize: FontSize.xxs,
                  lineHeight: '1.6',
                }}
              >
                <CheckCircle2 size={24} color={StateColors.HEALTHY} style={{ marginBottom: '8px' }} />
                <div style={{ color: BaseColors.textPrimary, fontWeight: 600 }}>Quorum Intact</div>
                <div>No active self-healing tasks.</div>
                <div style={{ marginTop: '6px', color: BaseColors.textMuted, fontSize: '9px' }}>
                  Select a node & click KILL NODE to test peer replica repair.
                </div>
              </div>
            ) : (
              repairQueue.map((t) => {
                const c =
                  t.status === 'REPAIRING'
                    ? StateColors.REPAIRING
                    : t.status === 'FAILED'
                    ? StateColors.DEAD
                    : StateColors.HEALTHY;

                return (
                  <div
                    key={t.id}
                    style={{
                      padding: '8px',
                      background: BaseColors.bg,
                      border: `1px solid ${BaseColors.border}`,
                      borderLeft: `3px solid ${c}`,
                      borderRadius: '2px',
                      marginBottom: '6px',
                    }}
                  >
                    <div
                      style={{
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                        marginBottom: '4px',
                      }}
                    >
                      <span
                        style={{
                          fontSize: FontSize.xxs,
                          color: BaseColors.textPrimary,
                          fontWeight: FontWeight.semibold,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                          maxWidth: '150px',
                        }}
                      >
                        {t.object_key}
                      </span>
                      <span
                        style={{
                          fontSize: '8px',
                          color: c,
                          background: `${c}20`,
                          border: `1px solid ${c}40`,
                          padding: '1px 4px',
                          borderRadius: '2px',
                        }}
                      >
                        {t.status}
                      </span>
                    </div>
                    <div style={{ fontSize: '9px', color: BaseColors.textMuted, display: 'flex', gap: '8px' }}>
                      <span>RF: {t.rf_current}/{t.rf_target}</span>
                      <span>SRC: {t.source}</span>
                      <span>TGT: {t.target}</span>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        )}

        {/* ── 5. FAILURES & DIAGNOSTICS VIEW ─────────────────────────────── */}
        {viewMode === 'FAILURES' && (
          <div>
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: '1fr 1fr',
                gap: '6px',
                marginBottom: '12px',
              }}
            >
              <div
                style={{
                  background: failureCount > 0 ? `${StateColors.DEAD}15` : BaseColors.bg,
                  border: `1px solid ${failureCount > 0 ? StateColors.DEAD : BaseColors.border}`,
                  borderRadius: '2px',
                  padding: '8px',
                  textAlign: 'center',
                }}
              >
                <div
                  style={{
                    fontSize: '22px',
                    fontWeight: FontWeight.bold,
                    color: failureCount > 0 ? StateColors.DEAD : BaseColors.textMuted,
                  }}
                >
                  {failureCount}
                </div>
                <div style={{ fontSize: '9px', color: BaseColors.textMuted }}>DEGRADED</div>
              </div>

              <div
                style={{
                  background: BaseColors.bg,
                  border: `1px solid ${StateColors.HEALTHY}40`,
                  borderRadius: '2px',
                  padding: '8px',
                  textAlign: 'center',
                }}
              >
                <div
                  style={{
                    fontSize: '22px',
                    fontWeight: FontWeight.bold,
                    color: StateColors.HEALTHY,
                  }}
                >
                  {nodes.length - failureCount}
                </div>
                <div style={{ fontSize: '9px', color: BaseColors.textMuted }}>HEALTHY</div>
              </div>
            </div>

            {failureCount === 0 ? (
              <div
                style={{
                  padding: '24px 8px',
                  textAlign: 'center',
                  color: BaseColors.textMuted,
                  fontSize: FontSize.xxs,
                  lineHeight: '1.6',
                }}
              >
                <CheckCircle2 size={24} color={StateColors.HEALTHY} style={{ marginBottom: '8px' }} />
                <div style={{ color: BaseColors.textPrimary, fontWeight: 600 }}>All Systems Nominal</div>
                <div>No node failures or network partitions detected.</div>
              </div>
            ) : (
              <div>
                <div
                  style={{
                    fontSize: FontSize.xxs,
                    color: StateColors.DEAD,
                    fontWeight: FontWeight.semibold,
                    marginBottom: '6px',
                    letterSpacing: '0.04em',
                  }}
                >
                  FAILED / DEGRADED ENTITIES
                </div>
                {nodes
                  .filter((n) => n.status !== 'HEALTHY')
                  .map((n) => {
                    const c =
                      StateColors[n.status as keyof typeof StateColors] ?? StateColors.DEAD;
                    return (
                      <div
                        key={n.id}
                        onClick={() => onSelectNode(n.id)}
                        style={{
                          padding: '8px',
                          background: `${c}10`,
                          border: `1px solid ${c}40`,
                          borderRadius: '2px',
                          marginBottom: '6px',
                          cursor: 'pointer',
                        }}
                      >
                        <div
                          style={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            marginBottom: '4px',
                          }}
                        >
                          <span style={{ fontSize: FontSize.xs, fontWeight: 600, color: BaseColors.textPrimary }}>
                            {n.id}
                          </span>
                          <span
                            style={{
                              fontSize: '9px',
                              color: c,
                              background: `${c}20`,
                              padding: '1px 5px',
                              borderRadius: '2px',
                            }}
                          >
                            {n.status}
                          </span>
                        </div>
                        <div style={{ fontSize: '9px', color: BaseColors.textMuted }}>
                          ADDR: {n.address}
                        </div>
                      </div>
                    );
                  })}
              </div>
            )}

            <div style={{ marginTop: '14px' }}>
              <div
                style={{
                  fontSize: FontSize.xxs,
                  color: BaseColors.textMuted,
                  fontWeight: FontWeight.semibold,
                  marginBottom: '6px',
                  letterSpacing: '0.04em',
                }}
              >
                ALL CLUSTER NODES
              </div>
              {nodes.map((n) => {
                const c =
                  StateColors[n.status as keyof typeof StateColors] ?? BaseColors.textMuted;
                return (
                  <div
                    key={n.id}
                    onClick={() => onSelectNode(n.id)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      padding: '4px 0',
                      borderBottom: `1px solid ${BaseColors.bg}`,
                      cursor: 'pointer',
                      fontSize: FontSize.xxs,
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <span
                        style={{
                          width: '5px',
                          height: '5px',
                          borderRadius: '50%',
                          background: c,
                        }}
                      />
                      <span style={{ color: BaseColors.textPrimary }}>{n.id}</span>
                    </div>
                    <span style={{ color: c, fontSize: '9px' }}>{n.status}</span>
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
