// :: product: FDM/NS
// :: majorVersion: 2
// :: fileVersion: 2
// :: description: Telemetry logger for patch activity and failures in current working dir.
// :: filename: failure_logger.go
// :: serialization: go

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
)

type PatchFailureLog struct {
	Timestamp string                `json:"timestamp"`
	Phase     string                `json:"phase"` // "preview", "apply", "compiler"
	File      string                `json:"file"`
	Error     string                `json:"error"`
	Methods   []string              `json:"methods,omitempty"`
	LineEcho  string                `json:"line_echo,omitempty"`
	Patches   []patcheng.FuzzyPatch `json:"patches,omitempty"`
}

type PatchActivityLog struct {
	Timestamp string   `json:"timestamp"`
	Action    string   `json:"action"` // "apply", "preview"
	File      string   `json:"file"`
	Status    string   `json:"status"` // "SUCCESS", "FAIL"
	Methods   []string `json:"methods,omitempty"`
	NetLines  int      `json:"net_lines"`
}

func detectPatchMethod(p patcheng.FuzzyPatch) string {
	switch {
	case p.FullOverwrite:
		return "overwrite"
	case p.SymbolName != "":
		return "replace_symbol"
	case p.IsReplaceAst:
		return "replace_ast"
	case p.IsReplaceElement:
		return "replace_element"
	case p.IsReplaceJson:
		return "replace_json_path"
	case p.IsNdclUpdate:
		return "ndcl_update"
	case p.IsMetaUpdate:
		return "meta_update"
	case p.IsDeleteFile:
		return "delete_file"
	case p.IsAnchored:
		return "replace_anchored"
	case len(p.ImportDirectives) > 0:
		return "import"
	default:
		return "replace_fuzzy"
	}
}

// appendFailureLog always writes to ./.appy_failures.jsonl relative to the appy executable
func appendFailureLog(_ string, logEntry PatchFailureLog) {
	if len(logEntry.Methods) == 0 && len(logEntry.Patches) > 0 {
		for _, p := range logEntry.Patches {
			logEntry.Methods = append(logEntry.Methods, detectPatchMethod(p))
		}
	}
	logEntry.Timestamp = time.Now().UTC().Format(time.RFC3339)

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

	f, err := os.OpenFile(".appy_failures.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(b)
}

// appendActivityLog writes to ./.appy_activity.jsonl relative to the appy executable
func appendActivityLog(entry PatchActivityLog) {
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	b = append(b, '\n')

	f, err := os.OpenFile(".appy_activity.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(b)
}
