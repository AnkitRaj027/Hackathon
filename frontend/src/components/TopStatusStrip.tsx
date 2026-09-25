// TopStatusStrip Component
// Section 67: Minimal technical status strip for high operational scanability.

import React from 'react';
import { ClusterStatus } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { Shield, Upload, RefreshCw, Radio } from 'lucide-react';

interface TopStatusStripProps {
  status: ClusterStatus | null;
  connected: boolean;
  onOpenUpload: () => void;
  onRefresh: () => void;
}

export const TopStatusStrip: React.FC<TopStatusStripProps> = ({
  status,
  connected,
  onOpenUpload,
  onRefresh,
}) => {
  return (
    <header
      style={{
        height: '46px',
        background: BaseColors.bg,
        backdropFilter: 'blur(8px)',
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
        fontFamily: '"IBM Plex Mono", monospace',
        fontSize: '12px',
        color: BaseColors.textPrimary,
      }}
    >
      {/* Brand & Cluster Identity */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontWeight: 700, letterSpacing: '0.08em' }}>
          <Shield size={16} color={StateColors.HEALTHY} />
          <span style={{ fontSize: '14px', color: BaseColors.textPrimary }}>VAULT</span>
          <span style={{ fontSize: '10px', color: BaseColors.textMuted, border: `1px solid ${BaseColors.border}`, padding: '1px 5px', borderRadius: '2px' }}>
            OPERATIONS
          </span>
        </div>

        <div style={{ width: '1px', height: '18px', background: BaseColors.border }} />

        {/* Live Cluster Status */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <span
            style={{
              width: '7px',
              height: '7px',
              borderRadius: '50%',
              background: status?.status === 'HEALTHY' ? StateColors.HEALTHY : StateColors.DEAD,
              boxShadow: `0 0 8px ${status?.status === 'HEALTHY' ? StateColors.HEALTHY : StateColors.DEAD}`,
            }}
          />
          <span style={{ fontWeight: 600, color: status?.status === 'HEALTHY' ? StateColors.HEALTHY : StateColors.DEAD }}>
            {status?.status || 'INITIALIZING'}
          </span>
        </div>
      </div>

      {/* Operational Metrics (Tabular Monospace) */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '24px' }}>
        <div>
          <span style={{ color: BaseColors.textMuted, marginRight: '6px' }}>NODES:</span>
          <span style={{ fontWeight: 600, color: BaseColors.textPrimary }}>
            {status ? `${status.healthy_nodes}/${status.total_nodes}` : '...'}
          </span>
        </div>

        <div>
          <span style={{ color: BaseColors.textMuted, marginRight: '6px' }}>TOPOLOGY:</span>
          <span style={{ fontWeight: 600, color: '#38bdf8' }}>
            {status?.erasure_coding ? 'RS 2+1 / RF 3' : 'RF 3 (W=3, R=1)'}
          </span>
        </div>

        <div>
          <span style={{ color: BaseColors.textMuted, marginRight: '6px' }}>OBJECTS:</span>
          <span style={{ fontWeight: 600, color: BaseColors.textPrimary }}>
            {status?.active_objects ?? 0}
          </span>
        </div>

        {/* Real-time SSE Connection Indicator */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '5px', fontSize: '11px', color: connected ? '#10b981' : '#ef4444' }}>
          <Radio size={12} />
          <span>{connected ? 'LIVE SSE' : 'DISCONNECTED'}</span>
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
            borderRadius: '3px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '4px',
            fontSize: '11px',
          }}
        >
          <RefreshCw size={12} />
          <span>SYNC</span>
        </button>

        <button
          onClick={onOpenUpload}
          style={{
            background: BaseColors.accent,
            border: 'none',
            color: '#ffffff',
            padding: '5px 12px',
            borderRadius: '3px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            fontWeight: 600,
            fontSize: '11px',
          }}
        >
          <Upload size={12} />
          <span>INGEST OBJECT</span>
        </button>
      </div>
    </header>
  );
};
