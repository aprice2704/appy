// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 7
// :: description: Unit tests for the appy server APIs.
// :: latestChange: Updated to test partial success (file-level transactions) and removed literal patch delimiters to protect parsers.
// :: filename: /home/aprice/dev/appy/server_test.go
// :: serialization: go

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

func setupTestWorkspace(t *testing.T) string {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module appytest\n\ngo 1.22\n"), 0644)
	os.WriteFile(filepath.Join(dir, "base.go"), []byte("package mypkg\n"), 0644)
	return dir
}

func TestAPI_Root(t *testing.T) {
	mux := newServer(setupTestWorkspace(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for root, got %d", w.Result().StatusCode)
	}
}

func TestAPI_RootNotFound(t *testing.T) {
	mux := newServer(setupTestWorkspace(t))
	req := httptest.NewRequest(http.MethodGet, "/random-path", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown path, got %d", w.Result().StatusCode)
	}
}

func TestAPI_MethodNotAllowed(t *testing.T) {
	mux := newServer(setupTestWorkspace(t))

	reqs := []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/preview", nil),
		httptest.NewRequest(http.MethodPut, "/api/apply", nil),
	}

	for _, req := range reqs {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("Expected 405 Method Not Allowed for %s %s, got %d", req.Method, req.URL.Path, w.Result().StatusCode)
		}
	}
}

func TestAPI_InvalidJSON(t *testing.T) {
	mux := newServer(setupTestWorkspace(t))

	reqs := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/preview", strings.NewReader("{bad json")),
		httptest.NewRequest(http.MethodPost, "/api/apply", strings.NewReader("{bad json")),
	}

	for _, req := range reqs {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusBadRequest {
			t.Errorf("Expected 400 Bad Request for invalid JSON on %s, got %d", req.URL.Path, w.Result().StatusCode)
		}
	}
}

func TestAPI_Preview_Valid(t *testing.T) {
	mux := newServer(setupTestWorkspace(t))

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: test.go
### replace
func A() {}
### with
func B() {}
### end
`, "###", patcheng.BundleDelim),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", res.StatusCode)
	}

	var response map[string]any
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	patches, ok := response["patches"].(map[string]any)
	if !ok || len(patches) != 1 {
		t.Fatalf("Expected 1 file in patches map, got %v", patches)
	}
}

func TestAPI_Preview_InvalidSyntax(t *testing.T) {
	mux := newServer(setupTestWorkspace(t))

	payload := Payload{Bundle: patcheng.BundleDelim + " replace\nbroken"}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid bundle syntax, got %d", w.Result().StatusCode)
	}
}

func TestAPI_Apply_ValidModify(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

	targetFile := filepath.Join(tempDir, "target.go")
	initialContent := "package mypkg\n\nfunc Old() {}\n"
	os.WriteFile(targetFile, []byte(initialContent), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: target.go
### replace
func Old() {}
### with
func New() {}
### end
`, "###", patcheng.BundleDelim),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", res.StatusCode, w.Body.String())
	}

	modifiedContentBytes, _ := os.ReadFile(targetFile)
	modifiedContent := string(modifiedContentBytes)

	if !strings.Contains(modifiedContent, "func New() {}") {
		t.Errorf("File was not correctly patched. Content:\n%s", modifiedContent)
	}
}

