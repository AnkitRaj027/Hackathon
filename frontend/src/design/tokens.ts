// Vault Design Tokens
// Section 45: Centralized tokens for technical precision and operational clarity.

export const StateColors = {
  HEALTHY: '#38bdf8',     // Cool blue/cyan
  DEGRADED: '#f59e0b',    // Amber
  SUSPECT: '#f97316',     // Muted orange
  DEAD: '#ef4444',        // Restrained crimson
  REPAIRING: '#10b981',   // Active emerald green
  STALE: '#64748b',       // Neutral slate
  CORRUPTED: '#dc2626',   // High-urgency error red
  SELECTED: '#f8fafc',    // High-contrast pure highlight
} as const;

export type SystemState = keyof typeof StateColors;

export const BaseColors = {
  bg: '#070a10',
  surface: '#0d131f',
  surfaceElevated: '#131b2e',
  surfaceCard: '#0f172a',
  border: '#1e293b',
  borderHover: '#334155',
  borderActive: '#475569',
  textPrimary: '#f8fafc',
  textSecondary: '#94a3b8',
  textMuted: '#64748b',
  accent: '#38bdf8',
  ringTrack: '#1e293b',
  ringActive: '#38bdf8',
  linkCurve: '#334155',
  linkHighlight: '#38bdf8',
} as const;

export const CameraTokens = {
  fov: 45,
  near: 0.1,
  far: 500,
  defaultPosition: [0, 16, 26] as [number, number, number],
  defaultTarget: [0, 0, 0] as [number, number, number],
  nodeFocusDistance: 6,
  minDistance: 5,
  maxDistance: 70,
  maxPolarAngle: Math.PI / 2.05, // Prevent going below ground grid
} as const;

export const TopologyTokens = {
  ringRadius: 10,
  ringThickness: 0.04,
  nodeChassisWidth: 2.2,
  nodeChassisHeight: 1.0,
  nodeChassisDepth: 2.8,
  chunkSlotWidth: 0.22,
  chunkSlotHeight: 0.45,
  chunkSlotDepth: 0.65,
  dataParticleSpeed: 1.8,
  lineThickness: 1.5,
  vnodeMarkerCount: 64,
} as const;

export const PanelTokens = {
  topStripHeight: 48,
  bottomTimelineHeight: 42,
  bottomTimelineExpandedHeight: 220,
  inspectorWidth: 380,
  modalZIndex: 100,
  inspectorZIndex: 50,
} as const;
