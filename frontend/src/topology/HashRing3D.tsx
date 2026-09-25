// HashRing3D Component
// Precision Consistent Hash Ring visualization with virtual node distribution
// and interactive token range inspection.

import React, { useMemo, useState } from 'react';
import * as THREE from 'three';
import { Html } from '@react-three/drei';
import { TopologyTokens, BaseColors, StateColors } from '../design/tokens';
import { StorageNode, ObjectDTO, ViewMode } from '../state/types';
import { FontFamily, FontSize } from '../design/typography';

interface HashRing3DProps {
  radius?: number;
  vnodePointsCount?: number;
  nodes?: StorageNode[];
  objects?: ObjectDTO[];
  viewMode?: ViewMode;
  onSelectNode?: (nodeId: string) => void;
}

interface SegmentInfo {
  index: number;
  startAngle: number;
  endAngle: number;
  startToken: string;
  endToken: string;
  ownerNodeId: string;
  chunkCount: number;
  objectCount: number;
}

export const HashRing3D: React.FC<HashRing3DProps> = ({
  radius = TopologyTokens.ringRadius,
  vnodePointsCount = 128,
  nodes = [],
  objects = [],
  viewMode = 'OVERVIEW',
  onSelectNode,
}) => {
  const [hoveredSegment, setHoveredSegment] = useState<SegmentInfo | null>(null);

  // ─── Token range segments per physical node ──────────────────────────────
  const segments = useMemo<SegmentInfo[]>(() => {
    if (nodes.length === 0) return [];
    const count = nodes.length;
    const maxUint32 = 0xffffffff;

    return nodes.map((node, i) => {
      const startAngle = (i / count) * Math.PI * 2;
      const endAngle = ((i + 1) / count) * Math.PI * 2;
      const startVal = Math.floor((i / count) * maxUint32);
      const endVal = Math.floor(((i + 1) / count) * maxUint32);

      const startToken = '0x' + startVal.toString(16).toUpperCase().padStart(8, '0');
      const endToken = '0x' + endVal.toString(16).toUpperCase().padStart(8, '0');

      // Count chunks and objects on this node
      let chunkCount = 0;
      let objCount = 0;
      objects.forEach((obj) => {
        let hasOnNode = false;
        obj.chunks.forEach((chk) => {
          if (chk.replicas.includes(node.id)) {
            chunkCount++;
            hasOnNode = true;
          }
        });
        if (hasOnNode) objCount++;
      });

      return {
        index: i,
        startAngle,
        endAngle,
        startToken,
        endToken,
        ownerNodeId: node.id,
        chunkCount,
        objectCount: objCount,
      };
    });
  }, [nodes, objects]);

  // ─── Fine virtual-node tick markers ───────────────────────────────────────
  const tickMarkers = useMemo(() => {
    const ticks: { pos: [number, number, number]; rot: number; isMajor: boolean }[] = [];
    for (let i = 0; i < vnodePointsCount; i++) {
      const angle = (i / vnodePointsCount) * Math.PI * 2;
      const x = Math.cos(angle) * radius;
      const z = Math.sin(angle) * radius;
      const isMajor = i % (vnodePointsCount / 8) === 0;
      ticks.push({
        pos: [x, -0.47, z],
        rot: -angle,
        isMajor,
      });
    }
    return ticks;
  }, [radius, vnodePointsCount]);

  const isTopologyMode = viewMode === 'TOPOLOGY';

  return (
    <group>
      {/* ── Precision Circular Track ────────────────────────────────────── */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.49, 0]}>
        <ringGeometry args={[radius - 0.04, radius + 0.04, 128]} />
        <meshBasicMaterial
          color={isTopologyMode ? BaseColors.accent : BaseColors.borderActive}
          transparent
          opacity={isTopologyMode ? 0.7 : 0.4}
        />
      </mesh>

      {/* ── Subtle Outer Guide Ring ─────────────────────────────────────── */}
      <mesh rotation={[-Math.PI / 2, 0, 0]} position={[0, -0.495, 0]}>
        <ringGeometry args={[radius - 0.35, radius - 0.34, 128]} />
        <meshBasicMaterial color={BaseColors.border} transparent opacity={0.25} />
      </mesh>

      {/* ── Virtual Node Precision Ticks ─────────────────────────────────── */}
      {tickMarkers.map((tick, idx) => (
        <mesh
          key={idx}
          position={tick.pos}
          rotation={[0, tick.rot, 0]}
        >
          <boxGeometry args={[tick.isMajor ? 0.28 : 0.12, 0.015, 0.02]} />
          <meshBasicMaterial
            color={tick.isMajor ? BaseColors.textSecondary : BaseColors.borderActive}
            transparent
            opacity={tick.isMajor ? 0.8 : 0.4}
          />
        </mesh>
      ))}

      {/* ── Interactive Segment Hit Areas ────────────────────────────────── */}
      {segments.map((seg) => {
        const midAngle = (seg.startAngle + seg.endAngle) / 2;
        const hitX = Math.cos(midAngle) * radius;
        const hitZ = Math.sin(midAngle) * radius;
        const isHovered = hoveredSegment?.index === seg.index;

        return (
          <group key={seg.index}>
            {/* Visual arc segment when hovered */}
            {isHovered && (
              <mesh
                rotation={[-Math.PI / 2, 0, seg.startAngle]}
                position={[0, -0.485, 0]}
              >
                <ringGeometry
                  args={[radius - 0.2, radius + 0.2, 32, 1, 0, seg.endAngle - seg.startAngle]}
                />
                <meshBasicMaterial
                  color={StateColors.HEALTHY}
                  transparent
                  opacity={0.35}
                  side={THREE.DoubleSide}
                />
              </mesh>
            )}

            {/* Invisible hit mesh for hovering */}
            <mesh
              position={[hitX, -0.45, hitZ]}
              onPointerOver={(e) => {
                e.stopPropagation();
                setHoveredSegment(seg);
                document.body.style.cursor = 'pointer';
              }}
              onPointerOut={(e) => {
                e.stopPropagation();
                setHoveredSegment(null);
                document.body.style.cursor = 'default';
              }}
              onClick={(e) => {
                e.stopPropagation();
                if (onSelectNode) onSelectNode(seg.ownerNodeId);
              }}
            >
              <sphereGeometry args={[1.5, 8, 8]} />
              <meshBasicMaterial transparent opacity={0} />
            </mesh>
          </group>
        );
      })}

      {/* ── Token Coordinate Markers ─────────────────────────────────────── */}
      <Html position={[radius + 1.2, -0.45, 0]} center style={{ pointerEvents: 'none' }}>
        <div style={{ fontFamily: FontFamily.mono, fontSize: '9px', color: BaseColors.textMuted, whiteSpace: 'nowrap' }}>
          0x00000000 (0°)
        </div>
      </Html>
      <Html position={[0, -0.45, radius + 1.2]} center style={{ pointerEvents: 'none' }}>
        <div style={{ fontFamily: FontFamily.mono, fontSize: '9px', color: BaseColors.textMuted, whiteSpace: 'nowrap' }}>
          0x40000000 (90°)
        </div>
      </Html>
      <Html position={[-radius - 1.2, -0.45, 0]} center style={{ pointerEvents: 'none' }}>
        <div style={{ fontFamily: FontFamily.mono, fontSize: '9px', color: BaseColors.textMuted, whiteSpace: 'nowrap' }}>
          0x80000000 (180°)
        </div>
      </Html>
      <Html position={[0, -0.45, -radius - 1.2]} center style={{ pointerEvents: 'none' }}>
        <div style={{ fontFamily: FontFamily.mono, fontSize: '9px', color: BaseColors.textMuted, whiteSpace: 'nowrap' }}>
          0xC0000000 (270°)
        </div>
      </Html>

      {/* ── Interactive Token Range Inspection Popover ──────────────────── */}
      {hoveredSegment && (
        <Html
          position={[
            Math.cos((hoveredSegment.startAngle + hoveredSegment.endAngle) / 2) * radius,
            0.6,
            Math.sin((hoveredSegment.startAngle + hoveredSegment.endAngle) / 2) * radius,
          ]}
          center
          style={{ pointerEvents: 'none' }}
        >
          <div
            style={{
              background: BaseColors.surfaceElevated,
              border: `1px solid ${BaseColors.accent}`,
              padding: '6px 10px',
              borderRadius: '2px',
              fontFamily: FontFamily.mono,
              fontSize: FontSize.xxs,
              color: BaseColors.textPrimary,
              boxShadow: '0 4px 16px rgba(0,0,0,0.6)',
              whiteSpace: 'nowrap',
              display: 'flex',
              flexDirection: 'column',
              gap: '2px',
            }}
          >
            <div style={{ color: BaseColors.textMuted, fontSize: '8px', letterSpacing: '0.06em' }}>
              TOKEN RANGE
            </div>
            <div style={{ color: BaseColors.accent, fontWeight: 600 }}>
              {hoveredSegment.startToken} → {hoveredSegment.endToken}
            </div>
            <div style={{ display: 'flex', gap: '8px', marginTop: '2px', color: BaseColors.textSecondary }}>
              <span>OWNER: <strong style={{ color: BaseColors.textPrimary }}>{hoveredSegment.ownerNodeId}</strong></span>
              <span>CHUNKS: <strong style={{ color: BaseColors.textPrimary }}>{hoveredSegment.chunkCount}</strong></span>
              <span>OBJECTS: <strong style={{ color: BaseColors.textPrimary }}>{hoveredSegment.objectCount}</strong></span>
            </div>
          </div>
        </Html>
      )}
    </group>
  );
};
