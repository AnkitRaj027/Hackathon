// ViewModePanel Component
// Each view mode renders a distinct info panel.
// All data comes from real backend state.

import React from 'react';
import { ViewMode, StorageNode, ObjectDTO, TopologyDTO, MetricsPoint, RepairTask } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';
import {
  Server, Package, Activity, Wrench, AlertTriangle,
  HardDrive, Layers, Clock, CheckCircle2, XCircle,
} from 'lucide-react';

interface ViewModePanelProps {
  viewMode: ViewMode;
  nodes: StorageNode[];
  objects: ObjectDTO[];
  topology: TopologyDTO | null;
  metrics: MetricsPoint[];
  repairQueue: RepairTask[];
  onSelectNode: (id: string) => void;
  onSelectObject: (key: string) => void;
}

// ─── Shared panel shell ───────────────────────────────────────────────────
const Panel: React.FC<{ title: string; icon: React.ReactNode; children: React.ReactNode }> = ({
  title, icon, children,
}) => (
  <div
    style={{
      position: 'absolute',
      top: '96px',
      left: '12px',
      width: '260px',
      bottom: '40px',
      background: BaseColors.surface,
      border: `1px solid ${BaseColors.border}`,
      borderRadius: '3px',
      display: 'flex',
      flexDirection: 'column',
      zIndex: 35,
      fontFamily: FontFamily.mono,
      overflow: 'hidden',
    }}
  >
    <div
      style={{
        padding: '8px 12px',
        borderBottom: `1px solid ${BaseColors.border}`,
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        background: BaseColors.bg,
        flexShrink: 0,
      }}
    >
      {icon}
      <span style={{ fontSize: FontSize.xs, fontWeight: FontWeight.semibold, color: BaseColors.textSecondary, letterSpacing: '0.04em' }}>
        {title}
      </span>
    </div>
    <div style={{ flex: 1, overflowY: 'auto', padding: '10px' }}>
      {children}
    </div>
  </div>
);

const Row: React.FC<{ label: string; value: React.ReactNode; accent?: string }> = ({ label, value, accent }) => (
  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '4px 0', borderBottom: `1px solid ${BaseColors.bg}` }}>
    <span style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted }}>{label}</span>
    <span style={{ fontSize: FontSize.xxs, color: accent ?? BaseColors.textPrimary, fontVariantNumeric: 'tabular-nums' }}>{value}</span>
  </div>
);

// ─── TOPOLOGY panel ───────────────────────────────────────────────────────
const TopologyPanel: React.FC<{ nodes: StorageNode[]; topology: TopologyDTO | null; onSelectNode: (id: string) => void }> = ({
  nodes, topology, onSelectNode,
}) => {
  const zones = [...new Set(nodes.map(n => n.zone || 'default'))];
  const racks = [...new Set(nodes.map(n => n.rack || 'rack-01'))];

  return (
    <Panel title="TOPOLOGY" icon={<Layers size={13} color={StateColors.HEALTHY} />}>
      {/* Ring info */}
      <div style={{ marginBottom: '12px' }}>
        <div style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted, fontWeight: FontWeight.semibold, marginBottom: '6px', letterSpacing: '0.06em' }}>CONSISTENT HASH RING</div>
        <Row label="Ring type" value={topology?.ring_type ?? 'consistent_hash'} />
        <Row label="Virtual nodes / node" value={topology?.vnodes_per_node ?? 128} accent={StateColors.HEALTHY} />
        <Row label="Total ring points" value={topology?.total_points ?? '—'} />
        <Row label="Physical nodes" value={nodes.length} />
      </div>

      {/* Zone breakdown */}
      <div style={{ marginBottom: '12px' }}>
        <div style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted, fontWeight: FontWeight.semibold, marginBottom: '6px', letterSpacing: '0.06em' }}>ZONE / RACK PLACEMENT</div>
        {zones.map(z => {
          const zNodes = nodes.filter(n => (n.zone || 'default') === z);
          return (
            <div key={z} style={{ padding: '5px 8px', background: BaseColors.bg, border: `1px solid ${BaseColors.border}`, borderRadius: '2px', marginBottom: '4px' }}>
              <div style={{ fontSize: FontSize.xxs, color: StateColors.HEALTHY, fontWeight: FontWeight.semibold, marginBottom: '3px' }}>{z}</div>
              {racks.filter(r => zNodes.some(n => (n.rack || 'rack-01') === r)).map(r => (
                <div key={r} style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted, paddingLeft: '8px' }}>
                  {r}: {zNodes.filter(n => (n.rack || 'rack-01') === r).map(n => n.id).join(', ')}
                </div>
              ))}
            </div>
          );
        })}
      </div>

      {/* Node list */}
      <div>
        <div style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted, fontWeight: FontWeight.semibold, marginBottom: '6px', letterSpacing: '0.06em' }}>NODES</div>
        {nodes.map(n => {
          const c = StateColors[n.status as keyof typeof StateColors] ?? BaseColors.textMuted;
          return (
            <div
              key={n.id}
              onClick={() => onSelectNode(n.id)}
              style={{ padding: '5px 8px', background: BaseColors.bg, border: `1px solid ${BaseColors.border}`, borderRadius: '2px', marginBottom: '3px', cursor: 'pointer', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                <span style={{ width: '5px', height: '5px', borderRadius: '50%', background: c, display: 'inline-block' }} />
                <span style={{ fontSize: FontSize.xxs, color: BaseColors.textPrimary }}>{n.id}</span>
              </div>
              <span style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted }}>{n.rtt_ms.toFixed(1)}ms</span>
            </div>
          );
        })}
      </div>
    </Panel>
  );
};

