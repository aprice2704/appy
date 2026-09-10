package main

import (
	"os"
	"path/filepath"
	"strings"
)

var superGlobTypeAliases = map[string]string{
	":go":   "*.go",
	":test": "*_test.go",
	":md":   "*.md",
	":ndcl": "*.ndcl.md",
	":json": "*.json",
	":js":   "*.js",
	":ts":   "*.ts",
	":html": "*.html",
	":css":  "*.css",
	":sh":   "*.sh",
}

type SuperGlobResult struct {
	Paths    []string `json:"paths"`
	Excludes []string `json:"excludes"`
}

func expandTypeAliases(s string) string {
	for alias, expansion := range superGlobTypeAliases {
		s = strings.ReplaceAll(s, alias, expansion)
	}
	return s
}

func expandBraces(pattern string) []string {
	openIdx := strings.Index(pattern, "{")
	if openIdx == -1 {
		return []string{pattern}
	}

	closeIdx := -1
	depth := 0
	for i := openIdx; i < len(pattern); i++ {
		if pattern[i] == '{' {
			depth++
		} else if pattern[i] == '}' {
			depth--
			if depth == 0 {
				closeIdx = i
				break
			}
		}
	}
	if closeIdx == -1 {
		return []string{pattern}
	}

	prefix := pattern[:openIdx]
	suffix := pattern[closeIdx+1:]
	inside := pattern[openIdx+1 : closeIdx]

	var options []string
	current := &strings.Builder{}
	innerDepth := 0
	for i := 0; i < len(inside); i++ {
		ch := inside[i]
		if ch == '{' {
			innerDepth++
			current.WriteByte(ch)
		} else if ch == '}' {
			innerDepth--
			current.WriteByte(ch)
		} else if (ch == ',' || ch == ' ') && innerDepth == 0 {
			s := strings.TrimSpace(current.String())
			if s != "" {
				options = append(options, s)
			}
			current.Reset()
		} else {
			current.WriteByte(ch)
		}
	}
	if rem := strings.TrimSpace(current.String()); rem != "" {
		options = append(options, rem)
	}

	var results []string
	for _, opt := range options {
		expandedCombo := prefix + opt + suffix
		results = append(results, expandBraces(expandedCombo)...)
	}
	return results
}

func ParseSuperGlob(raw string) SuperGlobResult {
	var res SuperGlobResult
	lines := strings.Split(raw, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		line = expandTypeAliases(line)
		expanded := expandBraces(line)

		for _, item := range expanded {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if strings.HasPrefix(item, "!") || strings.HasPrefix(item, "-") {
				rule := strings.TrimSpace(item[1:])
				if rule != "" {
					res.Excludes = append(res.Excludes, rule)
				}
			} else {
				res.Paths = append(res.Paths, item)
			}
		}
	}
	return res
}

type EvaluatedFile struct {
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Tokens   int64  `json:"tokens"`
	FileType string `json:"file_type"`
}

func EvaluateSuperGlob(absRootDir string, raw string, extraExcludes []string) ([]EvaluatedFile, SuperGlobResult, bool) {
	parsed := ParseSuperGlob(raw)
	allExcludes := append([]string{}, parsed.Excludes...)
	allExcludes = append(allExcludes, extraExcludes...)

	const maxWalkLimit = 10000
	var files []EvaluatedFile
	seen := make(map[string]bool)
	limitExceeded := false

	walkPaths(absRootDir, parsed.Paths, allExcludes, func(absPath, relName string) {
		if len(files) >= maxWalkLimit {
			limitExceeded = true
			return
		}
		relSlash := filepath.ToSlash(relName)
		if seen[relSlash] {
			return
		}
		seen[relSlash] = true

		info, err := os.Stat(absPath)
		if err != nil || info.IsDir() {
			return
		}
		files = append(files, EvaluatedFile{
			Path:     relSlash,
			Size:     info.Size(),
			Tokens:   info.Size() / 4,
			FileType: filepath.Ext(absPath),
		})
	})
	return files, parsed, limitExceeded
}
