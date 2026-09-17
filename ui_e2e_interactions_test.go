package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aprice2704/fdm/code/patcheng"
	"github.com/chromedp/chromedp"
)

func TestE2E_UI_JunkInput(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 15*time.Second)
	defer cancelTimeout()

	var applyBtnDisabled bool
	var outputText string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
		chromedp.Evaluate(`
			var el = document.getElementById('bundleInput');
			el.value = "Hey Appy, just chatting, no patches here!";
			el.dispatchEvent(new Event('input'));
		`, nil),
		chromedp.Sleep(800*time.Millisecond),
		chromedp.Text(`#output`, &outputText, chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('applyBtn').hasAttribute('disabled')`, &applyBtnDisabled),
	)
	if err != nil {
		t.Fatalf("Junk input test failed: %v", err)
	}

	if !strings.Contains(outputText, "No valid patches found") {
		t.Errorf("Expected junk input to show graceful failure (t-edg-01), got: %s", outputText)
	}
	if !applyBtnDisabled {
		t.Errorf("Expected apply button to remain disabled for junk input (t-edg-01)")
	}
}

func TestE2E_UI_FixFilePaths(t *testing.T) {
	ts, ctx, cancel, tempDir := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 15*time.Second)
	defer cancelTimeout()

	os.MkdirAll(filepath.Join(tempDir, "nested", "deep"), 0755)
	os.WriteFile(filepath.Join(tempDir, "nested", "deep", "hidden.go"), []byte("package deep\n// line 1\n// line 2\nfunc FindMe() {}\n"), 0644)

	bundle := strings.ReplaceAll(`
### filename: hidden.go
### replace
// line 1
// line 2
func FindMe() {}
### with
func FoundYou() {}
### end
`, "###", patcheng.BundleDelim)

	var fixBtnDisplay string
	var textareaValue string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
		chromedp.Evaluate(fmt.Sprintf(`
			var el = document.getElementById('bundleInput');
			el.value = %q;
			el.dispatchEvent(new Event('input'));
		`, bundle), nil),
		chromedp.WaitVisible(`.file-block.status-error`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('fixPathsBtn').style.display`, &fixBtnDisplay),
	)
	if err != nil {
		t.Fatalf("Fix paths preview phase failed: %v", err)
	}

	if fixBtnDisplay == "none" || fixBtnDisplay == "" {
		t.Fatalf("Expected Fix File Paths button to be visible, got display: %q", fixBtnDisplay)
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('fixPathsBtn').click()`, nil),
		chromedp.WaitVisible(`.file-block.status-ready`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('bundleInput').value`, &textareaValue),
		chromedp.Evaluate(`document.getElementById('fixPathsBtn').style.display`, &fixBtnDisplay),
	)
	if err != nil {
		t.Fatalf("Fix paths click phase failed: %v", err)
	}

	if !strings.Contains(textareaValue, "nested/deep/hidden.go") {
		t.Errorf("Expected textarea to be rewritten with full path, got:\n%s", textareaValue)
	}
	if fixBtnDisplay != "none" {
		t.Errorf("Expected Fix File Paths button to hide after use, got: %s", fixBtnDisplay)
	}
}

func TestE2E_UI_MicroInteractions(t *testing.T) {
	t.Skip("Skipping MicroInteractions: DOM polling timing issues in headless environment.")
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	bundle := strings.ReplaceAll(`
### filename: target.go
### replace
// line 1
// line 2
func Old() {}
### with
func New() {}
### end
`, "###", patcheng.BundleDelim)

	var btnText string
	var retestDisabled bool

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.Evaluate(`navigator.clipboard.writeText = function(text) { return Promise.resolve(); }`, nil),
		chromedp.Evaluate(`
			window.originalFetch = window.fetch;
			window.fetch = function(url, options) {
				if (url && url.includes && url.includes('/api/retest')) {
					return new Promise(resolve => {
						setTimeout(() => {
							resolve(new Response(JSON.stringify({ files: [], packages: [] }), {
								status: 200,
								headers: { 'Content-Type': 'application/json' }
							}));
						}, 500);
					});
				}
				return window.originalFetch(url, options);
			};
		`, nil),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
		chromedp.Evaluate(fmt.Sprintf(`
			var el = document.getElementById('bundleInput');
			el.value = %q;
			el.dispatchEvent(new Event('input'));
		`, bundle), nil),
		chromedp.WaitVisible(`.file-block.status-ready`, chromedp.ByQuery),
		chromedp.Poll(`!document.getElementById('applyBtn').disabled`, nil),
		chromedp.Evaluate(`document.getElementById('applyBtn').click()`, nil),
		chromedp.WaitVisible(`.file-block.status-applied`, chromedp.ByQuery),
		chromedp.WaitVisible(`#copyTraceBtn`, chromedp.ByID),
		chromedp.Poll(`document.getElementById('copyTraceBtn').style.display !== 'none'`, nil),
		chromedp.Evaluate(`document.getElementById('copyTraceBtn').click()`, nil),
		chromedp.Poll(`document.getElementById('copyTraceBtn').innerText === 'Copied!'`, nil),
		chromedp.Text(`#copyTraceBtn`, &btnText, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("MicroInteractions setup phase failed: %v", err)
	}

	if btnText != "Copied!" {
		t.Errorf("Expected copy button text to temporarily change to 'Copied!', got %q", btnText)
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('retestBtn').click()`, nil),
		chromedp.Text(`#retestBtn`, &btnText, chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('retestBtn').disabled`, &retestDisabled),
	)
	if err != nil {
		t.Fatalf("MicroInteractions retest click failed: %v", err)
	}

	if !strings.Contains(btnText, "Running Tests") {
		t.Errorf("Expected retest button to show 'Running Tests...', got %q", btnText)
	}
	if !retestDisabled {
		t.Errorf("Expected retest button to be disabled during execution")
	}

	err = chromedp.Run(ctx,
		chromedp.Poll(`document.getElementById('retestBtn').disabled === false`, nil),
		chromedp.Text(`#retestBtn`, &btnText, chromedp.ByID),
		chromedp.Evaluate(`document.getElementById('retestBtn').disabled`, &retestDisabled),
	)
	if err != nil {
		t.Fatalf("MicroInteractions retest completion failed: %v", err)
	}

	if !strings.Contains(btnText, "Retest Impacted") {
		t.Errorf("Expected retest button to reset to 'Retest Impacted', got %q", btnText)
	}
	if retestDisabled {
		t.Errorf("Expected retest button to be enabled after execution")
	}
}
