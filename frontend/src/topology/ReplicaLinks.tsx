// ReplicaLinks 3D Component
// Visualizes object -> chunk -> replica topology and inter-replica quorum consensus.

import React, { useMemo } from 'react';
import * as THREE from 'three';
import { BaseColors, StateColors } from '../design/tokens';

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
  // Arcs from coordinator / object to each replica node
  const radialArcs = useMemo(() => {
    const lineColor = new THREE.Color(isParity ? '#a855f7' : StateColors.HEALTHY);
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
      mid.y += Math.max(2.2, start.distanceTo(end) * 0.25);

      const curve = new THREE.QuadraticBezierCurve3(start, mid, end);
      const geom = new THREE.BufferGeometry().setFromPoints(curve.getPoints(32));
      return new THREE.Line(geom, mat);
    });
  }, [sourcePos, targetPositions, isParity]);

  // Inter-replica quorum mesh connecting the replica nodes to each other (Section 21)
  const quorumLinks = useMemo(() => {
    if (targetPositions.length < 2) return [];

    const mat = new THREE.LineDashedMaterial({
      color: new THREE.Color(BaseColors.accent),
      dashSize: 0.4,
      gapSize: 0.2,
      transparent: true,
      opacity: 0.5,
    });

    const lines: THREE.Line[] = [];
    for (let i = 0; i < targetPositions.length; i++) {
      const nextIdx = (i + 1) % targetPositions.length;
      if (targetPositions.length === 2 && i === 1) break; // Don't duplicate line for 2 nodes

      const p1 = new THREE.Vector3(...targetPositions[i]);
      const p2 = new THREE.Vector3(...targetPositions[nextIdx]);
      p1.y += 0.5;
      p2.y += 0.5;

      const geom = new THREE.BufferGeometry().setFromPoints([p1, p2]);
      const line = new THREE.Line(geom, mat);
      line.computeLineDistances();
      lines.push(line);
    }
    return lines;
  }, [targetPositions]);

  return (
    <group>
      {radialArcs.map((lineObj, idx) => (
        <primitive key={`arc-${idx}`} object={lineObj} />
      ))}
      {quorumLinks.map((lineObj, idx) => (
        <primitive key={`quorum-${idx}`} object={lineObj} />
      ))}
    </group>
  );
};
