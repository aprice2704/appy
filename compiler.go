// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 6
// :: description: Compiler pre-flight and formatting via patcheng registry.
// :: filename: /home/aprice/dev/appy/compiler.go
// :: serialization: go

package main

import (
	"context"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
)

func runCompilerPreFlight(registry *patcheng.ProfileRegistry, memoryResults map[string]string, skipSlow bool) map[string]string {
	compilerErrors := make(map[string]string)
	log.Printf("[DEBUG] runCompilerPreFlight: starting for %d files", len(memoryResults))
	for path, content := range memoryResults {
		if strings.TrimSpace(content) == "" {
			log.Printf("[DEBUG] runCompilerPreFlight: skipping empty file %s", path)
			continue
		}
		prof := registry.GetByExtension(filepath.Ext(path))
		if prof != nil && prof.Validator != nil {
			if skipSlow && !prof.FastValidator {
				log.Printf("[DEBUG] runCompilerPreFlight: skipping slow validator for %s", path)
				continue
			}
			log.Printf("[DEBUG] runCompilerPreFlight: validating %s", path)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := prof.Validator(ctx, []byte(content))
			cancel()
			if err != nil {
				log.Printf("[DEBUG] runCompilerPreFlight: validation failed for %s: %v", path, err)
				compilerErrors[path] = err.Error()
			} else {
				log.Printf("[DEBUG] runCompilerPreFlight: validation passed for %s", path)
			}
		} else {
			log.Printf("[DEBUG] runCompilerPreFlight: no validator found for %s", path)
		}
	}
	log.Printf("[DEBUG] runCompilerPreFlight: finished with %d errors", len(compilerErrors))
	return compilerErrors
}
