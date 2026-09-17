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

func TestAPI_Preview_Valid(t *testing.T) {
	mux := newTestServer(setupTestWorkspace(t))

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
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

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

	files, ok := response["files"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("Expected 1 file in response array, got %v", response["files"])
	}
}

func TestAPI_Preview_InvalidSyntax(t *testing.T) {
	mux := newTestServer(setupTestWorkspace(t))

	payload := Payload{Bundle: patcheng.BundleDelim + " replace\nbroken"}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request for invalid bundle syntax, got %d", w.Result().StatusCode)
	}
}

func TestAPI_Preview_WarningsAndHints(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	os.WriteFile(filepath.Join(tempDir, "exists.go"), []byte("package mypkg\n\n// line 1\n// line 2\nfunc Old() {}\n"), 0644)
	os.MkdirAll(filepath.Join(tempDir, "nested", "deep"), 0755)
	os.WriteFile(filepath.Join(tempDir, "nested", "deep", "missing.go"), []byte("package mypkg\n// line 1\n// line 2\nfunc Old() {}"), 0644)

	bundle := strings.ReplaceAll(`
### filename: exists.go
### replace
### with
func AccidentalOverwrite() {}
### end

### filename: missing.go
### replace
// line 1
// line 2
func Old() {}
### with
func New() {}
### end

### filename: exists.go
### replace
package mypkg

// line 1
// line 2
func Older() {}
### with
func New() {}
### end
`, "###", patcheng.BundleDelim)

	payload := Payload{Bundle: bundle}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for preview, got %d", w.Result().StatusCode)
	}

	var response map[string]any
	if err := json.NewDecoder(w.Result().Body).Decode(&response); err != nil {
		t.Fatalf("failed decoding preview response: %v", err)
	}

	files, ok := response["files"].([]any)
	if !ok {
		t.Fatalf("Expected files array, got: %v", response)
	}

	for _, f := range files {
		fileObj := f.(map[string]any)
		if fileObj["path"] == "exists.go" {
			if fileObj["status"] != "ERROR" {
				t.Errorf("Expected file status ERROR, got %v", fileObj["status"])
			}
			patches := fileObj["patches"].([]any)
			if len(patches) != 2 {
				t.Fatalf("Expected 2 patches for exists.go, got %d", len(patches))
			}
			p0 := patches[0].(map[string]any)
			if p0["error"] == nil || !strings.Contains(p0["error"].(string), "File already exists") {
				t.Errorf("Expected ignored warning for existing file creation, got: %v", p0["error"])
			}
			p1 := patches[1].(map[string]any)
			if p1["error"] == nil {
				t.Errorf("Expected error for fuzzy patch mismatch")
			}
			if p1["closest_match_hint"] == nil || !strings.Contains(p1["closest_match_hint"].(string), "func Old() {}") {
				t.Errorf("Expected hint to contain the closest match, got hint: %v", p1["closest_match_hint"])
			}
		} else if fileObj["path"] == "missing.go" {
			if fileObj["status"] != "ERROR" {
				t.Errorf("Expected ERROR status for missing target file, got: %v", fileObj["status"])
			}
			patches := fileObj["patches"].([]any)
			pM := patches[0].(map[string]any)
			if pM["error"] == nil || !strings.Contains(pM["error"].(string), "Target file missing") {
				t.Errorf("Expected error for missing target file, got: %v", pM["error"])
			}
		}
	}
}

func TestAPI_Contracts_T_API_01(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	os.WriteFile(filepath.Join(tempDir, "test.go"), []byte("package main\n"), 0644)

	bundle := strings.ReplaceAll(`
### filename: test.go
### overwrite
package main
func main() {}
### end
`, "###", patcheng.BundleDelim)

	payload := Payload{Bundle: bundle}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed marshaling payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var response map[string]any
	if err := json.NewDecoder(w.Result().Body).Decode(&response); err != nil {
		t.Fatalf("failed decoding preview response: %v", err)
	}

	files, ok := response["files"].([]any)
	if !ok || len(files) == 0 {
		t.Fatalf("t-api-01 failed: missing or invalid 'files' array")
	}

	fileObj := files[0].(map[string]any)
	if fileObj["status"] == nil || fileObj["net_lines"] == nil {
		t.Errorf("t-api-01 failed: file object missing required keys")
	}

	patches, ok := fileObj["patches"].([]any)
	if !ok || len(patches) == 0 {
		t.Fatalf("t-api-01 failed: missing or invalid 'patches' array")
	}

	patchObj := patches[0].(map[string]any)
	if patchObj["is_overwrite"] != true {
		t.Errorf("t-api-01 failed: expected is_overwrite to be true")
	}
}
