// StorageNodeChassis 3D Component
// Geometric, industrial server enclosure representing a real backend storage node.
// Visual state driven entirely by real node status — no decorative animation.

import React, { useRef, useState } from 'react';
import * as THREE from 'three';
import { useFrame } from '@react-three/fiber';
import { Html } from '@react-three/drei';
import { StorageNode } from '../state/types';
import { StateColors, BaseColors } from '../design/tokens';
import { MaterialRegistry } from '../design/materials';
import { LerpFactor } from '../design/motion';

interface StorageNodeProps {
  node: StorageNode;
  position: [number, number, number];
  isSelected: boolean;
  isHighlighted: boolean;
  isDimmed: boolean;
  onSelect: (nodeId: string) => void;
  chunkCount: number;
}

export const StorageNodeChassis: React.FC<StorageNodeProps> = ({
  node,
  position,
  isSelected,
  isHighlighted,
  isDimmed,
  onSelect,
  chunkCount,
}) => {
  const groupRef = useRef<THREE.Group>(null);
  const [hovered, setHovered] = useState(false);

  // Status-driven LED material (from MaterialRegistry — token-based)
  const ledMat = MaterialRegistry.getLedMaterial(node.status, isSelected);

  // Target opacity based on context:
  // - Selected / highlighted → full
  // - Dimmed (not related to current selection) → ghost
  // - Normal (no selection) → full
  const targetOpacity = isDimmed ? 0.18 : 1.0;
  const opacityRef = useRef(targetOpacity);

  // Subtle controlled pulsing for anomalous states (SUSPECT, DEGRADED, REPAIRING)
  // These animate because the real backend state is anomalous — not for decoration.
  useFrame((state) => {
    if (!groupRef.current) return;

    // Vertical pulse for active anomalous states
    const isAnomalous =
      node.status === 'SUSPECT' || node.status === 'DEGRADED' || node.status === 'REPAIRING';
    if (isAnomalous) {
      const t = state.clock.getElapsedTime();
      const pulse = Math.sin(t * 3) * 0.04;
      groupRef.current.position.y = position[1] + pulse;
    } else {
      groupRef.current.position.y = position[1];
    }

    // Smooth opacity transition for dimming
    opacityRef.current = THREE.MathUtils.lerp(opacityRef.current, targetOpacity, LerpFactor.opacity);
    groupRef.current.traverse((obj) => {
      if ((obj as THREE.Mesh).isMesh) {
        const mat = (obj as THREE.Mesh).material as THREE.MeshStandardMaterial;
        if (mat && mat.transparent !== undefined) {
          mat.transparent = true;
          mat.opacity = opacityRef.current;
        }
      }
    });
  });

  const stateColor = StateColors[node.status as keyof typeof StateColors] ?? StateColors.HEALTHY;
  const isOffline = node.status === 'DEAD';

  return (
    <group
      ref={groupRef}
      position={position}
      onClick={(e) => {
        e.stopPropagation();
        onSelect(node.id);
      }}
      onPointerOver={(e) => {
        e.stopPropagation();
        setHovered(true);
        document.body.style.cursor = 'pointer';
      }}
      onPointerOut={() => {
        setHovered(false);
        document.body.style.cursor = 'default';
      }}
    >
      {/* Selection / hover ring — only when selected or hovered */}
      {(isSelected || hovered || isHighlighted) && (
        <mesh position={[0, -0.48, 0]} rotation={[-Math.PI / 2, 0, 0]}>
          <ringGeometry args={[1.7, 1.85, 32]} />
          <meshBasicMaterial
            color={isSelected ? BaseColors.textPrimary : stateColor}
            transparent
            opacity={isSelected ? 0.9 : isHighlighted ? 0.6 : 0.35}
          />
        </mesh>
      )}

      {/* Main server chassis enclosure (2U industrial storage node) */}
      <mesh position={[0, 0, 0]} castShadow receiveShadow material={MaterialRegistry.chassisBase}>
        <boxGeometry args={[2.2, 0.9, 2.8]} />
      </mesh>

      {/* Front faceplate / bezel */}
      <mesh position={[0, 0, 1.41]} material={MaterialRegistry.chassisBezel}>
        <boxGeometry args={[2.16, 0.86, 0.05]} />
      </mesh>

      {/* Hot-swap drive bays (12 bays: 3 rows × 4 cols) */}
      {Array.from({ length: 12 }).map((_, i) => {
        const row = Math.floor(i / 4);
        const col = i % 4;
        const x = -0.75 + col * 0.5;
        const y = 0.25 - row * 0.25;
        const hasChunk = i < Math.min(chunkCount, 12);

        return (
          <group key={i} position={[x, y, 1.44]}>
            {/* Drive caddy bracket */}
            <mesh material={MaterialRegistry.baySlot}>
              <boxGeometry args={[0.42, 0.2, 0.02]} />
            </mesh>
            {/* Drive activity / presence LED */}
            <mesh position={[0.16, 0, 0.015]}>
              <boxGeometry args={[0.04, 0.08, 0.01]} />
              <meshStandardMaterial
                color={hasChunk && !isOffline ? StateColors.HEALTHY : BaseColors.borderHover}
                emissive={hasChunk && !isOffline ? StateColors.HEALTHY : BaseColors.bg}
                emissiveIntensity={hasChunk && !isOffline ? 0.8 : 0}
              />
            </mesh>
          </group>
        );
      })}

      {/* Main server status LED indicator bar */}
      <mesh position={[-0.95, 0.35, 1.44]} material={ledMat}>
        <boxGeometry args={[0.12, 0.06, 0.02]} />
      </mesh>

      {/* Rear cooling exhaust vents */}
      <mesh position={[0, 0, -1.41]} material={MaterialRegistry.baySlot}>
        <boxGeometry args={[2.0, 0.7, 0.04]} />
      </mesh>

      {/* HTML overlay label — compact technical format matching Section 14 */}
      <Html
        position={[0, 0.85, 0]}
        center
        distanceFactor={18}
        zIndexRange={[10, 40]}
        style={{
          pointerEvents: 'none',
          opacity: isDimmed ? 0.2 : 1,
          transition: 'opacity 200ms ease',
        }}
      >
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            gap: '2px',
            background: isSelected ? BaseColors.surfaceElevated : 'rgba(10, 15, 26, 0.88)',
            border: `1px solid ${isSelected ? BaseColors.textPrimary : isOffline ? StateColors.DEAD : BaseColors.border}`,
            padding: '4px 8px',
            borderRadius: '2px',
            fontFamily: '"IBM Plex Mono", monospace',
            fontSize: '11px',
            color: isOffline ? BaseColors.textMuted : BaseColors.textPrimary,
            whiteSpace: 'nowrap',
            boxShadow: isSelected ? '0 0 12px rgba(56, 189, 248, 0.3)' : 'none',
            userSelect: 'none',
            transition: 'border-color 200ms ease, box-shadow 200ms ease',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
            <span
              style={{
                width: '6px',
                height: '6px',
                borderRadius: '50%',
                background: stateColor,
                display: 'inline-block',
                flexShrink: 0,
              }}
            />
            <span style={{ fontWeight: 600, letterSpacing: '0.02em' }}>{node.id}</span>
            {node.is_partitioned && (
              <span
                style={{
                  background: `${StateColors.DEAD}25`,
                  color: StateColors.DEAD,
                  border: `1px solid ${StateColors.DEAD}50`,
                  padding: '0 3px',
                  borderRadius: '2px',
                  fontSize: '9px',
                  marginLeft: '2px',
                }}
              >
                PART
              </span>
            )}
          </div>
          <div
            style={{
              fontSize: '9px',
              color: isOffline ? StateColors.DEAD : BaseColors.textSecondary,
              paddingLeft: '11px',
              letterSpacing: '0.02em',
            }}
          >
            {isOffline
              ? 'DEAD'
              : `${node.status} · ${node.rtt_ms.toFixed(1)} ms · ${chunkCount} chk`}
          </div>
        </div>
      </Html>
    </group>
  );
};
