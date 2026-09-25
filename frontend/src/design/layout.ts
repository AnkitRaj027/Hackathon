// Vault Layout Tokens
// Panel dimensions, spacing scales, z-index layers, and border radii.
// All values referenced from here — never hard-coded in components.

export const Spacing = {
  px: '1px',
  xxs: '2px',
  xs: '4px',
  sm: '6px',
  md: '8px',
  lg: '12px',
  xl: '16px',
  xxl: '24px',
  xxxl: '32px',
} as const;

export const Radius = {
  none: '0px',
  sm: '2px',
  md: '3px',
  lg: '4px',
  pill: '99px',
} as const;

export const ZIndex = {
  scene: 0,
  overlay: 10,
  panel: 30,
  inspector: 40,
  timeline: 41,
  topbar: 50,
  modal: 100,
  tooltip: 110,
} as const;

export const PanelDimensions = {
  topBarHeight: 46,
  timelineCollapsed: 36,
  timelineExpanded: 260,
  catalogWidth: 240,
  inspectorWidth: 380,
  repairPanelWidth: 320,
} as const;

export const Opacity = {
  ghost: 0.12,
  dim: 0.35,
  muted: 0.55,
  visible: 0.85,
  full: 1.0,
} as const;
