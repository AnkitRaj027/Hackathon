// ObjectsCatalogDrawer Component
// Compact left-hand drawer to browse stored objects and highlight 3D replica topology.

import React, { useState } from 'react';
import { ObjectDTO, SelectionState } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { Database, ChevronLeft, ChevronRight, Layers } from 'lucide-react';

interface ObjectsCatalogProps {
  objects: ObjectDTO[];
  selection: SelectionState;
  onSelectObject: (key: string) => void;
}

export const ObjectsCatalogDrawer: React.FC<ObjectsCatalogProps> = ({
  objects,
  selection,
  onSelectObject,
}) => {
  const [isOpen, setIsOpen] = useState<boolean>(true);

  return (
    <div
      style={{
        position: 'absolute',
        top: '56px',
        left: '12px',
        bottom: '50px',
        width: isOpen ? '260px' : '36px',
        background: BaseColors.surfaceElevated,
        backdropFilter: 'blur(8px)',
        border: `1px solid ${BaseColors.border}`,
        borderRadius: '4px',
        display: 'flex',
        flexDirection: 'column',
        zIndex: 45,
        fontFamily: '"IBM Plex Mono", monospace',
        color: BaseColors.textPrimary,
        transition: 'width 180ms cubic-bezier(0.2, 0, 0, 1)',
        overflow: 'hidden',
      }}
    >
      {/* Header Bar */}
      <div
        style={{
          height: '36px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 10px',
          borderBottom: `1px solid ${BaseColors.border}`,
          background: 'rgba(10, 15, 26, 0.6)',
          cursor: 'pointer',
        }}
        onClick={() => setIsOpen(!isOpen)}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', overflow: 'hidden' }}>
          <Database size={14} color={StateColors.HEALTHY} />
          {isOpen && (
            <span style={{ fontSize: '11px', fontWeight: 600, whiteSpace: 'nowrap' }}>
              CATALOG ({objects.length})
            </span>
          )}
        </div>
        <button
          style={{ background: 'transparent', border: 'none', color: BaseColors.textMuted, cursor: 'pointer', padding: 0 }}
        >
          {isOpen ? <ChevronLeft size={14} /> : <ChevronRight size={14} />}
        </button>
      </div>

      {/* Object List */}
      {isOpen && (
        <div style={{ flex: 1, overflowY: 'auto', padding: '6px' }}>
          {objects.length === 0 ? (
            <div style={{ padding: '16px 8px', fontSize: '11px', color: BaseColors.textMuted, textAlign: 'center' }}>
              No objects stored in cluster. Click <strong>INGEST</strong> above to store files.
            </div>
          ) : (
            objects.map((obj) => {
              const isSelected = selection.type === 'object' && selection.objectKey === obj.key;
              const isEC = obj.scheme.includes('reed-solomon');
              return (
                <div
                  key={obj.key}
                  onClick={() => onSelectObject(obj.key)}
                  style={{
                    padding: '8px',
                    marginBottom: '4px',
                    background: isSelected ? `${StateColors.HEALTHY}20` : BaseColors.surface,
                    border: `1px solid ${isSelected ? StateColors.HEALTHY : BaseColors.border}`,
                    borderRadius: '3px',
                    cursor: 'pointer',
                    fontSize: '11px',
                  }}
                >
                  <div style={{ fontWeight: 600, color: isSelected ? BaseColors.textPrimary : BaseColors.textSecondary, wordBreak: 'break-all' }}>
                    {obj.key}
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', color: BaseColors.textMuted, fontSize: '10px', marginTop: '3px' }}>
                    <span>{(obj.size / (1024 * 1024)).toFixed(2)} MB</span>
                    <span style={{ color: isEC ? '#a855f7' : '#38bdf8' }}>
                      {isEC ? 'RS 2+1' : 'RF 3'}
                    </span>
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}
    </div>
  );
};
