// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: Manages patch transaction history and reversions.
// :: filename: history.go
// :: serialization: go

package main

import (
	"encoding/json"
	"fmt"
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

	txID := fmt.Sprintf("tx_%d", time.Now().UnixNano())
	txDir := filepath.Join(historyDir, txID+"_files")

	tx := HistoryTx{
		TxID:      txID,
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

		if op.Existed {
			destPath := filepath.Join(txDir, relPath+".bak")
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return fmt.Errorf("failed to make history tx dir: %v", err)
			}
			if err := os.WriteFile(destPath, *contentPtr, 0644); err != nil {
				return fmt.Errorf("failed to write history file backup: %v", err)
			}
		}
	}

	txBytes, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal history tx: %v", err)
	}

	if err := os.WriteFile(filepath.Join(historyDir, txID+".json"), txBytes, 0644); err != nil {
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
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" && e.Name() != ledgerFilename {
			txFiles = append(txFiles, e.Name())
		}
	}

	if len(txFiles) <= maxHistory {
		return
	}

	sort.Strings(txFiles) // tx_ timestamp sorts naturally
	for i := 0; i < len(txFiles)-maxHistory; i++ {
		oldTx := txFiles[i]
		base := oldTx[:len(oldTx)-5]
		_ = os.Remove(filepath.Join(historyDir, oldTx))
		_ = os.RemoveAll(filepath.Join(historyDir, base+"_files"))
	}
}

func listHistory(rootDir string) ([]HistoryTx, error) {
	historyDir := filepath.Join(rootDir, historyDirName)
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return []HistoryTx{}, nil
	}

	var history []HistoryTx
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" && e.Name() != ledgerFilename {
			b, err := os.ReadFile(filepath.Join(historyDir, e.Name()))
			if err == nil {
				var tx HistoryTx
				if err := json.Unmarshal(b, &tx); err == nil {
					history = append(history, tx)
				}
			}
		}
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp > history[j].Timestamp
	})

	if history == nil {
		history = []HistoryTx{}
	}
	return history, nil
}

func revertTransaction(rootDir, txID string) error {
	historyDir := filepath.Join(rootDir, historyDirName)
	txFile := filepath.Join(historyDir, txID+".json")

	b, err := os.ReadFile(txFile)
	if err != nil {
		return fmt.Errorf("transaction not found: %s", txID)
	}

	var tx HistoryTx
	if err := json.Unmarshal(b, &tx); err != nil {
		return fmt.Errorf("corrupt transaction file: %s", txID)
	}

	txDir := filepath.Join(historyDir, txID+"_files")

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
			_ = os.Remove(targetPath)
		}
	}

	return nil
}