func TestAPI_Apply_PathTraversalDenied(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

	payloads := []string{
		patcheng.BundleDelim + ` filename: ../../../etc/passwd` + "\n" + patcheng.BundleDelim + " replace\n" + patcheng.BundleDelim + " with\n" + patcheng.BundleDelim + " end",
		patcheng.BundleDelim + ` filename: /var/log/syslog` + "\n" + patcheng.BundleDelim + " replace\n" + patcheng.BundleDelim + " with\n" + patcheng.BundleDelim + " end",
	}

	for _, bundleText := range payloads {
		payload := Payload{Bundle: bundleText}
		body, _ := json.Marshal(payload)

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

func TestAPI_Apply_PartialSuccess(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

	os.WriteFile(filepath.Join(tempDir, "fileA.go"), []byte("package mypkg\nfunc A() {}"), 0644)
	os.WriteFile(filepath.Join(tempDir, "fileB.go"), []byte("package mypkg\nfunc B() {}"), 0644)

	bundle := strings.ReplaceAll(`
### filename: fileA.go
### replace
func A() {}
### with
func MODIFIED_A() {}
### end

### filename: fileB.go
### replace
func DOES_NOT_EXIST() {}
### with
func BROKEN() {}
### end
`, "###", patcheng.BundleDelim)

	payload := Payload{Bundle: bundle}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusMultiStatus {
		t.Fatalf("Expected 207 Multi-Status due to partial failure, got %d", w.Result().StatusCode)
	}

	// fileA should be modified
	contentA, _ := os.ReadFile(filepath.Join(tempDir, "fileA.go"))
	if !strings.Contains(string(contentA), "func MODIFIED_A() {}") {
		t.Errorf("fileA should have been modified on partial success. Content: %s", string(contentA))
	}

	// fileB should NOT be modified
	contentB, _ := os.ReadFile(filepath.Join(tempDir, "fileB.go"))
	if !strings.Contains(string(contentB), "func B() {}") {
		t.Errorf("fileB should remain unchanged. Content: %s", string(contentB))
	}
}

func TestAPI_Apply_CompilerPreFlightFailure(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

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

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusMultiStatus {
		t.Fatalf("Expected status 207 for compiler failure, got %d", res.StatusCode)
	}

	var response map[string]any
	json.NewDecoder(res.Body).Decode(&response)

	fileErrors, ok := response["file_errors"].(map[string]any)
	if !ok || fileErrors["badcode.go"] == nil {
		t.Fatalf("Expected a 'badcode.go' error key in file_errors, got: %v", response["file_errors"])
	}

	// Ensure the file on disk was NOT modified (No Limping)

	// Ensure the file on disk was NOT modified (No Limping)
	contentBytes, _ := os.ReadFile(targetFile)
	if strings.Contains(string(contentBytes), "deliberately invalid syntax") {
		t.Errorf("File was modified on disk despite compiler error!")
	}
}

func TestAPI_Apply_CreateNewFile_NativeEngine(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

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
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d. Body: %s", res.StatusCode, w.Body.String())
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

func TestAPI_Retest(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

	payload := RetestPayload{Packages: []string{"path"}} // stdlib package so it passes quickly
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/retest", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 for retest, got %d", res.StatusCode)
	}

	var response map[string]any
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if success, ok := response["success"].(bool); !ok || !success {
		t.Errorf("Expected retest to report success for 'path' package")
	}
}

func TestAPI_Apply_EmptySearchOverwritesIgnored(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

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
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK for gracefully ignoring existing file overwrite, got %d", w.Result().StatusCode)
	}

	var response map[string]any
	json.NewDecoder(w.Result().Body).Decode(&response)

	if modified, ok := response["files_modified"].(float64); !ok || modified != 0 {
		t.Errorf("Expected 0 files_modified, got: %v", response["files_modified"])
	}
}

func TestAPI_Preview_WarningsAndHints(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

	os.WriteFile(filepath.Join(tempDir, "exists.go"), []byte("package mypkg\n\nfunc Old() {}\n"), 0644)

	// Setup for path fixer test
	os.MkdirAll(filepath.Join(tempDir, "nested", "deep"), 0755)
	os.WriteFile(filepath.Join(tempDir, "nested", "deep", "missing.go"), []byte("package mypkg\nfunc Old() {}"), 0644)

	bundle := strings.ReplaceAll(`
### filename: exists.go
### replace
### with
func AccidentalOverwrite() {}
### end

### filename: missing.go
### replace
func Old() {}
### with
func New() {}
### end

### filename: exists.go
### replace
package mypkg
func Older() {}
### with
func New() {}
### end
`, "###", patcheng.BundleDelim)

	payload := Payload{Bundle: bundle}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for preview, got %d", w.Result().StatusCode)
	}

	var response map[string]any
	json.NewDecoder(w.Result().Body).Decode(&response)

	patches, ok := response["patches"].(map[string]any)
	if !ok {
		t.Fatalf("Expected patches map, got: %v", response)
	}

	// Check exists.go (creation -> ignored, modify with typo -> error + hint)
	existsPatches := patches["exists.go"].([]any)
	if len(existsPatches) != 2 {
		t.Fatalf("Expected 2 patches for exists.go, got %d", len(existsPatches))
	}

	p0 := existsPatches[0].(map[string]any)
	if p0["status"] != "ignored" || !strings.Contains(p0["message"].(string), "File already exists") {
		t.Errorf("Expected ignored warning for existing file creation, got: %v", p0)
	}

	p1 := existsPatches[1].(map[string]any)
	if p1["status"] != "error" {
		t.Errorf("Expected error for fuzzy patch mismatch, got: %v", p1)
	}
	if p1["hint"] == nil || !strings.Contains(p1["hint"].(string), "func Old() {}") {
		t.Errorf("Expected hint to contain the closest match, got hint: %v", p1["hint"])
	}

	// Check missing.go
	missingPatches := patches["missing.go"].([]any)
	pM := missingPatches[0].(map[string]any)
	if pM["status"] != "ignored" || !strings.Contains(pM["message"].(string), "Target file missing") {
		t.Errorf("Expected ignored warning for missing target file, got: %v", pM)
	}

	// Verify Path Fixer payload
	pathFixes, ok := response["path_fixes"].(map[string]any)
	if !ok || pathFixes["missing.go"] != "nested/deep/missing.go" {
		t.Errorf("Expected path_fixes map to contain nested resolution, got: %v", pathFixes)
	}
	if !strings.Contains(pM["message"].(string), "Click 'Fix File Paths'") {
		t.Errorf("Expected UI hint for missing file resolution, got: %s", pM["message"])
	}
}

