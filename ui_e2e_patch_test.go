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

func TestE2E_UI_ArmorLogic(t *testing.T) {
	ts, ctx, cancel, _ := setupTestServer(t)
	defer ts.Close()
	defer cancel()

	ctx, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	setInput := func(val string) chromedp.Action {
		return chromedp.Evaluate(fmt.Sprintf(`
var el = document.getElementById('bundleInput');
el.value = %q;
el.dispatchEvent(new Event('input'));
`, val), nil)
	}

	var inputVal string

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
// line 1
// line 2
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

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			var el = document.getElementById('bundleInput');
			el.value += " ";
			el.dispatchEvent(new Event('input'));
		`, nil),
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

	var applyBtnDisabled bool
	var stripeClass string

	err := chromedp.Run(ctx,
		chromedp.Navigate(ts.URL),
		chromedp.WaitVisible(`#bundleInput`, chromedp.ByQuery),
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

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.getElementById('applyBtn').click()`, nil),
	)
	if err != nil {
		t.Fatalf("Failed to click applyBtn: %v", err)
	}

	err = chromedp.Run(ctx,
		chromedp.WaitVisible(`.file-block.status-applied`, chromedp.ByQuery),
		chromedp.AttributeValue(`#file-block-target\.go`, "class", &stripeClass, nil, chromedp.ByQuery),
	)
	if err != nil {
		var outputHTML string
		chromedp.Run(ctx, chromedp.OuterHTML(`#output`, &outputHTML, chromedp.ByID))
		t.Fatalf("Apply phase failed waiting for DOM update: %v\nOutput HTML state:\n%s", err, outputHTML)
	}

	if !strings.Contains(stripeClass, "status-applied") {
		t.Errorf("Expected stripe to have 'status-applied' class, got: %s", stripeClass)
	}

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

	os.WriteFile(filepath.Join(tempDir, "meta.md"), []byte(":: fileVersion: 1\n\nBody"), 0644)

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

	contentBytes, err := os.ReadFile(filepath.Join(tempDir, "meta.md"))
	if err != nil {
		t.Fatalf("failed reading meta.md: %v", err)
	}
	content := string(contentBytes)

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
		chromedp.Evaluate(`document.querySelector('.decorator').innerText.includes('☢️')`, &hasNuclearIcon),
		chromedp.Evaluate(`document.getElementById('applyBtn').click()`, nil),
		chromedp.WaitVisible(`.file-block.status-applied`, chromedp.ByQuery),
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
