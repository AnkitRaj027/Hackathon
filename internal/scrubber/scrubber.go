package scrubber

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vault/internal/checksum"
)

// CorruptedChunkInfo details an integrity failure detected during a scrub.
type CorruptedChunkInfo struct {
	ChunkID          string
	Path             string
	ExpectedChecksum string
	ActualChecksum   string
	Reason           string
}

// ScrubReport summarizes the outcome of a disk scrub operation.
type ScrubReport struct {
	NodeID             string
	TotalChunksScanned int
	HealthyChunks      int
	CorruptedChunks    []CorruptedChunkInfo
	StartTime          time.Time
	Duration           time.Duration
}

// DiskScrubber performs physical cryptographic integrity sweeps over local chunk storage.
type DiskScrubber struct {
	nodeID  string
	dataDir string
}

// NewDiskScrubber creates a new DiskScrubber for a given storage node volume.
func NewDiskScrubber(nodeID, dataDir string) *DiskScrubber {
	return &DiskScrubber{
		nodeID:  nodeID,
		dataDir: dataDir,
	}
}

// Scrub scans all *.chunk files in dataDir and verifies each against its *.sha256 sidecar.
func (s *DiskScrubber) Scrub(ctx context.Context) (*ScrubReport, error) {
	start := time.Now()
	report := &ScrubReport{
		NodeID:    s.nodeID,
		StartTime: start,
	}

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("scrubber failed reading data directory %s: %w", s.dataDir, err)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".chunk") {
			continue
		}

		report.TotalChunksScanned++
		chunkID := strings.TrimSuffix(entry.Name(), ".chunk")
		chunkPath := filepath.Join(s.dataDir, entry.Name())
		sidecarPath := filepath.Join(s.dataDir, chunkID+".sha256")

		// 1. Read sidecar checksum
		sidecarBytes, err := os.ReadFile(sidecarPath)
		if err != nil {
			report.CorruptedChunks = append(report.CorruptedChunks, CorruptedChunkInfo{
				ChunkID: chunkID,
				Path:    chunkPath,
				Reason:  fmt.Sprintf("missing or unreadable checksum sidecar: %v", err),
			})
			slog.Error("BIT_ROT_DETECTED_MISSING_SIDECAR",
				"node", s.nodeID,
				"chunk_id", chunkID,
				"error", err,
			)
			continue
		}
		expectedSHA := strings.TrimSpace(string(sidecarBytes))

		// 2. Read physical chunk bytes
		chunkData, err := os.ReadFile(chunkPath)
		if err != nil {
			report.CorruptedChunks = append(report.CorruptedChunks, CorruptedChunkInfo{
				ChunkID:          chunkID,
				Path:             chunkPath,
				ExpectedChecksum: expectedSHA,
				Reason:           fmt.Sprintf("failed reading chunk data: %v", err),
			})
			slog.Error("BIT_ROT_DETECTED_UNREADABLE_CHUNK",
				"node", s.nodeID,
				"chunk_id", chunkID,
				"error", err,
			)
			continue
		}

		// 3. Cryptographic hash comparison
		actualSHA := checksum.ComputeBytes(chunkData)
		if !checksum.Verify(expectedSHA, actualSHA) {
			report.CorruptedChunks = append(report.CorruptedChunks, CorruptedChunkInfo{
				ChunkID:          chunkID,
				Path:             chunkPath,
				ExpectedChecksum: expectedSHA,
				ActualChecksum:   actualSHA,
				Reason:           "cryptographic SHA-256 mismatch (bit rot detected)",
			})
			slog.Error("BIT_ROT_DETECTED_HASH_MISMATCH",
				"node", s.nodeID,
				"chunk_id", chunkID,
				"expected", expectedSHA,
				"actual", actualSHA,
			)
			continue
		}

		report.HealthyChunks++
	}

	report.Duration = time.Since(start)
	slog.Info("scrub_cycle_completed",
		"node", s.nodeID,
		"scanned", report.TotalChunksScanned,
		"healthy", report.HealthyChunks,
		"corrupted", len(report.CorruptedChunks),
		"duration_ms", report.Duration.Milliseconds(),
	)

	return report, nil
}

// PeriodicScrubber runs background scrub cycles at a configured interval.
type PeriodicScrubber struct {
	scrubber *DiskScrubber
	interval time.Duration
	onReport func(*ScrubReport)

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewPeriodicScrubber creates a periodic background scrubber.
func NewPeriodicScrubber(scrubber *DiskScrubber, interval time.Duration, onReport func(*ScrubReport)) *PeriodicScrubber {
	if interval <= 0 {
		interval = 1 * time.Minute
	}
	return &PeriodicScrubber{
		scrubber: scrubber,
		interval: interval,
		onReport: onReport,
		done:     make(chan struct{}),
	}
}

// Start launches the background scrubbing loop.
func (p *PeriodicScrubber) Start(parentCtx context.Context) {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parentCtx)
	p.cancel = cancel
	p.mu.Unlock()

	go func() {
		defer close(p.done)
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rep, err := p.scrubber.Scrub(ctx)
				if err == nil && p.onReport != nil {
					p.onReport(rep)
				}
			}
		}
	}()
}

// Stop cleanly terminates the periodic background scrubber.
func (p *PeriodicScrubber) Stop() {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.mu.Unlock()
	<-p.done
}
