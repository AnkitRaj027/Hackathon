// ChaosConfirmationModal Component
// Section 70 & 71: High-stakes operational confirmation for destructive chaos actions.

import React from 'react';
import { BaseColors } from '../design/tokens';
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
        background: 'rgba(0, 0, 0, 0.75)',
        backdropFilter: 'blur(4px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 100,
        fontFamily: '"IBM Plex Mono", monospace',
      }}
    >
      <div
        style={{
          width: '440px',
          background: '#0d131f',
          border: '1px solid #ef4444',
          borderRadius: '4px',
          padding: '20px',
          boxShadow: '0 16px 48px rgba(0, 0, 0, 0.8)',
          color: BaseColors.textPrimary,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#ef4444', fontWeight: 700 }}>
            <AlertTriangle size={18} />
            <span>CONFIRM DESTRUCTIVE ACTION</span>
          </div>
          <button
            onClick={onCancel}
            style={{ background: 'transparent', border: 'none', color: BaseColors.textMuted, cursor: 'pointer' }}
          >
            <X size={16} />
          </button>
        </div>

        <p style={{ fontSize: '13px', lineHeight: 1.5, marginBottom: '14px', color: '#cbd5e1' }}>
          {isKill ? (
            <>
              You are about to terminate storage node{' '}
              <strong style={{ color: '#f8fafc' }}>{targetId}</strong>. This will cause an abrupt process crash,
              triggering the Failure Detector to transition the node from{' '}
              <span style={{ color: '#38bdf8' }}>HEALTHY</span> &rarr;{' '}
              <span style={{ color: '#f97316' }}>SUSPECT</span> &rarr;{' '}
              <span style={{ color: '#ef4444' }}>DEAD</span> and scheduling automatic replica repair sweeps.
            </>
          ) : (
            <>
              You are about to inject physical bit-rot corruption into chunk{' '}
              <strong style={{ color: '#f8fafc' }}>{targetId}</strong> on disk. The storage engine and coordinator
              scrubbers will detect a cryptographic SHA-256 digest mismatch and failover to surviving replicas.
            </>
          )}
        </p>

        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '20px' }}>
          <button
            onClick={onCancel}
            disabled={isProcessing}
            style={{
              padding: '7px 14px',
              background: 'transparent',
              border: `1px solid ${BaseColors.border}`,
              color: BaseColors.textSecondary,
              borderRadius: '3px',
              cursor: 'pointer',
              fontSize: '11px',
              fontWeight: 600,
            }}
          >
            CANCEL
          </button>

          <button
            onClick={onConfirm}
            disabled={isProcessing}
            style={{
              padding: '7px 16px',
              background: '#dc2626',
              border: 'none',
              color: '#ffffff',
              borderRadius: '3px',
              cursor: isProcessing ? 'wait' : 'pointer',
              fontSize: '11px',
              fontWeight: 700,
            }}
          >
            {isProcessing ? 'EXECUTING...' : isKill ? `KILL NODE (${targetId})` : 'CORRUPT CHUNK PAYLOAD'}
          </button>
        </div>
      </div>
    </div>
  );
};
