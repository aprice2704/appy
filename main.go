// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 25
// :: description: v1.5.16 - Restored rocket icons.
// :: filename: /home/aprice/dev/appy/main.go
// :: serialization: go

// :: fileVersion: 25
// :: description: v1.5.16 - Restored rocket icons.
// :: filename: /home/aprice/dev/appy/main.go
// :: serialization: go
// :: fileVersion: 26
// :: description: v1.5.18 - AST-Aware HTML/Astro patching and UI upgrades.
// :: filename: /home/aprice/dev/appy/main.go
// :: serialization: go
// :: fileVersion: 26
// :: description: v1.5.18 - AST-Aware HTML/Astro patching and UI upgrades.
// :: filename: /home/aprice/dev/appy/main.go
// :: serialization: go
// :: fileVersion: 27
// :: description: v1.5.19 - State-machine UI bug fixes, decorators, and test reporting.
// :: filename: /home/aprice/dev/appy/main.go
// :: serialization: go
package main

import (
	"context"
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

const AppVersion = "v1.5.19"

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
				json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Server Panic: %v", rec)})
			}
		}()
		h(w, r)
	}
}

func newServer(rootDir string) *http.ServeMux {
	mux := http.NewServeMux()
	absRootDir, _ := filepath.Abs(rootDir)
	LoadLedger(absRootDir)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		html := strings.ReplaceAll(indexHTML, "{TITLE}", filepath.Base(absRootDir))
		html = strings.ReplaceAll(html, "{VERSION}", AppVersion)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	mux.HandleFunc("/api/preview", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req Payload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		req.Bundle = preprocessDeleteBlocks(req.Bundle, patcheng.BundleDelim)

		parsed, err := patcheng.ParseTextBundle(req.Bundle, patcheng.DefaultRegistry)
		if err != nil {
			sendError(w, err.Error(), http.StatusBadRequest)
			return
		}

		responsePatches := make(map[string][]PreviewPatch)
		pathFixes := make(map[string]string)

		for rawFilename, patches := range parsed {
			if !isPathSafe(absRootDir, rawFilename) {
				continue
			}

			absPath := filepath.Join(absRootDir, rawFilename)
			contentBytes, _ := os.ReadFile(absPath)
			content := string(contentBytes)

			var filePreviews []PreviewPatch
			for _, p := range patches {
				delta := 0
				if p.FullOverwrite {
					delta = countLines(p.Replace) - countLines(content)
				} else {
					delta = countLines(p.Replace) - countLines(p.Search)
				}

				pp := PreviewPatch{
					Search:    p.Search,
					Replace:   p.Replace,
					Index:     p.Index,
					LineNum:   p.LineNum,
					Status:    "ok",
					LineDelta: delta,
				}

				patchHash := hashPatch(rawFilename, p.Search, p.Replace)
				appliedPatchesMu.RLock()
				alreadyApplied := appliedPatches[patchHash]
				appliedPatchesMu.RUnlock()

				if alreadyApplied {
					pp.Status = "applied"
				} else {
					prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))
					_, pErr := patcheng.ApplyFuzzyPatchesAgnostic(prof, content, []patcheng.FuzzyPatch{p})
					if pErr != nil {
						errMsg := pErr.Error()
						if strings.Contains(errMsg, "refusing to overwrite existing file") {
							pp.Status = "ignored"
							pp.Message = "File already exists. Use '%%% overwrite' to replace it."
						} else if strings.Contains(errMsg, "target file is empty or does not exist") {
							pp.Status = "ignored"
							pp.Message = "Target file missing. Click 'Fix File Paths'."
							if fixed := findUniquePathSuffix(absRootDir, rawFilename); fixed != "" {
								pathFixes[rawFilename] = fixed
							}
						} else {
							pp.Status = "error"
							pp.Message = pErr.Error()
							pp.Advisory = generateAdvisory(prof, pErr)
							pp.Hint = generateDiagnosticHint(prof, rawFilename, content, p.Search, p.NearLine)
						}
					}
				}

				filePreviews = append(filePreviews, pp)
			}
			responsePatches[rawFilename] = filePreviews
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(withND("appy/preview", "Simulation results", map[string]any{
			"patches":    responsePatches,
			"path_fixes": pathFixes,
		}))
	}))

	mux.HandleFunc("/api/apply", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req Payload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		req.Bundle = preprocessDeleteBlocks(req.Bundle, patcheng.BundleDelim)

		parsed, err := patcheng.ParseTextBundle(req.Bundle, patcheng.DefaultRegistry)
		if err != nil {
			sendError(w, err.Error(), http.StatusBadRequest)
			return
		}

		memoryResults := make(map[string]string)
		fileErrors := make(map[string]string)
		rejectedFiles := make(map[string]RejectedFile)
		appliedInThisBatch := make(map[string]bool)

		for rawFilename, patches := range parsed {
			if !isPathSafe(absRootDir, rawFilename) {
				sendError(w, "Path traversal denied", http.StatusBadRequest)
				return
			}
			absPath := filepath.Join(absRootDir, rawFilename)
			contentBytes, _ := os.ReadFile(absPath)
			prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))

			newContent, applyErr := patcheng.ApplyFuzzyPatchesAgnostic(prof, string(contentBytes), patches)
			if applyErr != nil {
				// Safety ignores do not trigger 207 Multi-Status
				if strings.Contains(applyErr.Error(), "refusing to overwrite existing file") {
					fileErrors[rawFilename] = applyErr.Error()
					continue
				}

				var successes []string
				var failedBlock *FailedPatch

				for _, p := range patches {
					_, pErr := patcheng.ApplyFuzzyPatchesAgnostic(prof, string(contentBytes), []patcheng.FuzzyPatch{p})
					if pErr == nil {
						successes = append(successes, patcheng.FormatMissingSnippet(p.Search))
					} else if failedBlock == nil {
						cur := ""
						hint := generateDiagnosticHint(prof, rawFilename, string(contentBytes), p.Search, p.NearLine)
						if strings.Contains(hint, ": ") {
							lines := strings.Split(hint, "\n")
							for _, l := range lines {
								if strings.Contains(l, ": ") && !strings.Contains(l, "elided") {
									cur = strings.TrimSpace(strings.SplitN(l, ": ", 2)[1])
									break
								}
							}
						}
						failedBlock = &FailedPatch{
							Directive: fmt.Sprintf("replace near %d", p.NearLine),
							Reason:    pErr.Error(),
							Current:   cur,
						}
					}
				}

				fileErrors[rawFilename] = applyErr.Error()
				rejectedFiles[rawFilename] = RejectedFile{
					Filename:                rawFilename,
					Status:                  "rejected",
					Reason:                  applyErr.Error() + " (all patches for this file were rolled back)",
					Committed:               false,
					SuccessfulMemoryPatches: successes,
					FailedPatch:             failedBlock,
				}
				continue
			}

			// Apply language-specific formatting before staging
			if prof != nil && prof.Formatter != nil {
				formatted, _, err := prof.Formatter(context.Background(), []byte(newContent))
				if err == nil {
					newContent = string(formatted)
				}
			}

			memoryResults[absPath] = newContent
			for _, p := range patches {
				appliedInThisBatch[hashPatch(rawFilename, p.Search, p.Replace)] = true
			}
		}

		if len(memoryResults) > 0 && !req.SkipCompiler {
			compErrs := runCompilerPreFlight(patcheng.DefaultRegistry, memoryResults, false)
			for p, e := range compErrs {
				rel, _ := filepath.Rel(absRootDir, p)
				fileErrors[rel] = e
				rejectedFiles[rel] = RejectedFile{
					Filename: rel,
					Status:   "rejected",
					Reason:   "Compiler Error: " + e,
				}
				delete(memoryResults, p)
			}
		}

		filesModified := 0
		if !req.CheckOnly {
			for path, content := range memoryResults {
				os.MkdirAll(filepath.Dir(path), 0755)
				os.WriteFile(path, []byte(content), 0644)
				filesModified++
			}
		}

		if filesModified > 0 {
			appliedPatchesMu.Lock()
			for h := range appliedInThisBatch {
				appliedPatches[h] = true
			}
			appliedPatchesMu.Unlock()
			SaveLedger(absRootDir)
		}

		status := http.StatusOK
		if len(rejectedFiles) > 0 {
			status = http.StatusMultiStatus
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(withND("appy/apply-success", "Result", map[string]any{
			"status":                     "ok",
			"files_modified":             filesModified,
			"file_errors":                fileErrors,
			"rejected_files":             rejectedFiles,
			"successful_files_committed": getKeysFromMemory(memoryResults, absRootDir),
		}))
	}))

	mux.HandleFunc("/api/retest", withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req RetestPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		opts := retest.Options{JSONMode: true, Args: req.Packages}
		report, err := retest.Run(r.Context(), opts)
		if err != nil {
			sendError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		b, _ := json.Marshal(report)
		var rm map[string]any
		json.Unmarshal(b, &rm)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(withND("appy/retest", "Test execution report", rm))
	}))

	return mux
}

func getKeysFromMemory(m map[string]string, root string) []string {
	var keys []string
	for k := range m {
		rel, _ := filepath.Rel(root, k)
		keys = append(keys, filepath.ToSlash(rel))
	}
	return keys
}

func main() {
	port := flag.String("port", "8085", "Port to run the appy server on")
	flag.Parse()

	go watchSelfForReload()

	cwd, _ := os.Getwd()
	fmt.Printf("Appy %s on http://localhost:%s\n", AppVersion, *port)

	http.ListenAndServe(":"+*port, newServer(cwd))
}
