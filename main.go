// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 34
// :: description: v1.6.16    -- Graceful skip of already-applied files.
// :: filename: main.go
// :: serialization: go
// :: latestChange: Bumped to 1.6.16 and added ledger check to gracefully skip files that are already fully applied.

package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
	"github.com/aprice2704/fdm/code/retest"
	_ "github.com/aprice2704/fdm/code/treesitter"
)

func watchSelfForReload() {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Hot reload watcher disabled: cannot determine executable path: %v", err)
		return
	}
	stat, err := os.Stat(execPath)
	if err != nil {
		log.Printf("Hot reload watcher disabled: cannot stat executable: %v", err)
		return
	}
	initialModTime := stat.ModTime()

	for {
		time.Sleep(1 * time.Second)
		stat, err := os.Stat(execPath)
		if err == nil && stat.ModTime().After(initialModTime) {
			log.Printf("Binary updated (mod time changed). Triggering hot reload (Exit 42)...")
			os.Exit(42)
		}
	}
}

const AppVersion = "v1.8.12"

//go:embed static/*
var staticFS embed.FS

func getFileMeta(prof *patcheng.LanguageProfile) (string, string) {
	if prof == nil {
		return "Text", "📄"
	}
	switch prof.ID {
	case "golang":
		return "Go", "🐹"
	case "javascript":
		return "JS", "🟨"
	case "typescript":
		return "TS", "🟦"
	case "python":
		return "Python", "🐍"
	case "markdown":
		return "Markdown", "📝"
	case "neuroscript":
		return "NeuroScript", "🧠"
	case "html":
		return "HTML", "🌐"
	case "css":
		return "CSS", "🎨"
	case "json":
		return "JSON", "📦"
	case "yaml":
		return "YAML", "⚙️"
	case "shell":
		return "Shell", "🐚"
	case "java":
		return "Java", "☕"
	case "cpp":
		return "C++", "⚙️"
	case "astro":
		return "Astro", "🚀"
	case "antlr":
		return "ANTLR", "🛠️"
	default:
		return prof.ID, "📄"
	}
}

func withRecoveryAndCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC: %v", rec)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				err := json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Server Panic: %v", rec)})
				if err != nil {
					log.Printf("[DEBUG] Failed to encode panic response: %v", err)
				}
			}
		}()
		h(w, r)
	}
}

