package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
	"github.com/aprice2704/fdm/code/retest"
)

func (s *AppyServer) handlePreview(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] /api/preview request received")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req Payload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[DEBUG] /api/preview: Invalid JSON payload: %v", err)
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	parsed, err := patcheng.ParseTextBundle(req.Bundle, patcheng.DefaultRegistry)
	if err != nil {
		log.Printf("[DEBUG] /api/preview: ParseTextBundle failed: %v", err)
		sendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("[DEBUG] /api/preview: successfully parsed %d files", len(parsed))

	var responseFiles []PreviewFile
	pathFixes := make(map[string]string)

	for rawFilename, patches := range parsed {
		if !isPathSafe(s.rootDir, rawFilename) {
			log.Printf("[DEBUG] /api/preview: unsafe path rejected: %s", rawFilename)
			continue
		}

		absPath := filepath.Join(s.rootDir, rawFilename)
		contentBytes, readErr := os.ReadFile(absPath)
		if readErr != nil && !os.IsNotExist(readErr) {
			log.Printf("[DEBUG] /api/preview: failed to read %s: %v", absPath, readErr)
		}
		content := string(contentBytes)
		prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))
		fType, fIcon := getFileMeta(prof)

		var filePreviews []PreviewPatch
		fileStatus := "READY"
		fileNetLines := 0

		for _, p := range patches {
			delta := 0
			pp := PreviewPatch{
				SearchBlock:  p.Search,
				ReplaceBlock: p.Replace,
				IsOverwrite:  p.FullOverwrite,
				IsDeleteFile: p.IsDeleteFile,
				IsAnchored:   p.IsAnchored,
			}
			fileNetLines += delta

			pp = PreviewPatch{
				SearchBlock:  p.Search,
				ReplaceBlock: p.Replace,
				IsOverwrite:  p.FullOverwrite,
				IsDeleteFile: p.IsDeleteFile,
			}

			patchHash := hashPatch(rawFilename, p.Search, p.Replace)
			appliedPatchesMu.RLock()
			alreadyApplied := appliedPatches[patchHash]
			appliedPatchesMu.RUnlock()

			if alreadyApplied {
				if fileStatus != "ERROR" && fileStatus != "IGNORED" {
					fileStatus = "APPLIED"
				}
			} else {
				_, pErr := patcheng.ApplyFuzzyPatchesAgnostic(prof, content, []patcheng.FuzzyPatch{p})
				if pErr != nil {
					log.Printf("[DEBUG] /api/preview: ApplyFuzzyPatchesAgnostic failed for patch in %s: %v", rawFilename, pErr)
					errMsg := pErr.Error()
					if strings.Contains(errMsg, "refusing to overwrite existing file") {
						pp.Error = "File already exists. Use '\\%%% overwrite' to replace it."
						if fileStatus != "ERROR" {
							fileStatus = "IGNORED"
						}
					} else if strings.Contains(errMsg, "target file is empty or does not exist") {
						pp.Error = "Target file missing. Click 'Fix File Paths'."
						fileStatus = "ERROR"
						if fixed := findUniquePathSuffix(s.rootDir, rawFilename); fixed != "" {
							pathFixes[rawFilename] = fixed
						}
					} else {
						fileStatus = "ERROR"
						pp.Error = pErr.Error()
						pp.ClosestMatchHint = generateDiagnosticHint(prof, rawFilename, content, p.Search, p.NearLine)
						pp.LLMFallbackHint = generateLLMFallbackHint(prof)
					}
				}
			}
			filePreviews = append(filePreviews, pp)
		}

		responseFiles = append(responseFiles, PreviewFile{
			Path:     rawFilename,
			Status:   fileStatus,
			NetLines: fileNetLines,
			FileType: fType,
			FileIcon: fIcon,
			Patches:  filePreviews,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	encErr := json.NewEncoder(w).Encode(withND("appy/preview", "Simulation results", map[string]any{
		"files":      responseFiles,
		"path_fixes": pathFixes,
	}))
	if encErr != nil {
		log.Printf("[DEBUG] /api/preview: failed to encode response: %v", encErr)
	}
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

	parsed, err := patcheng.ParseTextBundle(req.Bundle, patcheng.DefaultRegistry)
	if err != nil {
		log.Printf("[DEBUG] /api/apply: ParseTextBundle failed: %v", err)
		sendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("[DEBUG] /api/apply: successfully parsed %d files", len(parsed))

	memoryResults := make(map[string]string)
	filesToDelete := make(map[string]bool)
	fileHashes := make(map[string][]string)
	originalFiles := make(map[string]*[]byte)

	var applyFiles []ApplyFile
	var checkFiles []CompilerCheckFile
	hasErrors := false

	for rawFilename, patches := range parsed {
		if !isPathSafe(s.rootDir, rawFilename) {
			log.Printf("[DEBUG] /api/apply: unsafe path rejected: %s", rawFilename)
			sendError(w, "Path traversal denied", http.StatusBadRequest)
			return
		}
		absPath := filepath.Join(s.rootDir, rawFilename)
		contentBytes, errRead := os.ReadFile(absPath)
		if errRead == nil {
			cbCopy := make([]byte, len(contentBytes))
			copy(cbCopy, contentBytes)
			originalFiles[absPath] = &cbCopy
		} else {
			originalFiles[absPath] = nil
		}
		prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))
		fType, fIcon := getFileMeta(prof)

		allApplied := true
		appliedPatchesMu.RLock()
		for _, p := range patches {
			if !appliedPatches[hashPatch(rawFilename, p.Search, p.Replace)] {
				allApplied = false
				break
			}
		}
		appliedPatchesMu.RUnlock()

		if len(patches) > 0 && allApplied {
			log.Printf("[DEBUG] /api/apply: all patches for %s already in ledger, skipping gracefully", rawFilename)
			applyFiles = append(applyFiles, ApplyFile{
				Path:     rawFilename,
				Applied:  true,
				NetLines: 0,
				FileType: fType,
				FileIcon: fIcon,
			})
			continue
		}

		fileNetLines := 0
		isDeleteFile := false
		for _, p := range patches {
			if p.FullOverwrite {
				fileNetLines += countLines(p.Replace) - countLines(string(contentBytes))
			} else if p.IsDeleteFile {
				isDeleteFile = true
				fileNetLines -= countLines(string(contentBytes))
			} else {
				fileNetLines += countLines(p.Replace) - countLines(p.Search)
			}
		}

		newContent, applyErr := patcheng.ApplyFuzzyPatchesAgnostic(prof, string(contentBytes), patches)
		if applyErr != nil {
			log.Printf("[DEBUG] /api/apply: ApplyFuzzyPatchesAgnostic batch failed for %s: %v", rawFilename, applyErr)
			if strings.Contains(applyErr.Error(), "refusing to overwrite existing file") {
				continue // Safety ignores don't count as failures
			}
			hasErrors = true

			var failedBlock *FailedPatch
			for _, p := range patches {
				_, pErr := patcheng.ApplyFuzzyPatchesAgnostic(prof, string(contentBytes), []patcheng.FuzzyPatch{p})
				if pErr != nil && failedBlock == nil {
					cur := ""
					hint := generateDiagnosticHint(prof, rawFilename, string(contentBytes), p.Search, p.NearLine)
					if strings.Contains(hint, ": ") {
						for _, l := range strings.Split(hint, "\n") {
							if strings.Contains(l, ": ") && !strings.Contains(l, "elided") {
								cur = strings.TrimSpace(strings.SplitN(l, ": ", 2)[1])
								break
							}
						}
					}
					failedBlock = &FailedPatch{
						Error:           pErr.Error(),
						CurrentLineEcho: cur,
						LLMFallbackHint: generateLLMFallbackHint(prof),
					}
					log.Printf("[DEBUG] /api/apply: individual patch failed for %s: %v", rawFilename, pErr)
				}
			}

			applyFiles = append(applyFiles, ApplyFile{
				Path:        rawFilename,
				Applied:     false,
				NetLines:    fileNetLines,
				FileType:    fType,
				FileIcon:    fIcon,
				Error:       applyErr.Error(),
				FailedPatch: failedBlock,
			})
			continue
		}

		if prof != nil && prof.Formatter != nil && !isDeleteFile {
			log.Printf("[DEBUG] /api/apply: running formatter for %s", rawFilename)
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			formatted, _, err := prof.Formatter(ctx, []byte(newContent))
			cancel()
			if err == nil {
				newContent = string(formatted)
				log.Printf("[DEBUG] /api/apply: formatter succeeded for %s", rawFilename)
			} else {
				log.Printf("[DEBUG] /api/apply: formatter failed for %s (continuing anyway): %v", rawFilename, err)
			}
		}

		if isDeleteFile {
			filesToDelete[absPath] = true
		} else {
			memoryResults[absPath] = newContent
		}
		for _, p := range patches {
			fileHashes[absPath] = append(fileHashes[absPath], hashPatch(rawFilename, p.Search, p.Replace))
		}

		applyFiles = append(applyFiles, ApplyFile{
			Path:     rawFilename,
			Applied:  true,
			NetLines: fileNetLines,
			FileType: fType,
			FileIcon: fIcon,
		})
	}

	if len(memoryResults) > 0 && !req.SkipCompiler {
		log.Printf("[DEBUG] /api/apply: running compiler preflight")
		compErrs := runCompilerPreFlight(patcheng.DefaultRegistry, memoryResults, false)
		for p, e := range compErrs {
			rel, errRel := filepath.Rel(s.rootDir, p)
			if errRel != nil {
				log.Printf("[DEBUG] /api/apply: Rel failed for path %s: %v", p, errRel)
				rel = p
			}
			hasErrors = true

			if req.CheckOnly {
				checkFiles = append(checkFiles, CompilerCheckFile{
					Path:           filepath.ToSlash(rel),
					CompilerStatus: "FAIL",
					RawOutput:      e,
				})
			} else {
				for i, af := range applyFiles {
					if af.Path == filepath.ToSlash(rel) || af.Path == rel {
						applyFiles[i].Applied = false
						applyFiles[i].Error = "Compiler Error"
						applyFiles[i].FailedPatch = &FailedPatch{
							Error: "Compiler Error:\n" + e,
						}
						break
					}
				}
			}
			delete(memoryResults, p)
		}

		if req.CheckOnly {
			for p := range memoryResults {
				rel, errRel := filepath.Rel(s.rootDir, p)
				if errRel != nil {
					rel = p
				}
				checkFiles = append(checkFiles, CompilerCheckFile{
					Path:           filepath.ToSlash(rel),
					CompilerStatus: "PASS",
				})
			}
			for p := range filesToDelete {
				rel, errRel := filepath.Rel(s.rootDir, p)
				if errRel != nil {
					rel = p
				}
				checkFiles = append(checkFiles, CompilerCheckFile{
					Path:           filepath.ToSlash(rel),
					CompilerStatus: "PASS",
				})
			}
		}
	}

	if req.CheckOnly {
		w.Header().Set("Content-Type", "application/json")
		encErr := json.NewEncoder(w).Encode(withND("appy/compiler-check", "Compiler pre-flight results", map[string]any{
			"files": checkFiles,
		}))
		if encErr != nil {
			log.Printf("[DEBUG] /api/apply: CheckOnly response encoding failed: %v", encErr)
		}
		return
	}

	if !req.CheckOnly {
		if err := saveHistory(s.rootDir, originalFiles); err != nil {
			log.Printf("Warning: failed to save history: %v", err)
		}
		for path, content := range memoryResults {
			if errMk := os.MkdirAll(filepath.Dir(path), 0755); errMk != nil {
				log.Printf("[DEBUG] /api/apply: MkdirAll failed for %s: %v", path, errMk)
			}
			if errW := os.WriteFile(path, []byte(content), 0644); errW != nil {
				log.Printf("[DEBUG] /api/apply: WriteFile failed for %s: %v", path, errW)
			}
		}
		for path := range filesToDelete {
			if errRm := os.Remove(path); errRm != nil {
				log.Printf("[DEBUG] /api/apply: Remove failed for %s: %v", path, errRm)
			}
		}
	}

	if len(memoryResults) > 0 || len(filesToDelete) > 0 {
		appliedPatchesMu.Lock()
		for path := range memoryResults {
			for _, h := range fileHashes[path] {
				appliedPatches[h] = true
			}
		}
		for path := range filesToDelete {
			for _, h := range fileHashes[path] {
				appliedPatches[h] = true
			}
		}
		appliedPatchesMu.Unlock()
		SaveLedger(s.rootDir)
	}

	status := http.StatusOK
	if hasErrors {
		status = http.StatusMultiStatus
		log.Printf("[DEBUG] /api/apply: Returning StatusMultiStatus due to errors")
	} else {
		log.Printf("[DEBUG] /api/apply: Returning StatusOK")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encErr := json.NewEncoder(w).Encode(withND("appy/apply-success", "Result", map[string]any{
		"files": applyFiles,
	}))
	if encErr != nil {
		log.Printf("[DEBUG] /api/apply: Final response encoding failed: %v", encErr)
	}
}

func (s *AppyServer) handleRetest(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] /api/retest request received")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req RetestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[DEBUG] /api/retest: Invalid JSON payload: %v", err)
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	opts := retest.Options{JSONMode: true, Args: req.Packages}
	report, err := retest.Run(r.Context(), opts)
	if err != nil {
		log.Printf("[DEBUG] /api/retest: retest.Run failed: %v", err)
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var outFiles []RetestResponseFile
	for _, fail := range report.HardFails {
		outFiles = append(outFiles, RetestResponseFile{
			TestStatus: "FAIL",
			Package:    fail.Task.Package,
			RawOutput:  fail.Output,
		})
	}

	response := RetestResponse{
		Packages: req.Packages,
		Files:    outFiles,
	}

	b, errMarsh := json.Marshal(response)
	if errMarsh != nil {
		log.Printf("[DEBUG] /api/retest: JSON marshal failed: %v", errMarsh)
		sendError(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	var rm map[string]any
	if errUnmarsh := json.Unmarshal(b, &rm); errUnmarsh != nil {
		log.Printf("[DEBUG] /api/retest: JSON unmarshal failed: %v", errUnmarsh)
		sendError(w, "Failed to decode response structure", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encErr := json.NewEncoder(w).Encode(withND("appy/retest", "Test execution report", rm))
	if encErr != nil {
		log.Printf("[DEBUG] /api/retest: Final response encoding failed: %v", encErr)
	}
}

func (s *AppyServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] /api/history request received")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	hist, err := listHistory(s.rootDir)
	if err != nil {
		log.Printf("[DEBUG] /api/history: listHistory failed: %v", err)
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	encErr := json.NewEncoder(w).Encode(withND("appy/history", "Patch transaction history", map[string]any{
		"history": hist,
	}))
	if encErr != nil {
		log.Printf("[DEBUG] /api/history: Final response encoding failed: %v", encErr)
	}
}

func (s *AppyServer) handleRevert(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] /api/revert request received")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req RevertPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[DEBUG] /api/revert: Invalid JSON payload: %v", err)
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if err := revertTransaction(s.rootDir, req.TxID); err != nil {
		log.Printf("[DEBUG] /api/revert: revertTransaction failed for %s: %v", req.TxID, err)
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	encErr := json.NewEncoder(w).Encode(withND("appy/revert", "Revert completed", map[string]any{
		"reverted": true,
	}))
	if encErr != nil {
		log.Printf("[DEBUG] /api/revert: Final response encoding failed: %v", encErr)
	}
}
