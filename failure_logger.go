// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: Logs failed patch attempts for analysis and improvement.
// :: filename: failure_logger.go
// :: serialization: go

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
)

type PatchFailureLog struct {
	Timestamp string                `json:"timestamp"`
	Phase     string                `json:"phase"` // "preview", "apply", "compiler"
	File      string                `json:"file"`
	Error     string                `json:"error"`
	LineEcho  string                `json:"line_echo,omitempty"`
	Patches   []patcheng.FuzzyPatch `json:"patches,omitempty"`
}

func appendFailureLog(rootDir string, logEntry PatchFailureLog) {
	logPath := filepath.Join(rootDir, ".appy_failures.jsonl")
	logEntry.Timestamp = time.Now().UTC().Format(time.RFC3339)

	// Trim patches to prevent the JSONL file from ballooning to 700k+
	var trimmed []patcheng.FuzzyPatch
	for _, p := range logEntry.Patches {
		p.Replace = fmt.Sprintf("<elided: %d bytes>", len(p.Replace))
		if len(p.Search) > 500 {
			p.Search = p.Search[:500] + "...<truncated>"
		}
		trimmed = append(trimmed, p)
	}
	logEntry.Patches = trimmed

	b, err := json.Marshal(logEntry)
	if err != nil {
		return
	}
	b = append(b, '\n')

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(b)
}
