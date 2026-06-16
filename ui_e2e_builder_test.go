// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: End-to-End browser tests for the Appy UI (Builder Tab).
// :: filename: ui_e2e_builder_test.go
// :: serialization: go

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestE2E_UI_BuilderTab(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 20*time.Second)
	defer cancelTimeout()

	var totFiles string
	var totTokens string
	var dropdownValue string
	var resultDisplay string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		// 1. Navigate to Builder tab
		chromedp.Evaluate(`document.getElementById('btn-tab-bundle').click()`, nil),
		chromedp.WaitVisible(`#txtarPathsTable`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond), // Let initial DOM populate without strict dimension checks

		chromedp.Text(`#totFiles`, &totFiles, chromedp.ByID),
		chromedp.Text(`#totTokens`, &totTokens, chromedp.ByID),

		// 2. Set name and save config set (t-bld-04)
		chromedp.SetValue(`#newSetName`, "integration_test_set", chromedp.ByID),
		chromedp.Evaluate(`document.querySelector('button[onclick="saveCurrentSet()"]').click()`, nil),
		chromedp.Sleep(200*time.Millisecond),

		// Read dropdown to ensure it selected the new set
		chromedp.Evaluate(`document.getElementById('setSelect').value`, &dropdownValue),

		// 3. Build Txtar and wait for the result box
		chromedp.Evaluate(`document.getElementById('buildTxtarBtn').click()`, nil),
		chromedp.WaitVisible(`#txtarResult`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('txtarResult').style.display`, &resultDisplay),
	)
	if err != nil {
		t.Fatalf("Builder tab E2E phase failed: %v", err)
	}

	// Assertions
	if totFiles == "" || totTokens == "" {
		t.Errorf("Expected live stats to render text, got Files: %q, Tokens: %q (t-bld-01)", totFiles, totTokens)
	}

	if dropdownValue != "integration_test_set" {
		t.Errorf("Expected Config Set dropdown to select 'integration_test_set' after save, got: %q (t-bld-04)", dropdownValue)
	}

	if resultDisplay == "none" || resultDisplay == "" {
		t.Errorf("Expected txtarResult box to be visible after successful build")
	}
}

func TestE2E_UI_BuilderFixPaths(t *testing.T) {
	ts, ctx, cancel, tempDir := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 20*time.Second)
	defer cancelTimeout()

	// Create a nested file that the builder needs to resolve
	os.MkdirAll(filepath.Join(tempDir, "pkg", "core"), 0755)
	os.WriteFile(filepath.Join(tempDir, "pkg", "core", "engine.go"), []byte("package core"), 0644)

	var fixBtnDisplay string
	var textareaValue string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Evaluate(`document.getElementById('btn-tab-bundle').click()`, nil),
		chromedp.WaitVisible(`#txtarPathsTable`, chromedp.ByQuery),
		chromedp.Evaluate(`
			setTxtarPaths(['core/engine.go']);
			window._testStatsDone = false;
			updateTxtarStats().then(() => { window._testStatsDone = true; });
		`, nil),

		// Wait deterministically for the fetch and DOM update to complete
		chromedp.Poll(`window._testStatsDone === true`, nil),
		chromedp.Evaluate(`document.getElementById('builderFixPathsBtn').style.display`, &fixBtnDisplay),
	)
	if err != nil {
		t.Fatalf("Builder Fix Paths setup failed: %v", err)
	}

	if fixBtnDisplay == "none" || fixBtnDisplay == "" {
		t.Fatalf("Expected Builder Fix Paths button to be visible for unresolved path, got display: %q", fixBtnDisplay)
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			document.getElementById('builderFixPathsBtn').click();
			window._testStatsDone = false;
			// The click calls fixBuilderPaths(), which schedules an update.
			// We clear the timeout and force it immediately to await the resolution.
			clearTimeout(txtarStatsTimeout);
			updateTxtarStats().then(() => { window._testStatsDone = true; });
		`, nil),
		chromedp.Poll(`window._testStatsDone === true`, nil),
		chromedp.Evaluate(`getTxtarPaths().join('\n')`, &textareaValue),
		chromedp.Evaluate(`document.getElementById('builderFixPathsBtn').style.display`, &fixBtnDisplay),
	)
	if err != nil {
		t.Fatalf("Builder Fix Paths click failed: %v", err)
	}

	if !strings.Contains(textareaValue, "pkg/core/engine.go") {
		t.Errorf("Expected textarea to be rewritten with full path, got:\n%s", textareaValue)
	}
	if fixBtnDisplay != "none" {
		t.Errorf("Expected Builder Fix Paths button to hide after use, got: %s", fixBtnDisplay)
	}
}
