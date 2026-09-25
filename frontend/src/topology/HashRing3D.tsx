// HashRing3D Component
// Section 52: Spatial Consistent Hash Ring visualization with virtual node distribution.

import React, { useMemo } from 'react';
import * as THREE from 'three';
import { Html } from '@react-three/drei';
import { TopologyTokens, BaseColors } from '../design/tokens';
import { MaterialRegistry } from '../design/materials';

interface HashRing3DProps {
  radius?: number;
  vnodePointsCount?: number;
}

export const HashRing3D: React.FC<HashRing3DProps> = ({
  radius = TopologyTokens.ringRadius,
  vnodePointsCount = 64,
}) => {
  // Precompute virtual node tick markers around the circle
  const tickPoints = useMemo(() => {
    const points: [number, number, number][] = [];
    for (let i = 0; i < vnodePointsCount; i++) {
      const angle = (i / vnodePointsCount) * Math.PI * 2;
      const x = Math.cos(angle) * radius;
      const z = Math.sin(angle) * radius;
      points.push([x, -0.45, z]);
    }
    return points;
  }, [radius, vnodePointsCount]);

  return (
    <group>
      {/* 3D Circular Ring Track */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.48, 0]} material={MaterialRegistry.ringMaterial}>
        <ringGeometry args={[radius - 0.08, radius + 0.08, 128]} />
      </mesh>

      {/* Subtle Virtual Node Markers */}
      {tickPoints.map((pos, idx) => (
        <mesh key={idx} position={pos} material={MaterialRegistry.ringMarkerMaterial}>
          <boxGeometry args={[0.08, 0.02, 0.2]} />
        </mesh>
      ))}

      {/* Token Coordinate Compass Points */}
      <Html position={[radius + 1.2, -0.4, 0]} center style={{ pointerEvents: 'none' }}>
        <div style={{ fontFamily: '"IBM Plex Mono", monospace', fontSize: '9px', color: BaseColors.textMuted }}>
          0x00000000 (0°)
        </div>
      </Html>
      <Html position={[0, -0.4, radius + 1.2]} center style={{ pointerEvents: 'none' }}>
        <div style={{ fontFamily: '"IBM Plex Mono", monospace', fontSize: '9px', color: BaseColors.textMuted }}>
          0x40000000 (90°)
        </div>
      </Html>
      <Html position={[-radius - 1.2, -0.4, 0]} center style={{ pointerEvents: 'none' }}>
        <div style={{ fontFamily: '"IBM Plex Mono", monospace', fontSize: '9px', color: BaseColors.textMuted }}>
          0x80000000 (180°)
        </div>
      </Html>
      <Html position={[0, -0.4, -radius - 1.2]} center style={{ pointerEvents: 'none' }}>
        <div style={{ fontFamily: '"IBM Plex Mono", monospace', fontSize: '9px', color: BaseColors.textMuted }}>
          0xC0000000 (270°)
        </div>
      </Html>
    </group>
  );
};
