// Vault App Root
// Composes the distributed storage operations console from real backend state.

import React, { useState } from 'react';
import { useVaultState } from './state/useVaultState';
import { SceneCanvas } from './topology/SceneCanvas';
import { TopStatusStrip } from './components/TopStatusStrip';
import { OperationsPanel } from './components/OperationsPanel';
import { ContextualInspector } from './components/ContextualInspector';
import { EventTimeline } from './components/EventTimeline';
import { UploadModal } from './components/UploadModal';
import { ChaosConfirmationModal } from './components/ChaosConfirmationModal';
import { LoadingScreen } from './components/LoadingScreen';
import { ConnectionError } from './components/ConnectionError';
import { Maximize2 } from 'lucide-react';
import { BaseColors } from './design/tokens';
import { FontFamily, FontSize } from './design/typography';

export const App: React.FC = () => {
  const {
    status,
    nodes,
    objects,
    topology,
    events,
    metrics,
    repairQueue,
    connected,
    lastConnectedAt,
    activeReplication,
    selection,
    setSelection,
    viewMode,
    setViewMode,
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
  }>({ isOpen: false, type: 'kill-node', targetId: '' });
  const [chaosProcessing, setChaosProcessing] = useState<boolean>(false);

  // ─── Loading state — show until first snapshot arrives ────────────────
  const isLoading = !status && nodes.length === 0;

  // ─── Selection handlers ───────────────────────────────────────────────
  const handleSelectNode = (nodeId: string) => {
    setSelection(nodeId ? { type: 'node', nodeId } : { type: 'none' });
  };

  const handleSelectObject = (key: string) => {
    setSelection(key ? { type: 'object', objectKey: key } : { type: 'none' });
  };

  // ─── Chaos handlers ───────────────────────────────────────────────────
  const triggerKillNode = (nodeId: string) => {
    setChaosModal({ isOpen: true, type: 'kill-node', targetId: nodeId });
  };

  const triggerCorruptChunk = (chunkId: string) => {
    setChaosModal({ isOpen: true, type: 'corrupt-chunk', targetId: chunkId });
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
      console.error('Chaos action failed:', err.message);
    } finally {
      setChaosProcessing(false);
    }
  };

  return (
    <div style={{ width: '100vw', height: '100vh', position: 'relative', overflow: 'hidden', background: BaseColors.bg }}>
      {/* ── Loading overlay ────────────────────────────────────────── */}
      {isLoading && <LoadingScreen />}

      {/* ── Disconnected banner ────────────────────────────────────── */}
      {!connected && !isLoading && (
        <ConnectionError lastConnectedAt={lastConnectedAt} onRetry={refresh} />
      )}

      {/* ── 1. Top Status Bar (Section 9) ─────────────────────────── */}
      <TopStatusStrip
        status={status}
        connected={connected}
        metrics={metrics}
        onOpenUpload={() => setUploadOpen(true)}
        onRefresh={refresh}
      />

      {/* ── 2. Unified Operations Console Panel (Left Dock) ───────── */}
      <OperationsPanel
        viewMode={viewMode}
        onViewModeChange={setViewMode}
        nodes={nodes}
        objects={objects}
        topology={topology}
        metrics={metrics}
        repairQueue={repairQueue}
        selection={selection}
        onSelectNode={handleSelectNode}
        onSelectObject={handleSelectObject}
      />

      {/* ── 3. Primary 3D Topology Canvas (Section 8) ──────────────── */}
      <SceneCanvas
        nodes={nodes}
        objects={objects}
        selection={selection}
        onSelectNode={handleSelectNode}
        activeReplication={activeReplication}
        viewMode={viewMode}
      />

      {/* ── 4. Reset View button (when something is selected) ─────── */}
      {selection.type !== 'none' && (
        <button
          onClick={() => setSelection({ type: 'none' })}
          style={{
            position: 'absolute',
            bottom: '44px',
            right: '404px',
            background: BaseColors.surface,
            border: `1px solid ${BaseColors.border}`,
            color: BaseColors.textSecondary,
            padding: '5px 10px',
            borderRadius: '2px',
            cursor: 'pointer',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            fontSize: FontSize.xs,
            fontFamily: FontFamily.mono,
            zIndex: 40,
          }}
        >
          <Maximize2 size={11} />
          <span>RESET VIEW</span>
        </button>
      )}

      {/* ── 5. Right Contextual Inspector (Section 20) ────────────── */}
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

      {/* ── 6. Bottom Event Timeline (Section 24) ─────────────────── */}
      <EventTimeline events={events} />

      {/* ── 7. Ingestion Modal ────────────────────────────────────── */}
      <UploadModal
        isOpen={uploadOpen}
        onClose={() => setUploadOpen(false)}
        onUpload={uploadObject}
      />

      {/* ── 8. Chaos Confirmation Modal (Section 27) ──────────────── */}
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
