// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: Unit tests for types and ledger state.
// :: filename: code/cmd/appy/types_test.go
// :: serialization: go

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLedgerPersistence(t *testing.T) {
	dir := t.TempDir()

	// Set initial state
	appliedPatchesMu.Lock()
	appliedPatches = map[string]bool{"hash1": true, "hash2": true}
	appliedPatchesMu.Unlock()

	// Save to temp dir
	SaveLedger(dir)

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, ledgerFilename)); os.IsNotExist(err) {
		t.Fatal("SaveLedger failed to create ledger file")
	}

	// Clear memory
	appliedPatchesMu.Lock()
	appliedPatches = make(map[string]bool)
	appliedPatchesMu.Unlock()

	// Load from temp dir
	LoadLedger(dir)

	// Verify state restored
	appliedPatchesMu.RLock()
	defer appliedPatchesMu.RUnlock()
	if !appliedPatches["hash1"] || !appliedPatches["hash2"] || len(appliedPatches) != 2 {
		t.Errorf("LoadLedger failed to restore state, got: %v", appliedPatches)
	}
}

func TestLoadLedger_NotExist(t *testing.T) {
	// Should not panic or crash when the file doesn't exist
	LoadLedger(t.TempDir())
}

func TestLoadLedger_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ledgerFilename), []byte("{bad json"), 0644)
	LoadLedger(dir) // Should log warning but not crash
}

func TestSaveLedger_WriteError(t *testing.T) {
	// Write to a path that isn't a directory to force an error
	file, _ := os.CreateTemp("", "bad_dir")
	defer os.Remove(file.Name())
	SaveLedger(file.Name()) // Should log warning but not crash
}
