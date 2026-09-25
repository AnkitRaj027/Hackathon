// EventTimeline Component
// Real-time operational event stream driven by real backend SSE events.
// Clicking an event opens a full detail panel.

import React, { useState } from 'react';
import { StorageEvent } from '../state/types';
import { BaseColors, StateColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';
import { ChevronUp, ChevronDown, Activity, X } from 'lucide-react';

interface EventTimelineProps {
  events: StorageEvent[];
}

// ─── Badge config per event type ──────────────────────────────────────────

const getEventBadge = (type: string): { color: string; label: string } => {
  switch (type) {
    case 'NODE_STATE_CHANGED':
    case 'NODE_SUSPECTED':
      return { color: StateColors.SUSPECT, label: 'NODE' };
    case 'NODE_DEAD':
      return { color: StateColors.DEAD, label: 'DEAD' };
    case 'NODE_JOINED':
    case 'NODE_RECOVERED':
      return { color: StateColors.HEALTHY, label: 'NODE' };
    case 'CHUNK_REPLICATED':
    case 'REPLICATION_STARTED':
    case 'REPLICATION_COMPLETED':
      return { color: StateColors.HEALTHY, label: 'REPL' };
    case 'REPAIR_QUEUED':
      return { color: StateColors.STALE, label: 'REPAIR' };
    case 'REPAIR_STARTED':
    case 'REPAIR_PROGRESS':
      return { color: StateColors.REPAIRING, label: 'HEAL' };
    case 'REPAIR_COMPLETED':
      return { color: StateColors.HEALTHY, label: 'HEAL' };
    case 'REPAIR_FAILED':
      return { color: StateColors.DEAD, label: 'HEAL' };
    case 'CHECKSUM_FAILURE':
      return { color: StateColors.CORRUPTED, label: 'CORRUPT' };
    case 'OBJECT_STORED':
    case 'OBJECT_CREATED':
      return { color: StateColors.HEALTHY, label: 'PUT' };
    case 'OBJECT_DELETED':
      return { color: StateColors.DEAD, label: 'DEL' };
    case 'TOPOLOGY_CHANGED':
      return { color: StateColors.SUSPECT, label: 'TOPO' };
    case 'NETWORK_PARTITION':
      return { color: StateColors.DEAD, label: 'PART' };
    case 'NETWORK_HEALED':
      return { color: StateColors.REPAIRING, label: 'HEAL' };
    case 'NODE_HEARTBEAT':
      return { color: BaseColors.textMuted, label: 'HB' };
    default:
      return { color: BaseColors.textMuted, label: 'INFO' };
  }
};

// ─── Payload rendering for the detail panel ───────────────────────────────

const renderEventDetail = (ev: StorageEvent): React.ReactNode => {
  const badge = getEventBadge(ev.type);
  const p = ev.payload ?? {};

  const rows: [string, string][] = [];

  if (p.node_id ?? p.nodeId) rows.push(['NODE', p.node_id ?? p.nodeId]);
  if (p.status) rows.push(['STATUS', p.status]);
  if (p.chunk_id ?? p.chunkId) rows.push(['CHUNK', p.chunk_id ?? p.chunkId]);
  if (p.key) rows.push(['OBJECT', p.key]);
  if (p.source) rows.push(['SOURCE', p.source]);
  if (p.target) rows.push(['TARGET', p.target]);
  if (p.size !== undefined) rows.push(['SIZE', `${p.size} bytes`]);
  if (p.sha256) rows.push(['SHA-256', `${p.sha256.substring(0, 32)}...`]);
  if (p.expected) rows.push(['EXPECTED SHA', `${String(p.expected).substring(0, 32)}...`]);
  if (p.actual) rows.push(['ACTUAL SHA', `${String(p.actual).substring(0, 32)}...`]);
  if (p.rf_current !== undefined) rows.push(['RF', `${p.rf_current}/${p.rf_target ?? '?'}`]);
  if (p.reason) rows.push(['REASON', p.reason]);
  if (p.cause) rows.push(['CAUSE', p.cause]);
  if (p.action) rows.push(['ACTION', p.action]);
  if (p.scheme) rows.push(['SCHEME', p.scheme]);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <span
          style={{
            background: `${badge.color}20`,
            color: badge.color,
            border: `1px solid ${badge.color}50`,
            padding: '2px 6px',
            borderRadius: '2px',
            fontSize: FontSize.xs,
            fontWeight: FontWeight.semibold,
          }}
        >
          {ev.type}
        </span>
        <span style={{ color: BaseColors.textMuted, fontSize: FontSize.xs }}>
          {new Date(ev.timestamp).toLocaleTimeString(undefined, {
            hour12: false,
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
          })}
          {' '}
          {new Date(ev.timestamp).toLocaleDateString()}
        </span>
      </div>

      {rows.length > 0 && (
        <div
          style={{
            background: BaseColors.bg,
            border: `1px solid ${BaseColors.border}`,
            borderRadius: '2px',
            padding: '8px',
          }}
        >
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '100px 1fr',
              gap: '5px',
              fontSize: FontSize.xs,
            }}
          >
            {rows.map(([k, v]) => (
              <React.Fragment key={k}>
                <span style={{ color: BaseColors.textMuted }}>{k}</span>
                <span style={{ color: BaseColors.textPrimary, wordBreak: 'break-all' }}>{v}</span>
              </React.Fragment>
            ))}
          </div>
        </div>
      )}

      {/* Raw payload as fallback for unknown fields */}
      {Object.keys(p).length > 0 && rows.length === 0 && (
        <pre
          style={{
            background: BaseColors.bg,
            border: `1px solid ${BaseColors.border}`,
            borderRadius: '2px',
            padding: '8px',
            fontSize: FontSize.xxs,
            color: BaseColors.textSecondary,
            overflowX: 'auto',
            margin: 0,
          }}
        >
          {JSON.stringify(p, null, 2)}
        </pre>
      )}
    </div>
  );
};

