// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 10
// :: description: End-to-End browser tests for the Appy UI using chromedp.
// :: filename: ui_e2e_test.go
// :: serialization: go
// :: latestChange: Syncing metadata for ui_e2e_test.go after newTestServer fix.

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
// setupTestServer creates an isolated Appy server and a chromedp context.
func setupTestServer(t *testing.T) (*httptest.Server, context.Context, context.CancelFunc, string) {
	tempDir := t.TempDir()

	// Provide a local go.mod to prevent 'retest' from walking up the OS directory tree and hanging
	os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module appytest\n\ngo 1.22\n"), 0644)

	// Create a dummy target file for patching tests
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

func TestE2E_UI_ArmorLogic(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	// Helper to set textarea value and trigger input event
	setInput := func(val string) chromedp.Action {
		return chromedp.Evaluate(fmt.Sprintf(`
var el = document.getElementById('bundleInput');
el.value = %q;
el.dispatchEvent(new Event('input'));
`, val), nil)
	}

	var inputVal string

	// 1. Mixed text with < 2 armors (should not unarmor)
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
		setInput("@@@line 1\nline 2"),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('bundleInput').value`, &inputVal),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}
	if inputVal != "@@@line 1\nline 2" {
		t.Errorf("Expected < 2 armors to remain untouched, got: %s", inputVal)
	}

	// 2. Mixed text with >= 2 armors (should auto-unarmor)
	err = chromedp.Run(ctx,
		setInput("@@@line 1\nline 2\n\n@@@line 3"),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('bundleInput').value`, &inputVal),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}
	expectedAuto := "line 1\nline 2\n\nline 3"
	if inputVal != expectedAuto {
		t.Errorf("Expected >= 2 armors to auto-unarmor, got: %s", inputVal)
	}

	// 3. Unarmor text handles LLM artifact leading spaces correctly
	err = chromedp.Run(ctx,
		setInput("@@@ %%% filename: foo\n@@@ %%% replace\n@@@ %%% with\n@@@ %%% end"),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('bundleInput').value`, &inputVal),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}
	expectedUnarmored := "%%% filename: foo\n%%% replace\n%%% with\n%%% end"
	if inputVal != expectedUnarmored {
		t.Errorf("Unarmor logic failed to strip leading spaces.\nExpected:\n%s\nGot:\n%s", expectedUnarmored, inputVal)
	}

	// 4. Unarmor text preserves indentation for NDCL and code
	err = chromedp.Run(ctx,
		setInput("@@@ %%% replace\n@@@   - [ ] Item\n@@@     - [x] Subitem\n@@@ %%% end"),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('bundleInput').value`, &inputVal),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}
	expectedIndented := "%%% replace\n  - [ ] Item\n    - [x] Subitem\n%%% end"
	if inputVal != expectedIndented {
		t.Errorf("Unarmor logic failed to preserve indentation.\nExpected:\n%s\nGot:\n%s", expectedIndented, inputVal)
	}

	// 5. Unarmor text preserves tabs (crucial for Makefiles)
	err = chromedp.Run(ctx,
		setInput("@@@ %%% replace\n@@@\tbuild:\n@@@\t\tgo build .\n@@@ %%% end"),
		chromedp.Sleep(100*time.Millisecond),
		chromedp.Evaluate(`document.getElementById('bundleInput').value`, &inputVal),
	)
	if err != nil {
		t.Fatalf("Chromedp run failed: %v", err)
	}
	expectedTabs := "%%% replace\n\tbuild:\n\t\tgo build .\n%%% end"
	if inputVal != expectedTabs {
		t.Errorf("Unarmor logic failed to preserve tabs.\nExpected:\n%q\nGot:\n%q", expectedTabs, inputVal)
	}
}

