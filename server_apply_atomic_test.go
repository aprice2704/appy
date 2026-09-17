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

func TestAPI_AtomicApply_T_ATM_01_03(t *testing.T) {
	tempDir := setupTestWorkspace(t)
	mux := newTestServer(tempDir)

	os.WriteFile(filepath.Join(tempDir, "fileA.go"), []byte("package main\n"), 0644)
	os.WriteFile(filepath.Join(tempDir, "fileB.go"), []byte("package main\n"), 0644)

	bundle := strings.ReplaceAll(`
### filename: fileA.go
### overwrite
package main
func bad() { syntax error
### end

### filename: fileB.go
### overwrite
package main
func good() {}
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

	var response map[string]any
	if err := json.NewDecoder(w.Result().Body).Decode(&response); err != nil {
		t.Fatalf("failed decoding apply response: %v", err)
	}

	files, ok := response["files"].([]any)
	if !ok || len(files) != 2 {
		t.Fatalf("t-atm-01 failed: expected 2 files in response")
	}

	for _, f := range files {
		fMap := f.(map[string]any)
		if fMap["path"] == "fileA.go" {
			if fMap["applied"] == true {
				t.Errorf("t-atm-01 failed: fileA.go should not be applied due to syntax error")
			}
		}
		if fMap["path"] == "fileB.go" {
			if fMap["applied"] == false {
				t.Errorf("t-atm-01 failed: fileB.go should be applied despite fileA.go error")
			}
		}
	}

	contentB, err := os.ReadFile(filepath.Join(tempDir, "fileB.go"))
	if err != nil {
		t.Fatalf("failed reading fileB.go: %v", err)
	}
	if !strings.Contains(string(contentB), "func good()") {
		t.Errorf("t-atm-01 failed: fileB.go was not written to disk")
	}

	contentA, err := os.ReadFile(filepath.Join(tempDir, "fileA.go"))
	if err != nil {
		t.Fatalf("failed reading fileA.go: %v", err)
	}
	if strings.Contains(string(contentA), "syntax error") {
		t.Errorf("t-atm-01 failed: fileA.go was written to disk despite error")
	}
}
