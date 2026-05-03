// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 2
// :: description: v1.5.14 - Updated control plane buttons to match spec.
// :: filename: code/cmd/appy/ui_html.go
// :: serialization: go

package main

const htmlTop = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
        <link rel="icon" href='data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">🚀</text></svg>'>
    <title>{TITLE} | Appy</title>
    <style>`

const htmlMiddle = `</style>
</head>
<body>
    <div class="container">
        <h2 style="margin: 0;">  {TITLE} | Appy <span style="font-size: 0.6em; color: #888; font-weight: normal;">{VERSION}</span></h2>
        <textarea id="bundleInput" placeholder="Paste the raw LLM output here (including the markdown code blocks and %%% syntax)..." oninput="debouncePreview()"></textarea>
        
        <div class="controls">
            <button id="clearPasteBtn" onclick="clearAndPaste()" style="background: #4caf50; color: #fff;">Clear & Paste</button>
            <button id="unarmorBtn" onclick="unarmorText()">Remove @@@</button>
            <button id="checkBtn" onclick="checkSyntax()" disabled style="background: #ff9800; color: #fff;">Check Compiler</button>
            <button id="applyBtn" onclick="applyBundle()" disabled>Apply to Disk</button>
            <button id="fixPathsBtn" onclick="fixFilePaths()" style="background: #2196f3; color: #fff; display: none;">Fix File Paths</button>
            <button id="copyLedgerBtn" onclick="copyResultLedger()" style="display: none;">Copy Result Ledger</button>
            <button id="retestBtn" onclick="runRetest()" style="background: #007acc; color: #fff; display: none; font-weight: bold;">Retest Impacted</button>
            <button id="cancelRetestBtn" onclick="cancelRetest()" style="background: #f44336; color: #fff; display: none; font-weight: bold;">Stop Tests</button>
        </div>
        
        <div id="output" class="output">
            <em>Waiting for input...</em>
        </div>
    </div>
    <script>`

const htmlBottom = `</script>
</body>
</html>`
