package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ExecBashPayload struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type ExecBashResponse struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Duration string `json:"duration"`
	Error    string `json:"error,omitempty"`
}

func (s *AppyServer) handleExecBash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ExecBashPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	cmdStr := strings.TrimSpace(req.Command)
	if cmdStr == "" {
		sendError(w, "Command is empty", http.StatusBadRequest)
		return
	}

	// Strip any accidental leading prompt markers ($ or %)
	if strings.HasPrefix(cmdStr, "$ ") {
		cmdStr = strings.TrimSpace(cmdStr[2:])
	} else if strings.HasPrefix(cmdStr, "% ") {
		cmdStr = strings.TrimSpace(cmdStr[2:])
	}

	timeoutSec := req.Timeout
	if timeoutSec <= 0 || timeoutSec > 300 {
		timeoutSec = 60
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	homeDir, _ := os.UserHomeDir()
	appyRcGlobal := filepath.Join(homeDir, ".appy_bashrc")
	appyRcLocal := filepath.Join(s.rootDir, ".appy_bashrc")

	// Preamble ensuring alias expansion, standard Go/cargo PATHs, and loading .appy_bashrc if present
	preamble := `
shopt -s expand_aliases
export PATH="$HOME/go/bin:/usr/local/go/bin:$HOME/.local/bin:$HOME/.cargo/bin:$PATH"
alias p='piranha'
alias l='licecomb -disable nesting_flatten'
alias gs='git status'
alias gd='git diff'
`
	if _, err := os.Stat(appyRcGlobal); err == nil {
		preamble += fmt.Sprintf("[ -f %q ] && source %q\n", appyRcGlobal, appyRcGlobal)
	} else {
		// Fallback to sourcing ~/.bashrc safely
		preamble += "[ -f ~/.bashrc ] && source ~/.bashrc 2>/dev/null || true\n"
		// Re-ensure alias p points directly to piranha without terminal ccmd
		preamble += "alias p='piranha'\n"
	}
	if _, err := os.Stat(appyRcLocal); err == nil {
		preamble += fmt.Sprintf("[ -f %q ] && source %q\n", appyRcLocal, appyRcLocal)
	}

	wrappedCmd := preamble + "\n" + cmdStr

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, shell, "-c", wrappedCmd)
	cmd.Dir = s.rootDir

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	duration := time.Since(start).Round(time.Millisecond).String()

	exitCode := 0
	var errStr string
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
			errStr = err.Error()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ExecBashResponse{
		Output:   outBuf.String(),
		ExitCode: exitCode,
		Duration: duration,
		Error:    errStr,
	})
}
