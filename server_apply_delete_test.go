package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aprice2704/fdm/code/patcheng"
)

func TestAPI_Apply_DeleteFile(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	targetFile := filepath.Join(tempDir, "goner.go")
	os.WriteFile(targetFile, []byte("package mypkg\nfunc Old() {}\n"), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: goner.go
### delete_file
CONFIRM
### end
`, "###", patcheng.BundleDelim),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for delete_file, got %d", w.Result().StatusCode)
	}

	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Errorf("Expected file to be deleted from disk, but it still exists")
	}
}

func TestAPI_Apply_DeleteBlock(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	targetFile := filepath.Join(tempDir, "delete_me.go")
	os.WriteFile(targetFile, []byte("package mypkg\n\n// line 1\n// line 2\nfunc Old() {}\nfunc Keep() {}"), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: delete_me.go
### delete
// line 1
// line 2
func Old() {}
### end
`, "###", patcheng.BundleDelim),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	contentBytes, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed reading target file: %v", err)
	}
	if !strings.Contains(string(contentBytes), "func Keep() {}") || strings.Contains(string(contentBytes), "func Old() {}") {
		t.Errorf("Expected func Old() {} to be deleted, got: %q", string(contentBytes))
	}
}

func TestAPI_Apply_EmptyFileDeletion(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	targetFile := filepath.Join(tempDir, "delete_me.go")
	os.WriteFile(targetFile, []byte("package mypkg\n\n// line 1\n// line 2\nfunc Old() {}"), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: delete_me.go
### delete
package mypkg

// line 1
// line 2
func Old() {}
### end
`, "###", patcheng.BundleDelim),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	contentBytes, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed reading target file: %v", err)
	}
	if len(contentBytes) != 0 {
		t.Errorf("Expected file to be empty, got: %q", string(contentBytes))
	}
}
