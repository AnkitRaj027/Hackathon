// StorageNodeChassis 3D Component
// Section 53 & 54: Geometric, industrial, recognizable storage node server enclosure.

import React, { useRef, useState } from 'react';
import * as THREE from 'three';
import { useFrame } from '@react-three/fiber';
import { Html } from '@react-three/drei';
import { StorageNode } from '../state/types';
import { StateColors, BaseColors } from '../design/tokens';
import { MaterialRegistry } from '../design/materials';

interface StorageNodeProps {
  node: StorageNode;
  position: [number, number, number];
  isSelected: boolean;
  onSelect: (nodeId: string) => void;
  chunkCount: number;
}

export const StorageNodeChassis: React.FC<StorageNodeProps> = ({
  node,
  position,
  isSelected,
  onSelect,
  chunkCount,
}) => {
  const meshRef = useRef<THREE.Group>(null);
  const [hovered, setHovered] = useState(false);

  // Status-driven LED material
  const ledMat = MaterialRegistry.getLedMaterial(node.status, isSelected);

  // Subtle pulsing animation if degraded/suspect/repairing
  useFrame((state) => {
    if (meshRef.current && (node.status === 'SUSPECT' || node.status === 'DEGRADED' || node.status === 'REPAIRING')) {
      const t = state.clock.getElapsedTime();
      const pulse = (Math.sin(t * 4) + 1) * 0.5;
      meshRef.current.position.y = position[1] + pulse * 0.08;
    } else if (meshRef.current) {
      meshRef.current.position.y = position[1];
    }
  });

  const stateColor = StateColors[node.status] || StateColors.HEALTHY;
  const isOffline = node.status === 'DEAD';

  return (
    <group
      ref={meshRef}
      position={position}
      onClick={(e) => {
        e.stopPropagation();
        onSelect(node.id);
      }}
      onPointerOver={(e) => {
        e.stopPropagation();
        setHovered(true);
      }}
      onPointerOut={() => setHovered(false)}
    >
      {/* Selection / Hover Indicator Ring */}
      {(isSelected || hovered) && (
        <mesh position={[0, -0.48, 0]} rotation={[-Math.PI / 2, 0, 0]}>
          <ringGeometry args={[1.7, 1.85, 32]} />
          <meshBasicMaterial
            color={isSelected ? BaseColors.textPrimary : stateColor}
            transparent
            opacity={isSelected ? 0.9 : 0.4}
          />
        </mesh>
      )}

      {/* Main Server Chassis Enclosure (2U Industrial Storage Node) */}
      <mesh position={[0, 0, 0]} castShadow receiveShadow material={MaterialRegistry.chassisBase}>
        <boxGeometry args={[2.2, 0.9, 2.8]} />
      </mesh>

      {/* Front Faceplate / Bezel */}
      <mesh position={[0, 0, 1.41]} material={MaterialRegistry.chassisBezel}>
        <boxGeometry args={[2.16, 0.86, 0.05]} />
      </mesh>

      {/* Hot-Swap Drive Bays (12 Bays: 3 rows x 4 cols) */}
      {Array.from({ length: 12 }).map((_, i) => {
        const row = Math.floor(i / 4);
        const col = i % 4;
        const x = -0.75 + col * 0.5;
        const y = 0.25 - row * 0.25;
        const hasChunk = i < Math.min(chunkCount, 12);

        return (
          <group key={i} position={[x, y, 1.44]}>
            {/* Drive Caddy Bracket */}
            <mesh material={MaterialRegistry.baySlot}>
              <boxGeometry args={[0.42, 0.2, 0.02]} />
            </mesh>
            {/* Drive Activity / Presence LED */}
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

      {/* Main Server Status LED Indicator Bar */}
      <mesh position={[-0.95, 0.35, 1.44]} material={ledMat}>
        <boxGeometry args={[0.12, 0.06, 0.02]} />
      </mesh>

      {/* Rear Cooling Exhaust Vents */}
      <mesh position={[0, 0, -1.41]} material={MaterialRegistry.baySlot}>
        <boxGeometry args={[2.0, 0.7, 0.04]} />
      </mesh>

      {/* Real-Time HTML Overlay Label (Technical Node Badge) */}
      <Html
        position={[0, 0.8, 0]}
        center
        distanceFactor={18}
        zIndexRange={[10, 40]}
        style={{ pointerEvents: 'none' }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            background: isSelected ? BaseColors.surfaceCard : BaseColors.surface,
            border: `1px solid ${isSelected ? stateColor : BaseColors.border}`,
            padding: '3px 8px',
            borderRadius: '3px',
            fontFamily: '"IBM Plex Mono", monospace',
            fontSize: '11px',
            color: isOffline ? BaseColors.textMuted : BaseColors.textPrimary,
            whiteSpace: 'nowrap',
            boxShadow: isSelected ? `0 0 12px ${stateColor}40` : 'none',
            userSelect: 'none',
          }}
        >
          <span
            style={{
              width: '6px',
              height: '6px',
              borderRadius: '50%',
              background: stateColor,
              display: 'inline-block',
            }}
          />
          <span style={{ fontWeight: 600 }}>{node.id}</span>
          <span style={{ color: BaseColors.textMuted }}>•</span>
          <span style={{ color: BaseColors.textSecondary }}>{chunkCount} chk</span>
          {node.rtt_ms > 0 && !isOffline && (
            <span style={{ color: StateColors.HEALTHY, fontSize: '10px' }}>
              {node.rtt_ms.toFixed(1)}ms
            </span>
          )}
        </div>
      </Html>
    </group>
  );
};
