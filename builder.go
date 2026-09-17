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
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[DEBUG] getSets: failed to read .appy_sets.json: %v", err)
		}
		return make(map[string]TxtarPayload)
	}
	var sets map[string]TxtarPayload
	if err := json.Unmarshal(b, &sets); err != nil {
		log.Printf("[WARN] getSets: corrupt .appy_sets.json: %v", err)
		return make(map[string]TxtarPayload)
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

	var cleanExcludes []string
	for _, ex := range excludes {
		ex = strings.TrimSpace(ex)
		if ex != "" {
			cleanExcludes = append(cleanExcludes, ex)
		}
	}

	add := func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		path = filepath.Clean(path)
		if added[path] {
			return
		}

		baseName := filepath.Base(path)
		rel, err := filepath.Rel(absRootDir, path)
		hasValidRel := err == nil && !strings.HasPrefix(rel, "..") && rel != ".."

		for _, ex := range cleanExcludes {
			matchedBase, errBase := filepath.Match(ex, baseName)
			if errBase == nil && matchedBase {
				return
			}
			if hasValidRel {
				matchedSlash, errSlash := filepath.Match(ex, filepath.ToSlash(rel))
				if errSlash == nil && matchedSlash {
					return
				}
				matchedRel, errRelMatch := filepath.Match(ex, rel)
				if errRelMatch == nil && matchedRel {
					return
				}
			}
		}
		added[path] = true

		var name string
		if hasValidRel {
			name = rel
		} else {
			name = path
		}
		cb(path, name)
	}

	var validPaths []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			validPaths = append(validPaths, p)
		}
	}
	if len(validPaths) == 0 {
		return
	}

	for _, p := range validPaths {
		// Degenerate path guard: bare "." or paths ending in "/" match no files unless followed by * or **
		if p == "." || p == "./" {
			continue
		}
		if (strings.HasSuffix(p, "/") || strings.HasSuffix(p, "\\")) && !strings.Contains(p, "*") {
			continue
		}

		// Superglob support: if the pattern contains brace expansions, language tokens, or wildcards
		if strings.Contains(p, "{") || strings.Contains(p, "}") || strings.Contains(p, ":") {
			sgFiles, _, _ := EvaluateSuperGlob(absRootDir, p, cleanExcludes)
			for _, f := range sgFiles {
				abs := filepath.Join(absRootDir, filepath.FromSlash(f.Path))
				add(abs)
			}
			continue
		}

		var baseDir string
		var pattern string
		if strings.Contains(p, "**") {
			parts := strings.SplitN(p, "**", 2)
			baseDir = strings.TrimSpace(parts[0])
			pattern = "**" + parts[1]
		} else {
			baseDir = p
		}
		if baseDir == "" {
			baseDir = "."
		}
		if !filepath.IsAbs(baseDir) {
			targetCandidate := filepath.Join(absRootDir, baseDir)
			if _, err := os.Stat(targetCandidate); err != nil {
				if _, errAbsStat := os.Stat(baseDir); errAbsStat != nil {
					baseDir = targetCandidate
				} else {
					abs, errAbs := filepath.Abs(baseDir)
					if errAbs != nil {
						baseDir = targetCandidate
					} else {
						baseDir = abs
					}
				}
			} else {
				baseDir = targetCandidate
			}
		}

		stat, err := os.Stat(baseDir)
		if err != nil {
			if strings.TrimSpace(baseDir) != "" && strings.TrimSpace(baseDir) != "." {
				matches, errGlob := filepath.Glob(baseDir)
				if errGlob != nil {
					log.Printf("[DEBUG] walkPaths: glob failed for %s: %v", baseDir, errGlob)
				} else {
					for _, m := range matches {
						s, errStat := os.Stat(m)
						if errStat != nil || s.IsDir() {
							continue
						}
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
				name := d.Name()
				if name == ".git" || name == "vendor" || name == "node_modules" || name == ".appy_history" {
					return filepath.SkipDir
				}
				return nil
			}
			if pattern != "" && pattern != "**" {
				suffix := strings.TrimPrefix(pattern, "**")
				if suffix != "" && !strings.HasSuffix(filepath.ToSlash(path), suffix) && !strings.HasSuffix(path, suffix) {
					return nil
				}
			}
			add(path)
			return nil
		})
	}
}

const MaxTxtarFileCount = 500

func generateTxtar(absRootDir string, req TxtarPayload, largeFileLines int) ([]byte, int, error) {
	var buf bytes.Buffer
	buf.WriteString(req.Preface)
	if !strings.HasSuffix(req.Preface, "\n") {
		buf.WriteString("\n")
	}
	fileCount := 0
	seenPaths := make(map[string]bool)
	var limitErr error

	walkPaths(absRootDir, req.Paths, req.Excludes, func(absPath, relName string) {
		if limitErr != nil {
			return
		}
		if fileCount >= MaxTxtarFileCount {
			limitErr = fmt.Errorf("file limit exceeded: txtar bundle exceeded safety limit of %d files; refine include patterns or add excludes", MaxTxtarFileCount)
			return
		}
		seenPaths[relName] = true
		seenPaths[absPath] = true
		content, err := os.ReadFile(absPath)
		if err != nil {
			log.Printf("[DEBUG] generateTxtar: Read error %s: %v", absPath, err)
			buf.WriteString(fmt.Sprintf("-- %s --\n", filepath.ToSlash(relName)))
			buf.WriteString(fmt.Sprintf("[ERROR: File could not be read: %v]\n", err))
			fileCount++
			return
		}

		needsAnchor := true
		for _, ag := range req.Anchors {
			if ag == "" {
				continue
			}
			matchedBase, errBase := filepath.Match(ag, filepath.Base(absPath))
			if errBase == nil && matchedBase {
				needsAnchor = false
				break
			}
			matchedRel, errRel := filepath.Match(ag, relName)
			if errRel == nil && matchedRel {
				needsAnchor = false
				break
			}
		}

		if needsAnchor {
			anchoredContent, errInject := patcheng.InjectAnchors(relName, content, 5)
			if errInject != nil {
				log.Printf("[DEBUG] generateTxtar: Anchor injection failed for %s: %v", relName, errInject)
			} else {
				content = anchoredContent
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

	if limitErr != nil {
		return nil, 0, limitErr
	}

	for _, p := range req.Paths {
		pTrim := strings.TrimSpace(p)
		if pTrim == "" || strings.Contains(pTrim, "*") || strings.Contains(pTrim, "?") ||
			strings.Contains(pTrim, "{") || strings.Contains(pTrim, "}") || strings.Contains(pTrim, ":") {
			continue
		}
		cleanP := filepath.Clean(pTrim)
		if !seenPaths[cleanP] && !seenPaths[filepath.ToSlash(cleanP)] {
			targetCandidate := pTrim
			if !filepath.IsAbs(targetCandidate) {
				targetCandidate = filepath.Join(absRootDir, targetCandidate)
			}
			if _, err := os.Stat(targetCandidate); err != nil {
				buf.WriteString(fmt.Sprintf("-- %s --\n", filepath.ToSlash(pTrim)))
				buf.WriteString(fmt.Sprintf("[ERROR: File %q does not exist or could not be found under %s]\n", pTrim, absRootDir))
				fileCount++
			} else {
				buf.WriteString(fmt.Sprintf("-- %s --\n", filepath.ToSlash(pTrim)))
				buf.WriteString(fmt.Sprintf("[NOTE: File %q exists on disk but was excluded by bundle excludes]\n", pTrim))
				fileCount++
				continue
			}
		}
	}

	return buf.Bytes(), fileCount, nil
}
