// DataFlowParticles 3D Component
// Section 57 & 58: Event-driven data movement particles strictly tied to real backend operations.

import React, { useRef, useMemo } from 'react';
import * as THREE from 'three';
import { useFrame } from '@react-three/fiber';
import { StateColors } from '../design/tokens';

interface DataFlowParticlesProps {
  sourcePos: [number, number, number];
  targetPos: [number, number, number];
  isRepair?: boolean;
}

export const DataFlowParticles: React.FC<DataFlowParticlesProps> = ({
  sourcePos,
  targetPos,
  isRepair = false,
}) => {
  const meshRef = useRef<THREE.Mesh>(null);
  const progressRef = useRef<number>(0);

  const start = new THREE.Vector3(...sourcePos);
  const end = new THREE.Vector3(...targetPos);
  const mid = new THREE.Vector3().addVectors(start, end).multiplyScalar(0.5);
  mid.y += Math.max(3.0, start.distanceTo(end) * 0.3);

  const curve = useMemo(() => new THREE.QuadraticBezierCurve3(start, mid, end), [sourcePos, targetPos]);

  const trajectoryLine = useMemo(() => {
    const particleColor = new THREE.Color(isRepair ? StateColors.REPAIRING : StateColors.HEALTHY);
    const geom = new THREE.BufferGeometry().setFromPoints(curve.getPoints(30));
    const mat = new THREE.LineBasicMaterial({ color: particleColor, transparent: true, opacity: 0.4 });
    return new THREE.Line(geom, mat);
  }, [curve, isRepair]);

  useFrame((_, delta) => {
    if (meshRef.current) {
      progressRef.current = (progressRef.current + delta * 0.9) % 1.0;
      const point = curve.getPoint(progressRef.current);
      meshRef.current.position.copy(point);
    }
  });

  const particleColor = isRepair ? StateColors.REPAIRING : StateColors.HEALTHY;

  return (
    <group>
      {/* Active Trajectory Path */}
      <primitive object={trajectoryLine} />

      {/* Discrete Data Packet Mesh */}
      <mesh ref={meshRef}>
        <boxGeometry args={[0.22, 0.22, 0.22]} />
        <meshStandardMaterial
          color={particleColor}
          emissive={particleColor}
          emissiveIntensity={1.2}
          roughness={0.2}
        />
      </mesh>
    </group>
  );
};
