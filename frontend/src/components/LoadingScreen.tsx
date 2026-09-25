// LoadingScreen Component
// Operational language during initialization — no skeleton cards or spinners.

import React, { useEffect, useState } from 'react';
import { BaseColors } from '../design/tokens';
import { FontFamily, FontSize, FontWeight } from '../design/typography';
import { Shield } from 'lucide-react';

const STEPS = [
  'Connecting to coordinator...',
  'Loading cluster topology...',
  'Loading node state...',
  'Loading object metadata...',
  'Subscribing to live events...',
  'Initializing 3D scene...',
];

export const LoadingScreen: React.FC = () => {
  const [step, setStep] = useState(0);

  useEffect(() => {
    const iv = setInterval(() => {
      setStep((s) => Math.min(s + 1, STEPS.length - 1));
    }, 420);
    return () => clearInterval(iv);
  }, []);

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        background: BaseColors.bg,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        fontFamily: FontFamily.mono,
        zIndex: 9999,
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          marginBottom: '32px',
        }}
      >
        <Shield size={20} color="#38bdf8" />
        <span
          style={{
            fontSize: FontSize.xxl,
            fontWeight: FontWeight.bold,
            color: BaseColors.textPrimary,
            letterSpacing: '0.1em',
          }}
        >
          VAULT
        </span>
        <span
          style={{
            fontSize: FontSize.xs,
            color: BaseColors.textMuted,
            border: `1px solid ${BaseColors.border}`,
            padding: '1px 6px',
            borderRadius: '2px',
          }}
        >
          INITIALIZING
        </span>
      </div>

      <div
        style={{
          width: '320px',
          border: `1px solid ${BaseColors.border}`,
          borderRadius: '3px',
          overflow: 'hidden',
          marginBottom: '20px',
        }}
      >
        <div
          style={{
            height: '2px',
            background: '#38bdf8',
            width: `${((step + 1) / STEPS.length) * 100}%`,
            transition: 'width 400ms cubic-bezier(0.2, 0, 0, 1)',
          }}
        />
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px', width: '320px' }}>
        {STEPS.map((s, i) => (
          <div
            key={i}
            style={{
              fontSize: FontSize.sm,
              color:
                i < step
                  ? BaseColors.textMuted
                  : i === step
                  ? BaseColors.textPrimary
                  : '#1e293b',
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              transition: 'color 200ms ease',
            }}
          >
            <span
              style={{
                width: '6px',
                height: '6px',
                borderRadius: '50%',
                background:
                  i < step ? BaseColors.textMuted : i === step ? '#38bdf8' : '#1e293b',
                flexShrink: 0,
                transition: 'background 200ms ease',
              }}
            />
            {s}
          </div>
        ))}
      </div>
    </div>
  );
};
