// TopStatusStrip Component
// Standard, high-density operational status bar for infrastructure control plane.

import React from 'react';
import { ClusterStatus, MetricsPoint } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';
import { Shield, Upload, RefreshCw, Radio, Bot } from 'lucide-react';

interface TopStatusStripProps {
  status: ClusterStatus | null;
  connected: boolean;
  metrics: MetricsPoint[];
  onOpenUpload: () => void;
  onRefresh: () => void;
  onToggleAI?: () => void;
  isAIOpen?: boolean;
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
  onToggleAI,
  isAIOpen,
}) => {
  const latest = metrics.length > 0 ? metrics[metrics.length - 1] : null;
  const statusColor = clusterStatusColor(status?.status);

  return (
    <header
      style={{
        height: '48px',
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
        color: BaseColors.textPrimary,
        userSelect: 'none',
      }}
    >
      {/* Brand & System State */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <Shield size={16} color={BaseColors.accent} />
          <span
            style={{
              fontFamily: FontFamily.sans,
              fontWeight: FontWeight.bold,
              fontSize: FontSize.lg,
              letterSpacing: '0.03em',
              color: BaseColors.textPrimary,
            }}
          >
            VAULT
          </span>
          <span
            style={{
              fontFamily: FontFamily.mono,
              fontSize: FontSize.xxs,
              color: BaseColors.textMuted,
              border: `1px solid ${BaseColors.border}`,
              padding: '2px 5px',
              borderRadius: '2px',
              letterSpacing: '0.04em',
            }}
          >
            OPERATIONS
          </span>
        </div>

        <div style={{ width: '1px', height: '16px', background: BaseColors.border }} />

        {/* Cluster Status Badge */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <span
            style={{
              width: '7px',
              height: '7px',
              borderRadius: '50%',
              background: statusColor,
              display: 'inline-block',
            }}
          />
          <span
            style={{
              fontFamily: FontFamily.mono,
              fontSize: FontSize.sm,
              fontWeight: FontWeight.semibold,
              color: statusColor,
              letterSpacing: '0.02em',
            }}
          >
            {status?.status || 'INITIALIZING'}
          </span>
        </div>
      </div>

      {/* Cluster Telemetry Strip */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '20px',
          fontFamily: FontFamily.mono,
          fontSize: FontSize.sm,
        }}
      >
        <Metric label="NODES" value={status ? `${status.healthy_nodes}/${status.total_nodes}` : null} />
        <Metric label="OBJECTS" value={status ? String(status.active_objects) : null} />
        <Metric
          label="TOPOLOGY"
          value={
            status
              ? status.erasure_coding
                ? 'RS 2+1'
                : `RF ${status.replication_factor} (W${status.write_quorum} R${status.read_quorum})`
              : null
          }
          color={BaseColors.accent}
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

        {/* Live SSE Indicator */}
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
          <span>{connected ? 'LIVE' : 'OFFLINE'}</span>
        </div>
      </div>

      {/* Actions */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <button
          onClick={onRefresh}
          title="Refresh cluster snapshot"
          style={{
            background: 'transparent',
            border: `1px solid ${BaseColors.border}`,
            color: BaseColors.textSecondary,
            padding: '5px 10px',
            borderRadius: '2px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '5px',
            fontSize: FontSize.sm,
            fontFamily: FontFamily.mono,
          }}
        >
          <RefreshCw size={11} />
          <span>SYNC</span>
        </button>

        {onToggleAI && (
          <button
            onClick={onToggleAI}
            title="Toggle Vault AI Operations Copilot"
            style={{
              background: isAIOpen ? BaseColors.surfaceElevated : 'transparent',
              border: `1px solid ${isAIOpen ? BaseColors.accent : BaseColors.border}`,
              color: isAIOpen ? BaseColors.accent : BaseColors.textSecondary,
              padding: '5px 10px',
              borderRadius: '2px',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '5px',
              fontSize: FontSize.sm,
              fontFamily: FontFamily.mono,
            }}
          >
            <Bot size={13} color={isAIOpen ? BaseColors.accent : BaseColors.textSecondary} />
            <span>AI COPILOT</span>
          </button>
        )}

        <button
          onClick={onOpenUpload}
          style={{
            background: BaseColors.accent,
            border: 'none',
            color: '#000000',
            padding: '5px 12px',
            borderRadius: '2px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            fontWeight: FontWeight.semibold,
            fontSize: FontSize.sm,
            fontFamily: FontFamily.mono,
            letterSpacing: '0.02em',
          }}
        >
          <Upload size={12} />
          <span>INGEST OBJECT</span>
        </button>
      </div>
    </header>
  );
};

const Metric: React.FC<{ label: string; value: string | null; color?: string }> = ({
  label,
  value,
  color,
}) => (
  <div style={{ display: 'flex', alignItems: 'baseline', gap: '5px' }}>
    <span style={{ color: BaseColors.textMuted, fontSize: FontSize.xs }}>{label}:</span>
    <span
      style={{
        fontWeight: FontWeight.medium,
        color: color ?? BaseColors.textPrimary,
        fontVariantNumeric: 'tabular-nums',
        fontSize: FontSize.sm,
      }}
    >
      {value ?? '—'}
    </span>
  </div>
);
