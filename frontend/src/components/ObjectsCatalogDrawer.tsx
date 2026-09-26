// ObjectsCatalogDrawer Component
// Compact left-hand drawer to browse stored objects and highlight 3D replica topology.

import React, { useState } from 'react';
import { ObjectDTO, SelectionState } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize } from '../design/typography';
import { Database, ChevronLeft, ChevronRight } from 'lucide-react';

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
        top: '54px',
        left: '12px',
        bottom: '44px',
        width: isOpen ? '280px' : '36px',
        background: BaseColors.surface,
        border: `1px solid ${BaseColors.border}`,
        borderRadius: '2px',
        display: 'flex',
        flexDirection: 'column',
        zIndex: 45,
        fontFamily: FontFamily.mono,
        color: BaseColors.textPrimary,
        transition: 'width 140ms ease',
        overflow: 'hidden',
      }}
    >
      {/* Header Bar */}
      <div
        style={{
          height: '40px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 10px',
          borderBottom: `1px solid ${BaseColors.border}`,
          background: BaseColors.surfaceElevated,
          cursor: 'pointer',
          userSelect: 'none',
        }}
        onClick={() => setIsOpen(!isOpen)}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', overflow: 'hidden' }}>
          <Database size={15} color={BaseColors.accent} />
          {isOpen && (
            <span style={{ fontSize: FontSize.sm, fontWeight: 600, fontFamily: FontFamily.sans, letterSpacing: '0.04em' }}>
              CATALOG ({objects.length})
            </span>
          )}
        </div>
        <button
          style={{ background: 'transparent', border: 'none', color: BaseColors.textMuted, cursor: 'pointer', padding: 0 }}
        >
          {isOpen ? <ChevronLeft size={15} /> : <ChevronRight size={15} />}
        </button>
      </div>

      {/* Object List */}
      {isOpen && (
        <div style={{ flex: 1, overflowY: 'auto', padding: '6px' }}>
          {objects.length === 0 ? (
            <div style={{ padding: '16px 8px', fontSize: FontSize.xs, color: BaseColors.textMuted, textAlign: 'center', fontFamily: FontFamily.sans }}>
              No objects stored. Click <strong>INGEST</strong> above to store files.
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
                    padding: '6px 8px',
                    marginBottom: '4px',
                    background: isSelected ? BaseColors.surfaceElevated : 'transparent',
                    border: `1px solid ${isSelected ? BaseColors.accent : BaseColors.border}`,
                    borderRadius: '2px',
                    cursor: 'pointer',
                    fontSize: FontSize.xs,
                  }}
                >
                  <div style={{ fontWeight: 500, color: isSelected ? BaseColors.textPrimary : BaseColors.textSecondary, wordBreak: 'break-all', fontFamily: FontFamily.mono }}>
                    {obj.key}
                  </div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', color: BaseColors.textMuted, fontSize: '10px', marginTop: '2px', fontFamily: FontFamily.mono }}>
                    <span>{(obj.size / (1024 * 1024)).toFixed(2)} MB</span>
                    <span style={{ color: isEC ? '#a855f7' : BaseColors.accent }}>
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