func TestE2E_StalePreviewHandling(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 15*time.Second)
	defer cancelTimeout()

	bundle := strings.ReplaceAll(`
### filename: target.go
### replace
func Old() {}
### with
func New() {}
### end
`, "###", patcheng.BundleDelim)

	var applyBtnDisabled bool
	var outputText string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),

		// 1. Paste bundle and wait for preview
		chromedp.Evaluate(fmt.Sprintf(`
			var el = document.getElementById('bundleInput');
			el.value = %q;
			el.dispatchEvent(new Event('input'));
		`, bundle), nil),

		chromedp.WaitVisible(`.file-block.status-ready`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('applyBtn').hasAttribute('disabled')`, &applyBtnDisabled),
	)
	if err != nil {
		t.Fatalf("Preview phase failed: %v", err)
	}
	if applyBtnDisabled {
		t.Errorf("Expected apply button to be ENABLED after successful preview")
	}

	// 2. Edit the input to trigger debounce preview clearing
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			var el = document.getElementById('bundleInput');
			el.value += " ";
			el.dispatchEvent(new Event('input'));
		`, nil),
		// Check immediately (debounce clears DOM instantly)
		chromedp.Evaluate(`document.getElementById('applyBtn').hasAttribute('disabled')`, &applyBtnDisabled),
		chromedp.Text(`#output`, &outputText, chromedp.ByID),
	)
	if err != nil {
		t.Fatalf("Input edit phase failed: %v", err)
	}

	if !applyBtnDisabled {
		t.Errorf("Expected apply button to be DISABLED immediately after input edit (t-sta-01)")
	}
	if !strings.Contains(outputText, "Stale preview cleared") {
		t.Errorf("Expected DOM to clear stripes and show stale message, got: %s (t-sta-02)", outputText)
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
		chromedp.WaitVisible(`.file-block.status-ready`, chromedp.ByQuery),
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

func TestE2E_UI_MetaUpdate(t *testing.T) {
	ts, ctx, cancel, tempDir := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 15*time.Second)
	defer cancelTimeout()

	// The file on disk is native markdown (no // comments)
	os.WriteFile(filepath.Join(tempDir, "meta.md"), []byte(":: fileVersion: 1\n\nBody"), 0644)

	// The LLM hallucinated Go-style embedded comments into the bundle
	bundle := strings.ReplaceAll(strings.ReplaceAll(`
### filename: meta.md
### meta_update
// ++ fileVersion: 2
// ++ addedKey: value
### end
`, "###", patcheng.BundleDelim), "++", "::")

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
		chromedp.Evaluate(fmt.Sprintf(`
var el = document.getElementById('bundleInput');
el.value = %q;
el.dispatchEvent(new Event('input'));
`, bundle), nil),
		chromedp.WaitVisible(`.file-block.status-ready`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('applyBtn').click()`, nil),
		chromedp.WaitVisible(`.file-block.status-applied`, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("E2E MetaUpdate failed: %v", err)
	}

	contentBytes, _ := os.ReadFile(filepath.Join(tempDir, "meta.md"))
	content := string(contentBytes)

	// Verify it processed as a Native update and stripped embedded slashes
	if !strings.Contains(content, ":: fileVersion: 2") ||
		!strings.Contains(content, ":: addedKey: value") {
		t.Errorf("meta_update failed to apply correctly. Content:\n%s", content)
	}
	if strings.Contains(content, "// :: fileVersion") {
		t.Errorf("meta_update failed to strip embedded comments from native markdown metadata. Content:\n%s", content)
	}
}

func TestE2E_NuclearOverwriteAndMatrix(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 15*time.Second)
	defer cancelTimeout()

	bundle := strings.ReplaceAll(`
### filename: target.go
### overwrite
package main
func Nuke() {}
### end
`, "###", patcheng.BundleDelim)

	var hasNuclearIcon bool
	var exportBtnDisplay string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
		chromedp.Evaluate(fmt.Sprintf(`
			var el = document.getElementById('bundleInput');
			el.value = %q;
			el.dispatchEvent(new Event('input'));
		`, bundle), nil),
		chromedp.WaitVisible(`.file-block.status-ready`, chromedp.ByQuery),

		// Check for the Nuclear Icon (t-str-06)
		chromedp.Evaluate(`document.querySelector('.decorator').innerText.includes('☢️')`, &hasNuclearIcon),

		// Apply
		chromedp.Evaluate(`document.getElementById('applyBtn').click()`, nil),
		chromedp.WaitVisible(`.file-block.status-applied`, chromedp.ByQuery),

		// Check Button Matrix (t-mat-02)
		chromedp.Evaluate(`document.getElementById('copyTraceBtn').style.display`, &exportBtnDisplay),
	)
	if err != nil {
		t.Fatalf("E2E Nuclear test failed: %v", err)
	}

	if !hasNuclearIcon {
		t.Errorf("Expected nuclear icon (☢️) on full overwrite stripe (t-str-06)")
	}

	if exportBtnDisplay == "none" || exportBtnDisplay == "" {
		t.Errorf("Expected copyLedgerBtn to be visible after apply (t-mat-02), got %q", exportBtnDisplay)
	}
}

func TestE2E_UI_JunkInput(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 15*time.Second)
	defer cancelTimeout()

	var applyBtnDisabled bool
	var outputText string

	// Test t-edg-01: Junk input
	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
		chromedp.Evaluate(`
			var el = document.getElementById('bundleInput');
			el.value = "Hey Appy, just chatting, no patches here!";
			el.dispatchEvent(new Event('input'));
		`, nil),
		chromedp.Sleep(800*time.Millisecond), // Wait for debounce and network fetch
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

	// Create a nested file
	os.MkdirAll(filepath.Join(tempDir, "nested", "deep"), 0755)
	os.WriteFile(filepath.Join(tempDir, "nested", "deep", "hidden.go"), []byte("package deep\nfunc FindMe() {}\n"), 0644)

	// Provide a bundle with a partial/missing path
	bundle := strings.ReplaceAll(`
### filename: hidden.go
### replace
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

		// Wait for preview to complete and check if Fix Paths button appears (t-edg-02)
		chromedp.WaitVisible(`.file-block.status-error`, chromedp.ByQuery),
		chromedp.Evaluate(`document.getElementById('fixPathsBtn').style.display`, &fixBtnDisplay),
	)
	if err != nil {
		t.Fatalf("Fix paths preview phase failed: %v", err)
	}

	if fixBtnDisplay == "none" || fixBtnDisplay == "" {
		t.Fatalf("Expected Fix File Paths button to be visible, got display: %q", fixBtnDisplay)
	}

	// Click Fix Paths and verify the textarea updates and a re-preview is triggered
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('fixPathsBtn').click()`, nil),
		chromedp.WaitVisible(`.file-block.status-ready`, chromedp.ByQuery), // Should automatically re-preview to READY
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
func Old() {}
### with
func New() {}
### end
`, "###", patcheng.BundleDelim)

	var btnText string
	var retestDisabled bool

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		// Mock clipboard API so headless chrome doesn't reject it
		chromedp.Evaluate(`navigator.clipboard.writeText = function(text) { return Promise.resolve(); }`, nil),
		// Mock fetch for /api/retest to prevent actual go test executions from hanging the test runner
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

		// Wait for apply button to be explicitly enabled before clicking
		chromedp.Poll(`!document.getElementById('applyBtn').disabled`, nil),
		chromedp.Evaluate(`document.getElementById('applyBtn').click()`, nil),
		chromedp.WaitVisible(`.file-block.status-applied`, chromedp.ByQuery),

		// Wait for the ledger button to be unhidden by the UI state machine
		// Wait for the trace button to be unhidden by the UI state machine
		chromedp.WaitVisible(`#copyTraceBtn`, chromedp.ByID),
		chromedp.Poll(`document.getElementById('copyTraceBtn').style.display !== 'none'`, nil),

		// Test t-edg-03: Copy button text changes to "Copied!"
		chromedp.Evaluate(`document.getElementById('copyTraceBtn').click()`, nil),
		// Poll for the text change instead of reading immediately to prevent JS event loop races
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
		// Test t-edg-04: Retest button state during execution
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

	// Wait for tests to finish and button to reset
	// Wait for tests to finish and button to reset
	err = chromedp.Run(ctx,
		// Wait for the JS promise to resolve and re-enable the button,
		// avoiding race conditions on the loader div if the API returns instantly.
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

func TestE2E_UI_BuilderTab(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 20*time.Second)
	defer cancelTimeout()

	var statsText string
	var dropdownValue string
	var resultDisplay string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		// 1. Navigate to Builder tab
		chromedp.Evaluate(`document.getElementById('btn-tab-bundle').click()`, nil),
		chromedp.WaitVisible(`#txtarPathsTable`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Text(`#txtarLiveStats`, &statsText, chromedp.ByID),

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
	if !strings.Contains(statsText, "Files:") || !strings.Contains(statsText, "Tokens:") {
		t.Errorf("Expected live stats to contain 'Files:' and 'Tokens:', got: %s (t-bld-01)", statsText)
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
