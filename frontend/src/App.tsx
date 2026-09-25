import React, { useState } from 'react';
import { useVaultState } from './state/useVaultState';
import { SceneCanvas } from './topology/SceneCanvas';
import { TopStatusStrip } from './components/TopStatusStrip';
import { ObjectsCatalogDrawer } from './components/ObjectsCatalogDrawer';
import { ContextualInspector } from './components/ContextualInspector';
import { EventTimeline } from './components/EventTimeline';
import { UploadModal } from './components/UploadModal';
import { ChaosConfirmationModal } from './components/ChaosConfirmationModal';
import { Maximize2 } from 'lucide-react';
import { BaseColors } from './design/tokens';

export const App: React.FC = () => {
  const {
    status,
    nodes,
    objects,
    events,
    connected,
    activeReplication,
    selection,
    setSelection,
    killNode,
    corruptChunk,
    deleteObject,
    uploadObject,
    downloadObject,
    refresh,
  } = useVaultState();

  const [uploadOpen, setUploadOpen] = useState<boolean>(false);
  const [chaosModal, setChaosModal] = useState<{
    isOpen: boolean;
    type: 'kill-node' | 'corrupt-chunk';
    targetId: string;
  }>({
    isOpen: false,
    type: 'kill-node',
    targetId: '',
  });
  const [chaosProcessing, setChaosProcessing] = useState<boolean>(false);

  // Selection triggers
  const handleSelectNode = (nodeId: string) => {
    if (!nodeId) {
      setSelection({ type: 'none' });
    } else {
      setSelection({ type: 'node', nodeId });
    }
  };

  const handleSelectObject = (key: string) => {
    if (!key) {
      setSelection({ type: 'none' });
    } else {
      setSelection({ type: 'object', objectKey: key });
    }
  };

  // Chaos triggers
  const triggerKillNode = (nodeId: string) => {
    setChaosModal({
      isOpen: true,
      type: 'kill-node',
      targetId: nodeId,
    });
  };

  const triggerCorruptChunk = (chunkId: string) => {
    setChaosModal({
      isOpen: true,
      type: 'corrupt-chunk',
      targetId: chunkId,
    });
  };

  const confirmChaos = async () => {
    setChaosProcessing(true);
    try {
      if (chaosModal.type === 'kill-node') {
        await killNode(chaosModal.targetId);
      } else {
        await corruptChunk(chaosModal.targetId);
      }
      setChaosModal({ isOpen: false, type: 'kill-node', targetId: '' });
      refresh();
    } catch (err: any) {
      alert(`Chaos action error: ${err.message}`);
    } finally {
      setChaosProcessing(false);
    }
  };

  return (
    <div style={{ width: '100vw', height: '100vh', position: 'relative', overflow: 'hidden' }}>
      {/* 1. Top Minimal Operations Strip */}
      <TopStatusStrip
        status={status}
        connected={connected}
        onOpenUpload={() => setUploadOpen(true)}
        onRefresh={refresh}
      />

      {/* 2. Left Objects Catalog Drawer */}
      <ObjectsCatalogDrawer
        objects={objects}
        selection={selection}
        onSelectObject={handleSelectObject}
      />

      {/* 3. Primary 3D Spatial Topology Viewport */}
      <SceneCanvas
        nodes={nodes}
        objects={objects}
        selection={selection}
        onSelectNode={handleSelectNode}
        activeReplication={activeReplication}
      />

      {/* 4. Reset View Floating Control */}
      {selection.type !== 'none' && (
        <button
          onClick={() => setSelection({ type: 'none' })}
          style={{
            position: 'absolute',
            bottom: '46px',
            right: '400px',
            background: 'rgba(15, 23, 42, 0.85)',
            border: `1px solid ${BaseColors.border}`,
            color: BaseColors.textSecondary,
            padding: '6px 10px',
            borderRadius: '3px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            fontSize: '11px',
            fontFamily: '"IBM Plex Mono", monospace',
            zIndex: 40,
          }}
        >
          <Maximize2 size={12} />
          <span>RESET VIEW</span>
        </button>
      )}

      {/* 5. Right Contextual Inspector */}
      <ContextualInspector
        selection={selection}
        nodes={nodes}
        objects={objects}
        onClose={() => setSelection({ type: 'none' })}
        onSelectObject={handleSelectObject}
        onDownloadObject={downloadObject}
        onDeleteObject={deleteObject}
        onTriggerKillNode={triggerKillNode}
        onTriggerCorruptChunk={triggerCorruptChunk}
      />

      {/* 6. Bottom Event Timeline Stream */}
      <EventTimeline events={events} />

      {/* 7. Ingestion Modal */}
      <UploadModal
        isOpen={uploadOpen}
        onClose={() => setUploadOpen(false)}
        onUpload={uploadObject}
      />

      {/* 8. Chaos Confirmation Modal */}
      <ChaosConfirmationModal
        isOpen={chaosModal.isOpen}
        type={chaosModal.type}
        targetId={chaosModal.targetId}
        onConfirm={confirmChaos}
        onCancel={() => setChaosModal({ isOpen: false, type: 'kill-node', targetId: '' })}
        isProcessing={chaosProcessing}
      />
    </div>
  );
};

export default App;
