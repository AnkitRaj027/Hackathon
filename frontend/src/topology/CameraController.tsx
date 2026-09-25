// CameraController Component
// Section 59: Professional camera control with smooth target interpolation.

import React, { useRef, useEffect } from 'react';
import * as THREE from 'three';
import { useThree, useFrame } from '@react-three/fiber';
import { OrbitControls } from '@react-three/drei';
import { CameraTokens } from '../design/tokens';

interface CameraControllerProps {
  focusTarget: [number, number, number] | null;
  onResetView?: () => void;
}

export const CameraController: React.FC<CameraControllerProps> = ({ focusTarget }) => {
  const { camera } = useThree();
  const controlsRef = useRef<any>(null);
  const targetVec = useRef<THREE.Vector3>(new THREE.Vector3(0, 0, 0));
  const isFocusing = useRef<boolean>(false);

  useEffect(() => {
    if (focusTarget) {
      targetVec.current.set(...focusTarget);
      isFocusing.current = true;
    } else {
      targetVec.current.set(0, 0, 0);
      isFocusing.current = false;
    }
  }, [focusTarget]);

  useFrame(() => {
    if (controlsRef.current) {
      // Smoothly interpolate orbit controls target
      controlsRef.current.target.lerp(targetVec.current, 0.08);
      controlsRef.current.update();

      // If focusing on a specific node, gently dolly camera closer
      if (isFocusing.current && focusTarget) {
        const desiredPos = new THREE.Vector3(
          focusTarget[0],
          focusTarget[1] + 4,
          focusTarget[2] + CameraTokens.nodeFocusDistance
        );
        camera.position.lerp(desiredPos, 0.05);
      }
    }
  });

  return (
    <OrbitControls
      ref={controlsRef}
      makeDefault
      minDistance={CameraTokens.minDistance}
      maxDistance={CameraTokens.maxDistance}
      maxPolarAngle={CameraTokens.maxPolarAngle}
      enableDamping
      dampingFactor={0.08}
    />
  );
};
