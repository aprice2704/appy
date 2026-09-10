package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aprice2704/fdm/code/patcheng"
)

type FSTreeNode struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size,omitempty"`
	FileIcon string `json:"file_icon,omitempty"`
	FileType string `json:"file_type,omitempty"`
}

func (s *AppyServer) handleFSTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	reqDir := r.URL.Query().Get("dir")
	reqDir = filepath.Clean(reqDir)
	if reqDir == "." || reqDir == "/" || reqDir == "\\" {
		reqDir = ""
	}

	targetAbs := filepath.Join(s.rootDir, reqDir)
	if !isPathSafe(s.rootDir, targetAbs) {
		sendError(w, "Access denied", http.StatusForbidden)
		return
	}

	showHidden := r.URL.Query().Get("hidden") == "true"
	entries, err := os.ReadDir(targetAbs)
	if err != nil {
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var nodes []FSTreeNode
	for _, e := range entries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		if name == "vendor" || name == "node_modules" {
			continue
		}

		relPath := filepath.ToSlash(filepath.Join(reqDir, name))
		node := FSTreeNode{
			Name:  name,
			Path:  relPath,
			IsDir: e.IsDir(),
		}

		if !e.IsDir() {
			if info, err := e.Info(); err == nil {
				node.Size = info.Size()
			}
			prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(name))
			node.FileType, node.FileIcon = getFileMeta(prof)
		}
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir != nodes[j].IsDir {
			return nodes[i].IsDir
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"dir":   filepath.ToSlash(reqDir),
		"nodes": nodes,
	})
}

type SuperGlobEvalPayload struct {
	Expression string   `json:"expression"`
	Excludes   []string `json:"excludes"`
}

func (s *AppyServer) handleSuperGlobEval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req SuperGlobEvalPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	files, parsed, truncated := EvaluateSuperGlob(s.rootDir, req.Expression, req.Excludes)
	var totalBytes int64
	for _, f := range files {
		totalBytes += f.Size
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"files":      files,
		"file_count": len(files),
		"size_kb":    totalBytes / 1024,
		"tokens_est": totalBytes / 4,
		"paths":      parsed.Paths,
		"excludes":   parsed.Excludes,
		"truncated":  truncated,
	})
}
