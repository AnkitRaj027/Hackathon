// ChaosConfirmationModal Component
// Section 70 & 71: Operational confirmation for destructive chaos actions.

import React from 'react';
import { BaseColors, StateColors, PanelTokens } from '../design/tokens';
import { FontFamily, FontSize } from '../design/typography';
import { AlertTriangle, X } from 'lucide-react';

interface ChaosModalProps {
  isOpen: boolean;
  type: 'kill-node' | 'corrupt-chunk';
  targetId: string;
  onConfirm: () => void;
  onCancel: () => void;
  isProcessing: boolean;
}

export const ChaosConfirmationModal: React.FC<ChaosModalProps> = ({
  isOpen,
  type,
  targetId,
  onConfirm,
  onCancel,
  isProcessing,
}) => {
  if (!isOpen) return null;

  const isKill = type === 'kill-node';

  return (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        background: 'rgba(0, 0, 0, 0.72)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: PanelTokens.modalZIndex,
        fontFamily: FontFamily.sans,
      }}
    >
      <div
        style={{
          width: '480px',
          background: BaseColors.surface,
          border: `1px solid ${BaseColors.border}`,
          borderRadius: '2px',
          padding: '22px',
          color: BaseColors.textPrimary,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: StateColors.DEAD, fontWeight: 600, fontSize: FontSize.lg }}>
            <AlertTriangle size={17} />
            <span>CONFIRM DESTRUCTIVE ACTION</span>
          </div>
          <button
            onClick={onCancel}
            style={{ background: 'transparent', border: 'none', color: BaseColors.textMuted, cursor: 'pointer', padding: '2px' }}
          >
            <X size={17} />
          </button>
        </div>

        <p style={{ fontSize: FontSize.md, lineHeight: 1.6, marginBottom: '20px', color: BaseColors.textSecondary }}>
          {isKill ? (
            <>
              You are about to terminate storage node{' '}
              <strong style={{ color: BaseColors.textPrimary, fontFamily: FontFamily.mono }}>{targetId}</strong>. This triggers process termination, transitioning node from{' '}
              <span style={{ color: BaseColors.accent }}>HEALTHY</span> &rarr;{' '}
              <span style={{ color: StateColors.SUSPECT }}>SUSPECT</span> &rarr;{' '}
              <span style={{ color: StateColors.DEAD }}>DEAD</span> and scheduling automatic replica repair sweeps.
            </>
          ) : (
            <>
              You are about to inject physical bit-rot corruption into chunk{' '}
              <strong style={{ color: BaseColors.textPrimary, fontFamily: FontFamily.mono }}>{targetId}</strong> on disk. The storage engine and coordinator scrubbers will detect a cryptographic SHA-256 digest mismatch and failover to surviving replicas.
            </>
          )}
        </p>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px' }}>
          <button
            onClick={onCancel}
            disabled={isProcessing}
            style={{
              padding: '7px 16px',
              background: 'transparent',
              border: `1px solid ${BaseColors.border}`,
              color: BaseColors.textSecondary,
              borderRadius: '2px',
              cursor: 'pointer',
              fontSize: FontSize.sm,
              fontWeight: 500,
              fontFamily: FontFamily.sans,
            }}
          >
            Cancel
          </button>

          <button
            onClick={onConfirm}
            disabled={isProcessing}
            style={{
              padding: '7px 18px',
              background: StateColors.DEAD,
              border: 'none',
              color: '#ffffff',
              borderRadius: '2px',
              cursor: isProcessing ? 'wait' : 'pointer',
              fontSize: FontSize.sm,
              fontWeight: 600,
              fontFamily: FontFamily.sans,
            }}
          >
            {isProcessing ? 'Executing...' : isKill ? `Kill Node (${targetId})` : 'Corrupt Chunk'}
          </button>
        </div>
      </div>
    </div>
  );
};
