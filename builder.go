package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aprice2704/fdm/code/patcheng"
)

func getSets(rootDir string) map[string]TxtarPayload {
	b, err := os.ReadFile(filepath.Join(rootDir, ".appy_sets.json"))
	var sets map[string]TxtarPayload
	if err == nil {
		json.Unmarshal(b, &sets)
	}
	if sets == nil {
		sets = make(map[string]TxtarPayload)
	}
	return sets
}

func saveSets(rootDir string, sets map[string]TxtarPayload) error {
	b, err := json.MarshalIndent(sets, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(rootDir, ".appy_sets.json"), b, 0644)
}

func walkPaths(absRootDir string, paths []string, excludes []string, cb func(absPath string, relName string)) {
	added := make(map[string]bool)

	add := func(path string) {
		path = filepath.Clean(path)
		if added[path] {
			return
		}

		for _, ex := range excludes {
			if ex == "" {
				continue
			}
			if matched, _ := filepath.Match(ex, filepath.Base(path)); matched {
				return
			}
			rel, err := filepath.Rel(absRootDir, path)
			if err == nil && !strings.HasPrefix(rel, "..") {
				if matched, _ := filepath.Match(ex, rel); matched {
					return
				}
			}
		}
		added[path] = true

		rel, err := filepath.Rel(absRootDir, path)
		var name string
		if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
			name = rel
		} else {
			name = path
		}
		cb(path, name)
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		var baseDir string
		var pattern string
		if strings.Contains(p, "**") {
			parts := strings.SplitN(p, "**", 2)
			baseDir = parts[0]
			pattern = "**" + parts[1]
		} else {
			baseDir = p
		}
		if baseDir == "" {
			baseDir = "."
		}
		if !filepath.IsAbs(baseDir) {
			baseDir = filepath.Join(absRootDir, baseDir)
		}

		stat, err := os.Stat(baseDir)
		if err != nil {
			matches, err := filepath.Glob(baseDir)
			if err == nil {
				for _, m := range matches {
					if s, err := os.Stat(m); err == nil && !s.IsDir() {
						add(m)
					}
				}
			}
			continue
		}
		if !stat.IsDir() {
			add(baseDir)
			continue
		}

		filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules" || d.Name() == ".appy_history" {
					return filepath.SkipDir
				}
				return nil
			}
			if pattern != "" && pattern != "**" {
				suffix := strings.TrimPrefix(pattern, "**")
				if !strings.HasSuffix(filepath.ToSlash(path), suffix) && !strings.HasSuffix(path, suffix) {
					return nil
				}
			}
			add(path)
			return nil
		})
	}
}

func generateTxtar(absRootDir string, req TxtarPayload, largeFileLines int) ([]byte, int, error) {
	var buf bytes.Buffer
	buf.WriteString(req.Preface)
	if !strings.HasSuffix(req.Preface, "\n") {
		buf.WriteString("\n")
	}
	fileCount := 0

	walkPaths(absRootDir, req.Paths, req.Excludes, func(absPath, relName string) {
		content, err := os.ReadFile(absPath)
		if err != nil {
			log.Printf("[DEBUG] generateTxtar: Read error %s: %v", absPath, err)
			return
		}

		needsAnchor := true
		for _, ag := range req.Anchors {
			if ag == "" {
				continue
			}
			if matched, _ := filepath.Match(ag, filepath.Base(absPath)); matched {
				needsAnchor = false
				break
			}
			if matched, _ := filepath.Match(ag, relName); matched {
				needsAnchor = false
				break
			}
		}

		if needsAnchor {
			anchoredContent, err := patcheng.InjectAnchors(relName, content, 10)
			if err == nil {
				content = anchoredContent
			} else {
				log.Printf("[DEBUG] generateTxtar: Anchor injection failed for %s: %v", relName, err)
			}
		}

		if needsAnchor && countLines(string(content)) > largeFileLines {
			warning := []byte("⚠️ APPY NOTE: This file is overly large. If you need to touch it, please split it into sensible pieces if possible.\n\n")
			content = append(warning, content...)
		}

		buf.WriteString(fmt.Sprintf("-- %s --\n", filepath.ToSlash(relName)))
		buf.Write(content)
		if !strings.HasSuffix(string(content), "\n") {
			buf.WriteString("\n")
		}
		fileCount++
	})

	return buf.Bytes(), fileCount, nil
}
