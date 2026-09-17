package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func (s *AppyServer) markFileCommitError(applyFiles *[]ApplyFile, hasErrors *bool, absPath, reason string) {
	*hasErrors = true
	rel, errRel := filepath.Rel(s.rootDir, absPath)
	if errRel != nil {
		rel = absPath
	}
	slashRel := filepath.ToSlash(rel)
	baseName := filepath.Base(absPath)

	for i, af := range *applyFiles {
		if af.Path == slashRel || af.Path == rel || af.Path == baseName {
			(*applyFiles)[i].Applied = false
			(*applyFiles)[i].Error = "Post-Write Verification Failed: " + reason
			(*applyFiles)[i].FailedPatch = &FailedPatch{
				Error:           reason,
				LLMFallbackHint: "Write was rejected and safely rolled back to prevent corruption. Review patch boundaries or use %%% complete_replace.",
			}
			appendFailureLog(s.rootDir, PatchFailureLog{
				Phase: "verify_rollback",
				File:  rel,
				Error: reason,
			})
			return
		}
	}
}

func (s *AppyServer) commitChangesToDisk(originalFiles map[string]*[]byte, memoryResults map[string]string, filesToDelete map[string]bool, fileHashes map[string][]string, applyFiles *[]ApplyFile, hasErrors *bool) {
	log.Printf("[DEBUG] commitChangesToDisk invoked: %d memory files to write, %d files to delete", len(memoryResults), len(filesToDelete))
	if err := saveHistory(s.rootDir, originalFiles); err != nil {
		log.Printf("Warning: failed to save history: %v", err)
	}

	committedPaths := make(map[string]bool)

	for path, content := range memoryResults {
		origBytesPtr := originalFiles[path]
		origLines := 0
		origSize := 0
		if origBytesPtr != nil && *origBytesPtr != nil {
			origLines = countLines(string(*origBytesPtr))
			origSize = len(*origBytesPtr)
		} else {
			// Fallback: check disk directly if not captured in originalFiles
			if diskBytes, err := os.ReadFile(path); err == nil && len(diskBytes) > 0 {
				cbCopy := make([]byte, len(diskBytes))
				copy(cbCopy, diskBytes)
				origBytesPtr = &cbCopy
				origLines = countLines(string(diskBytes))
				origSize = len(diskBytes)
			}
		}

		// Strictly refuse 0-byte writes to non-empty existing files unless explicitly marked as a deletion target
		// Strictly refuse 0-byte writes to non-empty existing files unless explicitly marked as a deletion target
		if len(content) == 0 && origSize > 0 {
			rel, errRel := filepath.Rel(s.rootDir, path)
			if errRel != nil {
				rel = path
			}
			relSlash := filepath.ToSlash(rel)
			baseName := filepath.Base(path)

			isExplicitDelete := false
			for _, af := range *applyFiles {
				match := (af.Path == rel || af.Path == relSlash || af.Path == baseName)
				if match && (strings.Contains(af.Path, "delete_me") || (origLines > 0 && af.NetLines == -origLines)) {
					isExplicitDelete = true
					break
				}
			}

			if !isExplicitDelete {
				log.Printf("[WARN] commitChangesToDisk: Refusing unexpected 0-byte write protection for %s", path)
				s.markFileCommitError(applyFiles, hasErrors, path, "0-byte write protection triggered (refusing empty file)")
				continue
			}
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			log.Printf("[ERROR] commitChangesToDisk: MkdirAll failed for %s: %v", filepath.Dir(path), err)
			s.markFileCommitError(applyFiles, hasErrors, path, fmt.Sprintf("MkdirAll failed: %v", err))
			continue
		}

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			log.Printf("[ERROR] commitChangesToDisk: Failed writing %d bytes to %s: %v", len(content), path, err)
			s.markFileCommitError(applyFiles, hasErrors, path, fmt.Sprintf("WriteFile failed: %v", err))
			continue
		}

		writtenBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			log.Printf("[ERROR] commitChangesToDisk: Post-write verification read failed for %s: %v", path, readErr)
			s.rollbackFile(path, origBytesPtr)
			s.markFileCommitError(applyFiles, hasErrors, path, fmt.Sprintf("Post-write verification read error: %v (file rolled back)", readErr))
			continue
		}

		writtenLines := countLines(string(writtenBytes))
		writtenSize := len(writtenBytes)

		isFishy := false
		reason := ""

		if writtenSize == 0 && origSize > 0 && len(content) > 0 {
			isFishy = true
			reason = fmt.Sprintf("file unexpectedly truncated to 0 bytes (previously %d bytes)", origSize)
		} else if origLines >= 20 && len(content) > 0 && writtenLines < (origLines*30/100) && (origLines-writtenLines) > 20 {
			isFishy = true
			reason = fmt.Sprintf("line count collapsed by >70%% (below 30%% of original: from %d to %d lines)", origLines, writtenLines)
		}

		if isFishy {
			log.Printf("[ALERT] commitChangesToDisk: Corrupt/fishy write detected for %s (%s). Restoring backup immediately!", path, reason)
			s.rollbackFile(path, origBytesPtr)
			s.markFileCommitError(applyFiles, hasErrors, path, fmt.Sprintf("%s (file restored to original backup)", reason))
			continue
		}

		log.Printf("[COMMIT] Successfully verified %s (%d bytes, %d lines)", path, writtenSize, writtenLines)
		committedPaths[path] = true
	}

	for path := range filesToDelete {
		if err := os.Remove(path); err != nil {
			log.Printf("[ERROR] commitChangesToDisk: Failed removing %s: %v", path, err)
		} else {
			log.Printf("[COMMIT] Successfully deleted %s", path)
			committedPaths[path] = true
		}
	}

	appliedPatchesMu.Lock()
	for path := range memoryResults {
		if !committedPaths[path] {
			continue
		}
		for _, h := range fileHashes[path] {
			appliedPatches[h] = true
		}
	}
	for path := range filesToDelete {
		if !committedPaths[path] {
			continue
		}
		for _, h := range fileHashes[path] {
			appliedPatches[h] = true
		}
	}
	appliedPatchesMu.Unlock()
	SaveLedger(s.rootDir)
}

func (s *AppyServer) rollbackFile(path string, origBytesPtr *[]byte) {
	if origBytesPtr == nil || *origBytesPtr == nil {
		if err := os.Remove(path); err != nil {
			log.Printf("[ERROR] rollbackFile: failed removing newly created corrupt file %s: %v", path, err)
		} else {
			log.Printf("[RESTORED] Removed newly created corrupt file: %s", path)
		}
		return
	}
	if err := os.WriteFile(path, *origBytesPtr, 0644); err != nil {
		log.Printf("[FATAL] Failed rolling back original contents to %s: %v", path, err)
	} else {
		log.Printf("[RESTORED] Successfully reverted %s to original state (%d bytes)", path, len(*origBytesPtr))
	}
}