// ─── DATA panel ───────────────────────────────────────────────────────────
const DataPanel: React.FC<{ nodes: StorageNode[]; objects: ObjectDTO[]; metrics: MetricsPoint[]; onSelectObject: (k: string) => void }> = ({
  nodes, objects, metrics, onSelectObject,
}) => {
  const totalBytes = objects.reduce((s, o) => s + o.size, 0);
  const totalChunks = objects.reduce((s, o) => s + o.chunks.length, 0);
  const ecObjects = objects.filter(o => o.scheme.includes('reed'));
  const latest = metrics[metrics.length - 1];

  return (
    <Panel title="DATA DISTRIBUTION" icon={<Package size={13} color={StateColors.HEALTHY} />}>
      {/* Cluster summary */}
      <div style={{ marginBottom: '12px' }}>
        <div style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted, fontWeight: FontWeight.semibold, marginBottom: '6px', letterSpacing: '0.06em' }}>CLUSTER SUMMARY</div>
        <Row label="Total objects" value={objects.length} accent={StateColors.HEALTHY} />
        <Row label="Total bytes" value={`${(totalBytes / 1024).toFixed(1)} KB`} />
        <Row label="Total chunks" value={totalChunks} />
        <Row label="EC objects" value={ecObjects.length} accent={ecObjects.length > 0 ? '#a78bfa' : BaseColors.textMuted} />
        <Row label="RF objects" value={objects.length - ecObjects.length} />
        {latest && (
          <>
            <Row label="Write" value={`${latest.write_mbps.toFixed(2)} MB/s`} accent={StateColors.HEALTHY} />
            <Row label="Read" value={`${latest.read_mbps.toFixed(2)} MB/s`} accent={StateColors.HEALTHY} />
            <Row label="Latency" value={`${latest.avg_latency_ms.toFixed(1)} ms`} accent={latest.avg_latency_ms > 50 ? StateColors.SUSPECT : BaseColors.textSecondary} />
          </>
        )}
      </div>

      {/* Per-node object counts */}
      <div style={{ marginBottom: '12px' }}>
        <div style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted, fontWeight: FontWeight.semibold, marginBottom: '6px', letterSpacing: '0.06em' }}>CHUNK DISTRIBUTION</div>
        {nodes.map(n => {
          const count = objects.reduce((s, o) => s + o.chunks.filter(c => c.replicas.includes(n.id)).length, 0);
          const pct = totalChunks > 0 ? (count / totalChunks) * 100 : 0;
          const c = StateColors[n.status as keyof typeof StateColors] ?? BaseColors.textMuted;
          return (
            <div key={n.id} style={{ marginBottom: '6px' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: FontSize.xxs, marginBottom: '2px' }}>
                <span style={{ color: BaseColors.textSecondary }}>{n.id}</span>
                <span style={{ color: BaseColors.textMuted }}>{count} chunks ({pct.toFixed(0)}%)</span>
              </div>
              <div style={{ height: '3px', background: BaseColors.bg, borderRadius: '2px', overflow: 'hidden' }}>
                <div style={{ height: '100%', width: `${pct}%`, background: c, borderRadius: '2px', transition: 'width 400ms ease' }} />
              </div>
            </div>
          );
        })}
      </div>

      {/* Object list */}
      <div>
        <div style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted, fontWeight: FontWeight.semibold, marginBottom: '6px', letterSpacing: '0.06em' }}>OBJECTS ({objects.length})</div>
        {objects.length === 0 ? (
          <div style={{ color: BaseColors.textMuted, fontSize: FontSize.xxs }}>No objects stored. Use INGEST OBJECT to store files.</div>
        ) : objects.map(obj => (
          <div
            key={obj.key}
            onClick={() => onSelectObject(obj.key)}
            style={{ padding: '5px 8px', background: BaseColors.bg, border: `1px solid ${BaseColors.border}`, borderRadius: '2px', marginBottom: '3px', cursor: 'pointer' }}
          >
            <div style={{ fontSize: FontSize.xxs, color: BaseColors.textPrimary, marginBottom: '2px', wordBreak: 'break-all' }}>{obj.key}</div>
            <div style={{ display: 'flex', gap: '8px', fontSize: '9px', color: BaseColors.textMuted }}>
              <span>{(obj.size / 1024).toFixed(1)} KB</span>
              <span style={{ color: obj.scheme.includes('reed') ? '#a78bfa' : StateColors.HEALTHY }}>{obj.scheme.includes('reed') ? 'RS 2+1' : 'RF 3'}</span>
              <span>{obj.chunks.length} chunks</span>
            </div>
          </div>
        ))}
      </div>
    </Panel>
  );
};

