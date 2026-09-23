package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aprice2704/fdm/code/patcheng"
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
		appendFailureLog(s.rootDir, PatchFailureLog{
			Phase: "parse",
			File:  "<bundle>",
			Error: err.Error(),
		})
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

		preview, fixes := s.previewSingleFile(rawFilename, patches)
		for k, v := range fixes {
			pathFixes[k] = v
		}
		responseFiles = append(responseFiles, preview)
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

func (s *AppyServer) previewSingleFile(rawFilename string, patches []patcheng.FuzzyPatch) (PreviewFile, map[string]string) {
	absPath := filepath.Join(s.rootDir, rawFilename)
	contentBytes, readErr := os.ReadFile(absPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		log.Printf("[DEBUG] /api/preview: failed to read %s: %v", absPath, readErr)
	}
	content := string(contentBytes)
	prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))
	fType, fIcon := getFileMeta(prof)

	var filePreviews []PreviewPatch
	pathFixes := make(map[string]string)
	fileStatus := FileStatusReady
	fileNetLines := 0

	for _, p := range patches {
		pp, statusOverride, fix := s.previewSinglePatch(rawFilename, content, prof, p)
		if fix != "" {
			pathFixes[rawFilename] = fix
		}
		if statusOverride != "" && fileStatus != FileStatusError {
			fileStatus = statusOverride
		}
		filePreviews = append(filePreviews, pp)
	}

	return PreviewFile{
		Path:     rawFilename,
		Status:   fileStatus,
		NetLines: fileNetLines,
		FileType: FileType(fType),
		FileIcon: fIcon,
		Patches:  filePreviews,
	}, pathFixes
}

func (s *AppyServer) previewSinglePatch(rawFilename, content string, prof *patcheng.LanguageProfile, p patcheng.FuzzyPatch) (PreviewPatch, FileStatus, string) {
	pp := PreviewPatch{
		SearchBlock:  p.Search,
		ReplaceBlock: p.Replace,
		IsOverwrite:  p.FullOverwrite,
		IsDeleteFile: p.IsDeleteFile,
		IsAnchored:   p.IsAnchored,
	}

	// Auto-extract imports before fuzzy validation
	if !p.FullOverwrite && !p.IsDeleteFile && len(p.ImportDirectives) == 0 {
		if strings.Contains(p.Search, "imp"+"ort (") || strings.HasPrefix(strings.TrimSpace(p.Search), "imp"+"ort \"") {
			dirs := extractImportDirectivesFromText(p.Replace)
			if len(dirs) > 0 {
				p.ImportDirectives = dirs
				p.Search = ""
				p.Replace = ""
			}
		}
	}

	patchHash := hashPatch(rawFilename, p.Search, p.Replace)
	appliedPatchesMu.RLock()
	alreadyApplied := appliedPatches[patchHash]
	appliedPatchesMu.RUnlock()

	if alreadyApplied {
		return pp, FileStatusApplied, ""
	}

	var pErr error
	if valErr := ValidateFuzzySearchBlock(p); valErr != nil {
		pErr = valErr
	} else if p.FullOverwrite && p.Search == "CREATE_ASSERT" && len(content) > 0 {
		pErr = fmt.Errorf("refusing to create file: %s already exists (size: %d bytes). Use 'complete_replace' or 'overwrite'", rawFilename, len(content))
	} else if p.FullOverwrite && p.Search == "REPLACE_ASSERT" && len(content) == 0 {
		// Safe auto-promotion: if file does not exist or is empty, promote complete_replace to create/overwrite
		p.Search = "CREATE_ASSERT"
		_, pErr = patcheng.ApplyFuzzyPatchesAgnostic(prof, content, []patcheng.FuzzyPatch{p})
	} else {
		_, pErr = patcheng.ApplyFuzzyPatchesAgnostic(prof, content, []patcheng.FuzzyPatch{p})
	}

	if pErr == nil {
		return pp, "", ""
	}

	log.Printf("[DEBUG] /api/preview: patch preview failed for %s: %v", rawFilename, pErr)
	errMsg := pErr.Error()

	if strings.Contains(errMsg, "refusing to overwrite existing file") {
		pp.Error = "File already exists. Use '\\%%% overwrite' to replace it."
		return pp, FileStatusIgnored, ""
	}

	if strings.Contains(errMsg, "target file is empty or does not exist") {
		pp.Error = "Target file missing. Click 'Fix File Paths'."
		fixed := findUniquePathSuffix(s.rootDir, rawFilename)
		return pp, FileStatusError, fixed
	}

	if strings.Contains(errMsg, "ambiguous") {
		errMsg = "FATAL AMBIGUITY: " + errMsg + " Stop guessing with fuzzy patches! Switch to 'replace_block', 'replace_symbol', or use occurrence index (e.g. '%%% replace 2')."
	}
	pp.Error = errMsg
	pp.ClosestMatchHint = generateDiagnosticHint(prof, rawFilename, content, p.Search, p.NearLine)
	pp.LLMFallbackHint = generateLLMFallbackHint(prof)

	appendFailureLog(s.rootDir, PatchFailureLog{
		Phase:   "preview",
		File:    rawFilename,
		Error:   pErr.Error(),
		Patches: []patcheng.FuzzyPatch{p},
	})

	return pp, FileStatusError, ""
}
