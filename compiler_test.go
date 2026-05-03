// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 2
// :: description: Unit tests for compiler pre-flight.
// :: filename: code/cmd/appy/compiler_test.go
// :: serialization: go

package main

import (
	"testing"

	"github.com/aprice2704/fdm/code/patcheng"
)

func TestRunCompilerPreFlight(t *testing.T) {
	t.Run("Valid Code", func(t *testing.T) {
		mem := map[string]string{
			"main.go": "package main\n\nfunc main() {}\n",
		}
		errs := runCompilerPreFlight(patcheng.DefaultRegistry, mem, false)
		if len(errs) > 0 {
			t.Errorf("Expected no error for valid code, got: %v", errs)
		}
	})

	t.Run("Invalid Code", func(t *testing.T) {
		mem := map[string]string{
			"main.go": "package main\n\nfunc main() { syntax error!! }\n",
		}
		errs := runCompilerPreFlight(patcheng.DefaultRegistry, mem, false)
		if len(errs) == 0 {
			t.Fatal("Expected error for invalid code")
		}
	})
}
