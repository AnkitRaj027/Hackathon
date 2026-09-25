// ConnectionError Component
// Operationally useful error panel — not a generic "oops" screen.

import React from 'react';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';
import { AlertTriangle, RefreshCw } from 'lucide-react';

interface ConnectionErrorProps {
  lastConnectedAt: string | null;
  onRetry: () => void;
}

export const ConnectionError: React.FC<ConnectionErrorProps> = ({
  lastConnectedAt,
  onRetry,
}) => (
  <div
    style={{
      position: 'fixed',
      top: '56px',
      left: '50%',
      transform: 'translateX(-50%)',
      background: BaseColors.surfaceCard,
      border: `1px solid ${StateColors.DEAD}40`,
      borderLeft: `3px solid ${StateColors.DEAD}`,
      borderRadius: '3px',
      padding: '12px 16px',
      zIndex: 200,
      fontFamily: FontFamily.mono,
      display: 'flex',
      alignItems: 'flex-start',
      gap: '12px',
      maxWidth: '420px',
      boxShadow: `0 4px 24px rgba(0,0,0,0.5)`,
    }}
  >
    <AlertTriangle size={16} color={StateColors.DEAD} style={{ flexShrink: 0, marginTop: '2px' }} />
    <div style={{ flex: 1 }}>
      <div
        style={{
          fontSize: FontSize.sm,
          fontWeight: FontWeight.semibold,
          color: StateColors.DEAD,
          marginBottom: '6px',
          letterSpacing: '0.04em',
        }}
      >
        COORDINATOR UNAVAILABLE
      </div>
      <div style={{ fontSize: FontSize.xs, color: BaseColors.textSecondary, lineHeight: '1.6' }}>
        Unable to receive live cluster state.
      </div>
      {lastConnectedAt && (
        <div style={{ fontSize: FontSize.xs, color: BaseColors.textMuted, marginTop: '4px' }}>
          Last confirmed:{' '}
          <span style={{ color: BaseColors.textSecondary }}>
            {new Date(lastConnectedAt).toLocaleTimeString()} UTC
          </span>
        </div>
      )}
      <div
        style={{
          fontSize: FontSize.xs,
          color: StateColors.SUSPECT,
          marginTop: '4px',
        }}
      >
        Connection: RECONNECTING
      </div>
    </div>
    <button
      onClick={onRetry}
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
        flexShrink: 0,
      }}
    >
      <RefreshCw size={11} />
      RETRY
    </button>
  </div>
);
