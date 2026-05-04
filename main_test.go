// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 2
// :: description: Unit tests for main package helpers.
// :: latestChange: Updated to cover findTextAnchor and path resolution logic.
// :: filename: /home/aprice/dev/appy/main_test.go
// :: serialization: go

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aprice2704/fdm/code/patcheng"
)

func TestHashPatch(t *testing.T) {
	h1 := hashPatch("file.go", "search string", "replace string")
	h2 := hashPatch("file.go", "search string", "replace string")
	h3 := hashPatch("file.go", "search string", "different")

	if h1 != h2 {
		t.Errorf("Expected deterministic hash for identical inputs")
	}
	if h1 == h3 {
		t.Errorf("Expected different hashes for different inputs")
	}
}

func TestNormalizeSpace(t *testing.T) {
	input := "  func \t Old() {\n\tprintln(1)  \r\n} "
	expected := "funcOld(){println(1)}"
	if got := normalizeSpace(input); got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestGetNonEmptyLines(t *testing.T) {
	input := "\n  \nfunc Old() {}\n\n\n\t\nfunc New() {}"
	lines := getNonEmptyLines(input)
	if len(lines) != 2 {
		t.Fatalf("Expected 2 non-empty lines, got %d", len(lines))
	}
	if lines[0] != "func Old() {}" || lines[1] != "func New() {}" {
		t.Errorf("Unexpected lines extracted: %v", lines)
	}
}

func TestFindTextAnchor(t *testing.T) {
	content := "package main\n\nfunc A() {}\n\nfunc Old() {\n\tprintln(\"old\")\n}\n\nfunc B() {}"

	t.Run("Finds Typo Match", func(t *testing.T) {
		search := "func old() {\n\tprintln(\"old\")\n}"
		match := findTextAnchor(content, search)
		if !strings.Contains(match, "func Old() {") {
			t.Errorf("Failed to find closest text match. Got:\n%s", match)
		}
		if !strings.Contains(match, "3: func A() {}") {
			t.Errorf("Failed to include preceding context lines. Got:\n%s", match)
		}
	})

	t.Run("Elides Large Blocks", func(t *testing.T) {
		longContent := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n13\n14\n15"
		longSearch := "4\n5\n6\n7\n8\n9\n10\n11\n12\n13" // 10 lines
		match := findTextAnchor(longContent, longSearch)
		if !strings.Contains(match, "elided") {
			t.Errorf("Expected elision for blocks > 8 lines. Got:\n%s", match)
		}
		if !strings.Contains(match, "4: 4") || !strings.Contains(match, "13: 13") {
			t.Errorf("Expected boundaries to be preserved. Got:\n%s", match)
		}
	})

	t.Run("Empty Search", func(t *testing.T) {
		if match := findTextAnchor(content, "   \n\t"); match != "" {
			t.Errorf("Expected empty hint for empty search, got %q", match)
		}
	})
}

func TestPreprocessDeleteBlocks(t *testing.T) {
	input := "### delete\nold code\n### end\n"
	expected := "### replace\nold code\n### with\n### end\n"
	got := preprocessDeleteBlocks(input, "###")
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestFindUniquePathSuffix(t *testing.T) {
	dir := t.TempDir()
	// Setup mock filesystem
	os.MkdirAll(filepath.Join(dir, "sys", "auth"), 0755)
	os.MkdirAll(filepath.Join(dir, "plugins", "auth"), 0755)
	os.MkdirAll(filepath.Join(dir, ".git", "auth"), 0755) // Should be ignored
	os.WriteFile(filepath.Join(dir, "sys", "auth", "login.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "plugins", "auth", "login.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "sys", "auth", "unique.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, ".git", "auth", "login.go"), []byte(""), 0644)

	t.Run("Unique Match", func(t *testing.T) {
		match := findUniquePathSuffix(dir, "auth/unique.go")
		expected := filepath.ToSlash(filepath.Join("sys", "auth", "unique.go"))
		if match != expected {
			t.Errorf("Expected %q, got %q", expected, match)
		}
	})

	t.Run("Ambiguous Match", func(t *testing.T) {
		match := findUniquePathSuffix(dir, "auth/login.go")
		if match != "" {
			t.Errorf("Expected empty string for ambiguous match, got %q", match)
		}
	})

	t.Run("No Match", func(t *testing.T) {
		match := findUniquePathSuffix(dir, "missing.go")
		if match != "" {
			t.Errorf("Expected empty string for missing file, got %q", match)
		}
	})

	t.Run("Exact Root Relative Match", func(t *testing.T) {
		match := findUniquePathSuffix(dir, "sys/auth/login.go")
		expected := filepath.ToSlash(filepath.Join("sys", "auth", "login.go"))
		if match != expected {
			t.Errorf("Expected %q, got %q", expected, match)
		}
	})
}

func TestGenerateDiagnosticHint(t *testing.T) {
	content := "package main\n\ntype MyStruct struct{}\n\nfunc (m *MyStruct) Process() {}\n\nfunc Standalone() {}\n"

	t.Run("Finds AST Context for Method", func(t *testing.T) {
		hint := generateDiagnosticHint(patcheng.GoProfile, "file.go", content, "func (m *MyStruct) Process() {\n\t// broken", 0)
		if !strings.Contains(hint, "Targeting func (*MyStruct) Process") {
			t.Errorf("Failed to find method context. Got:\n%s", hint)
		}
	})

	t.Run("Finds AST Context for Type", func(t *testing.T) {
		hint := generateDiagnosticHint(patcheng.GoProfile, "file.go", content, "type MyStruct struct{", 0)
		if !strings.Contains(hint, "Targeting type MyStruct") {
			t.Errorf("Failed to find type context. Got:\n%s", hint)
		}
	})

	t.Run("Finds AST Context via NearLine", func(t *testing.T) {
		hint := generateDiagnosticHint(patcheng.GoProfile, "file.go", content, "foo", 7) // Line 7 is Standalone()
		if !strings.Contains(hint, "Targeting func Standalone") {
			t.Errorf("Failed to find nearLine context. Got:\n%s", hint)
		}
	})
}

func TestGenerateLLMFallbackHint(t *testing.T) {
	t.Run("Empty Profile", func(t *testing.T) {
		res := generateLLMFallbackHint(nil)
		if res != "" {
			t.Errorf("Expected empty hint for nil profile, got %q", res)
		}
	})

	t.Run("Includes Profile Nudge", func(t *testing.T) {
		prof := &patcheng.LanguageProfile{
			ID:                  "testlang",
			PreferredStrategies: []string{"magic", "fuzzy"},
		}
		res := generateLLMFallbackHint(prof)
		if !strings.Contains(res, "LLM Nudge: Preferred patching strategies for testlang are [magic, fuzzy]") {
			t.Errorf("Failed to inject LLM nudge, got: %q", res)
		}
	})
}
