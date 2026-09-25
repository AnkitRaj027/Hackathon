// Vault Motion Tokens
// All animation durations and easing curves — no ad-hoc transition values in components.

export const Duration = {
  instant: 0,
  fast: 100,      // ms — micro feedback
  normal: 200,    // ms — default UI transitions
  slow: 350,      // ms — panel slide-ins
  xslow: 500,     // ms — camera movement / state transitions
} as const;

export const Easing = {
  linear: 'linear',
  easeOut: 'cubic-bezier(0.2, 0, 0, 1)',
  easeIn: 'cubic-bezier(0.4, 0, 1, 1)',
  easeInOut: 'cubic-bezier(0.4, 0, 0.2, 1)',
  spring: 'cubic-bezier(0.34, 1.56, 0.64, 1)',
} as const;

// Three.js lerp factor for per-frame animations (use in useFrame)
// A value of 0.05 gives smooth, restrained interpolation.
export const LerpFactor = {
  camera: 0.05,
  emissive: 0.08,
  opacity: 0.06,
} as const;
