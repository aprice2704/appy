package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
)

func updateApplyFileCompilerError(applyFiles *[]ApplyFile, rel string, errOutput string) {
	targetSlash := filepath.ToSlash(rel)
	for i, af := range *applyFiles {
		if af.Path != targetSlash && af.Path != rel {
			continue
		}
		(*applyFiles)[i].Applied = false
		trimmedErr := strings.TrimSpace(errOutput)
		firstLine := trimmedErr
		if idx := strings.Index(firstLine, "\n"); idx != -1 {
			firstLine = strings.TrimSpace(firstLine[:idx])
		}
		(*applyFiles)[i].Error = "Compiler Error: " + firstLine
		(*applyFiles)[i].FailedPatch = &FailedPatch{
			Error:           "Compiler Error:\n" + trimmedErr,
			CurrentLineEcho: trimmedErr,
			LLMFallbackHint: "Check compiler output and adjust types or imports.",
		}
		return
	}
}

func (s *AppyServer) runCompilerChecks(w http.ResponseWriter, req Payload, memoryResults map[string]string, filesToDelete map[string]bool, applyFiles *[]ApplyFile, hasErrors *bool) bool {
	for p, content := range memoryResults {
		prof := patcheng.DefaultRegistry.GetByExtension(filepath.Ext(p))
		if prof == nil || prof.Formatter == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		formatted, _, err := prof.Formatter(ctx, []byte(content))
		cancel()
		if err == nil && len(formatted) > 0 {
			memoryResults[p] = string(formatted)
		}
	}

	compErrs := runCompilerPreFlight(patcheng.DefaultRegistry, memoryResults, false)
	var checkFiles []CompilerCheckFile

	for p, e := range compErrs {
		rel, errRel := filepath.Rel(s.rootDir, p)
		if errRel != nil {
			rel = p
		}
		*hasErrors = true

		if req.CheckOnly {
			checkFiles = append(checkFiles, CompilerCheckFile{
				Path:           filepath.ToSlash(rel),
				CompilerStatus: CompilerStatusFail,
				RawOutput:      e,
			})
		} else {
			updateApplyFileCompilerError(applyFiles, rel, e)
		}
		appendFailureLog(s.rootDir, PatchFailureLog{
			Phase: "compiler",
			File:  rel,
			Error: e,
		})
		delete(memoryResults, p)
	}

	if !req.CheckOnly {
		return false
	}

	for p := range memoryResults {
		rel, errRel := filepath.Rel(s.rootDir, p)
		if errRel != nil {
			rel = p
		}
		checkFiles = append(checkFiles, CompilerCheckFile{
			Path:           filepath.ToSlash(rel),
			CompilerStatus: CompilerStatusPass,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(withND("appy/compiler-check", "Compiler pre-flight results", map[string]any{
		"files": checkFiles,
	})); err != nil {
		log.Printf("[ERROR] runCompilerChecks: failed encoding check results: %v", err)
	}
	return true
}
