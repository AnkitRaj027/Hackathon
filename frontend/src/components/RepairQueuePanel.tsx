// RepairQueuePanel Component
// Compact, inline repair operations status — not a generic card grid.
// Driven entirely by real backend repair events via useVaultState.

import React, { useState } from 'react';
import { RepairTask } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';
import { Wrench, ChevronDown, ChevronUp, CheckCircle2, XCircle, Clock, Activity } from 'lucide-react';

interface RepairQueuePanelProps {
  tasks: RepairTask[];
}

const STATUS_ICON: Record<string, React.ReactNode> = {
  QUEUED: <Clock size={11} color={StateColors.STALE} />,
  REPAIRING: <Activity size={11} color={StateColors.REPAIRING} />,
  COMPLETED: <CheckCircle2 size={11} color={StateColors.HEALTHY} />,
  FAILED: <XCircle size={11} color={StateColors.DEAD} />,
};

const STATUS_COLOR: Record<string, string> = {
  QUEUED: StateColors.STALE,
  REPAIRING: StateColors.REPAIRING,
  COMPLETED: StateColors.HEALTHY,
  FAILED: StateColors.DEAD,
};

export const RepairQueuePanel: React.FC<RepairQueuePanelProps> = ({ tasks }) => {
  const [expanded, setExpanded] = useState(true);

  const active = tasks.filter((t) => t.status === 'REPAIRING').length;
  const queued = tasks.filter((t) => t.status === 'QUEUED').length;
  const failed = tasks.filter((t) => t.status === 'FAILED').length;

  if (tasks.length === 0) return null;

  return (
    <div
      style={{
        position: 'absolute',
        top: '92px',
        right: '12px',
        width: '320px',
        background: BaseColors.surface,
        border: `1px solid ${BaseColors.border}`,
        borderLeft: `3px solid ${active > 0 ? StateColors.REPAIRING : StateColors.STALE}`,
        borderRadius: '3px',
        zIndex: 39,
        fontFamily: FontFamily.mono,
        color: BaseColors.textPrimary,
        boxShadow: '0 4px 20px rgba(0,0,0,0.4)',
        transition: 'border-left-color 300ms ease',
      }}
    >
      {/* Header */}
      <div
        onClick={() => setExpanded((e) => !e)}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '8px 10px',
          cursor: 'pointer',
          userSelect: 'none',
          borderBottom: expanded ? `1px solid ${BaseColors.border}` : 'none',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '7px' }}>
          <Wrench size={12} color={active > 0 ? StateColors.REPAIRING : BaseColors.textMuted} />
          <span
            style={{
              fontSize: FontSize.xs,
              fontWeight: FontWeight.semibold,
              letterSpacing: '0.04em',
              color: BaseColors.textSecondary,
            }}
          >
            REPAIR QUEUE
          </span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          {active > 0 && (
            <span style={{ fontSize: FontSize.xxs, color: StateColors.REPAIRING }}>
              {active} ACTIVE
            </span>
          )}
          {queued > 0 && (
            <span style={{ fontSize: FontSize.xxs, color: StateColors.STALE }}>
              {queued} QUEUED
            </span>
          )}
          {failed > 0 && (
            <span style={{ fontSize: FontSize.xxs, color: StateColors.DEAD }}>
              {failed} FAILED
            </span>
          )}
          {expanded ? (
            <ChevronUp size={12} color={BaseColors.textMuted} />
          ) : (
            <ChevronDown size={12} color={BaseColors.textMuted} />
          )}
        </div>
      </div>

      {/* Task List */}
      {expanded && (
        <div style={{ maxHeight: '240px', overflowY: 'auto' }}>
          {tasks.map((task) => {
            const statusColor = STATUS_COLOR[task.status] ?? BaseColors.textMuted;
            return (
              <div
                key={task.id}
                style={{
                  padding: '7px 10px',
                  borderBottom: `1px solid ${BaseColors.border}`,
                  fontSize: FontSize.xs,
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '3px',
                  background:
                    task.status === 'REPAIRING'
                      ? `${StateColors.REPAIRING}08`
                      : 'transparent',
                  transition: 'background 200ms ease',
                }}
              >
                {/* Object / chunk ID */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  {STATUS_ICON[task.status]}
                  <span
                    style={{
                      color: BaseColors.textPrimary,
                      fontWeight: FontWeight.semibold,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      maxWidth: '190px',
                    }}
                    title={task.chunk_id}
                  >
                    {task.object_key}
                  </span>
                  <span
                    style={{
                      fontSize: FontSize.xxs,
                      color: statusColor,
                      background: `${statusColor}15`,
                      border: `1px solid ${statusColor}30`,
                      padding: '0px 4px',
                      borderRadius: '2px',
                      marginLeft: 'auto',
                      flexShrink: 0,
                    }}
                  >
                    {task.status}
                  </span>
                </div>

                {/* RF and node info */}
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '50px 1fr',
                    gap: '2px',
                    color: BaseColors.textMuted,
                    fontSize: FontSize.xxs,
                  }}
                >
                  <span>RF</span>
                  <span style={{ color: BaseColors.textSecondary }}>
                    {task.rf_current}/{task.rf_target}
                  </span>
                  <span>SOURCE</span>
                  <span style={{ color: BaseColors.textSecondary }}>{task.source}</span>
                  <span>TARGET</span>
                  <span style={{ color: BaseColors.textSecondary }}>{task.target}</span>
                </div>

                {/* Active repair progress indicator */}
                {task.status === 'REPAIRING' && (
                  <div
                    style={{
                      height: '2px',
                      background: BaseColors.border,
                      borderRadius: '1px',
                      marginTop: '2px',
                      overflow: 'hidden',
                    }}
                  >
                    <div
                      style={{
                        height: '100%',
                        background: StateColors.REPAIRING,
                        width: '40%',
                        borderRadius: '1px',
                        animation: 'repair-progress 1.5s ease-in-out infinite alternate',
                      }}
                    />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      <style>{`
        @keyframes repair-progress {
          from { margin-left: 0; width: 40%; }
          to   { margin-left: 60%; width: 40%; }
        }
      `}</style>
    </div>
  );
};
