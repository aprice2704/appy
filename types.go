// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 6
// :: description: Types and failure structures compliant with Appy UI Spec v1.5.22.
// :: filename: types.go
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

type HistoryTx struct {
	TxID      string          `json:"tx_id"`
	Timestamp int64           `json:"timestamp"`
	Files     []HistoryFileOp `json:"files"`
}

type HistoryFileOp struct {
	Path    string `json:"path"`
	Existed bool   `json:"existed"`
}

type RevertPayload struct {
	TxID string `json:"tx_id"`
}

type PreviewFile struct {
	Path     string         `json:"path"`
	Status   string         `json:"status"` // READY, ERROR, IGNORED
	NetLines int            `json:"net_lines"`
	FileType string         `json:"file_type,omitempty"`
	FileIcon string         `json:"file_icon,omitempty"`
	Patches  []PreviewPatch `json:"patches"`
}

type PreviewPatch struct {
	SearchBlock      string `json:"search_block,omitempty"`
	ReplaceBlock     string `json:"replace_block"`
	IsOverwrite      bool   `json:"is_overwrite,omitempty"`
	IsDeleteFile     bool   `json:"is_delete_file,omitempty"`
	IsAnchored       bool   `json:"is_anchored,omitempty"`
	Error            string `json:"error,omitempty"`
	ClosestMatchHint string `json:"closest_match_hint,omitempty"`
	LLMFallbackHint  string `json:"llm_fallback_hint,omitempty"`
}

type ApplyFile struct {
	Path        string       `json:"path"`
	Applied     bool         `json:"applied"`
	NetLines    int          `json:"net_lines"`
	FileType    string       `json:"file_type,omitempty"`
	FileIcon    string       `json:"file_icon,omitempty"`
	HashBefore  string       `json:"hash_before,omitempty"`
	HashAfter   string       `json:"hash_after,omitempty"`
	LedgerEntry string       `json:"ledger_entry,omitempty"`
	Error       string       `json:"error,omitempty"`
	FailedPatch *FailedPatch `json:"failed_patch,omitempty"`
}

type CompilerCheckFile struct {
	Path           string   `json:"path"`
	CompilerStatus string   `json:"compiler_status"` // PASS, FAIL
	Diagnostics    []string `json:"diagnostics,omitempty"`
	RawOutput      string   `json:"raw_output,omitempty"`
}

type FailedPatch struct {
	Error           string `json:"error,omitempty"`
	CurrentLineEcho string `json:"current_line_echo,omitempty"`
	LLMFallbackHint string `json:"llm_fallback_hint,omitempty"`
}

type RetestResponse struct {
	Packages []string             `json:"packages"`
	Files    []RetestResponseFile `json:"files"`
}

type RetestResponseFile struct {
	Path           string `json:"path"`
	TestStatus     string `json:"test_status"` // PASS, FAIL
	Package        string `json:"package"`
	Summary        string `json:"summary"`
	FailureExcerpt string `json:"failure_excerpt,omitempty"`
	RawOutput      string `json:"raw_output,omitempty"`
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
