// SceneCanvas Component
// 3D scene as the primary operations interface.
// All topology is derived from real backend state — no random positioning.

import React, { useMemo } from 'react';
import { Canvas } from '@react-three/fiber';
import { StorageNodeChassis } from './StorageNodeChassis';
import { HashRing3D } from './HashRing3D';
import { ReplicaLinks } from './ReplicaLinks';
import { DataFlowParticles } from './DataFlowParticles';
import { CameraController } from './CameraController';
import { StorageNode, ObjectDTO, SelectionState, ViewMode, ActiveTransfer } from '../state/types';
import { TopologyTokens, BaseColors, CameraTokens } from '../design/tokens';

interface SceneCanvasProps {
  nodes: StorageNode[];
  objects: ObjectDTO[];
  selection: SelectionState;
  onSelectNode: (nodeId: string) => void;
  activeReplication: ActiveTransfer | null;
  viewMode: ViewMode;
}

export const SceneCanvas: React.FC<SceneCanvasProps> = ({
  nodes,
  objects,
  selection,
  onSelectNode,
  activeReplication,
  viewMode,
}) => {
  // ─── Deterministic node layout on the ring ────────────────────────────
  // Evenly spaced around the hash ring circumference.
  // Nodes with the same rack are grouped together (zone-aware if backend
  // provides rack/zone metadata).
  const nodePositions = useMemo(() => {
    const posMap: Record<string, [number, number, number]> = {};
    const count = nodes.length || 1;
    const r = TopologyTokens.ringRadius;

    // Group nodes by zone for spatial organization
    const zones: Record<string, StorageNode[]> = {};
    nodes.forEach((n) => {
      const z = n.zone || 'default';
      if (!zones[z]) zones[z] = [];
      zones[z].push(n);
    });

    const zoneKeys = Object.keys(zones).sort();
    let globalIdx = 0;

    zoneKeys.forEach((zoneKey) => {
      zones[zoneKey].forEach((node) => {
        const angle = (globalIdx / count) * Math.PI * 2;
        const x = Math.cos(angle) * r;
        const z = Math.sin(angle) * r;
        posMap[node.id] = [x, 0, z];
        globalIdx++;
      });
    });

    return posMap;
  }, [nodes]);

  // ─── Camera focus ─────────────────────────────────────────────────────
  const focusTarget = useMemo<[number, number, number] | null>(() => {
    if (selection.type === 'node' && selection.nodeId && nodePositions[selection.nodeId]) {
      return nodePositions[selection.nodeId];
    }
    return null;
  }, [selection, nodePositions]);

  // ─── Chunk counts per node ────────────────────────────────────────────
  const nodeChunkCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    nodes.forEach((n) => (counts[n.id] = 0));
    objects.forEach((obj) =>
      obj.chunks.forEach((chk) =>
        chk.replicas.forEach((rep) => {
          if (counts[rep] !== undefined) counts[rep]++;
        })
      )
    );
    return counts;
  }, [nodes, objects]);

  // ─── Replica highlighting logic ───────────────────────────────────────
  // Nodes that hold replicas of the selected object/chunk are "active".
  // All other nodes are dimmed in object/chunk selection context.
  const { activeNodeIds, dimmedNodeIds } = useMemo(() => {
    const active = new Set<string>();
    const dimmed = new Set<string>();

    if (selection.type === 'object' && selection.objectKey) {
      const obj = objects.find((o) => o.key === selection.objectKey);
      if (obj) {
        obj.chunks.forEach((c) => c.replicas.forEach((r) => active.add(r)));
      }
    } else if (selection.type === 'node' && selection.nodeId) {
      active.add(selection.nodeId);
    }

    // In FAILURES view: highlight dead/suspect nodes
    if (viewMode === 'FAILURES') {
      nodes.forEach((n) => {
        if (n.status === 'DEAD' || n.status === 'SUSPECT') {
          active.add(n.id);
        }
      });
    }

    if (active.size > 0) {
      nodes.forEach((n) => {
        if (!active.has(n.id)) dimmed.add(n.id);
      });
    }

    return { activeNodeIds: active, dimmedNodeIds: dimmed };
  }, [selection, objects, nodes, viewMode]);

  // ─── Replica link targets ─────────────────────────────────────────────
  const replicaLinkTargets = useMemo(() => {
    if (selection.type !== 'object' || !selection.objectKey) return [];
    const obj = objects.find((o) => o.key === selection.objectKey);
    if (!obj) return [];
    const nodeSet = new Set<string>();
    obj.chunks.forEach((c) => c.replicas.forEach((r) => nodeSet.add(r)));
    return Array.from(nodeSet)
      .map((nid) => nodePositions[nid])
      .filter(Boolean) as [number, number, number][];
  }, [selection, objects, nodePositions]);

  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        position: 'relative',
        background: BaseColors.bg,
      }}
    >
      <Canvas
        camera={{
          position: CameraTokens.defaultPosition,
          fov: CameraTokens.fov,
          near: CameraTokens.near,
          far: CameraTokens.far,
        }}
        gl={{ antialias: true, alpha: false }}
        onPointerMissed={() => onSelectNode('')}
        shadows
      >
        {/* Scene base color */}
        <color attach="background" args={[BaseColors.bg]} />

        {/* Lighting — calibrated for metallic server hardware readability */}
        <ambientLight intensity={0.55} />
        <directionalLight position={[15, 25, 20]} intensity={1.1} castShadow />
        <directionalLight position={[-15, 15, -20]} intensity={0.35} />

        {/* Ground reference grid — subtle spatial orientation */}
        <gridHelper
          args={[60, 60, '#1c202a', '#10131a']}
          position={[0, -0.5, 0]}
        />

        {/* Consistent hash ring — interactive token segment visualization */}
        <HashRing3D
          radius={TopologyTokens.ringRadius}
          nodes={nodes}
          objects={objects}
          viewMode={viewMode}
          onSelectNode={onSelectNode}
        />

        {/* Coordinator telemetry hub at center */}
        <group position={[0, 0, 0]}>
          <mesh position={[0, 0, 0]}>
            <cylinderGeometry args={[1.1, 1.3, 0.35, 32]} />
            <meshStandardMaterial
              color="#0d1017"
              metalness={0.7}
              roughness={0.4}
            />
          </mesh>
          <mesh position={[0, 0.19, 0]}>
            <cylinderGeometry args={[0.25, 0.25, 0.06, 16]} />
            <meshStandardMaterial
              color={BaseColors.accent}
              emissive={BaseColors.accent}
              emissiveIntensity={0.5}
            />
          </mesh>
        </group>

        {/* Storage node chassis — one per real backend node */}
        {nodes.map((node) => {
          const pos = nodePositions[node.id] || [0, 0, 0];
          const isActive = activeNodeIds.has(node.id);
          const isDimmed = dimmedNodeIds.has(node.id);
          return (
            <StorageNodeChassis
              key={node.id}
              node={node}
              position={pos}
              isSelected={selection.type === 'node' && selection.nodeId === node.id}
              isHighlighted={isActive && !isDimmed}
              isDimmed={isDimmed}
              onSelect={onSelectNode}
              chunkCount={nodeChunkCounts[node.id] || 0}
            />
          );
        })}

        {/* Replica arc links when object is selected */}
        {replicaLinkTargets.length > 0 && (
          <ReplicaLinks sourcePos={[0, 0.4, 0]} targetPositions={replicaLinkTargets} />
        )}

        {/* Data-movement particle — driven by real replication/repair event */}
        {activeReplication && nodePositions[activeReplication.target] && (
          <DataFlowParticles
            sourcePos={nodePositions[activeReplication.source] || [0, 0.4, 0]}
            targetPos={nodePositions[activeReplication.target]}
            isRepair={activeReplication.isRepair}
          />
        )}

        {/* Camera — focus follows user selection or significant events */}
        <CameraController focusTarget={focusTarget} />
      </Canvas>
    </div>
  );
};
