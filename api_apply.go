package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
)

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

	memoryResults := make(map[string]string)
	filesToDelete := make(map[string]bool)
	fileHashes := make(map[string][]string)
	originalFiles := make(map[string]*[]byte)
	var applyFiles []ApplyFile
	hasErrors := false

	for rawFilename, patches := range parsed {
		if !isPathSafe(s.rootDir, rawFilename) {
			sendError(w, "Path traversal denied", http.StatusBadRequest)
			return
		}

		af, newContent, isDelete, hashes, err := s.applySingleFile(rawFilename, patches, originalFiles)
		if err != nil {
			if strings.Contains(err.Error(), "refusing to overwrite existing file") {
				continue // Safety ignores don't count as errors
			}
			hasErrors = true
			applyFiles = append(applyFiles, af)
			continue
		}

		applyFiles = append(applyFiles, af)
		absPath := filepath.Join(s.rootDir, rawFilename)
		if isDelete {
			filesToDelete[absPath] = true
		} else if af.Applied {
			memoryResults[absPath] = newContent
		}
		fileHashes[absPath] = hashes
	}

	if len(memoryResults) > 0 && !req.SkipCompiler {
		if s.runCompilerChecks(w, req, memoryResults, filesToDelete, &applyFiles, &hasErrors) {
			return
		}
	}

	if !req.CheckOnly {
		s.commitChangesToDisk(originalFiles, memoryResults, filesToDelete, fileHashes)
	}

	status := http.StatusOK
	if hasErrors {
		status = http.StatusMultiStatus
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(withND("appy/apply-success", "Result", map[string]any{
		"files": applyFiles,
	}))
}

func (s *AppyServer) applySingleFile(rawFilename string, patches []patcheng.FuzzyPatch, originalFiles map[string]*[]byte) (ApplyFile, string, bool, []string, error) {
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

	var methods []string
	for _, p := range patches {
		methods = append(methods, detectPatchMethod(p))
	}

	// Ledger skip check
	if len(patches) > 0 && s.allPatchesInLedger(rawFilename, patches) {
		appendActivityLog(PatchActivityLog{
			Action:   "apply",
			File:     rawFilename,
			Status:   "SUCCESS",
			Methods:  methods,
			NetLines: 0,
		})
		return ApplyFile{
			Path:     rawFilename,
			Applied:  true,
			NetLines: 0,
			FileType: fType,
			FileIcon: fIcon,
		}, "", false, nil, nil
	}

	// Pre-process and apply
	contentStr := string(contentBytes)
	fileNetLines := 0
	isDelete := false
	var hashes []string

	for i, p := range patches {
		if !p.FullOverwrite && !p.IsDeleteFile && len(p.ImportDirectives) == 0 {
			if strings.Contains(p.Search, "imp"+"ort (") || strings.HasPrefix(strings.TrimSpace(p.Search), "imp"+"ort \"") {
				dirs := extractImportDirectivesFromText(p.Replace)
				if len(dirs) > 0 {
					patches[i].ImportDirectives = dirs
					patches[i].Search = ""
					patches[i].Replace = ""
					p = patches[i]
				}
			}
		}

		if valErr := ValidateFuzzySearchBlock(p); valErr != nil {
			appendActivityLog(PatchActivityLog{
				Action:   "apply",
				File:     rawFilename,
				Status:   "FAIL",
				Methods:  methods,
				NetLines: 0,
			})
			return s.buildApplyFailure(rawFilename, fType, fIcon, contentBytes, patches, valErr), "", false, nil, valErr
		}

		if p.FullOverwrite {
			if p.Search == "CREATE_ASSERT" && len(contentBytes) > 0 {
				errExist := fmt.Errorf("refusing to create file: %s already exists (size: %d bytes). Use 'complete_replace' or 'overwrite'", rawFilename, len(contentBytes))
				appendActivityLog(PatchActivityLog{
					Action:   "apply",
					File:     rawFilename,
					Status:   "FAIL",
					Methods:  methods,
					NetLines: 0,
				})
				return s.buildApplyFailure(rawFilename, fType, fIcon, contentBytes, patches, errExist), "", false, nil, errExist
			}
			if p.Search == "REPLACE_ASSERT" && len(contentBytes) == 0 {
				errMissing := fmt.Errorf("refusing to complete_replace file: %s does not exist or is empty. Use 'create' or 'overwrite'", rawFilename)
				appendActivityLog(PatchActivityLog{
					Action:   "apply",
					File:     rawFilename,
					Status:   "FAIL",
					Methods:  methods,
					NetLines: 0,
				})
				return s.buildApplyFailure(rawFilename, fType, fIcon, contentBytes, patches, errMissing), "", false, nil, errMissing
			}
			linesBefore := countLines(contentStr)
			fileNetLines += countLines(p.Replace) - linesBefore
		} else if p.IsDeleteFile {
			isDelete = true
			fileNetLines -= countLines(contentStr)
		} else {
			fileNetLines += countLines(p.Replace) - countLines(p.Search)
		}
		hashes = append(hashes, hashPatch(rawFilename, p.Search, p.Replace))
	}

	newContent, applyErr := patcheng.ApplyFuzzyPatchesAgnostic(prof, contentStr, patches)
	if applyErr != nil {
		appendActivityLog(PatchActivityLog{
			Action:   "apply",
			File:     rawFilename,
			Status:   "FAIL",
			Methods:  methods,
			NetLines: 0,
		})
		return s.buildApplyFailure(rawFilename, fType, fIcon, contentBytes, patches, applyErr), "", false, nil, applyErr
	}

	if prof != nil && prof.Formatter != nil && !isDelete {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		formatted, _, err := prof.Formatter(ctx, []byte(newContent))
		cancel()
		if err == nil {
			newContent = string(formatted)
		}
	}

	appendActivityLog(PatchActivityLog{
		Action:   "apply",
		File:     rawFilename,
		Status:   "SUCCESS",
		Methods:  methods,
		NetLines: fileNetLines,
	})

	return ApplyFile{
		Path:     rawFilename,
		Applied:  true,
		NetLines: fileNetLines,
		FileType: fType,
		FileIcon: fIcon,
	}, newContent, isDelete, hashes, nil
}

