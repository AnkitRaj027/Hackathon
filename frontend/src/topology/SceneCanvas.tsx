// SceneCanvas Component
// Section 50: 3D scene as the primary operations interface.

import React, { useMemo } from 'react';
import { Canvas } from '@react-three/fiber';
import { StorageNodeChassis } from './StorageNodeChassis';
import { HashRing3D } from './HashRing3D';
import { ReplicaLinks } from './ReplicaLinks';
import { DataFlowParticles } from './DataFlowParticles';
import { CameraController } from './CameraController';
import { StorageNode, ObjectDTO, SelectionState } from '../state/types';
import { TopologyTokens, BaseColors, CameraTokens } from '../design/tokens';

interface SceneCanvasProps {
  nodes: StorageNode[];
  objects: ObjectDTO[];
  selection: SelectionState;
  onSelectNode: (nodeId: string) => void;
  onSelectChunk?: (chunkId: string) => void;
  activeReplication: {
    source: string;
    target: string;
    chunkId: string;
  } | null;
}

export const SceneCanvas: React.FC<SceneCanvasProps> = ({
  nodes,
  objects,
  selection,
  onSelectNode,
  activeReplication,
}) => {
  // Deterministic spatial placement of storage nodes around the consistent hash ring
  const nodePositions = useMemo(() => {
    const posMap: Record<string, [number, number, number]> = {};
    const count = nodes.length || 1;
    const r = TopologyTokens.ringRadius;

    nodes.forEach((node, idx) => {
      // Deterministic angular positions (e.g. 3 nodes at 0°, 120°, 240°)
      const angle = (idx / count) * Math.PI * 2;
      const x = Math.cos(angle) * r;
      const z = Math.sin(angle) * r;
      posMap[node.id] = [x, 0, z];
    });

    return posMap;
  }, [nodes]);

  // Compute camera focus target
  const focusTarget = useMemo<[number, number, number] | null>(() => {
    if (selection.type === 'node' && selection.nodeId && nodePositions[selection.nodeId]) {
      return nodePositions[selection.nodeId];
    }
    return null;
  }, [selection, nodePositions]);

  // Count chunks hosted on each node
  const nodeChunkCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    nodes.forEach((n) => (counts[n.id] = 0));

    objects.forEach((obj) => {
      obj.chunks.forEach((chk) => {
        chk.replicas.forEach((rep) => {
          if (counts[rep] !== undefined) counts[rep]++;
        });
      });
    });

    return counts;
  }, [nodes, objects]);

  // Target positions for replica linking if an object or chunk is selected
  const replicaLinkTargets = useMemo(() => {
    if (selection.type === 'object' && selection.objectKey) {
      const obj = objects.find((o) => o.key === selection.objectKey);
      if (!obj) return [];

      const activeNodes = new Set<string>();
      obj.chunks.forEach((c) => c.replicas.forEach((r) => activeNodes.add(r)));

      return Array.from(activeNodes)
        .map((nid) => nodePositions[nid])
        .filter(Boolean);
    }
    return [];
  }, [selection, objects, nodePositions]);

  return (
    <div style={{ width: '100%', height: '100%', position: 'relative', background: BaseColors.bg }}>
      <Canvas
        camera={{
          position: CameraTokens.defaultPosition,
          fov: CameraTokens.fov,
          near: CameraTokens.near,
          far: CameraTokens.far,
        }}
        gl={{ antialias: true, alpha: false }}
        onPointerMissed={() => onSelectNode('')}
      >
        {/* Spatial Reference Grid & Lighting */}
        <color attach="background" args={[BaseColors.bg]} />
        <ambientLight intensity={0.65} />
        <directionalLight position={[15, 25, 20]} intensity={1.2} castShadow />
        <directionalLight position={[-15, 15, -20]} intensity={0.4} />

        {/* Ground Reference Grid */}
        <gridHelper args={[60, 60, BaseColors.borderActive, BaseColors.border]} position={[0, -0.5, 0]} />

        {/* 3D Consistent Hash Ring */}
        <HashRing3D radius={TopologyTokens.ringRadius} />

        {/* Storage Node Hardware Chassis */}
        {nodes.map((node) => {
          const pos = nodePositions[node.id] || [0, 0, 0];
          return (
            <StorageNodeChassis
              key={node.id}
              node={node}
              position={pos}
              isSelected={selection.type === 'node' && selection.nodeId === node.id}
              onSelect={onSelectNode}
              chunkCount={nodeChunkCounts[node.id] || 0}
            />
          );
        })}

        {/* Coordinator Central Hub Representation */}
        <group position={[0, 0, 0]}>
          <mesh position={[0, 0, 0]}>
            <cylinderGeometry args={[1.2, 1.4, 0.4, 32]} />
            <meshStandardMaterial color="#0f172a" metalness={0.8} roughness={0.3} />
          </mesh>
          <mesh position={[0, 0.22, 0]}>
            <cylinderGeometry args={[0.3, 0.3, 0.08, 16]} />
            <meshStandardMaterial color="#38bdf8" emissive="#38bdf8" emissiveIntensity={0.8} />
          </mesh>
        </group>

        {/* Replica Link Arcs when an Object is Selected */}
        {replicaLinkTargets.length > 0 && (
          <ReplicaLinks sourcePos={[0, 0.4, 0]} targetPositions={replicaLinkTargets} />
        )}

        {/* Real Data Movement Particles */}
        {activeReplication && nodePositions[activeReplication.target] && (
          <DataFlowParticles
            sourcePos={nodePositions[activeReplication.source] || [0, 0.4, 0]}
            targetPos={nodePositions[activeReplication.target]}
            isRepair={activeReplication.source !== 'coordinator'}
          />
        )}

        {/* Camera Controls */}
        <CameraController focusTarget={focusTarget} />
      </Canvas>
    </div>
  );
};
