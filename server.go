package main

import (
	"embed"
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
	mux.HandleFunc("/api/sets", withRecoveryAndCORS(s.handleSets))
	mux.HandleFunc("/api/txtar", withRecoveryAndCORS(s.handleTxtar))
	mux.HandleFunc("/api/txtar_stats", withRecoveryAndCORS(s.handleTxtarStats))
	mux.HandleFunc("/api/resolve_path", withRecoveryAndCORS(s.handleResolvePath))
	mux.HandleFunc("/api/bundle", withRecoveryAndCORS(s.handleBundle))

	// Patching & History API endpoints
	mux.HandleFunc("/api/preview", withRecoveryAndCORS(s.handlePreview))
	mux.HandleFunc("/api/apply", withRecoveryAndCORS(s.handleApply))
	mux.HandleFunc("/api/retest", withRecoveryAndCORS(s.handleRetest))
	mux.HandleFunc("/api/history", withRecoveryAndCORS(s.handleHistory))
	mux.HandleFunc("/api/revert", withRecoveryAndCORS(s.handleRevert))

	return mux
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
	html := strings.ReplaceAll(string(indexBytes), "{TITLE}", filepath.Base(s.rootDir))
	html = strings.ReplaceAll(html, "{VERSION}", AppVersion)
	html = strings.ReplaceAll(html, "{ROOT_DIR}", s.rootDir)
	w.Header().Set("Content-Type", "text/html")
	_, writeErr := w.Write([]byte(html))
	if writeErr != nil {
		log.Printf("[DEBUG] Failed to write index.html response: %v", writeErr)
	}
}
