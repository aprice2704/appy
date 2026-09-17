package main

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

const defaultLargeFileLines = 350

func newTestServer(rootDir string) *http.ServeMux {
	return newServer(rootDir, defaultLargeFileLines)
}

func setupTestWorkspace(t testing.TB) string {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module appytest\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "base.go"), []byte("package mypkg\n"), 0644)
	return dir
}
