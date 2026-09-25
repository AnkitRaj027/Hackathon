// UploadModal Component
// Ingestion interface supporting both 3-way Replication and Reed-Solomon Erasure Coding.

import React, { useState } from 'react';
import { BaseColors } from '../design/tokens';
import { Upload, X, CheckCircle, AlertCircle } from 'lucide-react';

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
          width: '460px',
          background: '#0d131f',
          border: `1px solid ${BaseColors.border}`,
          borderRadius: '4px',
          padding: '20px',
          boxShadow: '0 16px 48px rgba(0, 0, 0, 0.8)',
          color: BaseColors.textPrimary,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#38bdf8', fontWeight: 700 }}>
            <Upload size={16} />
            <span>INGEST OBJECT STREAM</span>
          </div>
          <button
            onClick={onClose}
            style={{ background: 'transparent', border: 'none', color: BaseColors.textMuted, cursor: 'pointer' }}
          >
            <X size={16} />
          </button>
        </div>

        <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <div>
            <label style={{ display: 'block', fontSize: '11px', color: BaseColors.textMuted, marginBottom: '5px' }}>
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
                padding: '8px',
                background: '#090d16',
                border: `1px solid ${BaseColors.border}`,
                borderRadius: '3px',
                color: BaseColors.textPrimary,
                fontSize: '11px',
              }}
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '11px', color: BaseColors.textMuted, marginBottom: '5px' }}>
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
                padding: '8px',
                background: '#090d16',
                border: `1px solid ${BaseColors.border}`,
                borderRadius: '3px',
                color: BaseColors.textPrimary,
                fontSize: '11px',
              }}
            />
          </div>

          <div>
            <label style={{ display: 'block', fontSize: '11px', color: BaseColors.textMuted, marginBottom: '5px' }}>
              STORAGE ENCODING SCHEME
            </label>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px' }}>
              <button
                type="button"
                onClick={() => setScheme('replication')}
                style={{
                  padding: '10px 8px',
                  background: scheme === 'replication' ? 'rgba(56, 189, 248, 0.15)' : '#090d16',
                  border: `1px solid ${scheme === 'replication' ? '#38bdf8' : BaseColors.border}`,
                  borderRadius: '3px',
                  color: scheme === 'replication' ? '#38bdf8' : BaseColors.textSecondary,
                  cursor: 'pointer',
                  textAlign: 'left',
                  fontSize: '11px',
                }}
              >
                <div style={{ fontWeight: 600 }}>3x Full Replication</div>
                <div style={{ fontSize: '9px', color: BaseColors.textMuted, marginTop: '2px' }}>
                  RF=3, W=3, R=1 quorum
                </div>
              </button>

              <button
                type="button"
                onClick={() => setScheme('erasure')}
                style={{
                  padding: '10px 8px',
                  background: scheme === 'erasure' ? 'rgba(168, 85, 247, 0.15)' : '#090d16',
                  border: `1px solid ${scheme === 'erasure' ? '#a855f7' : BaseColors.border}`,
                  borderRadius: '3px',
                  color: scheme === 'erasure' ? '#a855f7' : BaseColors.textSecondary,
                  cursor: 'pointer',
                  textAlign: 'left',
                  fontSize: '11px',
                }}
              >
                <div style={{ fontWeight: 600 }}>Reed-Solomon (2+1)</div>
                <div style={{ fontSize: '9px', color: BaseColors.textMuted, marginTop: '2px' }}>
                  50% storage overhead savings
                </div>
              </button>
            </div>
          </div>

          {error && (
            <div style={{ color: '#f87171', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '6px' }}>
              <AlertCircle size={14} />
              <span>{error}</span>
            </div>
          )}

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', marginTop: '10px' }}>
            <button
              type="button"
              onClick={onClose}
              disabled={uploading}
              style={{
                padding: '7px 14px',
                background: 'transparent',
                border: `1px solid ${BaseColors.border}`,
                color: BaseColors.textSecondary,
                borderRadius: '3px',
                cursor: 'pointer',
                fontSize: '11px',
              }}
            >
              CANCEL
            </button>
            <button
              type="submit"
              disabled={uploading || !file}
              style={{
                padding: '7px 16px',
                background: '#0284c7',
                border: 'none',
                color: '#ffffff',
                borderRadius: '3px',
                cursor: uploading ? 'wait' : 'pointer',
                fontSize: '11px',
                fontWeight: 600,
              }}
            >
              {uploading ? 'STREAMING...' : 'PERSIST OBJECT'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
