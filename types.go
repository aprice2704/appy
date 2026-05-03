// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 4
// :: description: Types and failure structures for Appy v1.5.12.
// :: filename: code/cmd/appy/types.go
// :: serialization: go

package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type Payload struct {
	Bundle       string `json:"bundle"`
	SkipCompiler bool   `json:"skip_compiler"`
	CheckOnly    bool   `json:"check_only"`
}

type RetestPayload struct {
	Packages []string `json:"packages"`
}

type PreviewPatch struct {
	Search    string `json:"search"`
	Replace   string `json:"replace"`
	Index     int    `json:"index"`
	LineNum   int    `json:"line_num"`
	Status    string `json:"status"` // "ok", "ignored", "error", "applied"
	Message   string `json:"message"`
	Hint      string `json:"hint"`
	LineDelta int    `json:"line_delta"`
	Advisory  string `json:"advisory,omitempty"`
	Recovery  string `json:"recovery,omitempty"` // Suggested next patch syntax
}

type RejectedFile struct {
	Filename                string       `json:"filename"`
	Status                  string       `json:"file_commit_status"` // "rejected"
	Reason                  string       `json:"reason"`
	Committed               bool         `json:"committed"`
	SuccessfulMemoryPatches []string     `json:"successful_memory_patches"`
	FailedPatch             *FailedPatch `json:"failed_patch,omitempty"`
}

type FailedPatch struct {
	Directive string `json:"directive"`
	Reason    string `json:"reason"`
	Current   string `json:"current_line,omitempty"` // Matched line echo
}

var (
	appliedPatchesMu sync.RWMutex
	appliedPatches   = make(map[string]bool)
)

const ledgerFilename = ".appy_ledger.json"

func LoadLedger(workspaceRoot string) {
	appliedPatchesMu.Lock()
	defer appliedPatchesMu.Unlock()
	b, err := os.ReadFile(filepath.Join(workspaceRoot, ledgerFilename))
	if err == nil {
		if err := json.Unmarshal(b, &appliedPatches); err != nil {
			log.Printf("Warning: failed to unmarshal patch ledger: %v", err)
		}
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: failed to read patch ledger: %v", err)
	}
}

func SaveLedger(workspaceRoot string) {
	appliedPatchesMu.RLock()
	defer appliedPatchesMu.RUnlock()
	b, err := json.MarshalIndent(appliedPatches, "", "  ")
	if err != nil {
		log.Printf("Error: failed to marshal patch ledger: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, ledgerFilename), b, 0644); err != nil {
		log.Printf("Error: failed to write patch ledger to disk: %v", err)
	}
}
