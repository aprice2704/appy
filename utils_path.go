package main

import (
	"os"
	"path/filepath"
	"strings"
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

func findAllPathSuffixMatches(rootDir, targetSuffix string) []string {
	targetSuffix = filepath.Clean(targetSuffix)
	targetSuffixWithSep := string(filepath.Separator) + targetSuffix
	var matches []string

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
			matches = append(matches, filepath.ToSlash(rel))
		}
		return nil
	})
	return matches
}

func findUniquePathSuffix(rootDir, targetSuffix string) string {
	targetSuffix = filepath.Clean(targetSuffix)
	targetSlash := filepath.ToSlash(targetSuffix)
	matches := findAllPathSuffixMatches(rootDir, targetSuffix)

	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) > 1 {
		for _, m := range matches {
			if m == targetSlash {
				return m
			}
		}

		targetParts := strings.Split(targetSlash, "/")
		bestScore := -1
		var bestMatch string
		ambiguous := false

		for _, m := range matches {
			mParts := strings.Split(m, "/")
			score := 0
			tIdx := len(targetParts) - 1
			mIdx := len(mParts) - 1
			for tIdx >= 0 && mIdx >= 0 && targetParts[tIdx] == mParts[mIdx] {
				score++
				tIdx--
				mIdx--
			}
			if score > bestScore {
				bestScore = score
				bestMatch = m
				ambiguous = false
			} else if score == bestScore {
				ambiguous = true
			}
		}

		if !ambiguous && bestMatch != "" {
			return bestMatch
		}
	}
	return ""
}

func resolveDirectoryUnderRoot(rootDir, dirName string, sampleFiles []string) string {
	dirName = filepath.Clean(dirName)
	var candidateDirs []string

	filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" || name == ".appy_history" {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(rootDir, path)
			if relErr == nil && rel != "." && !strings.HasPrefix(rel, "..") {
				if d.Name() == dirName || filepath.Base(rel) == dirName || rel == dirName || strings.HasSuffix(rel, string(filepath.Separator)+dirName) {
					candidateDirs = append(candidateDirs, rel)
				}
			}
		}
		return nil
	})

	if len(candidateDirs) == 1 {
		return candidateDirs[0]
	}

	if len(candidateDirs) > 1 && len(sampleFiles) > 0 {
		bestScore := -1
		var bestDir string
		for _, cDir := range candidateDirs {
			score := 0
			for _, sf := range sampleFiles {
				fullCheck := filepath.Join(rootDir, cDir, sf)
				if _, err := os.Stat(fullCheck); err != nil {
					continue
				}
				score++
			}
			if score > bestScore {
				bestScore = score
				bestDir = cDir
			}
		}
		if bestScore <= 0 || bestDir == "" {
			return candidateDirs[0]
		}
		return bestDir
	}

	if len(candidateDirs) > 0 {
		return candidateDirs[0]
	}
	return dirName
}
