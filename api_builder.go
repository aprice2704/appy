package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
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
		"file_path":  outPath,
		"file_count": fileCount,
	}
	if err := json.NewEncoder(w).Encode(withND("appy/txtar", "Generated txtar bundle", res)); err != nil {
		log.Printf("[DEBUG] /api/txtar: encoding response failed: %v", err)
	}
}

func (s *AppyServer) handleTxtarCopy(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] /api/txtar_copy request received")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FileName string `json:"file_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	fileName := filepath.Base(req.FileName)
	if fileName == "" || !strings.HasSuffix(fileName, ".txtar") {
		sendError(w, "Invalid or missing file_name", http.StatusBadRequest)
		return
	}
	absPath, err := filepath.Abs(filepath.Join(s.rootDir, fileName))
	if err != nil {
		sendError(w, fmt.Sprintf("Failed to resolve path: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		sendError(w, fmt.Sprintf("File %s does not exist", fileName), http.StatusNotFound)
		return
	}

	fileURI := "file://" + filepath.ToSlash(absPath)

	var copyErr error
	if _, err := exec.LookPath("wl-copy"); err == nil {
		cmd := exec.Command("wl-copy", "-t", "text/uri-list", fileURI)
		copyErr = cmd.Run()
	} else if _, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "text/uri-list")
		cmd.Stdin = strings.NewReader(fileURI)
		copyErr = cmd.Run()
	} else {
		copyErr = fmt.Errorf("neither wl-copy nor xclip found on system")
	}

	if copyErr != nil {
		log.Printf("[DEBUG] /api/txtar_copy: clipboard tool error: %v", copyErr)
		sendError(w, fmt.Sprintf("Clipboard tool error: %v", copyErr), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":   true,
		"file_name": fileName,
		"file_path": absPath,
		"uri":       fileURI,
	})
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

		isSuperGlob := strings.Contains(pTrim, "{") || strings.Contains(pTrim, "}") ||
			strings.Contains(pTrim, ":") || strings.Contains(pTrim, "*") || strings.Contains(pTrim, "?")

		if isSuperGlob {
			sgFiles, _, _ := EvaluateSuperGlob(s.rootDir, pTrim, req.Excludes)
			pFiles = int64(len(sgFiles))
			for _, f := range sgFiles {
				pBytes += f.Size
			}
			if pFiles == 0 {
				pathStatuses[pTrim] = "zero_matches"
			} else {
				pathStatuses[pTrim] = "valid"
			}
			pathStats[pTrim] = map[string]int64{"files": pFiles, "tokens": pBytes / 4}
			continue
		}

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

	limitExceeded := false
	if len(req.Paths) > 0 {
		walkPaths(s.rootDir, req.Paths, req.Excludes, func(absPath, relName string) {
			if fileCount >= MaxTxtarFileCount {
				limitExceeded = true
				return
			}
			info, err := os.Stat(absPath)
			if err == nil && !info.IsDir() {
				fileCount++
				totalBytes += info.Size()
			}
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"file_count":     fileCount,
		"limit_exceeded": limitExceeded,
		"size_kb":        totalBytes / 1024,
		"tokens_est":     totalBytes / 4,
		"path_fixes":     pathFixes,
		"path_statuses":  pathStatuses,
		"path_stats":     pathStats,
	})
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
