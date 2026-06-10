// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: Logs failed patch attempts for analysis and improvement.
// :: filename: failure_logger.go
// :: serialization: go

package main

import (
	"encoding/json"
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
