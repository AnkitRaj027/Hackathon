// ViewModeSwitcher Component
// Controls which visual emphasis mode is active over the same underlying topology.

import React from 'react';
import { ViewMode } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';

const MODES: { id: ViewMode; label: string; description: string }[] = [
  { id: 'OVERVIEW', label: 'OVERVIEW', description: 'Full cluster state' },
  { id: 'TOPOLOGY', label: 'TOPOLOGY', description: 'Nodes, racks, ring' },
  { id: 'DATA', label: 'DATA', description: 'Objects, chunks, replicas' },
  { id: 'REPAIRS', label: 'REPAIRS', description: 'Repair queue & transfers' },
  { id: 'FAILURES', label: 'FAILURES', description: 'Dead nodes, corrupted chunks' },
];

interface ViewModeSwitcherProps {
  activeView: ViewMode;
  onChange: (mode: ViewMode) => void;
  repairCount: number;
  failureCount: number;
}

export const ViewModeSwitcher: React.FC<ViewModeSwitcherProps> = ({
  activeView,
  onChange,
  repairCount,
  failureCount,
}) => {
  const getBadgeColor = (id: ViewMode): string | null => {
    if (id === 'REPAIRS' && repairCount > 0) return StateColors.REPAIRING;
    if (id === 'FAILURES' && failureCount > 0) return StateColors.DEAD;
    return null;
  };

  return (
    <div
      style={{
        position: 'absolute',
        top: '56px',
        left: '50%',
        transform: 'translateX(-50%)',
        display: 'flex',
        background: BaseColors.bg,
        border: `1px solid ${BaseColors.border}`,
        borderRadius: '3px',
        overflow: 'hidden',
        zIndex: 40,
        fontFamily: FontFamily.mono,
      }}
    >
      {MODES.map((mode, i) => {
        const isActive = activeView === mode.id;
        const badge = getBadgeColor(mode.id);
        return (
          <button
            key={mode.id}
            onClick={() => onChange(mode.id)}
            title={mode.description}
            style={{
              padding: '6px 14px',
              background: isActive ? BaseColors.surfaceElevated : 'transparent',
              border: 'none',
              borderLeft: i > 0 ? `1px solid ${BaseColors.border}` : 'none',
              color: isActive ? BaseColors.textPrimary : BaseColors.textMuted,
              fontFamily: FontFamily.mono,
              fontSize: FontSize.xs,
              fontWeight: isActive ? FontWeight.semibold : FontWeight.regular,
              cursor: 'pointer',
              letterSpacing: '0.04em',
              position: 'relative',
              transition: 'background 150ms ease, color 150ms ease',
              display: 'flex',
              alignItems: 'center',
              gap: '5px',
            }}
          >
            {mode.label}
            {badge && (
              <span
                style={{
                  width: '6px',
                  height: '6px',
                  borderRadius: '50%',
                  background: badge,
                  flexShrink: 0,
                }}
              />
            )}
          </button>
        );
      })}
    </div>
  );
};
