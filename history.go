// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 2
// :: description: Manages patch transaction history and reversions.
// :: filename: history.go
// :: serialization: go

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const historyDirName = ".appy_history"
const maxHistory = 10

func saveHistory(rootDir string, originalFiles map[string]*[]byte) error {
	historyDir := filepath.Join(rootDir, historyDirName)
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history dir: %v", err)
	}

	rawTxID := fmt.Sprintf("tx_%d", time.Now().UnixNano())
	txDir := filepath.Join(historyDir, rawTxID+"_files")

	tx := HistoryTx{
		TxID:      TxID(rawTxID),
		Timestamp: time.Now().Unix(),
		Files:     []HistoryFileOp{},
	}

	for path, contentPtr := range originalFiles {
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			continue // Skip paths we can't make relative
		}

		op := HistoryFileOp{
			Path:    filepath.ToSlash(relPath),
			Existed: contentPtr != nil,
		}
		tx.Files = append(tx.Files, op)

		if !op.Existed {
			continue
		}

		destPath := filepath.Join(txDir, relPath+".bak")
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to make history tx dir: %v", err)
		}
		if err := os.WriteFile(destPath, *contentPtr, 0644); err != nil {
			return fmt.Errorf("failed to write history file backup: %v", err)
		}
	}

	txBytes, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history tx: %v", err)
	}

	if err := os.WriteFile(filepath.Join(historyDir, rawTxID+".json"), txBytes, 0644); err != nil {
		return fmt.Errorf("failed to write history tx file: %v", err)
	}

	pruneHistory(historyDir)
	return nil
}

func pruneHistory(historyDir string) {
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return
	}

	var txFiles []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" || e.Name() == ledgerFilename {
			continue
		}
		txFiles = append(txFiles, e.Name())
	}

	if len(txFiles) <= maxHistory {
		return
	}

	sort.Strings(txFiles)
	for i := 0; i < len(txFiles)-maxHistory; i++ {
		oldTx := txFiles[i]
		base := oldTx[:len(oldTx)-5]
		if err := os.Remove(filepath.Join(historyDir, oldTx)); err != nil {
			log.Printf("[DEBUG] pruneHistory: remove failed for %s: %v", oldTx, err)
		}
		if err := os.RemoveAll(filepath.Join(historyDir, base+"_files")); err != nil {
			log.Printf("[DEBUG] pruneHistory: removeAll failed for %s_files: %v", base, err)
		}
	}
}

func listHistory(rootDir string) ([]HistoryTx, error) {
	historyDir := filepath.Join(rootDir, historyDirName)
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []HistoryTx{}, nil
		}
		return nil, fmt.Errorf("failed to read history directory %s: %w", historyDir, err)
	}

	var history []HistoryTx
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" || e.Name() == ledgerFilename {
			continue
		}
		b, err := os.ReadFile(filepath.Join(historyDir, e.Name()))
		if err != nil {
			log.Printf("[DEBUG] listHistory: read file failed for %s: %v", e.Name(), err)
			continue
		}
		var tx HistoryTx
		if err := json.Unmarshal(b, &tx); err != nil {
			log.Printf("[DEBUG] listHistory: unmarshal failed for %s: %v", e.Name(), err)
			continue
		}
		history = append(history, tx)
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp > history[j].Timestamp
	})

	if history == nil {
		history = []HistoryTx{}
	}
	return history, nil
}

func revertTransaction(rootDir string, txID TxID) error {
	historyDir := filepath.Join(rootDir, historyDirName)
	txFile := filepath.Join(historyDir, string(txID)+".json")

	b, err := os.ReadFile(txFile)
	if err != nil {
		return fmt.Errorf("transaction not found: %s", txID)
	}

	var tx HistoryTx
	if err := json.Unmarshal(b, &tx); err != nil {
		return fmt.Errorf("corrupt transaction file: %s", txID)
	}

	txDir := filepath.Join(historyDir, string(txID)+"_files")

	for _, op := range tx.Files {
		targetPath := filepath.Join(rootDir, filepath.FromSlash(op.Path))
		if !isPathSafe(rootDir, targetPath) {
			continue
		}

		if op.Existed {
			srcPath := filepath.Join(txDir, filepath.FromSlash(op.Path)+".bak")
			content, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read backup for %s: %v", op.Path, err)
			}

			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to make target dir for %s: %v", op.Path, err)
			}
			if err := os.WriteFile(targetPath, content, 0644); err != nil {
				return fmt.Errorf("failed to restore file %s: %v", op.Path, err)
			}
		} else {
			if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
				log.Printf("[DEBUG] revertTransaction: remove failed for %s: %v", targetPath, err)
			}
		}
	}

	return nil
}
