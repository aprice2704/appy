package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func cleanExcludesList(excludes []string) []string {
	var clean []string
	for _, ex := range excludes {
		ex = strings.TrimSpace(ex)
		if ex == "" {
			continue
		}
		clean = append(clean, ex)
	}
	return clean
}

func isExcludedPath(baseName, rel string, hasValidRel bool, cleanExcludes []string) bool {
	for _, ex := range cleanExcludes {
		matchedBase, errBase := filepath.Match(ex, baseName)
		if errBase == nil && matchedBase {
			return true
		}
		if !hasValidRel {
			continue
		}
		matchedSlash, errSlash := filepath.Match(ex, filepath.ToSlash(rel))
		if errSlash == nil && matchedSlash {
			return true
		}
		matchedRel, errRelMatch := filepath.Match(ex, rel)
		if errRelMatch == nil && matchedRel {
			return true
		}
	}
	return false
}

func resolveWalkBaseDir(absRootDir, p string) (string, string) {
	var baseDir, pattern string
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
	if filepath.IsAbs(baseDir) {
		return baseDir, pattern
	}

	targetCandidate := filepath.Join(absRootDir, baseDir)
	if _, err := os.Stat(targetCandidate); err == nil {
		return targetCandidate, pattern
	}
	if _, err := os.Stat(baseDir); err == nil {
		abs, errAbs := filepath.Abs(baseDir)
		if errAbs == nil {
			return abs, pattern
		}
	}
	return targetCandidate, pattern
}

func walkDirectoryRecursive(baseDir, pattern string, add func(string)) {
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

func walkPaths(absRootDir string, paths []string, excludes []string, cb func(absPath string, relName string)) {
	added := make(map[string]bool)
	cleanExcludes := cleanExcludesList(excludes)

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

		if isExcludedPath(baseName, rel, hasValidRel, cleanExcludes) {
			return
		}
		added[path] = true

		name := path
		if hasValidRel {
			name = rel
		}
		cb(path, name)
	}

	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" || p == "." || p == "./" {
			continue
		}
		if (strings.HasSuffix(p, "/") || strings.HasSuffix(p, "\\")) && !strings.Contains(p, "*") {
			continue
		}

		if strings.ContainsAny(p, "{}:") {
			sgFiles, _, _ := EvaluateSuperGlob(absRootDir, p, cleanExcludes)
			for _, f := range sgFiles {
				add(filepath.Join(absRootDir, filepath.FromSlash(f.Path)))
			}
			continue
		}

		baseDir, pattern := resolveWalkBaseDir(absRootDir, p)
		stat, err := os.Stat(baseDir)
		if err != nil {
			if strings.TrimSpace(baseDir) != "" && strings.TrimSpace(baseDir) != "." {
				matches, errGlob := filepath.Glob(baseDir)
				if errGlob != nil {
					log.Printf("[DEBUG] walkPaths: glob failed for %s: %v", baseDir, errGlob)
					continue
				}
				for _, m := range matches {
					s, errStat := os.Stat(m)
					if errStat != nil || s.IsDir() {
						continue
					}
					add(m)
				}
			}
			continue
		}

		if !stat.IsDir() {
			add(baseDir)
			continue
		}

		walkDirectoryRecursive(baseDir, pattern, add)
	}
}
