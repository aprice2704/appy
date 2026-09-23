package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWalkPaths_DegeneratePathsProtection(t *testing.T) {
	dir := t.TempDir()

	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed creating test subdir: %v", err)
	}
	file1 := filepath.Join(dir, "root.go")
	file2 := filepath.Join(subDir, "nested.go")
	os.WriteFile(file1, []byte("package main"), 0644)
	os.WriteFile(file2, []byte("package sub"), 0644)

	t.Run("Bare Dot Matches Zero Files", func(t *testing.T) {
		var matched []string
		walkPaths(dir, []string{"."}, nil, func(absPath, relName string) {
			matched = append(matched, relName)
		})
		if len(matched) != 0 {
			t.Errorf("Expected bare '.' to match 0 files, got %d: %v", len(matched), matched)
		}
	})

	t.Run("Dot Slash Matches Zero Files", func(t *testing.T) {
		var matched []string
		walkPaths(dir, []string{"./"}, nil, func(absPath, relName string) {
			matched = append(matched, relName)
		})
		if len(matched) != 0 {
			t.Errorf("Expected './' to match 0 files, got %d: %v", len(matched), matched)
		}
	})

	t.Run("Trailing Slash Without Wildcard Matches Zero Files", func(t *testing.T) {
		var matched []string
		walkPaths(dir, []string{"sub/"}, nil, func(absPath, relName string) {
			matched = append(matched, relName)
		})
		if len(matched) != 0 {
			t.Errorf("Expected 'sub/' to match 0 files, got %d: %v", len(matched), matched)
		}
	})

	t.Run("Empty Paths Slice Matches Zero Files", func(t *testing.T) {
		var matched []string
		walkPaths(dir, []string{}, nil, func(absPath, relName string) {
			matched = append(matched, relName)
		})
		if len(matched) != 0 {
			t.Errorf("Expected empty paths to match 0 files, got %d: %v", len(matched), matched)
		}
	})

	t.Run("Wildcards Still Match Files", func(t *testing.T) {
		var matched []string
		walkPaths(dir, []string{"sub/**"}, nil, func(absPath, relName string) {
			matched = append(matched, relName)
		})
		if len(matched) != 1 {
			t.Errorf("Expected 'sub/**' to match 1 file, got %d: %v", len(matched), matched)
		}
	})
}

func TestGenerateTxtar_ExceedsMaxFileCountLimit(t *testing.T) {
	dir := t.TempDir()

	subDir := filepath.Join(dir, "many")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed creating test subdir: %v", err)
	}

	// Seed 505 small files
	totalFiles := MaxTxtarFileCount + 5
	for i := 0; i < totalFiles; i++ {
		fn := filepath.Join(subDir, fmt.Sprintf("file_%03d.txt", i))
		if err := os.WriteFile(fn, []byte("test content"), 0644); err != nil {
			t.Fatalf("failed seeding file %d: %v", i, err)
		}
	}

	req := TxtarPayload{
		Paths: []string{"many/**"},
	}

	_, count, err := generateTxtar(dir, req, 350)
	if err == nil {
		t.Fatalf("Expected error when exceeding MaxTxtarFileCount (%d), but generation succeeded with %d files", MaxTxtarFileCount, count)
	}
	if !errors.Is(err, ErrTxtarFileLimitExceeded) {
		t.Errorf("Expected error to wrap ErrTxtarFileLimitExceeded, got: %v", err)
	}
}
