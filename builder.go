package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aprice2704/fdm/code/patcheng"
)

var ErrTxtarFileLimitExceeded = errors.New("file limit exceeded: txtar bundle exceeded safety limit")

const MaxTxtarFileCount = 500

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

func shouldAnchorFile(absPath, relName string, anchors []string) bool {
	base := filepath.Base(absPath)
	for _, ag := range anchors {
		if ag == "" {
			continue
		}
		matchedBase, errBase := filepath.Match(ag, base)
		if errBase == nil && matchedBase {
			return false
		}
		matchedRel, errRel := filepath.Match(ag, relName)
		if errRel == nil && matchedRel {
			return false
		}
	}
	return true
}

func appendMissingPathPlaceholders(buf *bytes.Buffer, absRootDir string, paths []string, seenPaths map[string]bool) int {
	addedCount := 0
	for _, p := range paths {
		pTrim := strings.TrimSpace(p)
		if pTrim == "" || strings.ContainsAny(pTrim, "*?{}:") {
			continue
		}
		cleanP := filepath.Clean(pTrim)
		if seenPaths[cleanP] || seenPaths[filepath.ToSlash(cleanP)] {
			continue
		}
		targetCandidate := pTrim
		if !filepath.IsAbs(targetCandidate) {
			targetCandidate = filepath.Join(absRootDir, targetCandidate)
		}
		buf.WriteString(fmt.Sprintf("-- %s --\n", filepath.ToSlash(pTrim)))
		if _, err := os.Stat(targetCandidate); err != nil {
			buf.WriteString(fmt.Sprintf("[ERROR: File %q does not exist or could not be found under %s]\n", pTrim, absRootDir))
		} else {
			buf.WriteString(fmt.Sprintf("[NOTE: File %q exists on disk but was excluded by bundle excludes]\n", pTrim))
		}
		addedCount++
	}
	return addedCount
}

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
			limitErr = fmt.Errorf("%w: txtar bundle exceeded safety limit of %d files; refine include patterns or add excludes", ErrTxtarFileLimitExceeded, MaxTxtarFileCount)
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

		needsAnchor := shouldAnchorFile(absPath, relName, req.Anchors)
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

	fileCount += appendMissingPathPlaceholders(&buf, absRootDir, req.Paths, seenPaths)
	return buf.Bytes(), fileCount, nil
}
