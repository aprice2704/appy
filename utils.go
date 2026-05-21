// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 7
// :: description: Core utilities, path resolution, and LLM hint generation.
// :: filename: utils.go
// :: serialization: go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aprice2704/fdm/code/patcheng"
)

func isPathSafe(root, target string) bool {
	target = filepath.Clean(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

func findUniquePathSuffix(rootDir, targetSuffix string) string {
	targetSuffix = filepath.Clean(targetSuffix)
	targetSuffixWithSep := string(filepath.Separator) + targetSuffix
	var match string
	var count int

	filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" || name == ".appy_history" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}
		if rel == targetSuffix || strings.HasSuffix(rel, targetSuffixWithSep) {
			match = rel
			count++
		}
		return nil
	})

	if count == 1 {
		return filepath.ToSlash(match)
	}
	return ""
}

func hashPatch(file, search, replace string) string {
	h := sha256.Sum256([]byte(file + "\x00" + search + "\x00" + replace))
	return hex.EncodeToString(h[:])
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), "")
}

func getNonEmptyLines(s string) []string {
	var res []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			res = append(res, l)
		}
	}
	return res
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func formatHintLine(contentIdx, bestStart int, searchLines, contentLines []string) string {
	searchIdx := contentIdx - bestStart
	if searchIdx >= 0 && searchIdx < len(searchLines) {
		if strings.TrimSpace(searchLines[searchIdx]) == "..." {
			return fmt.Sprintf("  %d: ...\n", contentIdx+1)
		}
	}
	return fmt.Sprintf("  %d: %s\n", contentIdx+1, contentLines[contentIdx])
}

func findTextAnchor(content string, search string) string {
	searchLines := getNonEmptyLines(search)
	if len(searchLines) == 0 {
		return ""
	}
	contentLines := strings.Split(content, "\n")
	bestScore, bestStart := -1, -1

	for i := 0; i <= len(contentLines)-len(searchLines); i++ {
		score := 0
		for j := 0; j < len(searchLines); j++ {
			if strings.TrimSpace(searchLines[j]) == "..." {
				score++
				continue
			}
			if normalizeSpace(contentLines[i+j]) == normalizeSpace(searchLines[j]) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestStart = i
		}
	}

	if bestScore > 0 && bestStart >= 0 {
		start := bestStart - 2
		if start < 0 {
			start = 0
		}
		end := bestStart + len(searchLines) + 2
		if end > len(contentLines) {
			end = len(contentLines)
		}
		var sb strings.Builder
		if end-start <= 8 {
			for i := start; i < end; i++ {
				sb.WriteString(formatHintLine(i, bestStart, searchLines, contentLines))
			}
		} else {
			for i := start; i < start+3; i++ {
				sb.WriteString(formatHintLine(i, bestStart, searchLines, contentLines))
			}
			sb.WriteString(fmt.Sprintf("... [%d lines elided] ...\n", (end-3)-(start+3)))
			for i := end - 3; i < end; i++ {
				sb.WriteString(formatHintLine(i, bestStart, searchLines, contentLines))
			}
		}
		return sb.String()
	}
	return ""
}

func generateDiagnosticHint(profile *patcheng.LanguageProfile, filename, content, search string, nearLine int) string {
	if profile != nil && profile.HintGenerator != nil {
		return profile.HintGenerator([]byte(content), search, nearLine)
	}
	return findTextAnchor(content, search)
}

func generateLLMFallbackHint(prof *patcheng.LanguageProfile) string {
	if prof != nil && len(prof.PreferredStrategies) > 0 {
		return fmt.Sprintf("LLM Nudge: Preferred patching strategies for %s are [%s]. If your current strategy failed, escalate to the next one in this sequence.", prof.ID, strings.Join(prof.PreferredStrategies, ", "))
	}
	return ""
}

func withND(schema, desc string, payload map[string]any) map[string]any {
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["__nd"] = map[string]any{
		"appy_version": AppVersion,
		"description":  desc,
		"identity":     map[string]string{"schema": schema, "serialization": "json"},
	}
	return payload
}

func sendError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(withND("appy/error", "HTTP error response from appy server", map[string]any{"error": msg}))
}
