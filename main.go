// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 31
// :: description: v1.6.0    -- treesitter and many other updates
// :: filename: main.go
// :: serialization: go
// :: latestChange: Bumped to 1.6.3 and fixed Makefile tab stripping in unarmorText.
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

const AppVersion = "v1.6.3"

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
		html = strings.ReplaceAll(html, "{ROOT_DIR}", absRootDir)
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

		var responseFiles []PreviewFile
		pathFixes := make(map[string]string)

		for rawFilename, patches := range parsed {
			if !isPathSafe(absRootDir, rawFilename) {
				continue
			}

			absPath := filepath.Join(absRootDir, rawFilename)
			contentBytes, _ := os.ReadFile(absPath)
			content := string(contentBytes)
			prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))
			fType, fIcon := getFileMeta(prof)

			var filePreviews []PreviewPatch
			fileStatus := "READY"
			fileNetLines := 0

			for _, p := range patches {
				delta := 0
				if p.FullOverwrite {
					delta = countLines(p.Replace) - countLines(content)
				} else {
					delta = countLines(p.Replace) - countLines(p.Search)
				}
				fileNetLines += delta

				pp := PreviewPatch{
					SearchBlock:  p.Search,
					ReplaceBlock: p.Replace,
					IsOverwrite:  p.FullOverwrite,
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
		json.NewEncoder(w).Encode(withND("appy/preview", "Simulation results", map[string]any{
			"files":      responseFiles,
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
		appliedInThisBatch := make(map[string]bool)

		var applyFiles []ApplyFile
		var checkFiles []CompilerCheckFile
		hasErrors := false

		for rawFilename, patches := range parsed {
			if !isPathSafe(absRootDir, rawFilename) {
				sendError(w, "Path traversal denied", http.StatusBadRequest)
				return
			}
			absPath := filepath.Join(absRootDir, rawFilename)
			contentBytes, _ := os.ReadFile(absPath)
			prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))
			fType, fIcon := getFileMeta(prof)

			fileNetLines := 0
			for _, p := range patches {
				if p.FullOverwrite {
					fileNetLines += countLines(p.Replace) - countLines(string(contentBytes))
				} else {
					fileNetLines += countLines(p.Replace) - countLines(p.Search)
				}
			}

			newContent, applyErr := patcheng.ApplyFuzzyPatchesAgnostic(prof, string(contentBytes), patches)
			if applyErr != nil {
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

			applyFiles = append(applyFiles, ApplyFile{
				Path:     rawFilename,
				Applied:  true,
				NetLines: fileNetLines,
				FileType: fType,
				FileIcon: fIcon,
			})
		}

		if len(memoryResults) > 0 && !req.SkipCompiler {
			compErrs := runCompilerPreFlight(patcheng.DefaultRegistry, memoryResults, false)
			for p, e := range compErrs {
				rel, _ := filepath.Rel(absRootDir, p)
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
					rel, _ := filepath.Rel(absRootDir, p)
					checkFiles = append(checkFiles, CompilerCheckFile{
						Path:           filepath.ToSlash(rel),
						CompilerStatus: "PASS",
					})
				}
			}
		}

		if req.CheckOnly {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(withND("appy/compiler-check", "Compiler pre-flight results", map[string]any{
				"files": checkFiles,
			}))
			return
		}

		if !req.CheckOnly {
			for path, content := range memoryResults {
				os.MkdirAll(filepath.Dir(path), 0755)
				os.WriteFile(path, []byte(content), 0644)
			}
		}

		if len(memoryResults) > 0 {
			appliedPatchesMu.Lock()
			for h := range appliedInThisBatch {
				appliedPatches[h] = true
			}
			appliedPatchesMu.Unlock()
			SaveLedger(absRootDir)
		}

		status := http.StatusOK
		if hasErrors {
			status = http.StatusMultiStatus
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(withND("appy/apply-success", "Result", map[string]any{
			"files": applyFiles,
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

		b, _ := json.Marshal(response)
		var rm map[string]any
		json.Unmarshal(b, &rm)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(withND("appy/retest", "Test execution report", rm))
	}))

	return mux
}

func main() {
	port := flag.String("port", "8085", "Port to run the appy server on")
	flag.Parse()

	go watchSelfForReload()

	cwd, _ := os.Getwd()
	fmt.Printf("Appy %s on http://localhost:%s\n", AppVersion, *port)

	http.ListenAndServe(":"+*port, newServer(cwd))
}