func (s *AppyServer) allPatchesInLedger(rawFilename string, patches []patcheng.FuzzyPatch) bool {
	appliedPatchesMu.RLock()
	defer appliedPatchesMu.RUnlock()
	for _, p := range patches {
		if !appliedPatches[hashPatch(rawFilename, p.Search, p.Replace)] {
			return false
		}
	}
	return true
}

func (s *AppyServer) buildApplyFailure(rawFilename, fType, fIcon string, contentBytes []byte, patches []patcheng.FuzzyPatch, err error) ApplyFile {
	prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))
	var failedBlock *FailedPatch
	for _, p := range patches {
		hint := generateDiagnosticHint(prof, rawFilename, string(contentBytes), p.Search, p.NearLine)
		cur := ""
		if strings.Contains(hint, ": ") {
			for _, l := range strings.Split(hint, "\n") {
				if strings.Contains(l, ": ") && !strings.Contains(l, "elided") {
					cur = strings.TrimSpace(strings.SplitN(l, ": ", 2)[1])
					break
				}
			}
		}
		failedBlock = &FailedPatch{
			Error:           err.Error(),
			CurrentLineEcho: cur,
			LLMFallbackHint: generateLLMFallbackHint(prof),
		}
		break
	}

	appendFailureLog(s.rootDir, PatchFailureLog{
		Phase:    "apply",
		File:     rawFilename,
		Error:    err.Error(),
		LineEcho: failedBlock.CurrentLineEcho,
		Patches:  patches,
	})

	return ApplyFile{
		Path:        rawFilename,
		Applied:     false,
		FileType:    fType,
		FileIcon:    fIcon,
		Error:       err.Error(),
		FailedPatch: failedBlock,
	}
}

func (s *AppyServer) runCompilerChecks(w http.ResponseWriter, req Payload, memoryResults map[string]string, filesToDelete map[string]bool, applyFiles *[]ApplyFile, hasErrors *bool) bool {
	compErrs := runCompilerPreFlight(patcheng.DefaultRegistry, memoryResults, false)
	var checkFiles []CompilerCheckFile

	for p, e := range compErrs {
		rel, errRel := filepath.Rel(s.rootDir, p)
		if errRel != nil {
			rel = p
		}
		*hasErrors = true

		if req.CheckOnly {
			checkFiles = append(checkFiles, CompilerCheckFile{
				Path:           filepath.ToSlash(rel),
				CompilerStatus: "FAIL",
				RawOutput:      e,
			})
		} else {
			for i, af := range *applyFiles {
				if af.Path == filepath.ToSlash(rel) || af.Path == rel {
					(*applyFiles)[i].Applied = false
					(*applyFiles)[i].Error = "Compiler Error"
					(*applyFiles)[i].FailedPatch = &FailedPatch{
						Error:           "Compiler Error:\n" + e,
						LLMFallbackHint: "Check compiler output and adjust types or imports.",
					}
					break
				}
			}
		}
		appendFailureLog(s.rootDir, PatchFailureLog{
			Phase: "compiler",
			File:  rel,
			Error: e,
		})
		delete(memoryResults, p)
	}

	if req.CheckOnly {
		for p := range memoryResults {
			rel, _ := filepath.Rel(s.rootDir, p)
			checkFiles = append(checkFiles, CompilerCheckFile{
				Path:           filepath.ToSlash(rel),
				CompilerStatus: "PASS",
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(withND("appy/compiler-check", "Compiler pre-flight results", map[string]any{
			"files": checkFiles,
		}))
		return true
	}
	return false
}

func (s *AppyServer) commitChangesToDisk(originalFiles map[string]*[]byte, memoryResults map[string]string, filesToDelete map[string]bool, fileHashes map[string][]string) {
	if err := saveHistory(s.rootDir, originalFiles); err != nil {
		log.Printf("Warning: failed to save history: %v", err)
	}
	for path, content := range memoryResults {
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		_ = os.WriteFile(path, []byte(content), 0644)
	}
	for path := range filesToDelete {
		_ = os.Remove(path)
	}

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
