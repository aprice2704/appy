package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/aprice2704/fdm/code/patcheng"
	"github.com/aprice2704/fdm/code/retest"
)

func (s *AppyServer) handleForget(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] /api/forget request received")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req ForgetPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	parsed, err := patcheng.ParseTextBundle(req.Bundle, patcheng.DefaultRegistry)
	if err != nil {
		sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	patches, ok := parsed[req.Path]
	if !ok {
		sendError(w, "Path not found in bundle", http.StatusBadRequest)
		return
	}

	appliedPatchesMu.Lock()
	for _, p := range patches {
		h := hashPatch(req.Path, p.Search, p.Replace)
		delete(appliedPatches, h)
	}
	appliedPatchesMu.Unlock()
	SaveLedger(s.rootDir)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(withND("appy/forget", "Reset patch in ledger", map[string]any{
		"success": true,
	})); err != nil {
		log.Printf("[ERROR] handleForget: response encode failed: %v", err)
	}
}

func (s *AppyServer) handleRetest(w http.ResponseWriter, r *http.Request) {
	log.Printf("[DEBUG] /api/retest request received")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req RetestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	opts := retest.Options{JSONMode: true, Args: req.Packages}
	report, err := retest.Run(r.Context(), opts)
	if err != nil {
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var outFiles []RetestResponseFile
	for _, fail := range report.HardFails {
		outFiles = append(outFiles, RetestResponseFile{
			TestStatus: TestStatusFail,
			Package:    fail.Task.Package,
			RawOutput:  fail.Output,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	respObj := RetestResponse{Packages: req.Packages, Files: outFiles}
	if err := json.NewEncoder(w).Encode(withND("appy/retest", "Test execution report", map[string]any{
		"packages": respObj.Packages,
		"files":    respObj.Files,
	})); err != nil {
		log.Printf("[ERROR] handleRetest: response encode failed: %v", err)
	}
}

func (s *AppyServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	hist, err := listHistory(s.rootDir)
	if err != nil {
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(withND("appy/history", "Patch transaction history", map[string]any{
		"history": hist,
	})); err != nil {
		log.Printf("[ERROR] handleHistory: response encode failed: %v", err)
	}
}

func (s *AppyServer) handleRevert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req RevertPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if err := revertTransaction(s.rootDir, req.TxID); err != nil {
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(withND("appy/revert", "Revert completed", map[string]any{
		"reverted": true,
	})); err != nil {
		log.Printf("[ERROR] handleRevert: response encode failed: %v", err)
	}
}
