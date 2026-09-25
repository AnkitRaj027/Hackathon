// EventTimeline Component
// Section 69: Compact real-time event stream driven by real backend SSE events.

import React, { useState } from 'react';
import { StorageEvent } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { ChevronUp, ChevronDown, Activity, Terminal } from 'lucide-react';

interface EventTimelineProps {
  events: StorageEvent[];
}

export const EventTimeline: React.FC<EventTimelineProps> = ({ events }) => {
  const [expanded, setExpanded] = useState<boolean>(false);

  const latestEvent = events[0];

  const getEventBadge = (type: string) => {
    switch (type) {
      case 'NODE_STATE_CHANGED':
        return { color: StateColors.SUSPECT, label: 'NODE' };
      case 'CHUNK_REPLICATED':
        return { color: StateColors.HEALTHY, label: 'REPL' };
      case 'REPAIR_STARTED':
      case 'REPAIR_COMPLETED':
        return { color: StateColors.REPAIRING, label: 'HEAL' };
      case 'CHECKSUM_FAILURE':
        return { color: StateColors.CORRUPTED, label: 'CORRUPT' };
      case 'OBJECT_STORED':
        return { color: '#38bdf8', label: 'PUT' };
      case 'OBJECT_DELETED':
        return { color: '#f87171', label: 'DEL' };
      default:
        return { color: BaseColors.textMuted, label: 'INFO' };
    }
  };

  return (
    <div
      style={{
        position: 'absolute',
        bottom: 0,
        left: 0,
        right: 0,
        height: expanded ? '220px' : '36px',
        background: 'rgba(10, 15, 26, 0.95)',
        backdropFilter: 'blur(8px)',
        borderTop: `1px solid ${BaseColors.border}`,
        display: 'flex',
        flexDirection: 'column',
        zIndex: 40,
        fontFamily: '"IBM Plex Mono", monospace',
        fontSize: '11px',
        color: BaseColors.textPrimary,
        transition: 'height 200ms cubic-bezier(0.2, 0, 0, 1)',
      }}
    >
      {/* Top Banner Bar */}
      <div
        onClick={() => setExpanded(!expanded)}
        style={{
          height: '36px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '0 14px',
          cursor: 'pointer',
          userSelect: 'none',
          borderBottom: expanded ? `1px solid ${BaseColors.border}` : 'none',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <Activity size={13} color="#38bdf8" />
          <span style={{ fontWeight: 600, color: BaseColors.textSecondary }}>OPERATIONAL TIMELINE:</span>
          {latestEvent ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span style={{ color: BaseColors.textMuted }}>
                {new Date(latestEvent.timestamp).toLocaleTimeString()}
              </span>
              <span
                style={{
                  background: `${getEventBadge(latestEvent.type).color}20`,
                  color: getEventBadge(latestEvent.type).color,
                  border: `1px solid ${getEventBadge(latestEvent.type).color}50`,
                  padding: '1px 5px',
                  borderRadius: '2px',
                  fontSize: '10px',
                  fontWeight: 600,
                }}
              >
                {latestEvent.type}
              </span>
              <span style={{ color: BaseColors.textPrimary }}>
                {typeof latestEvent.payload === 'object'
                  ? JSON.stringify(latestEvent.payload)
                  : latestEvent.payload}
              </span>
            </div>
          ) : (
            <span style={{ color: BaseColors.textMuted }}>Listening for real-time telemetry events...</span>
          )}
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', color: BaseColors.textMuted }}>
          <span>{events.length} events</span>
          {expanded ? <ChevronDown size={14} /> : <ChevronUp size={14} />}
        </div>
      </div>

      {/* Expanded Real-Time Log Stream */}
      {expanded && (
        <div style={{ flex: 1, overflowY: 'auto', padding: '8px 14px' }}>
          {events.length === 0 ? (
            <div style={{ color: BaseColors.textMuted, padding: '12px 0' }}>
              No operational events recorded in this session.
            </div>
          ) : (
            events.map((ev, idx) => {
              const badge = getEventBadge(ev.type);
              return (
                <div
                  key={idx}
                  style={{
                    display: 'flex',
                    alignItems: 'baseline',
                    gap: '10px',
                    padding: '3px 0',
                    borderBottom: '1px solid rgba(30, 41, 59, 0.4)',
                    fontSize: '11px',
                  }}
                >
                  <span style={{ color: BaseColors.textMuted, width: '70px', flexShrink: 0 }}>
                    {new Date(ev.timestamp).toLocaleTimeString()}
                  </span>
                  <span
                    style={{
                      background: `${badge.color}20`,
                      color: badge.color,
                      border: `1px solid ${badge.color}40`,
                      padding: '1px 4px',
                      borderRadius: '2px',
                      fontSize: '9px',
                      fontWeight: 600,
                      width: '65px',
                      textAlign: 'center',
                      flexShrink: 0,
                    }}
                  >
                    {badge.label}
                  </span>
                  <span style={{ color: BaseColors.textSecondary, wordBreak: 'break-all' }}>
                    {typeof ev.payload === 'object' ? JSON.stringify(ev.payload) : ev.payload}
                  </span>
                </div>
              );
            })
          )}
        </div>
      )}
    </div>
  );
};
