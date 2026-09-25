// src/components/Modal.tsx
import React, { useEffect } from 'react';
import { BaseColors, PanelTokens } from '../design/tokens';

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  children: React.ReactNode;
  title?: string;
}

export const Modal: React.FC<ModalProps> = ({ isOpen, onClose, children, title }) => {
  // Prevent background scrolling when modal is open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [isOpen]);

  if (!isOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.75)',
        backdropFilter: 'blur(4px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: PanelTokens.modalZIndex,
        animation: 'fadeIn 150ms ease-out',
      }}
      onClick={e => {
        // Close when clicking outside the dialog content
        if ((e.target as HTMLElement).dataset?.overlay === 'true') {
          onClose();
        }
      }}
    >
      <div
        data-overlay="true"
        style={{
          position: 'absolute',
          inset: 0,
        }}
      />
      <div
        style={{
          position: 'relative',
          width: '440px',
          background: BaseColors.surface,
          border: `1px solid ${BaseColors.border}`,
          borderRadius: '4px',
          padding: '20px',
          boxShadow: '0 16px 48px rgba(0,0,0,0.8)',
          color: BaseColors.textPrimary,
          fontFamily: '"IBM Plex Mono", monospace',
        }}
      >
        {title && (
          <h2 style={{ margin: 0, marginBottom: '12px', fontSize: '16px', color: BaseColors.textPrimary }}>
            {title}
          </h2>
        )}
        {children}
      </div>
      <style>{`
        @keyframes fadeIn {
          from { opacity: 0; }
          to { opacity: 1; }
        }
      `}</style>
    </div>
  );
};
