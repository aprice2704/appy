package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
)

func (s *AppyServer) applySingleFile(rawFilename string, patches []patcheng.FuzzyPatch, originalFiles map[string]*[]byte) (ApplyFile, string, bool, bool, []string, error) {
	absPath := filepath.Join(s.rootDir, rawFilename)
	contentBytes, errRead := os.ReadFile(absPath)
	if errRead == nil {
		cbCopy := make([]byte, len(contentBytes))
		copy(cbCopy, contentBytes)
		originalFiles[absPath] = &cbCopy
	} else {
		originalFiles[absPath] = nil
	}

	prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))
	fType, fIcon := getFileMeta(prof)

	var methods []string
	for _, p := range patches {
		methods = append(methods, detectPatchMethod(p))
	}

	// Ledger skip check
	if len(patches) > 0 && s.allPatchesInLedger(rawFilename, patches) {
		log.Printf("[DEBUG] /api/apply: %s skipped (all patches already in ledger)", rawFilename)
		appendActivityLog(PatchActivityLog{
			Action:   "apply",
			File:     rawFilename,
			Status:   "SUCCESS",
			Methods:  methods,
			NetLines: 0,
		})
		return ApplyFile{
			Path:     rawFilename,
			Applied:  true,
			NetLines: 0,
			FileType: fType,
			FileIcon: fIcon,
		}, "", false, true, nil, nil
	}

	contentStr := string(contentBytes)
	fileNetLines := 0
	isDelete := false
	var hashes []string

	for i, p := range patches {
		if !p.FullOverwrite && !p.IsDeleteFile && len(p.ImportDirectives) == 0 {
			if strings.Contains(p.Search, "imp"+"ort (") || strings.HasPrefix(strings.TrimSpace(p.Search), "imp"+"ort \"") {
				dirs := extractImportDirectivesFromText(p.Replace)
				if len(dirs) > 0 {
					patches[i].ImportDirectives = dirs
					patches[i].Search = ""
					patches[i].Replace = ""
					p = patches[i]
				}
			}
		}

		if valErr := ValidateFuzzySearchBlock(p); valErr != nil {
			appendActivityLog(PatchActivityLog{
				Action:   "apply",
				File:     rawFilename,
				Status:   "FAIL",
				Methods:  methods,
				NetLines: 0,
			})
			return s.buildApplyFailure(rawFilename, fType, fIcon, contentBytes, patches, valErr), "", false, false, nil, valErr
		}

		if p.FullOverwrite {
			if p.Search == "CREATE_ASSERT" && len(contentBytes) > 0 {
				errExist := fmt.Errorf("refusing to create file: %s already exists (size: %d bytes). Use 'complete_replace' or 'overwrite'", rawFilename, len(contentBytes))
				appendActivityLog(PatchActivityLog{
					Action:   "apply",
					File:     rawFilename,
					Status:   "FAIL",
					Methods:  methods,
					NetLines: 0,
				})
				return s.buildApplyFailure(rawFilename, fType, fIcon, contentBytes, patches, errExist), "", false, false, nil, errExist
			}
			if p.Search == "REPLACE_ASSERT" && len(contentBytes) == 0 {
				errMissing := fmt.Errorf("refusing to complete_replace file: %s does not exist or is empty. Use 'create' or 'overwrite'", rawFilename)
				appendActivityLog(PatchActivityLog{
					Action:   "apply",
					File:     rawFilename,
					Status:   "FAIL",
					Methods:  methods,
					NetLines: 0,
				})
				return s.buildApplyFailure(rawFilename, fType, fIcon, contentBytes, patches, errMissing), "", false, false, nil, errMissing
			}
			linesBefore := countLines(contentStr)
			fileNetLines += countLines(p.Replace) - linesBefore
		} else if p.IsDeleteFile {
			isDelete = true
			fileNetLines -= countLines(contentStr)
		} else {
			fileNetLines += countLines(p.Replace) - countLines(p.Search)
		}
		hashes = append(hashes, hashPatch(rawFilename, p.Search, p.Replace))
	}

	newContent, applyErr := patcheng.ApplyFuzzyPatchesAgnostic(prof, contentStr, patches)
	if applyErr != nil {
		appendActivityLog(PatchActivityLog{
			Action:   "apply",
			File:     rawFilename,
			Status:   "FAIL",
			Methods:  methods,
			NetLines: 0,
		})
		return s.buildApplyFailure(rawFilename, fType, fIcon, contentBytes, patches, applyErr), "", false, false, nil, applyErr
	}

	if prof != nil && prof.Formatter != nil && !isDelete {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		formatted, _, err := prof.Formatter(ctx, []byte(newContent))
		cancel()
		if err != nil {
			log.Printf("[DEBUG] applySingleFile: formatter warning for %s: %v", rawFilename, err)
		} else {
			newContent = string(formatted)
		}
	}

	appendActivityLog(PatchActivityLog{
		Action:   "apply",
		File:     rawFilename,
		Status:   "SUCCESS",
		Methods:  methods,
		NetLines: fileNetLines,
	})

	return ApplyFile{
		Path:     rawFilename,
		Applied:  true,
		NetLines: fileNetLines,
		FileType: fType,
		FileIcon: fIcon,
	}, newContent, isDelete, false, hashes, nil
}

func (s *AppyServer) allPatchesInLedger(rawFilename string, patches []patcheng.FuzzyPatch) bool {
	appliedPatchesMu.RLock()
	defer appliedPatchesMu.RUnlock()
	for _, p := range patches {
		if !appliedPatches[hashPatch(rawFilename, p.Search, p.Replace)] {
			return false
		}
	}
	return true
}

func (s *AppyServer) buildApplyFailure(rawFilename, fType, fIcon string, contentBytes []byte, patches []patcheng.FuzzyPatch, err error) ApplyFile {
	prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(rawFilename))
	var failedBlock *FailedPatch
	for _, p := range patches {
		hint := generateDiagnosticHint(prof, rawFilename, string(contentBytes), p.Search, p.NearLine)
		cur := ""
		if strings.Contains(hint, ": ") {
			for _, l := range strings.Split(hint, "\n") {
				if strings.Contains(l, ": ") && !strings.Contains(l, "elided") {
					cur = strings.TrimSpace(strings.SplitN(l, ": ", 2)[1])
					break
				}
			}
		}
		failedBlock = &FailedPatch{
			Error:           err.Error(),
			CurrentLineEcho: cur,
			LLMFallbackHint: generateLLMFallbackHint(prof),
		}
		break
	}

	appendFailureLog(s.rootDir, PatchFailureLog{
		Phase:    "apply",
		File:     rawFilename,
		Error:    err.Error(),
		LineEcho: failedBlock.CurrentLineEcho,
		Patches:  patches,
	})

	return ApplyFile{
		Path:        rawFilename,
		Applied:     false,
		FileType:    fType,
		FileIcon:    fIcon,
		Error:       err.Error(),
		FailedPatch: failedBlock,
	}
}
