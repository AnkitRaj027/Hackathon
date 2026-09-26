// Vault Typography Tokens
// Standard, professional typography pairing Inter for interface clarity with
// JetBrains Mono for precision technical data and telemetry.

export const FontFamily = {
  sans: '"Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", sans-serif',
  mono: '"JetBrains Mono", "SF Mono", "Cascadia Code", Menlo, Consolas, monospace',
} as const;

export const FontSize = {
  xxs: '11px',
  xs: '12px',
  sm: '13px',
  md: '14px',
  lg: '15px',
  xl: '17px',
  xxl: '20px',
  display: '24px',
} as const;

export const FontWeight = {
  regular: 400,
  medium: 500,
  semibold: 600,
  bold: 700,
} as const;

export const LineHeight = {
  tight: 1.3,
  normal: 1.5,
  relaxed: 1.65,
} as const;

export const LetterSpacing = {
  tight: '-0.015em',
  normal: '0em',
  wide: '0.025em',
  wider: '0.05em',
} as const;
