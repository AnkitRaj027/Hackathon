// Vault Design Tokens
// Minimalistic, professional design tokens for infrastructure observability.

export const StateColors = {
  HEALTHY: '#38bdf8',     // Clean precision sky blue
  DEGRADED: '#eab308',    // Restrained industrial amber
  SUSPECT: '#f97316',     // Muted warning orange
  DEAD: '#ef4444',        // Controlled crimson
  REPAIRING: '#10b981',   // Crisp emerald green
  STALE: '#64748b',       // Neutral slate
  CORRUPTED: '#dc2626',   // High-urgency error red
  SELECTED: '#ffffff',    // High-contrast pure highlight
} as const;

export type SystemState = keyof typeof StateColors;

export const BaseColors = {
  bg: '#090a0f',              // Deep neutral carbon/obsidian
  surface: '#0f1117',         // Matte industrial panel surface
  surfaceElevated: '#151821', // Clean elevated layer / hover
  surfaceCard: '#12141c',     // Card surface
  border: '#1c202a',          // Crisp 1px boundary
  borderHover: '#2c3242',     // Hover boundary
  borderActive: '#3d4559',    // Active / selection boundary
  textPrimary: '#f3f4f6',     // Crisp primary text
  textSecondary: '#9ca3af',   // Secondary metadata
  textMuted: '#525966',       // Muted technical labels
  accent: '#38bdf8',          // Restrained precision accent
  ringTrack: '#181b24',       // Subtle ring track
  ringActive: '#38bdf8',      // Active ring highlight
  linkCurve: '#232938',       // Subtle topology arcs
  linkHighlight: '#38bdf8',   // Selected replica links
} as const;

export const CameraTokens = {
  fov: 42,
  near: 0.1,
  far: 500,
  defaultPosition: [0, 16, 26] as [number, number, number],
  defaultTarget: [0, 0, 0] as [number, number, number],
  nodeFocusDistance: 6,
  minDistance: 5,
  maxDistance: 70,
  maxPolarAngle: Math.PI / 2.05,
} as const;

export const TopologyTokens = {
  ringRadius: 10,
  ringThickness: 0.03,
  nodeChassisWidth: 2.2,
  nodeChassisHeight: 1.0,
  nodeChassisDepth: 2.8,
  chunkSlotWidth: 0.22,
  chunkSlotHeight: 0.45,
  chunkSlotDepth: 0.65,
  dataParticleSpeed: 1.8,
  lineThickness: 1.2,
  vnodeMarkerCount: 128,
} as const;

export const PanelTokens = {
  topStripHeight: 48,
  bottomTimelineHeight: 38,
  bottomTimelineExpandedHeight: 260,
  inspectorWidth: 400,
  operationsPanelWidth: 320,
  modalZIndex: 100,
  inspectorZIndex: 50,
} as const;