// ─── REPAIRS panel ────────────────────────────────────────────────────────
const RepairsPanel: React.FC<{ tasks: RepairTask[] }> = ({ tasks }) => {
  const byStatus = {
    REPAIRING: tasks.filter(t => t.status === 'REPAIRING'),
    QUEUED: tasks.filter(t => t.status === 'QUEUED'),
    FAILED: tasks.filter(t => t.status === 'FAILED'),
    COMPLETED: tasks.filter(t => t.status === 'COMPLETED'),
  };

  return (
    <Panel title="REPAIR QUEUE" icon={<Wrench size={13} color={tasks.some(t => t.status === 'REPAIRING') ? StateColors.REPAIRING : BaseColors.textMuted} />}>
      {/* Summary */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '6px', marginBottom: '12px' }}>
        {[
          { label: 'ACTIVE', count: byStatus.REPAIRING.length, color: StateColors.REPAIRING },
          { label: 'QUEUED', count: byStatus.QUEUED.length, color: StateColors.STALE },
          { label: 'FAILED', count: byStatus.FAILED.length, color: StateColors.DEAD },
          { label: 'DONE', count: byStatus.COMPLETED.length, color: StateColors.HEALTHY },
        ].map(s => (
          <div key={s.label} style={{ background: BaseColors.bg, border: `1px solid ${BaseColors.border}`, borderRadius: '2px', padding: '6px 8px', textAlign: 'center' }}>
            <div style={{ fontSize: '18px', fontWeight: FontWeight.bold, color: s.count > 0 ? s.color : BaseColors.textMuted, lineHeight: 1 }}>{s.count}</div>
            <div style={{ fontSize: '9px', color: BaseColors.textMuted, marginTop: '2px', letterSpacing: '0.04em' }}>{s.label}</div>
          </div>
        ))}
      </div>

      {tasks.length === 0 ? (
        <div style={{ padding: '16px', textAlign: 'center', color: BaseColors.textMuted, fontSize: FontSize.xxs }}>
          <CheckCircle2 size={24} color={StateColors.HEALTHY} style={{ marginBottom: '8px' }} />
          <div>No active repair tasks.</div>
          <div style={{ marginTop: '4px', color: BaseColors.textMuted }}>Kill a node to trigger repair.</div>
        </div>
      ) : (
        <div>
          {tasks.map(t => {
            const c = t.status === 'REPAIRING' ? StateColors.REPAIRING
              : t.status === 'FAILED' ? StateColors.DEAD
              : t.status === 'COMPLETED' ? StateColors.HEALTHY
              : BaseColors.textMuted;
            return (
              <div key={t.id} style={{ padding: '7px 8px', background: BaseColors.bg, border: `1px solid ${BaseColors.border}`, borderLeft: `3px solid ${c}`, borderRadius: '2px', marginBottom: '5px' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                  <span style={{ fontSize: FontSize.xxs, color: BaseColors.textPrimary, fontWeight: FontWeight.semibold, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '140px' }}>{t.object_key}</span>
                  <span style={{ fontSize: '9px', color: c, background: `${c}15`, border: `1px solid ${c}30`, padding: '0 4px', borderRadius: '2px' }}>{t.status}</span>
                </div>
                <div style={{ display: 'grid', gridTemplateColumns: '50px 1fr', gap: '2px', fontSize: '9px', color: BaseColors.textMuted }}>
                  <span>RF</span><span style={{ color: BaseColors.textSecondary }}>{t.rf_current}/{t.rf_target}</span>
                  <span>SOURCE</span><span style={{ color: BaseColors.textSecondary }}>{t.source}</span>
                  <span>TARGET</span><span style={{ color: BaseColors.textSecondary }}>{t.target}</span>
                </div>
                {t.status === 'REPAIRING' && (
                  <div style={{ height: '2px', background: BaseColors.border, borderRadius: '1px', marginTop: '5px', overflow: 'hidden' }}>
                    <div style={{ height: '100%', background: StateColors.REPAIRING, width: '45%', animation: 'sweep 1.5s ease infinite alternate' }} />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
      <style>{`@keyframes sweep { from { margin-left:0; } to { margin-left:55%; } }`}</style>
    </Panel>
  );
};

// ─── FAILURES panel ───────────────────────────────────────────────────────
const FailuresPanel: React.FC<{ nodes: StorageNode[]; onSelectNode: (id: string) => void }> = ({ nodes, onSelectNode }) => {
  const failed = nodes.filter(n => n.status === 'DEAD' || n.status === 'SUSPECT' || n.status === 'DEGRADED' || n.status === 'PARTITIONED');
  const healthy = nodes.filter(n => n.status === 'HEALTHY' || n.status === 'REPAIRING');

  return (
    <Panel title="FAILURE ANALYSIS" icon={<AlertTriangle size={13} color={failed.length > 0 ? StateColors.DEAD : BaseColors.textMuted} />}>
      {/* Summary */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '6px', marginBottom: '12px' }}>
        <div style={{ background: failed.length > 0 ? `${StateColors.DEAD}15` : BaseColors.bg, border: `1px solid ${failed.length > 0 ? StateColors.DEAD : BaseColors.border}`, borderRadius: '2px', padding: '8px', textAlign: 'center' }}>
          <div style={{ fontSize: '22px', fontWeight: FontWeight.bold, color: failed.length > 0 ? StateColors.DEAD : BaseColors.textMuted }}>{failed.length}</div>
          <div style={{ fontSize: '9px', color: BaseColors.textMuted, letterSpacing: '0.04em' }}>DEGRADED</div>
        </div>
        <div style={{ background: BaseColors.bg, border: `1px solid ${StateColors.HEALTHY}40`, borderRadius: '2px', padding: '8px', textAlign: 'center' }}>
          <div style={{ fontSize: '22px', fontWeight: FontWeight.bold, color: StateColors.HEALTHY }}>{healthy.length}</div>
          <div style={{ fontSize: '9px', color: BaseColors.textMuted, letterSpacing: '0.04em' }}>HEALTHY</div>
        </div>
      </div>

      {failed.length === 0 ? (
        <div style={{ padding: '16px', textAlign: 'center', color: BaseColors.textMuted, fontSize: FontSize.xxs }}>
          <CheckCircle2 size={24} color={StateColors.HEALTHY} style={{ marginBottom: '8px' }} />
          <div>All nodes operational.</div>
          <div style={{ marginTop: '4px' }}>No active failures detected.</div>
        </div>
      ) : (
        <div style={{ marginBottom: '12px' }}>
          <div style={{ fontSize: FontSize.xxs, color: StateColors.DEAD, fontWeight: FontWeight.semibold, marginBottom: '6px', letterSpacing: '0.04em' }}>FAILED / DEGRADED NODES</div>
          {failed.map(n => {
            const c = StateColors[n.status as keyof typeof StateColors] ?? BaseColors.textMuted;
            return (
              <div
                key={n.id}
                onClick={() => onSelectNode(n.id)}
                style={{ padding: '8px', background: `${c}08`, border: `1px solid ${c}40`, borderLeft: `3px solid ${c}`, borderRadius: '2px', marginBottom: '5px', cursor: 'pointer' }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '4px' }}>
                  <span style={{ fontSize: FontSize.xs, color: BaseColors.textPrimary, fontWeight: FontWeight.semibold }}>{n.id}</span>
                  <span style={{ fontSize: '9px', color: c, background: `${c}20`, border: `1px solid ${c}40`, padding: '1px 5px', borderRadius: '2px' }}>{n.status}</span>
                </div>
                <div style={{ display: 'grid', gridTemplateColumns: '50px 1fr', gap: '2px', fontSize: '9px', color: BaseColors.textMuted }}>
                  <span>Address</span><span style={{ color: BaseColors.textSecondary }}>{n.address}</span>
                  <span>Zone</span><span style={{ color: BaseColors.textSecondary }}>{n.zone || 'N/A'}</span>
                  {n.is_partitioned && <><span>Partition</span><span style={{ color: StateColors.DEAD }}>YES</span></>}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* All nodes status */}
      <div>
        <div style={{ fontSize: FontSize.xxs, color: BaseColors.textMuted, fontWeight: FontWeight.semibold, marginBottom: '6px', letterSpacing: '0.04em' }}>ALL NODES</div>
        {nodes.map(n => {
          const c = StateColors[n.status as keyof typeof StateColors] ?? BaseColors.textMuted;
          const icon = n.status === 'DEAD' || n.status === 'SUSPECT' ? <XCircle size={10} color={c} /> : <CheckCircle2 size={10} color={c} />;
          return (
            <div key={n.id} onClick={() => onSelectNode(n.id)} style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '4px 0', cursor: 'pointer', borderBottom: `1px solid ${BaseColors.bg}` }}>
              {icon}
              <span style={{ fontSize: FontSize.xxs, color: BaseColors.textPrimary, flex: 1 }}>{n.id}</span>
              <span style={{ fontSize: '9px', color: c }}>{n.status}</span>
            </div>
          );
        })}
      </div>
    </Panel>
  );
};

// ─── Main export ──────────────────────────────────────────────────────────
export const ViewModePanel: React.FC<ViewModePanelProps> = (props) => {
  const { viewMode, nodes, objects, topology, metrics, repairQueue, onSelectNode, onSelectObject } = props;

  switch (viewMode) {
    case 'TOPOLOGY':
      return <TopologyPanel nodes={nodes} topology={topology} onSelectNode={onSelectNode} />;
    case 'DATA':
      return <DataPanel nodes={nodes} objects={objects} metrics={metrics} onSelectObject={onSelectObject} />;
    case 'REPAIRS':
      return <RepairsPanel tasks={repairQueue} />;
    case 'FAILURES':
      return <FailuresPanel nodes={nodes} onSelectNode={onSelectNode} />;
    case 'OVERVIEW':
    default:
      return null; // OVERVIEW shows the default ObjectsCatalogDrawer
  }
};
