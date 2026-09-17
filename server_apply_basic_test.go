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

func TestAPI_Apply_ValidModify(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	targetFile := filepath.Join(tempDir, "target.go")
	initialContent := "package mypkg\n\n// line 1\n// line 2\nfunc Old() {}\n"
	os.WriteFile(targetFile, []byte(initialContent), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: target.go
### replace
// line 1
// line 2
func Old() {}
### with
// line 1
// line 2
func New() {}
### end
`, "###", patcheng.BundleDelim),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Result().StatusCode, w.Body.String())
	}

	modifiedContentBytes, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed reading target file: %v", err)
	}
	if !strings.Contains(string(modifiedContentBytes), "func New() {}") {
		t.Errorf("File was not correctly patched. Content:\n%s", string(modifiedContentBytes))
	}
}

func TestAPI_Apply_PathTraversalDenied(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	payloads := []string{
		patcheng.BundleDelim + ` filename: ../../../etc/passwd` + "\n" + patcheng.BundleDelim + " replace\n" + patcheng.BundleDelim + " with\n" + patcheng.BundleDelim + " end",
		patcheng.BundleDelim + ` filename: /var/log/syslog` + "\n" + patcheng.BundleDelim + " replace\n" + patcheng.BundleDelim + " with\n" + patcheng.BundleDelim + " end",
	}

	for _, bundleText := range payloads {
		payload := Payload{Bundle: bundleText}
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed marshaling payload: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request for path traversal %s, got %d", bundleText, w.Result().StatusCode)
		}
		if !strings.Contains(w.Body.String(), "Path traversal denied") {
			t.Errorf("Expected path traversal error message, got: %s", w.Body.String())
		}
	}
}

func TestAPI_Apply_CreateNewFile_NativeEngine(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: deep/dir/newfile.go
### replace
### with
package mypkg
func Boot() {}
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
		t.Fatalf("Expected status 200, got %d. Body: %s", w.Result().StatusCode, w.Body.String())
	}

	contentBytes, err := os.ReadFile(filepath.Join(tempDir, "deep", "dir", "newfile.go"))
	if err != nil {
		t.Fatalf("Failed to read newly created file: %v", err)
	}
	expected := "package mypkg\n\nfunc Boot() {}\n"
	if string(contentBytes) != expected {
		t.Errorf("New file content mismatch.\nExpected: %q\nGot: %q", expected, string(contentBytes))
	}
}

func TestAPI_Apply_EmptySearchOverwritesIgnored(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	targetFile := filepath.Join(tempDir, "important.go")
	os.WriteFile(targetFile, []byte("package mypkg\nfunc Old() {}"), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: important.go
### replace
### with
func AccidentalOverwrite() {}
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
		t.Errorf("Expected 200 OK for gracefully ignoring existing file overwrite, got %d", w.Result().StatusCode)
	}

	var response map[string]any
	if err := json.NewDecoder(w.Result().Body).Decode(&response); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	files, ok := response["files"].([]any)
	if ok && len(files) > 0 {
		t.Errorf("Expected 0 files applied (ignored), got %v", files)
	}
}
