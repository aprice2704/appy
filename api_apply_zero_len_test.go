package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestApply_ZeroLengthProtectionOnLedgerSkip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "appy_zero_len_test_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	targetRelPath := filepath.Join("pkg", "sample.go")
	targetAbsPath := filepath.Join(tempDir, targetRelPath)
	if err := os.MkdirAll(filepath.Dir(targetAbsPath), 0755); err != nil {
		t.Fatalf("failed creating subdirs: %v", err)
	}

	initialContent := []byte("package sample\n\nfunc Hello() string {\n\treturn \"world\"\n}\n")
	if err := os.WriteFile(targetAbsPath, initialContent, 0644); err != nil {
		t.Fatalf("failed seeding sample file: %v", err)
	}

	server := &AppyServer{rootDir: tempDir}

	patchBundle := "%%% filename: " + targetRelPath + "\n" +
		"%%% replace near 1\n" +
		"func Hello() string {\n" +
		"\treturn \"world\"\n" +
		"}\n" +
		"%%% with\n" +
		"func Hello() string {\n" +
		"\treturn \"universe\"\n" +
		"}\n" +
		"%%% end\n"

	// Step 1: First Apply - should succeed and update file
	// Step 1: First Apply - should succeed and update file
	reqBody1, err := json.Marshal(Payload{Bundle: patchBundle, SkipCompiler: true})
	if err != nil {
		t.Fatalf("failed marshaling reqBody1: %v", err)
	}
	req1 := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(reqBody1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()

	server.handleApply(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first apply failed: code=%d body=%s", rec1.Code, rec1.Body.String())
	}

	afterFirstApply, err := os.ReadFile(targetAbsPath)
	if err != nil {
		t.Fatalf("failed reading file after first apply: %v", err)
	}
	if len(afterFirstApply) == 0 {
		t.Fatalf("file was unexpectedly truncated to 0 bytes on first apply")
	}

	// Step 2: Second Apply - patch is now in the ledger.
	// This must skip gracefully and MUST NOT truncate the file on disk to 0 bytes.
	reqBody2, err := json.Marshal(Payload{Bundle: patchBundle, SkipCompiler: true})
	if err != nil {
		t.Fatalf("failed marshaling reqBody2: %v", err)
	}
	req2 := httptest.NewRequest(http.MethodPost, "/api/apply", bytes.NewReader(reqBody2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()

	server.handleApply(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second apply failed: code=%d body=%s", rec2.Code, rec2.Body.String())
	}

	afterSecondApply, err := os.ReadFile(targetAbsPath)
	if err != nil {
		t.Fatalf("failed reading file after second apply: %v", err)
	}

	if len(afterSecondApply) == 0 {
		t.Fatalf("CRITICAL BUG: File was wiped to 0 bytes on ledger-skipped apply! File size is 0 bytes")
	}

	if string(afterSecondApply) != string(afterFirstApply) {
		t.Fatalf("file content changed on second apply: got %q, expected %q", string(afterSecondApply), string(afterFirstApply))
	}
}

func TestCommitChangesToDisk_RefusesZeroByteOverwrites(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "appy_commit_zero_test_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	targetAbsPath := filepath.Join(tempDir, "existing.go")
	originalContent := []byte("package existing\n\nvar Existing = 123\n")
	if err := os.WriteFile(targetAbsPath, originalContent, 0644); err != nil {
		t.Fatalf("failed writing original file: %v", err)
	}

	server := &AppyServer{rootDir: tempDir}

	// Simulate a case where an empty string was stored in memoryResults
	memoryResults := map[string]string{
		targetAbsPath: "",
	}
	filesToDelete := make(map[string]bool)
	fileHashes := make(map[string][]string)
	originalFiles := make(map[string]*[]byte)

	var applyFiles []ApplyFile
	applyFiles = append(applyFiles, ApplyFile{
		Path:    "existing.go",
		Applied: true,
	})
	hasErrors := false

	server.commitChangesToDisk(originalFiles, memoryResults, filesToDelete, fileHashes, &applyFiles, &hasErrors)

	if !hasErrors {
		t.Fatalf("expected hasErrors=true on 0-byte refusal, got false")
	}
	if applyFiles[0].Applied {
		t.Fatalf("expected applyFiles[0].Applied=false after rollback/refusal")
	}

	contentAfter, err := os.ReadFile(targetAbsPath)
	if err != nil {
		t.Fatalf("failed reading file: %v", err)
	}

	if len(contentAfter) == 0 {
		t.Fatalf("commitChangesToDisk permitted 0-byte overwrite of an existing file!")
	}
	if string(contentAfter) != string(originalContent) {
		t.Fatalf("commitChangesToDisk mutated file unexpectedly: got %q", string(contentAfter))
	}
}

func TestCommitChangesToDisk_AllowsNonEmptyWrites(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "appy_commit_normal_test_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	targetAbsPath := filepath.Join(tempDir, "file.go")
	server := &AppyServer{rootDir: tempDir}

	validContent := "package file\n\nconst Version = \"2.2\"\n"
	memoryResults := map[string]string{
		targetAbsPath: validContent,
	}

	var applyFiles []ApplyFile
	applyFiles = append(applyFiles, ApplyFile{
		Path:    "file.go",
		Applied: true,
	})
	hasErrors := false

	server.commitChangesToDisk(make(map[string]*[]byte), memoryResults, make(map[string]bool), make(map[string][]string), &applyFiles, &hasErrors)

	written, err := os.ReadFile(targetAbsPath)
	if err != nil {
		t.Fatalf("failed reading file: %v", err)
	}
	if string(written) != validContent {
		t.Fatalf("commitChangesToDisk failed to write valid non-empty content: got %q", string(written))
	}
}
