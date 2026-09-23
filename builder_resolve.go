package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type ResolveDirPayload struct {
	DirName     string   `json:"dir_name"`
	SampleFiles []string `json:"sample_files"`
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

	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("[DEBUG] handleResolvePath: stat failed on %s: %v", name, err)
			}
		} else {
			rel, err := filepath.Rel(s.rootDir, name)
			outPath := filepath.ToSlash(name)
			if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
				outPath = filepath.ToSlash(rel)
			}
			if encErr := json.NewEncoder(w).Encode(map[string]string{"path": outPath}); encErr != nil {
				log.Printf("[ERROR] handleResolvePath: encoding path failed: %v", encErr)
			}
			return
		}
	}

	directPath := filepath.Join(s.rootDir, name)
	if _, err := os.Stat(directPath); err == nil {
		if encErr := json.NewEncoder(w).Encode(map[string]string{"path": filepath.ToSlash(name)}); encErr != nil {
			log.Printf("[ERROR] handleResolvePath: encoding direct match failed: %v", encErr)
		}
		return
	}

	match := findUniquePathSuffix(s.rootDir, name)
	if match != "" {
		if encErr := json.NewEncoder(w).Encode(map[string]string{"path": match}); encErr != nil {
			log.Printf("[ERROR] handleResolvePath: encoding suffix match failed: %v", encErr)
		}
		return
	}

	matches := findAllPathSuffixMatches(s.rootDir, name)
	if len(matches) > 1 {
		if encErr := json.NewEncoder(w).Encode(map[string]any{
			"path":       filepath.ToSlash(name),
			"ambiguous":  true,
			"candidates": matches,
		}); encErr != nil {
			log.Printf("[ERROR] handleResolvePath: encoding ambiguous candidates failed: %v", encErr)
		}
		return
	}
	if encErr := json.NewEncoder(w).Encode(map[string]string{"path": filepath.ToSlash(name)}); encErr != nil {
		log.Printf("[ERROR] handleResolvePath: encoding default path failed: %v", encErr)
	}
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

func formatSuggestionPath(fullCandidate, rootDir string, isAbs bool, isDir bool) string {
	outPath := filepath.ToSlash(fullCandidate)
	if !isAbs {
		rel, relErr := filepath.Rel(rootDir, fullCandidate)
		if relErr == nil && !strings.HasPrefix(rel, "..") {
			outPath = filepath.ToSlash(rel)
		}
	}
	if isDir {
		outPath += "/"
	}
	return outPath
}

func (s *AppyServer) handleAutocompletePath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	prefix := strings.TrimSpace(r.URL.Query().Get("prefix"))

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
	if err != nil {
		log.Printf("[DEBUG] handleAutocompletePath: ReadDir failed for %s: %v", searchDir, err)
	} else {
		lowerPrefix := strings.ToLower(filePrefix)
		for _, e := range entries {
			name := e.Name()
			if name == ".git" || name == "node_modules" || name == ".appy_history" {
				continue
			}
			if filePrefix != "" && !strings.HasPrefix(strings.ToLower(name), lowerPrefix) {
				continue
			}
			fullCandidate := filepath.Join(searchDir, name)
			suggestions = append(suggestions, formatSuggestionPath(fullCandidate, s.rootDir, isAbs, e.IsDir()))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"suggestions": suggestions}); err != nil {
		log.Printf("[ERROR] handleAutocompletePath: encoding response failed: %v", err)
	}
}