func TestAPI_Apply_TracksHistory(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

	targetFile := filepath.Join(tempDir, "history.go")
	os.WriteFile(targetFile, []byte("package mypkg\nfunc Old() {}"), 0644)

	bundle := strings.ReplaceAll(`
### filename: history.go
### replace
func Old() {}
### with
func New() {}
### end
`, "###", patcheng.BundleDelim)

	payload := Payload{Bundle: bundle}
	body, _ := json.Marshal(payload)

	// 1. Apply it
	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK on apply")
	}

	// 2. Preview it again to verify hash memory
	reqPreview := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewReader(body))
	wPreview := httptest.NewRecorder()
	mux.ServeHTTP(wPreview, reqPreview)

	var response map[string]any
	json.NewDecoder(wPreview.Result().Body).Decode(&response)
	patches := response["patches"].(map[string]any)
	hist := patches["history.go"].([]any)[0].(map[string]any)

	if hist["status"] != "applied" {
		t.Errorf("Expected status 'applied', got %v", hist["status"])
	}
}

func TestAPI_Apply_DeleteBlock(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

	targetFile := filepath.Join(tempDir, "delete_me.go")
	os.WriteFile(targetFile, []byte("package mypkg\nfunc Old() {}\nfunc Keep() {}"), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: delete_me.go
### delete
func Old() {}
### end
`, "###", patcheng.BundleDelim),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	contentBytes, _ := os.ReadFile(targetFile)
	if !strings.Contains(string(contentBytes), "func Keep() {}") || strings.Contains(string(contentBytes), "func Old() {}") {
		t.Errorf("Expected func Old() {} to be deleted, got: %q", string(contentBytes))
	}
}

func TestAPI_Apply_EmptyFileDeletion(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

	targetFile := filepath.Join(tempDir, "delete_me.go")
	os.WriteFile(targetFile, []byte("package mypkg\nfunc Old() {}"), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: delete_me.go
### replace
package mypkg
func Old() {}
### with
### end
`, "###", patcheng.BundleDelim),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	contentBytes, _ := os.ReadFile(targetFile)
	if len(contentBytes) != 0 {
		t.Errorf("Expected file to be empty, got: %q", string(contentBytes))
	}
}

func TestAPI_Apply_ReplaceSymbol(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newServer(tempDir)

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
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Result().StatusCode)
	}

	content, _ := os.ReadFile(targetFile)
	if !strings.Contains(string(content), "func Old(ctx context.Context)") {
		t.Errorf("Failed to replace symbol via API. Content:\n%s", string(content))
	}
}

func TestWithRecoveryAndCORS(t *testing.T) {
	handler := withRecoveryAndCORS(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	t.Run("OPTIONS Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("Expected 200 OK for OPTIONS, got %d", w.Result().StatusCode)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Errorf("Missing CORS headers")
		}
	})

	t.Run("Panic Recovery", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Result().StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected 500 Internal Server Error, got %d", w.Result().StatusCode)
		}
		if !strings.Contains(w.Body.String(), "Server Panic: test panic") {
			t.Errorf("Expected panic message in JSON, got: %s", w.Body.String())
		}
	})
}
