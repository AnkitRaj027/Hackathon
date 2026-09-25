package scrubber_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"vault/internal/checksum"
	"vault/internal/scrubber"
)

func TestDiskScrubber_DetectsHealthAndBitRot(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vault-scrub-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Create a healthy chunk
	healthyData := []byte("this is healthy data for chunk 1")
	healthySHA := checksum.ComputeBytes(healthyData)
	if err := os.WriteFile(filepath.Join(tempDir, "chunk-1.chunk"), healthyData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "chunk-1.sha256"), []byte(healthySHA), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Create a corrupted chunk (simulating bit rot: sidecar has original hash, file has altered bytes)
	originalData := []byte("original pristine content")
	originalSHA := checksum.ComputeBytes(originalData)
	corruptedData := []byte("corrupted bit-rotted content")
	if err := os.WriteFile(filepath.Join(tempDir, "chunk-2.chunk"), corruptedData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "chunk-2.sha256"), []byte(originalSHA), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Create a chunk with missing sidecar
	if err := os.WriteFile(filepath.Join(tempDir, "chunk-3.chunk"), []byte("data with no sidecar"), 0644); err != nil {
		t.Fatal(err)
	}

	s := scrubber.NewDiskScrubber("node-test", tempDir)
	report, err := s.Scrub(context.Background())
	if err != nil {
		t.Fatalf("scrub failed: %v", err)
	}

	if report.TotalChunksScanned != 3 {
		t.Errorf("expected 3 total chunks scanned, got %d", report.TotalChunksScanned)
	}
	if report.HealthyChunks != 1 {
		t.Errorf("expected 1 healthy chunk, got %d", report.HealthyChunks)
	}
	if len(report.CorruptedChunks) != 2 {
		t.Fatalf("expected 2 corrupted chunks detected, got %d", len(report.CorruptedChunks))
	}

	// Verify details
	foundBitRot := false
	foundMissingSidecar := false
	for _, c := range report.CorruptedChunks {
		if c.ChunkID == "chunk-2" {
			foundBitRot = true
			if c.ExpectedChecksum != originalSHA {
				t.Errorf("expected hash %s, got %s", originalSHA, c.ExpectedChecksum)
			}
		}
		if c.ChunkID == "chunk-3" {
			foundMissingSidecar = true
		}
	}

	if !foundBitRot {
		t.Errorf("did not report chunk-2 for bit rot")
	}
	if !foundMissingSidecar {
		t.Errorf("did not report chunk-3 for missing sidecar")
	}
}