// ─── Main Component ────────────────────────────────────────────────────────

export const EventTimeline: React.FC<EventTimelineProps> = ({ events }) => {
  const [expanded, setExpanded] = useState<boolean>(false);
  const [selectedEvent, setSelectedEvent] = useState<StorageEvent | null>(null);

  const latestEvent = events[0];

  return (
    <>
      {/* Detail overlay when an event is clicked */}
      {selectedEvent && (
        <div
          style={{
            position: 'absolute',
            bottom: '40px',
            left: '252px',
            width: '420px',
            background: BaseColors.surface,
            border: `1px solid ${BaseColors.border}`,
            borderRadius: '3px',
            zIndex: 45,
            fontFamily: FontFamily.mono,
            boxShadow: '0 8px 32px rgba(0,0,0,0.5)',
          }}
        >
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              padding: '8px 12px',
              borderBottom: `1px solid ${BaseColors.border}`,
            }}
          >
            <span
              style={{
                fontSize: FontSize.xs,
                fontWeight: FontWeight.semibold,
                color: BaseColors.textSecondary,
                letterSpacing: '0.04em',
              }}
            >
              EVENT DETAIL
            </span>
            <button
              onClick={() => setSelectedEvent(null)}
              style={{
                background: 'transparent',
                border: 'none',
                color: BaseColors.textMuted,
                cursor: 'pointer',
                padding: '2px',
                display: 'flex',
              }}
            >
              <X size={13} />
            </button>
          </div>
          <div style={{ padding: '12px' }}>{renderEventDetail(selectedEvent)}</div>
        </div>
      )}

      {/* Timeline Bar */}
      <div
        style={{
          position: 'absolute',
          bottom: 0,
          left: 0,
          right: 0,
          height: expanded ? '260px' : '36px',
          background: BaseColors.surface,
          backdropFilter: 'blur(8px)',
          borderTop: `1px solid ${BaseColors.border}`,
          display: 'flex',
          flexDirection: 'column',
          zIndex: 40,
          fontFamily: FontFamily.mono,
          fontSize: FontSize.sm,
          color: BaseColors.textPrimary,
          transition: 'height 200ms cubic-bezier(0.2, 0, 0, 1)',
        }}
      >
        {/* Collapsed Banner */}
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
            flexShrink: 0,
            borderBottom: expanded ? `1px solid ${BaseColors.border}` : 'none',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <Activity size={13} color={StateColors.HEALTHY} />
            <span
              style={{
                fontWeight: FontWeight.semibold,
                color: BaseColors.textSecondary,
                letterSpacing: '0.04em',
              }}
            >
              OPERATIONAL TIMELINE:
            </span>
            {latestEvent ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ color: BaseColors.textMuted, fontSize: FontSize.xs }}>
                  {new Date(latestEvent.timestamp).toLocaleTimeString()}
                </span>
                <span
                  style={{
                    background: `${getEventBadge(latestEvent.type).color}20`,
                    color: getEventBadge(latestEvent.type).color,
                    border: `1px solid ${getEventBadge(latestEvent.type).color}50`,
                    padding: '1px 5px',
                    borderRadius: '2px',
                    fontSize: FontSize.xs,
                    fontWeight: FontWeight.semibold,
                  }}
                >
                  {latestEvent.type}
                </span>
                <span style={{ color: BaseColors.textSecondary, fontSize: FontSize.xs }}>
                  {typeof latestEvent.payload === 'object'
                    ? (latestEvent.payload?.key ??
                       latestEvent.payload?.node_id ??
                       latestEvent.payload?.chunk_id ??
                       latestEvent.payload?.reason ??
                       '')
                    : String(latestEvent.payload ?? '')}
                </span>
              </div>
            ) : (
              <span style={{ color: BaseColors.textMuted, fontSize: FontSize.xs }}>
                Listening for real-time telemetry events...
              </span>
            )}
          </div>

          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              color: BaseColors.textMuted,
              fontSize: FontSize.xs,
            }}
          >
            <span>{events.length} events</span>
            {expanded ? <ChevronDown size={14} /> : <ChevronUp size={14} />}
          </div>
        </div>

        {/* Expanded Event Stream */}
        {expanded && (
          <div style={{ flex: 1, overflowY: 'auto', padding: '0' }}>
            {events.length === 0 ? (
              <div
                style={{
                  color: BaseColors.textMuted,
                  padding: '12px 14px',
                  fontSize: FontSize.xs,
                }}
              >
                No operational events recorded in this session.
              </div>
            ) : (
              events.map((ev, idx) => {
                const badge = getEventBadge(ev.type);
                const isSelected = selectedEvent === ev;
                return (
                  <div
                    key={idx}
                    onClick={() => setSelectedEvent(isSelected ? null : ev)}
                    style={{
                      display: 'flex',
                      alignItems: 'baseline',
                      gap: '10px',
                      padding: '4px 14px',
                      borderBottom: `1px solid ${BaseColors.bg}`,
                      fontSize: FontSize.xs,
                      cursor: 'pointer',
                      background: isSelected
                        ? `${badge.color}10`
                        : 'transparent',
                      transition: 'background 100ms ease',
                    }}
                  >
                    {/* Timestamp */}
                    <span
                      style={{
                        color: BaseColors.textMuted,
                        width: '75px',
                        flexShrink: 0,
                        fontVariantNumeric: 'tabular-nums',
                      }}
                    >
                      {new Date(ev.timestamp).toLocaleTimeString(undefined, {
                        hour12: false,
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                      })}
                    </span>

                    {/* Source hint */}
                    <span
                      style={{
                        color: BaseColors.textMuted,
                        width: '90px',
                        flexShrink: 0,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {ev.payload?.node_id ?? ev.payload?.source ?? ev.source ?? '—'}
                    </span>

                    {/* Badge */}
                    <span
                      style={{
                        background: `${badge.color}20`,
                        color: badge.color,
                        border: `1px solid ${badge.color}40`,
                        padding: '0px 4px',
                        borderRadius: '2px',
                        fontSize: FontSize.xxs,
                        fontWeight: FontWeight.semibold,
                        width: '56px',
                        textAlign: 'center',
                        flexShrink: 0,
                      }}
                    >
                      {badge.label}
                    </span>

                    {/* Event type */}
                    <span
                      style={{
                        color: BaseColors.textSecondary,
                        flexShrink: 0,
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {ev.type}
                    </span>

                    {/* Payload summary */}
                    <span
                      style={{
                        color: BaseColors.textMuted,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                        fontSize: FontSize.xxs,
                      }}
                    >
                      {typeof ev.payload === 'object'
                        ? (ev.payload?.key ?? ev.payload?.chunk_id ?? ev.payload?.reason ?? '')
                        : String(ev.payload ?? '')}
                    </span>
                  </div>
                );
              })
            )}
          </div>
        )}
      </div>
    </>
  );
};
