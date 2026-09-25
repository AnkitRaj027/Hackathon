// TopStatusStrip Component
// Minimal operational status bar — compact, information-dense.
// All values come from real backend state.

import React from 'react';
import { ClusterStatus, MetricsPoint } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';
import { Shield, Upload, RefreshCw, Radio } from 'lucide-react';

interface TopStatusStripProps {
  status: ClusterStatus | null;
  connected: boolean;
  metrics: MetricsPoint[];
  onOpenUpload: () => void;
  onRefresh: () => void;
}

const clusterStatusColor = (s: string | undefined): string => {
  if (!s) return StateColors.STALE;
  if (s === 'HEALTHY') return StateColors.HEALTHY;
  if (s === 'DEGRADED') return StateColors.DEGRADED;
  return StateColors.DEAD;
};

export const TopStatusStrip: React.FC<TopStatusStripProps> = ({
  status,
  connected,
  metrics,
  onOpenUpload,
  onRefresh,
}) => {
  // Latest metrics point — only show if backend has real data
  const latest = metrics.length > 0 ? metrics[metrics.length - 1] : null;

  const statusColor = clusterStatusColor(status?.status);

  return (
    <header
      style={{
        height: '46px',
        background: BaseColors.bg,
        borderBottom: `1px solid ${BaseColors.border}`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 16px',
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        zIndex: 50,
        fontFamily: FontFamily.mono,
        fontSize: FontSize.md,
        color: BaseColors.textPrimary,
      }}
    >
      {/* Brand */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            fontWeight: FontWeight.bold,
            letterSpacing: '0.08em',
          }}
        >
          <Shield size={15} color={StateColors.HEALTHY} />
          <span style={{ fontSize: FontSize.xl, color: BaseColors.textPrimary }}>VAULT</span>
          <span
            style={{
              fontSize: FontSize.xxs,
              color: BaseColors.textMuted,
              border: `1px solid ${BaseColors.border}`,
              padding: '1px 5px',
              borderRadius: '2px',
            }}
          >
            DISTRIBUTED STORAGE
          </span>
        </div>

        <div style={{ width: '1px', height: '18px', background: BaseColors.border }} />

        {/* Cluster health indicator */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <span
            style={{
              width: '7px',
              height: '7px',
              borderRadius: '50%',
              background: statusColor,
              boxShadow: `0 0 8px ${statusColor}`,
              transition: 'background 300ms ease, box-shadow 300ms ease',
            }}
          />
          <span
            style={{
              fontWeight: FontWeight.semibold,
              color: statusColor,
              transition: 'color 300ms ease',
            }}
          >
            {status?.status || 'INITIALIZING'}
          </span>
        </div>
      </div>

      {/* Live cluster metrics (center) */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '22px',
          fontSize: FontSize.xs,
        }}
      >
        <Metric label="NODES" value={status ? `${status.healthy_nodes}/${status.total_nodes}` : null} />
        <Metric label="OBJECTS" value={status ? String(status.active_objects) : null} />
        <Metric
          label="TOPOLOGY"
          value={status ? (status.erasure_coding ? 'RS 2+1' : `RF ${status.replication_factor} · W${status.write_quorum} R${status.read_quorum}`) : null}
          color={StateColors.HEALTHY}
        />
        {latest && (
          <>
            <Metric
              label="WRITE"
              value={`${latest.write_mbps.toFixed(1)} MB/s`}
              color={BaseColors.textSecondary}
            />
            <Metric
              label="READ"
              value={`${latest.read_mbps.toFixed(1)} MB/s`}
              color={BaseColors.textSecondary}
            />
            <Metric
              label="LATENCY"
              value={`${latest.avg_latency_ms.toFixed(1)} ms`}
              color={latest.avg_latency_ms > 50 ? StateColors.SUSPECT : BaseColors.textSecondary}
            />
          </>
        )}

        {/* SSE connection status */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '5px',
            color: connected ? StateColors.REPAIRING : StateColors.DEAD,
            fontSize: FontSize.xs,
          }}
        >
          <Radio size={11} />
          <span>{connected ? 'LIVE' : 'DISCONNECTED'}</span>
        </div>
      </div>

      {/* Action Controls */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <button
          onClick={onRefresh}
          title="Refresh cluster snapshot"
          style={{
            background: 'transparent',
            border: `1px solid ${BaseColors.border}`,
            color: BaseColors.textSecondary,
            padding: '4px 8px',
            borderRadius: '2px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '4px',
            fontSize: FontSize.xs,
            fontFamily: FontFamily.mono,
          }}
        >
          <RefreshCw size={11} />
          <span>SYNC</span>
        </button>

        <button
          onClick={onOpenUpload}
          style={{
            background: BaseColors.accent,
            border: 'none',
            color: BaseColors.bg,
            padding: '5px 12px',
            borderRadius: '2px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            fontWeight: FontWeight.semibold,
            fontSize: FontSize.xs,
            fontFamily: FontFamily.mono,
          }}
        >
          <Upload size={11} />
          <span>INGEST OBJECT</span>
        </button>
      </div>
    </header>
  );
};

// ─── Reusable inline metric cell ──────────────────────────────────────────

const Metric: React.FC<{ label: string; value: string | null; color?: string }> = ({
  label,
  value,
  color,
}) => (
  <div>
    <span style={{ color: BaseColors.textMuted, marginRight: '5px' }}>{label}:</span>
    <span
      style={{
        fontWeight: FontWeight.semibold,
        color: color ?? BaseColors.textPrimary,
        fontVariantNumeric: 'tabular-nums',
      }}
    >
      {value ?? '—'}
    </span>
  </div>
);
