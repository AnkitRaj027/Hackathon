// UploadModal Component
// Ingestion interface supporting both 3-way Replication and Reed-Solomon Erasure Coding.

import React, { useState } from 'react';
import { BaseColors } from '../design/tokens';
import { FontFamily, FontSize } from '../design/typography';
import { Upload, X, AlertCircle } from 'lucide-react';

interface UploadModalProps {
  isOpen: boolean;
  onClose: () => void;
  onUpload: (file: File, key: string, scheme: 'replication' | 'erasure') => Promise<any>;
}

export const UploadModal: React.FC<UploadModalProps> = ({ isOpen, onClose, onUpload }) => {
  const [file, setFile] = useState<File | null>(null);
  const [key, setKey] = useState<string>('');
  const [scheme, setScheme] = useState<'replication' | 'erasure'>('replication');
  const [uploading, setUploading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) return;

    setUploading(true);
    setError(null);
    try {
      await onUpload(file, key || file.name, scheme);
      onClose();
    } catch (err: any) {
      setError(err.message || 'Upload operation failed');
    } finally {
      setUploading(false);
    }
  };

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
        zIndex: 100,
        fontFamily: FontFamily.sans,
      }}
    >
      <div
        style={{
          width: '500px',
          background: BaseColors.surface,
          border: `1px solid ${BaseColors.border}`,
          borderRadius: '2px',
          padding: '22px',
          color: BaseColors.textPrimary,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: BaseColors.accent, fontWeight: 600, fontSize: FontSize.lg }}>
            <Upload size={17} />
            <span>INGEST OBJECT STREAM</span>
          </div>
          <button
            onClick={onClose}
            style={{ background: 'transparent', border: 'none', color: BaseColors.textMuted, cursor: 'pointer', padding: '2px' }}
          >
            <X size={17} />
          </button>
        </div>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <div>
            <label style={{ display: 'block', fontSize: FontSize.sm, color: BaseColors.textSecondary, marginBottom: '6px', fontWeight: 500 }}>
              SOURCE FILE
            </label>
            <input
              type="file"
              required
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) {
                  setFile(f);
                  if (!key) setKey(f.name);
                }
              }}
              style={{
                width: '100%',
                padding: '8px 10px',
                background: BaseColors.bg,
                border: `1px solid ${BaseColors.border}`,
                borderRadius: '2px',
                color: BaseColors.textPrimary,
                fontSize: FontSize.sm,
                fontFamily: FontFamily.mono,
              }}
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: FontSize.sm, color: BaseColors.textSecondary, marginBottom: '6px', fontWeight: 500 }}>
              OBJECT KEY / PATH
            </label>
            <input
              type="text"
              value={key}
              onChange={(e) => setKey(e.target.value)}
              placeholder="e.g. telemetry/dump-01.bin"
              required
              style={{
                width: '100%',
                padding: '8px 12px',
                background: BaseColors.bg,
                border: `1px solid ${BaseColors.border}`,
                borderRadius: '2px',
                color: BaseColors.textPrimary,
                fontSize: FontSize.md,
                fontFamily: FontFamily.mono,
                outline: 'none',
              }}
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: FontSize.sm, color: BaseColors.textSecondary, marginBottom: '6px', fontWeight: 500 }}>
              STORAGE ENCODING SCHEME
            </label>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '10px' }}>
              <button
                type="button"
                onClick={() => setScheme('replication')}
                style={{
                  padding: '10px 12px',
                  background: scheme === 'replication' ? BaseColors.surfaceElevated : BaseColors.bg,
                  border: `1px solid ${scheme === 'replication' ? BaseColors.accent : BaseColors.border}`,
                  borderRadius: '2px',
                  color: scheme === 'replication' ? BaseColors.accent : BaseColors.textSecondary,
                  cursor: 'pointer',
                  textAlign: 'left',
                  fontSize: FontSize.sm,
                }}
              >
                <div style={{ fontWeight: 600, fontSize: FontSize.sm }}>3x Full Replication</div>
                <div style={{ fontSize: FontSize.xs, color: BaseColors.textMuted, marginTop: '3px', fontFamily: FontFamily.mono }}>
                  RF=3, W=3, R=1 quorum
                </div>
              </button>

              <button
                type="button"
                onClick={() => setScheme('erasure')}
                style={{
                  padding: '10px 12px',
                  background: scheme === 'erasure' ? BaseColors.surfaceElevated : BaseColors.bg,
                  border: `1px solid ${scheme === 'erasure' ? '#a855f7' : BaseColors.border}`,
                  borderRadius: '2px',
                  color: scheme === 'erasure' ? '#a855f7' : BaseColors.textSecondary,
                  cursor: 'pointer',
                  textAlign: 'left',
                  fontSize: FontSize.sm,
                }}
              >
                <div style={{ fontWeight: 600, fontSize: FontSize.sm }}>Reed-Solomon (2+1)</div>
                <div style={{ fontSize: FontSize.xs, color: BaseColors.textMuted, marginTop: '3px', fontFamily: FontFamily.mono }}>
                  50% storage overhead
                </div>
              </button>
            </div>
          </div>

          {error && (
            <div style={{ color: '#ef4444', fontSize: FontSize.sm, display: 'flex', alignItems: 'center', gap: '6px' }}>
              <AlertCircle size={15} />
              <span>{error}</span>
            </div>
          )}

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '8px' }}>
            <button
              type="button"
              onClick={onClose}
              disabled={uploading}
              style={{
                padding: '7px 16px',
                background: 'transparent',
                border: `1px solid ${BaseColors.border}`,
                color: BaseColors.textSecondary,
                borderRadius: '2px',
                cursor: 'pointer',
                fontSize: FontSize.sm,
                fontWeight: 500,
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={uploading || !file}
              style={{
                padding: '7px 18px',
                background: BaseColors.accent,
                border: 'none',
                color: '#000000',
                borderRadius: '2px',
                cursor: uploading ? 'wait' : 'pointer',
                fontSize: FontSize.sm,
                fontWeight: 600,
              }}
            >
              {uploading ? 'Streaming...' : 'Persist Object'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
