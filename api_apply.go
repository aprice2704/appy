package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/aprice2704/fdm/code/patcheng"
)

type errRefusingOverwrite struct {
	msg string
}

func (e *errRefusingOverwrite) Error() string {
	return e.msg
}

func isRefusingOverwrite(err error) bool {
	if err == nil {
		return false
	}
	var target *errRefusingOverwrite
	if errors.As(err, &target) {
		return true
	}
	return strings.Contains(err.Error(), "refusing to overwrite existing file")
}

func (s *AppyServer) handleApply(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] /api/apply request received")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req Payload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[DEBUG] /api/apply: Invalid JSON payload: %v", err)
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Honor top-level compiler skip directives inside patch bundles
	if strings.Contains(req.Bundle, "%%% compiler: skip") ||
		strings.Contains(req.Bundle, "%%% compiler:skip") ||
		strings.Contains(req.Bundle, "%%% skip_compiler") {
		req.SkipCompiler = true
	}

	parsed, err := patcheng.ParseTextBundle(req.Bundle, patcheng.DefaultRegistry)
	if err != nil {
		log.Printf("[DEBUG] /api/apply: ParseTextBundle failed: %v", err)
		sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	memoryResults := make(map[string]string)
	filesToDelete := make(map[string]bool)
	fileHashes := make(map[string][]string)
	originalFiles := make(map[string]*[]byte)
	var applyFiles []ApplyFile
	hasErrors := false

	for rawFilename, patches := range parsed {
		if !isPathSafe(s.rootDir, rawFilename) {
			log.Printf("[DEBUG] /api/apply: Path traversal denied for %s", rawFilename)
			sendError(w, "Path traversal denied", http.StatusBadRequest)
			return
		}

		af, newContent, isDelete, isLedgerSkip, hashes, err := s.applySingleFile(rawFilename, patches, originalFiles)
		if err != nil {
			log.Printf("[DEBUG] /api/apply: applySingleFile error on %s: %v", rawFilename, err)
			if isRefusingOverwrite(err) {
				continue
			}
			hasErrors = true
			applyFiles = append(applyFiles, af)
			continue
		}

		log.Printf("[DEBUG] /api/apply: applySingleFile finished for %s (applied=%v, isDelete=%v, isLedgerSkip=%v)", rawFilename, af.Applied, isDelete, isLedgerSkip)
		applyFiles = append(applyFiles, af)
		absPath := filepath.Join(s.rootDir, rawFilename)
		if isDelete {
			filesToDelete[absPath] = true
		} else if af.Applied && !isLedgerSkip {
			memoryResults[absPath] = newContent
		}
		fileHashes[absPath] = hashes
	}

	log.Printf("[DEBUG] /api/apply: memoryResults count = %d, filesToDelete count = %d, checkOnly = %v, skipCompiler = %v",
		len(memoryResults), len(filesToDelete), req.CheckOnly, req.SkipCompiler)

	if len(memoryResults) > 0 && !req.SkipCompiler {
		if s.runCompilerChecks(w, req, memoryResults, filesToDelete, &applyFiles, &hasErrors) {
			log.Printf("[DEBUG] /api/apply: runCompilerChecks halted execution (checkOnly=true or errors returned)")
			return
		}
	}

	if !req.CheckOnly {
		s.commitChangesToDisk(originalFiles, memoryResults, filesToDelete, fileHashes, &applyFiles, &hasErrors)
	}

	status := http.StatusOK
	if hasErrors {
		status = http.StatusMultiStatus
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(withND("appy/apply-success", "Result", map[string]any{
		"files": applyFiles,
	})); err != nil {
		log.Printf("[ERROR] handleApply: failed encoding response: %v", err)
	}
}
