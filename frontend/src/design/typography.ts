// Vault Typography Tokens
// Section 48: Information-oriented, high legibility, monospace tabular numbers.

export const FontFamily = {
  sans: 'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
  mono: '"IBM Plex Mono", "JetBrains Mono", Consolas, Menlo, monospace',
} as const;

export const Typography = {
  fontSans: FontFamily.sans,
  fontMono: FontFamily.mono,

  // Text sizes
  sizeXs: '11px',
  sizeSm: '12px',
  sizeBase: '13px',
  sizeMd: '14px',
  sizeLg: '16px',
  sizeXl: '20px',

  // Font weights
  weightRegular: 400,
  weightMedium: 500,
  weightSemibold: 600,
  weightBold: 700,

  // Line heights
  leadingTight: 1.2,
  leadingNormal: 1.4,
  leadingRelaxed: 1.6,

  // Technical formatting
  tabularNumbers: {
    fontVariantNumeric: 'tabular-nums',
  },
} as const;