func newServer(rootDir string, largeFileLines int, quickAdds ...string) *http.ServeMux {
	qa := ""
	if len(quickAdds) > 0 {
		qa = quickAdds[0]
	}
	mux := http.NewServeMux()
	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		log.Fatalf("Failed to resolve absolute root dir: %v", err)
	}
	LoadLedger(absRootDir)

	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	var qaHtml strings.Builder
	if qa != "" {
		qaHtml.WriteString(`<label style="color: #94a3b8; font-size: 13px; margin-top: 10px;">Quick Add:</label>`)
		qaHtml.WriteString(`<div style="display: flex; gap: 8px; flex-wrap: wrap;">`)
		for _, path := range strings.Split(qa, ",") {
			p := strings.TrimSpace(path)
			if p != "" {
				qaHtml.WriteString(fmt.Sprintf(`
				<div style="display: flex; gap: 2px;">
					<button onclick="addTxtarPath('%s')" style="background: #334155; border: 1px solid #475569; border-radius: 4px 0 0 4px; color: white;">%s</button>
					<button onclick="autoQuickAdd('%s')" title="Auto-bundle %s" style="background: #0ea5e9; border: 1px solid #0284c7; border-radius: 0 4px 4px 0; color: white; padding: 0 8px;">⚡</button>
				</div>`, p, p, p, p))
			}
		}
		qaHtml.WriteString(`</div>`)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		indexBytes, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "Internal Server Error", 500)
			return
		}
		html := strings.ReplaceAll(string(indexBytes), "{TITLE}", filepath.Base(absRootDir))
		html = strings.ReplaceAll(html, "{VERSION}", AppVersion)
		html = strings.ReplaceAll(html, "{ROOT_DIR}", absRootDir)
		html = strings.ReplaceAll(html, "{QUICK_ADDS_HTML}", qaHtml.String())
		w.Header().Set("Content-Type", "text/html")
		_, writeErr := w.Write([]byte(html))
		if writeErr != nil {
			log.Printf("[DEBUG] Failed to write index.html response: %v", writeErr)
		}
	})

	mux.HandleFunc("/api/preview", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
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
			if !isPathSafe(absRootDir, rawFilename) {
				log.Printf("[DEBUG] /api/preview: unsafe path rejected: %s", rawFilename)
				continue
			}

			absPath := filepath.Join(absRootDir, rawFilename)
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
							pp.Error = "File already exists. Use '%%% overwrite' to replace it."
							if fileStatus != "ERROR" {
								fileStatus = "IGNORED"
							}
						} else if strings.Contains(errMsg, "target file is empty or does not exist") {
							pp.Error = "Target file missing. Click 'Fix File Paths'."
							fileStatus = "ERROR"
							if fixed := findUniquePathSuffix(absRootDir, rawFilename); fixed != "" {
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
	}))

	mux.HandleFunc("/api/apply", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
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
			if !isPathSafe(absRootDir, rawFilename) {
				log.Printf("[DEBUG] /api/apply: unsafe path rejected: %s", rawFilename)
				sendError(w, "Path traversal denied", http.StatusBadRequest)
				return
			}
			absPath := filepath.Join(absRootDir, rawFilename)
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
				rel, errRel := filepath.Rel(absRootDir, p)
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
					rel, errRel := filepath.Rel(absRootDir, p)
					if errRel != nil {
						rel = p
					}
					checkFiles = append(checkFiles, CompilerCheckFile{
						Path:           filepath.ToSlash(rel),
						CompilerStatus: "PASS",
					})
				}
				for p := range filesToDelete {
					rel, errRel := filepath.Rel(absRootDir, p)
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
			if err := saveHistory(absRootDir, originalFiles); err != nil {
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
			SaveLedger(absRootDir)
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
	}))

	mux.HandleFunc("/api/retest", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
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

		// Map the internal retest.Report to the strict RetestResponse UI schema
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
	}))

	mux.HandleFunc("/api/history", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[DEBUG] /api/history request received")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		hist, err := listHistory(absRootDir)
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
	}))

	mux.HandleFunc("/api/revert", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
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
		if err := revertTransaction(absRootDir, req.TxID); err != nil {
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
	}))

	mux.HandleFunc("/api/resolve_path", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		name := r.URL.Query().Get("name")
		if name == "" {
			sendError(w, "missing name parameter", http.StatusBadRequest)
			return
		}
		match := findUniquePathSuffix(absRootDir, name)
		w.Header().Set("Content-Type", "application/json")
		if match != "" {
			json.NewEncoder(w).Encode(map[string]string{"path": match})
		} else {
			json.NewEncoder(w).Encode(map[string]string{"path": name})
		}
	}))

	mux.HandleFunc("/api/txtar", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[DEBUG] /api/txtar request received")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req TxtarPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[DEBUG] /api/txtar: Invalid JSON: %v", err)
			sendError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		var buf bytes.Buffer
		buf.WriteString(req.Preface)
		if !strings.HasSuffix(req.Preface, "\n") {
			buf.WriteString("\n")
		}

		fileCount := 0
		errWalk := filepath.WalkDir(absRootDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				log.Printf("[DEBUG] /api/txtar: Walk error at %s: %v", path, err)
				return nil
			}

			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				if d.Name() == "vendor" || d.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}

			rel, errRel := filepath.Rel(absRootDir, path)
			if errRel != nil {
				log.Printf("[DEBUG] /api/txtar: Rel error %s: %v", path, errRel)
				return nil
			}

			included := false
			if len(req.Paths) == 0 {
				included = true
			} else {
				for _, p := range req.Paths {
					p = strings.TrimPrefix(p, "/") // allow abs from root (/)
					if p == "." || p == "" {
						included = true
						break
					}
					if rel == p || strings.HasPrefix(rel, p+"/") {
						included = true
						break
					}
					if strings.Contains(p, "**") {
						parts := strings.SplitN(p, "**", 2)
						if strings.HasPrefix(rel, parts[0]) && strings.HasSuffix(rel, parts[1]) {
							included = true
							break
						}
					} else {
						if matched, _ := filepath.Match(p, filepath.Base(rel)); matched {
							included = true
							break
						}
						if matched, _ := filepath.Match(p, rel); matched {
							included = true
							break
						}
					}
				}
			}
			if !included {
				return nil
			}

			for _, ex := range req.Excludes {
				if ex == "" {
					continue
				}
				matched, matchErr := filepath.Match(ex, d.Name())
				if matchErr != nil {
					log.Printf("[DEBUG] /api/txtar: Bad exclude pattern %s: %v", ex, matchErr)
					continue
				}
				if !matched {
					matched, matchErr = filepath.Match(ex, rel)
					if matchErr != nil {
						log.Printf("[DEBUG] /api/txtar: Bad exclude pattern %s: %v", ex, matchErr)
						continue
					}
				}
				if matched {
					log.Printf("[DEBUG] /api/txtar: Excluded %s due to rule %s", rel, ex)
					return nil
				}
			}

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				log.Printf("[DEBUG] /api/txtar: Read error %s: %v", path, readErr)
				return nil
			}

			needsAnchor := true
			for _, ag := range req.Anchors {
				if ag == "" {
					continue
				}
				matched, matchErr := filepath.Match(ag, d.Name())
				if matchErr == nil && matched {
					needsAnchor = false
					break
				}
				matched, matchErr = filepath.Match(ag, rel)
				if matchErr == nil && matched {
					needsAnchor = false
					break
				}
			}

			if needsAnchor {
				anchoredContent, err := patcheng.InjectAnchors(rel, content, 10)
				if err == nil {
					content = anchoredContent
				} else {
					log.Printf("[DEBUG] /api/txtar: Anchor injection failed for %s: %v", rel, err)
				}
			}

			if needsAnchor && countLines(string(content)) > largeFileLines {
				warning := []byte("⚠️ APPY NOTE: This file is overly large. If you need to touch it, please split it into sensible pieces if possible.\n\n")
				content = append(warning, content...)
			}

			buf.WriteString(fmt.Sprintf("-- %s --\n", filepath.ToSlash(rel)))
			buf.Write(content)
			if !strings.HasSuffix(string(content), "\n") {
				buf.WriteString("\n")
			}
			fileCount++
			return nil
		})
		if errWalk != nil {
			log.Printf("[DEBUG] /api/txtar: WalkDir fatal error: %v", errWalk)
		}

		fileName := req.FileName
		if fileName == "" {
			fileName = fmt.Sprintf("appy_bundle_%d.txtar", time.Now().Unix())
		} else {
			fileName = filepath.Base(fileName)
			if !strings.HasSuffix(fileName, ".txtar") {
				fileName += ".txtar"
			}
		}
		outPath := filepath.Join(absRootDir, fileName)
		if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
			log.Printf("[DEBUG] /api/txtar: WriteFile error: %v", err)
			sendError(w, fmt.Sprintf("Failed to write %s: %v", fileName, err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		res := map[string]any{
			"success":    true,
			"file_url":   "/api/bundle?name=" + fileName,
			"file_name":  fileName,
			"file_count": fileCount,
		}
		if err := json.NewEncoder(w).Encode(withND("appy/txtar", "Generated txtar bundle", res)); err != nil {
			log.Printf("[DEBUG] /api/txtar: encoding response failed: %v", err)
		}
	}))

	mux.HandleFunc("/api/txtar_stats", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req TxtarPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		fileCount := 0
		var totalBytes int64 = 0

		errWalk := filepath.WalkDir(absRootDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				if d.Name() == "vendor" || d.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, errRel := filepath.Rel(absRootDir, path)
			if errRel != nil {
				return nil
			}

			included := false
			if len(req.Paths) == 0 {
				included = true
			} else {
				for _, p := range req.Paths {
					p = strings.TrimPrefix(p, "/")
					if p == "." || p == "" {
						included = true
						break
					}
					if rel == p || strings.HasPrefix(rel, p+"/") {
						included = true
						break
					}
					if strings.Contains(p, "**") {
						parts := strings.SplitN(p, "**", 2)
						if strings.HasPrefix(rel, parts[0]) && strings.HasSuffix(rel, parts[1]) {
							included = true
							break
						}
					} else {
						if matched, _ := filepath.Match(p, filepath.Base(rel)); matched {
							included = true
							break
						}
						if matched, _ := filepath.Match(p, rel); matched {
							included = true
							break
						}
					}
				}
			}
			if !included {
				return nil
			}

			for _, ex := range req.Excludes {
				if ex == "" {
					continue
				}
				matched, matchErr := filepath.Match(ex, d.Name())
				if matchErr != nil {
					continue
				}
				if !matched {
					matched, _ = filepath.Match(ex, rel)
				}
				if matched {
					return nil
				}
			}

			info, infoErr := d.Info()
			if infoErr == nil {
				fileCount++
				totalBytes += info.Size()
			}
			return nil
		})
		if errWalk != nil {
			log.Printf("[DEBUG] /api/txtar_stats: WalkDir error: %v", errWalk)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"file_count": fileCount,
			"size_kb":    totalBytes / 1024,
			"tokens_est": totalBytes / 4,
		})
	}))

	mux.HandleFunc("/api/bundle", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if !strings.HasSuffix(name, ".txtar") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		http.ServeFile(w, r, filepath.Join(absRootDir, name))
	}))

	return mux
}

func main() {
	port := flag.String("port", "8085", "Port to run the appy server on")
	quickAdds := flag.String("quick-adds", "", "Comma-separated list of paths for Quick Add Txtar buttons")
	largeFileLines := flag.Int("large-file-lines", 350, "Line threshold to inject 'split file' warnings into txtar bundles")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Appy %s - The Stateful Patch Console\n\n", AppVersion)
		fmt.Fprintf(os.Stderr, "Usage:\n  appy [flags]\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n  appy -port 8080\n  appy -quick-adds=\"always/,code/definitions/\"\n")
	}
	flag.Parse()

	go watchSelfForReload()

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current working directory: %v", err)
	}
	fmt.Printf("Appy %s on http://localhost:%s\n", AppVersion, *port)

	log.Fatal(http.ListenAndServe(":"+*port, newServer(cwd, *largeFileLines, *quickAdds)))
}
