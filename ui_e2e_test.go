// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 5
// :: description: End-to-End browser tests for the Appy UI using chromedp.
// :: filename: code/cmd/appy/ui_e2e_test.go
// :: serialization: go

package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// setupTestServer creates an isolated Appy server and a chromedp context.
func setupTestServer(t *testing.T) (*httptest.Server, context.Context, context.CancelFunc, string) {
	tempDir := t.TempDir()

	// Create a dummy target file for patching tests
	err := os.WriteFile(filepath.Join(tempDir, "target.go"), []byte("package main\n\nfunc Old() {}\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create dummy target file: %v", err)
	}

	ts := httptest.NewServer(newServer(tempDir))

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

func TestE2E_UI_InitialState(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	var checkBtnDisabled, applyBtnDisabled bool
	var unarmorDisplay string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('checkBtn').hasAttribute('disabled')`, &checkBtnDisabled),
		chromedp.Evaluate(`document.getElementById('applyBtn').hasAttribute('disabled')`, &applyBtnDisabled),
		chromedp.Evaluate(`document.getElementById('unarmorBtn').style.display || "none"`, &unarmorDisplay),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}

	if !checkBtnDisabled || !applyBtnDisabled {
		t.Errorf("Expected check and apply buttons to be disabled on load")
	}
	if unarmorDisplay != "none" && unarmorDisplay != "" {
		t.Errorf("Expected unarmor button to be hidden on load, got display: %s", unarmorDisplay)
	}
}

func TestE2E_UI_ArmorLogic(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	var display string

	// Helper to set textarea value and trigger input event
	setInput := func(val string) chromedp.Action {
		return chromedp.Evaluate(fmt.Sprintf(`
			var el = document.getElementById('bundleInput');
			el.value = %q;
			el.dispatchEvent(new Event('input'));
		`, val), nil)
	}

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),

		// 1. Mixed text (should hide)
		setInput("@@@line 1\nline 2"),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('unarmorBtn').style.display || "none"`, &display),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}
	if display != "none" {
		t.Errorf("Expected unarmor button to be hidden for mixed text, got %s", display)
	}

	err = chromedp.Run(ctx,
		// 2. Strict armored text (should show)
		setInput("@@@line 1\n@@@line 2\n\n@@@line 3"),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('unarmorBtn').style.display`, &display),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}
	if display != "inline-block" {
		t.Errorf("Expected unarmor button to be shown for strictly armored text, got %s", display)
	}
}

func TestE2E_UI_PreviewAndApplyFlow(t *testing.T) {
	ts, ctx, cancel, tempDir := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 15*time.Second)
	defer cancelTimeout()

	// Create a valid bundle targeting our dummy file
	bundle := strings.ReplaceAll(`
### filename: target.go
### replace
func Old() {}
### with
func New() {}
### end
`, "###", patcheng.BundleDelim)

	var applyBtnDisabled bool
	var stripeClass string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),

		// Paste the bundle and wait for the debounce/fetch cycle
		chromedp.Evaluate(fmt.Sprintf(`
			var el = document.getElementById('bundleInput');
			el.value = %q;
			el.dispatchEvent(new Event('input'));
		`, bundle), nil),

		// Wait for the OK stripe to render
		chromedp.WaitVisible(`.file-block.status-ok`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('applyBtn').hasAttribute('disabled')`, &applyBtnDisabled),
	)
	if err != nil {
		t.Fatalf("Preview phase failed: %v", err)
	}

	if applyBtnDisabled {
		t.Errorf("Expected apply button to be ENABLED after successful preview")
	}

	// Click Apply via direct JS evaluation to guarantee execution
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('applyBtn').click()`, nil),
	)
	if err != nil {
		t.Fatalf("Failed to click applyBtn: %v", err)
	}

	// Wait for the stripe to turn grey (applied)
	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`.file-block.status-applied`, chromedp.ByQuery),
		chromedp.AttributeValue(`#file-block-target\.go`, "class", &stripeClass, nil, chromedp.ByQuery),
	)
	if err != nil {
		// If it times out, dump the exact HTML state of the output box so we can diagnose
		var outputHTML string
		chromedp.Run(ctx, chromedp.OuterHTML(`#output`, &outputHTML, chromedp.ByID))
		t.Fatalf("Apply phase failed waiting for DOM update: %v\nOutput HTML state:\n%s", err, outputHTML)
	}

	if !strings.Contains(stripeClass, "status-applied") {
		t.Errorf("Expected stripe to have 'status-applied' class, got: %s", stripeClass)
	}

	// Verify the file was actually written to disk by the backend
	contentBytes, err := os.ReadFile(filepath.Join(tempDir, "target.go"))
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}
	if !strings.Contains(string(contentBytes), "func New() {}") {
		t.Errorf("File on disk was not modified correctly. Content:\n%s", string(contentBytes))
	}
}
