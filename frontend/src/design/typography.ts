// Vault Typography Tokens
// All font values centralized — no ad-hoc font sizes scattered in components.

export const FontFamily = {
  mono: '"IBM Plex Mono", "JetBrains Mono", "Fira Code", monospace',
  sans: '"Inter", "Helvetica Neue", sans-serif',
} as const;

export const FontSize = {
  xxs: '9px',
  xs: '10px',
  sm: '11px',
  md: '12px',
  lg: '13px',
  xl: '14px',
  xxl: '16px',
} as const;

export const FontWeight = {
  regular: 400,
  medium: 500,
  semibold: 600,
  bold: 700,
} as const;

export const LineHeight = {
  tight: 1.2,
  normal: 1.5,
  relaxed: 1.75,
} as const;

export const LetterSpacing = {
  tight: '-0.02em',
  normal: '0em',
  wide: '0.04em',
  wider: '0.08em',
} as const;
