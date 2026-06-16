// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: End-to-End browser tests for the Appy UI (Core & Setup).
// :: filename: ui_e2e_core_test.go
// :: serialization: go

package main

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// setupTestServer creates an isolated Appy server and a chromedp context.
func setupTestServer(t *testing.T) (*httptest.Server, context.Context, context.CancelFunc, string) {
	tempDir := t.TempDir()

	// Provide a local go.mod to prevent 'retest' from walking up the OS directory tree and hanging
	os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module appytest\n\ngo 1.22\n"), 0644)

	// Create a dummy target file for patching tests
	err := os.WriteFile(filepath.Join(tempDir, "target.go"), []byte("package main\n\nfunc Old() {}\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy target file: %v", err)
	}

	ts := httptest.NewServer(newTestServer(tempDir))

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
	)
	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)

	// Capture browser console logs to aid in debugging test failures
	ctx, cancel := chromedp.NewContext(allocCtx)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventExceptionThrown:
			t.Logf("Browser Exception: %s", ev.ExceptionDetails.Text)
		case *runtime.EventConsoleAPICalled:
			var args []string
			for _, arg := range ev.Args {
				args = append(args, string(arg.Value))
			}
			t.Logf("Browser Console: %s", strings.Join(args, " "))
		}
	})

	return ts, ctx, cancel, tempDir
}

func TestE2E_LayoutAndHeaders(t *testing.T) {
	ts, ctx, cancel, tempDir := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	var title, version, sandboxRoot string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`h2`, chromedp.ByQuery),
		chromedp.Text(`h2`, &title, chromedp.ByQuery),
		chromedp.Text(`h2 span`, &version, chromedp.ByQuery),
		chromedp.Text(`.header-zone div`, &sandboxRoot, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}

	// Verify t-lay-01: title matches last element of sandbox path
	expectedTitle := filepath.Base(tempDir)
	if !strings.Contains(title, expectedTitle) {
		t.Errorf("Expected title to contain %q, got %q", expectedTitle, title)
	}

	// Verify t-lay-02: version is present
	if !strings.Contains(version, AppVersion) {
		t.Errorf("Expected version to contain %q, got %q", AppVersion, version)
	}

	// Verify t-lay-03: sandbox root is displayed
	if !strings.Contains(sandboxRoot, tempDir) {
		t.Errorf("Expected sandbox root to contain %q, got %q", tempDir, sandboxRoot)
	}
}

func TestE2E_UI_InitialState(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	var checkBtnDisabled, applyBtnDisabled bool

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('checkBtn').hasAttribute('disabled')`, &checkBtnDisabled),
		chromedp.Evaluate(`document.getElementById('applyBtn').hasAttribute('disabled')`, &applyBtnDisabled),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}

	if !checkBtnDisabled || !applyBtnDisabled {
		t.Errorf("Expected check and apply buttons to be disabled on load")
	}
}
