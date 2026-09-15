package main

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed static/*
var staticFS embed.FS

type AppyServer struct {
	rootDir        string
	largeFileLines int
}

func newServer(rootDir string, largeFileLines int) *http.ServeMux {
	s := &AppyServer{
		rootDir:        rootDir,
		largeFileLines: largeFileLines,
	}

	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		log.Fatalf("Failed to resolve absolute root dir: %v", err)
	}
	s.rootDir = absRootDir

	mux := http.NewServeMux()
	LoadLedger(s.rootDir)

	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/", s.handleIndex)

	// Builder API endpoints
	// Builder API endpoints
	mux.HandleFunc("/api/info", withRecoveryAndCORS(s.handleInfo))
	mux.HandleFunc("/api/sets", withRecoveryAndCORS(s.handleSets))
	mux.HandleFunc("/api/txtar", withRecoveryAndCORS(s.handleTxtar))
	mux.HandleFunc("/api/txtar_copy", withRecoveryAndCORS(s.handleTxtarCopy))
	mux.HandleFunc("/api/txtar_stats", withRecoveryAndCORS(s.handleTxtarStats))
	mux.HandleFunc("/api/resolve_path", withRecoveryAndCORS(s.handleResolvePath))
	mux.HandleFunc("/api/resolve_directory", withRecoveryAndCORS(s.handleResolveDirectory))
	mux.HandleFunc("/api/autocomplete_path", withRecoveryAndCORS(s.handleAutocompletePath))
	mux.HandleFunc("/api/fs/tree", withRecoveryAndCORS(s.handleFSTree))
	mux.HandleFunc("/api/superglob/eval", withRecoveryAndCORS(s.handleSuperGlobEval))
	mux.HandleFunc("/api/exec_bash", withRecoveryAndCORS(s.handleExecBash))
	mux.HandleFunc("/api/bundle", withRecoveryAndCORS(s.handleBundle))

	// Patching & History API endpoints
	mux.HandleFunc("/api/preview", withRecoveryAndCORS(s.handlePreview))
	mux.HandleFunc("/api/apply", withRecoveryAndCORS(s.handleApply))
	mux.HandleFunc("/api/forget", withRecoveryAndCORS(s.handleForget))
	mux.HandleFunc("/api/retest", withRecoveryAndCORS(s.handleRetest))
	mux.HandleFunc("/api/history", withRecoveryAndCORS(s.handleHistory))
	mux.HandleFunc("/api/revert", withRecoveryAndCORS(s.handleRevert))

	return mux
}

func (s *AppyServer) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	repoName := filepath.Base(s.rootDir)
	json.NewEncoder(w).Encode(map[string]any{
		"repo_name": repoName,
		"root_dir":  s.rootDir,
		"version":   AppVersion,
	})
}

func (s *AppyServer) readPartial(name string) string {
	b, err := staticFS.ReadFile("static/" + name)
	if err != nil {
		log.Printf("[WARN] Failed to read static template partial %s: %v", name, err)
		return ""
	}
	return string(b)
}

func (s *AppyServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	indexBytes, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	html := string(indexBytes)
	html = strings.ReplaceAll(html, "<!-- INCLUDE:tab_patch -->", s.readPartial("tab_patch.html"))
	html = strings.ReplaceAll(html, "<!-- INCLUDE:tab_builder -->", s.readPartial("tab_builder.html"))
	html = strings.ReplaceAll(html, "<!-- INCLUDE:tab_explorer -->", s.readPartial("tab_explorer.html"))
	html = strings.ReplaceAll(html, "<!-- INCLUDE:tab_history -->", s.readPartial("tab_history.html"))
	html = strings.ReplaceAll(html, "<!-- INCLUDE:modal_scoped -->", s.readPartial("modal_scoped.html"))
	html = strings.ReplaceAll(html, "<!-- INCLUDE:drawer_bash -->", s.readPartial("drawer_bash.html"))

	html = strings.ReplaceAll(html, "{TITLE}", filepath.Base(s.rootDir))
	html = strings.ReplaceAll(html, "{VERSION}", AppVersion)
	html = strings.ReplaceAll(html, "{ROOT_DIR}", s.rootDir)
	w.Header().Set("Content-Type", "text/html")
	if _, writeErr := w.Write([]byte(html)); writeErr != nil {
		log.Printf("[DEBUG] Failed to write index.html response: %v", writeErr)
	}
}
