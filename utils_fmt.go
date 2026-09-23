package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/aprice2704/fdm/code/patcheng"
)

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
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		res = append(res, l)
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
		if score <= bestScore {
			continue
		}
		bestScore = score
		bestStart = i
	}

	if bestScore <= 0 || bestStart < 0 {
		return ""
	}

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
		return sb.String()
	}

	for i := start; i < start+3; i++ {
		sb.WriteString(formatHintLine(i, bestStart, searchLines, contentLines))
	}
	sb.WriteString(fmt.Sprintf("... [%d lines elided] ...\n", (end-3)-(start+3)))
	for i := end - 3; i < end; i++ {
		sb.WriteString(formatHintLine(i, bestStart, searchLines, contentLines))
	}
	return sb.String()
}

func generateDiagnosticHint(profile *patcheng.LanguageProfile, filename, content, search string, nearLine int) string {
	// NearLine windowing is deprecated; search across the whole file to eliminate false rejections
	if profile != nil && profile.HintGenerator != nil {
		return profile.HintGenerator([]byte(content), search, 0)
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

func extractImportDirectivesFromText(text string) []patcheng.ImportDirective {
	var dirs []patcheng.ImportDirective
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || trimmed == "import (" || trimmed == ")" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "import ")
		parts := strings.Fields(trimmed)
		if len(parts) == 1 && strings.HasPrefix(parts[0], "\"") {
			dirs = append(dirs, patcheng.ImportDirective{
				Path: strings.Trim(parts[0], `"`),
			})
		} else if len(parts) >= 2 && strings.HasPrefix(parts[1], "\"") {
			dirs = append(dirs, patcheng.ImportDirective{
				Alias: parts[0],
				Path:  strings.Trim(parts[1], `"`),
			})
		}
	}
	return dirs
}

func sendError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(withND("appy/error", "HTTP error response from appy server", map[string]any{"error": msg}))
}

func ValidateFuzzySearchBlock(p patcheng.FuzzyPatch) error {
	if p.IsAnchored || p.FullOverwrite || p.IsDeleteFile || p.IsReplaceAst || p.IsReplaceElement || p.IsMetaUpdate || p.IsNdclUpdate || p.IsReplaceJson || p.SymbolName != "" || p.LineMatch {
		return nil
	}

	if strings.TrimSpace(p.Search) == "" {
		return nil
	}

	lines := getNonEmptyLines(p.Search)
	for i, l := range lines {
		if strings.TrimSpace(l) != "..." {
			continue
		}
		if i < 1 || len(lines)-i-1 < 1 {
			return fmt.Errorf("REJECTED: Invalid use of elision (...). You must provide at least 1 line of strictly unique context on BOTH sides of the elision.")
		}
	}

	stripped := p.Search
	for _, char := range []string{" ", "\t", "\n", "\r", "{", "}", "(", ")", "[", "]"} {
		stripped = strings.ReplaceAll(stripped, char, "")
	}

	log.Printf("[DEBUG] ValidateFuzzySearchBlock: stripped length %d for search block", len(stripped))

	if len(stripped) < 10 {
		return fmt.Errorf("REJECTED: Fuzzy search block is too weak. It contains fewer than 10 substantive characters. Stop trying to match single braces. Use 'replace_ast' or 'replace_symbol'.")
	}

	return nil
}
