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

func TestAPI_Apply_RejectsWeakFuzzyPatches(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	targetFile := filepath.Join(tempDir, "short.go")
	os.WriteFile(targetFile, []byte("package mypkg\n\nfunc Weak() {}\n"), 0644)

	payload := Payload{
		Bundle: strings.ReplaceAll(`
### filename: short.go
### replace
func Weak() {}
### with
func Strong() {}
### end
`, "###", patcheng.BundleDelim),
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusMultiStatus && res.StatusCode != http.StatusOK {
		t.Fatalf("Expected 207 or 200, got %d", res.StatusCode)
	}

	var response map[string]any
	json.NewDecoder(res.Body).Decode(&response)

	files, ok := response["files"].([]any)
	if !ok || len(files) == 0 {
		t.Fatalf("Expected files array, got %v", response)
	}

	fileObj := files[0].(map[string]any)
	if fileObj["applied"] == true {
		t.Errorf("Expected file to be rejected, but it was applied")
	}

	errMsg, _ := fileObj["error"].(string)
	if !strings.Contains(errMsg, "too small") && !strings.Contains(errMsg, "too weak") {
		t.Errorf("Expected weak/small patch rejection error, got: %v", errMsg)
	}
}
