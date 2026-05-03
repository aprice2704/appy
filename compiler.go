// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 5
// :: description: Compiler pre-flight and formatting via patcheng registry.
// :: filename: /home/aprice/dev/appy/compiler.go
// :: serialization: go

package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/aprice2704/fdm/code/patcheng"
)

func runCompilerPreFlight(registry *patcheng.ProfileRegistry, memoryResults map[string]string, skipSlow bool) map[string]string {
	compilerErrors := make(map[string]string)
	for path, content := range memoryResults {
		if strings.TrimSpace(content) == "" {
			continue
		}
		prof := registry.GetByExtension(filepath.Ext(path))
		if prof != nil && prof.Validator != nil {
			if skipSlow && !prof.FastValidator {
				continue
			}
			if err := prof.Validator(context.Background(), []byte(content)); err != nil {
				compilerErrors[path] = err.Error()
			}
		}
	}
	return compilerErrors
}
