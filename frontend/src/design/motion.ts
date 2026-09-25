// Vault Motion Tokens
// Section 77: Intentional, event-driven motion curves.

export const Motion = {
  // Durations (ms)
  durationFast: 150,
  durationMedium: 400,
  durationSlow: 1200,

  // CSS Timing functions
  easeOutCubic: 'cubic-bezier(0.215, 0.61, 0.355, 1)',
  easeInOutCubic: 'cubic-bezier(0.645, 0.045, 0.355, 1)',
  linear: 'linear',

  // Three.js animation lerp factors
  cameraLerp: 0.06,
  particleLerp: 0.04,
} as const;
