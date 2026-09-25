// ReplicaLinks 3D Component
// Section 56 & 63: Curved 3D arcs highlighting replica relationships for selected entities.

import React, { useMemo } from 'react';
import * as THREE from 'three';
import { BaseColors } from '../design/tokens';

interface ReplicaLinksProps {
  sourcePos: [number, number, number];
  targetPositions: [number, number, number][];
  isParity?: boolean;
}

export const ReplicaLinks: React.FC<ReplicaLinksProps> = ({
  sourcePos,
  targetPositions,
  isParity = false,
}) => {
  const lines = useMemo(() => {
    const lineColor = new THREE.Color(isParity ? '#a855f7' : BaseColors.linkHighlight);
    const mat = new THREE.LineBasicMaterial({
      color: lineColor,
      transparent: true,
      opacity: 0.85,
    });

    return targetPositions.map((target) => {
      const start = new THREE.Vector3(...sourcePos);
      const end = new THREE.Vector3(...target);

      // Midpoint raised into 3D arc
      const mid = new THREE.Vector3()
        .addVectors(start, end)
        .multiplyScalar(0.5);
      mid.y += Math.max(2.5, start.distanceTo(end) * 0.28);

      const curve = new THREE.QuadraticBezierCurve3(start, mid, end);
      const geom = new THREE.BufferGeometry().setFromPoints(curve.getPoints(36));
      return new THREE.Line(geom, mat);
    });
  }, [sourcePos, targetPositions, isParity]);

  return (
    <group>
      {lines.map((lineObj, idx) => (
        <primitive key={idx} object={lineObj} />
      ))}
    </group>
  );
};
