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

func TestAPI_Apply_ReplaceSymbol(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	targetFile := filepath.Join(tempDir, "symbol.go")
	os.WriteFile(targetFile, []byte("package mypkg\nfunc Old() {}\n"), 0644)

	bundle := strings.ReplaceAll(`
### filename: symbol.go
### replace_symbol Old
### with
func Old(ctx context.Context) {}
### end
`, "###", patcheng.BundleDelim)

	payload := Payload{Bundle: bundle}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Result().StatusCode)
	}

	content, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed reading target file: %v", err)
	}
	if !strings.Contains(string(content), "func Old(ctx context.Context)") {
		t.Errorf("Failed to replace symbol via API. Content:\n%s", string(content))
	}
}

func TestAPI_Apply_TracksHistory(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	targetFile := filepath.Join(tempDir, "history.go")
	os.WriteFile(targetFile, []byte("package mypkg\n\n// line 1\n// line 2\nfunc Old() {}"), 0644)

	bundle := strings.ReplaceAll(`
### filename: history.go
### replace
// line 1
// line 2
func Old() {}
### with
// line 1
// line 2
func New() {}
### end
`, "###", patcheng.BundleDelim)

	payload := Payload{Bundle: bundle}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK on apply")
	}

	reqPreview := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewReader(body))
	wPreview := httptest.NewRecorder()
	mux.ServeHTTP(wPreview, reqPreview)

	var response map[string]any
	if err := json.NewDecoder(wPreview.Result().Body).Decode(&response); err != nil {
		t.Fatalf("failed decoding preview response: %v", err)
	}

	files := response["files"].([]any)
	fileObj := files[0].(map[string]any)
	if fileObj["status"] != "APPLIED" {
		t.Errorf("Expected status 'APPLIED', got %v", fileObj["status"])
	}
}

func TestAPI_Apply_CompilerPreFlightFailure(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	targetFile := filepath.Join(tempDir, "badcode.go")
	os.WriteFile(targetFile, []byte("package mypkg\nfunc Old() {}\n"), 0644)

	goodFile := filepath.Join(tempDir, "goodcode.go")
	os.WriteFile(goodFile, []byte("package mypkg\nfunc Good() {}\n"), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: badcode.go
### replace
func Old() {}
### with
func Old() { this is deliberately invalid syntax!
### end
### filename: goodcode.go
### replace
func Good() {}
### with
func Good() { println("ok") }
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

	res := w.Result()
	if res.StatusCode != http.StatusMultiStatus {
		t.Fatalf("Expected status 207 for compiler failure, got %d", res.StatusCode)
	}

	var response map[string]any
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	files, ok := response["files"].([]any)
	if !ok {
		t.Fatalf("Expected files array, got %v", response)
	}

	foundErr := false
	for _, f := range files {
		fileObj := f.(map[string]any)
		if fileObj["path"] == "badcode.go" && fileObj["applied"] == false {
			foundErr = true
		}
	}
	if !foundErr {
		t.Fatalf("Expected badcode.go to be marked applied: false")
	}

	contentBytes, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("failed reading target file: %v", err)
	}
	if strings.Contains(string(contentBytes), "deliberately invalid syntax") {
		t.Errorf("File was modified on disk despite compiler error!")
	}
}

func TestAPI_Apply_PartialSuccess(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	os.WriteFile(filepath.Join(tempDir, "fileA.go"), []byte("package mypkg\n\n// line 1\n// line 2\nfunc A() {}"), 0644)
	os.WriteFile(filepath.Join(tempDir, "fileB.go"), []byte("package mypkg\n\n// line 1\n// line 2\nfunc B() {}"), 0644)

	bundle := strings.ReplaceAll(`
### filename: fileA.go
### replace
// line 1
// line 2
func A() {}
### with
// line 1
// line 2
func MODIFIED_A() {}
### end

### filename: fileB.go
### replace
// line 1
// line 2
func DOES_NOT_EXIST() {}
### with
// line 1
// line 2
func BROKEN() {}
### end
`, "###", patcheng.BundleDelim)

	payload := Payload{Bundle: bundle}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusMultiStatus {
		t.Fatalf("Expected 207 Multi-Status due to partial failure, got %d", w.Result().StatusCode)
	}

	contentA, err := os.ReadFile(filepath.Join(tempDir, "fileA.go"))
	if err != nil {
		t.Fatalf("failed reading fileA.go: %v", err)
	}
	if !strings.Contains(string(contentA), "func MODIFIED_A() {}") {
		t.Errorf("fileA should have been modified on partial success. Content: %s", string(contentA))
	}

	contentB, err := os.ReadFile(filepath.Join(tempDir, "fileB.go"))
	if err != nil {
		t.Fatalf("failed reading fileB.go: %v", err)
	}
	if !strings.Contains(string(contentB), "func B() {}") {
		t.Errorf("fileB should remain unchanged. Content: %s", string(contentB))
	}
}
