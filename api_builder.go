package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *AppyServer) handleSets(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(getSets(s.rootDir))
		return
	}
	if r.Method == http.MethodPost {
		var sets map[string]TxtarPayload
		if err := json.NewDecoder(r.Body).Decode(&sets); err != nil {
			sendError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := saveSets(s.rootDir, sets); err != nil {
			sendError(w, "Failed to save sets", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func (s *AppyServer) handleTxtar(w http.ResponseWriter, r *http.Request) {
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

	b, fileCount, err := generateTxtar(s.rootDir, req, s.largeFileLines)
	if err != nil {
		sendError(w, fmt.Sprintf("Generation failed: %v", err), http.StatusInternalServerError)
		return
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
	outPath := filepath.Join(s.rootDir, fileName)
	if err := os.WriteFile(outPath, b, 0644); err != nil {
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
}

func (s *AppyServer) handleTxtarStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req TxtarPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	var fileCount int
	var totalBytes int64
	pathFixes := make(map[string]string)
	pathStatuses := make(map[string]string)
	pathStats := make(map[string]map[string]int64)

	for _, p := range req.Paths {
		pTrim := strings.TrimSpace(p)
		if pTrim == "" {
			continue
		}

		var pFiles int64
		var pBytes int64

		if strings.Contains(pTrim, "*") || strings.Contains(pTrim, "?") {
			walkPaths(s.rootDir, []string{pTrim}, req.Excludes, func(absPath, relName string) {
				if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
					pFiles++
					pBytes += info.Size()
				}
			})
			if pFiles == 0 {
				pathStatuses[pTrim] = "zero_matches"
			} else {
				pathStatuses[pTrim] = "valid"
			}
			pathStats[pTrim] = map[string]int64{"files": pFiles, "tokens": pBytes / 4}
			continue
		}

		// Exact path check
		baseDir := pTrim
		if !filepath.IsAbs(baseDir) {
			baseDir = filepath.Join(s.rootDir, baseDir)
		}
		if _, err := os.Stat(baseDir); os.IsNotExist(err) {
			pathStatuses[pTrim] = "not_found"
			if fixed := findUniquePathSuffix(s.rootDir, pTrim); fixed != "" && fixed != filepath.ToSlash(pTrim) {
				pathFixes[pTrim] = fixed
			}
			pathStats[pTrim] = map[string]int64{"files": 0, "tokens": 0}
		} else {
			walkPaths(s.rootDir, []string{pTrim}, req.Excludes, func(absPath, relName string) {
				if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
					pFiles++
					pBytes += info.Size()
				}
			})
			if pFiles == 0 {
				pathStatuses[pTrim] = "zero_matches"
			} else {
				pathStatuses[pTrim] = "valid"
			}
			pathStats[pTrim] = map[string]int64{"files": pFiles, "tokens": pBytes / 4}
		}
	}

	if len(req.Paths) > 0 {
		walkPaths(s.rootDir, req.Paths, req.Excludes, func(absPath, relName string) {
			info, err := os.Stat(absPath)
			if err == nil && !info.IsDir() {
				fileCount++
				totalBytes += info.Size()
			}
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"file_count":    fileCount,
		"size_kb":       totalBytes / 1024,
		"tokens_est":    totalBytes / 4,
		"path_fixes":    pathFixes,
		"path_statuses": pathStatuses,
		"path_stats":    pathStats,
	})
}

func (s *AppyServer) handleResolvePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		sendError(w, "missing name parameter", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// 1. If it's an absolute path that exists outside the root, preserve full path verbatim
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			rel, err := filepath.Rel(s.rootDir, name)
			if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
				json.NewEncoder(w).Encode(map[string]string{"path": filepath.ToSlash(rel)})
			} else {
				json.NewEncoder(w).Encode(map[string]string{"path": filepath.ToSlash(name)})
			}
			return
		}
	}

	// 2. If it exists directly inside root, return the exact relative path
	directPath := filepath.Join(s.rootDir, name)
	if _, err := os.Stat(directPath); err == nil {
		json.NewEncoder(w).Encode(map[string]string{"path": filepath.ToSlash(name)})
		return
	}

	// 3. Fallback: try finding unique suffix match
	match := findUniquePathSuffix(s.rootDir, name)
	if match != "" {
		json.NewEncoder(w).Encode(map[string]string{"path": match})
	} else {
		matches := findAllPathSuffixMatches(s.rootDir, name)
		if len(matches) > 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"path":       filepath.ToSlash(name),
				"ambiguous":  true,
				"candidates": matches,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"path": filepath.ToSlash(name)})
	}
}

type ResolveDirPayload struct {
	DirName     string   `json:"dir_name"`
	SampleFiles []string `json:"sample_files"`
}

func (s *AppyServer) handleResolveDirectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req ResolveDirPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	resolved := resolveDirectoryUnderRoot(s.rootDir, req.DirName, req.SampleFiles)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"path": filepath.ToSlash(resolved),
	})
}

func (s *AppyServer) handleAutocompletePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	prefix := r.URL.Query().Get("prefix")
	prefix = strings.TrimSpace(prefix)

	var searchDir string
	var filePrefix string
	isAbs := filepath.IsAbs(prefix)

	if isAbs {
		searchDir = filepath.Dir(prefix)
		filePrefix = filepath.Base(prefix)
		if strings.HasSuffix(prefix, "/") || strings.HasSuffix(prefix, "\\") {
			searchDir = prefix
			filePrefix = ""
		}
	} else {
		cleanPrefix := filepath.Clean(prefix)
		if strings.HasSuffix(prefix, "/") || strings.HasSuffix(prefix, "\\") || prefix == "" {
			searchDir = filepath.Join(s.rootDir, cleanPrefix)
			filePrefix = ""
		} else {
			searchDir = filepath.Join(s.rootDir, filepath.Dir(cleanPrefix))
			filePrefix = filepath.Base(cleanPrefix)
		}
	}

	entries, err := os.ReadDir(searchDir)
	var suggestions []string
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if name == ".git" || name == "node_modules" || name == ".appy_history" {
				continue
			}
			if filePrefix == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(filePrefix)) {
				fullCandidate := filepath.Join(searchDir, name)
				var outPath string
				if isAbs {
					outPath = filepath.ToSlash(fullCandidate)
				} else {
					rel, relErr := filepath.Rel(s.rootDir, fullCandidate)
					if relErr == nil && !strings.HasPrefix(rel, "..") {
						outPath = filepath.ToSlash(rel)
					} else {
						outPath = filepath.ToSlash(fullCandidate)
					}
				}
				if e.IsDir() {
					outPath += "/"
				}
				suggestions = append(suggestions, outPath)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"suggestions": suggestions})
}

func (s *AppyServer) handleBundle(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if !strings.HasSuffix(name, ".txtar") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	http.ServeFile(w, r, filepath.Join(s.rootDir, name))
}
