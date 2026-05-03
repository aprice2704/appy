// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 7
// :: description: v1.5.14 - Refactored renderResult for DOM manipulation and RHS chip support.
// :: filename: code/cmd/appy/ui_script.go
// :: serialization: go

package main

const jsScript = `
const bt = String.fromCharCode(96);
const tbt = bt + bt + bt;

const inputEl = document.getElementById('bundleInput');
const outputEl = document.getElementById('output');
const applyBtn = document.getElementById('applyBtn');
const checkBtn = document.getElementById('checkBtn');
const clearPasteBtn = document.getElementById('clearPasteBtn');
const unarmorBtn = document.getElementById('unarmorBtn');
const fixPathsBtn = document.getElementById('fixPathsBtn');
const copyLedgerBtn = document.getElementById('copyLedgerBtn');
const retestBtn = document.getElementById('retestBtn');

let previewTimeout;
window.lastLedgerText = "";

async function clearAndPaste() {
    inputEl.value = "";
    try {
        const text = await navigator.clipboard.readText();
        inputEl.value = text;
        syncUIState();
        sendRequest('/api/preview');
    } catch (err) {
        console.error("Clipboard access denied:", err);
        outputEl.innerHTML = "<div class='error'>Clipboard access denied. Please paste manually (Ctrl+V).</div>";
    }
}

function syncUIState() {
    const hasContent = inputEl.value.trim().length > 0;
    checkBtn.disabled = !hasContent;
    
    // Check for @@@ armor
    if (inputEl.value.includes('@@@')) {
        unarmorBtn.classList.add('show');
        unarmorBtn.style.display = 'inline-block';
    } else {
        unarmorBtn.classList.remove('show');
        unarmorBtn.style.display = 'none';
    }
}

function unarmorText() {
    inputEl.value = inputEl.value.replace(/@@@/g, '');
    syncUIState();
    sendRequest('/api/preview');
}

function debouncePreview() {
    syncUIState();
    clearTimeout(previewTimeout);
    previewTimeout = setTimeout(() => {
        sendRequest('/api/preview');
    }, 400);
}

async function checkSyntax() {
    sendRequest('/api/apply', false, true); 
}

async function applyBundle() {
    sendRequest('/api/apply', true, false);
}

async function sendRequest(endpoint, skipCompiler = false, checkOnly = false) {
    const bundle = inputEl.value;
    if (!bundle.trim()) return;

    try {
        const res = await fetch(endpoint, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ bundle, skip_compiler: skipCompiler, check_only: checkOnly })
        });
                const data = await res.json();
        
        if (data.error) {
            outputEl.innerHTML = "<div class='error' style='margin-top:15px; padding: 15px; border: 1px solid #f44336; border-radius: 4px; background: rgba(244,67,54,0.1);'><strong>Server Error:</strong> " + escapeHtml(data.error) + "</div>";
            return;
        }

        if (endpoint === '/api/apply') {
            renderResult(data);
        } else {
            renderPreview(data);
        }
    } catch (err) {
        outputEl.innerHTML = "<div class='error'>Error: " + err.message + "</div>";
    }
}

function renderResult(data) {
    // 1. Build the Ledger Text (for the Copy Button)
    let ledger = "**Appy Result Ledger**\n\n";
    if (data.successful_files_committed && data.successful_files_committed.length > 0) {
        ledger += "Committed files:\n" + data.successful_files_committed.map(f => "- " + bt + f + bt).join('\n') + "\n\n";
    }
    if (data.rejected_files && Object.keys(data.rejected_files).length > 0) {
        ledger += "Rejected files:\n";
        for (const [file, details] of Object.entries(data.rejected_files)) {
            ledger += "- " + bt + file + bt + " (file_commit_status: rejected)\n";
            ledger += "  Issue: " + details.reason + "\n";
            if (details.failed_patch && details.failed_patch.current_line) {
                ledger += "  Current line echo: " + bt + details.failed_patch.current_line + bt + "\n";
            }
        }
    }
        window.lastLedgerText = ledger;
    window.committedFiles = data.successful_files_committed;

    // 2. DOM Manipulation (Modify stripes instead of scorching the earth)
    if (data.successful_files_committed) {
        data.successful_files_committed.forEach(f => {
            const el = document.getElementById('file-block-' + f);
            if (el) {
                el.className = 'file-block status-applied';
                const badge = el.querySelector('.status-badge');
                if (badge) {
                    badge.className = 'status-badge status-applied';
                    badge.innerText = 'APPLIED';
                }
            }
        });
    }

    if (data.rejected_files) {
        for (const [file, details] of Object.entries(data.rejected_files)) {
            const el = document.getElementById('file-block-' + file);
            if (el) {
                el.className = 'file-block status-error';
                const badge = el.querySelector('.status-badge');
                if (badge) {
                    badge.className = 'status-badge status-error';
                    badge.innerText = 'ERROR';
                }
                // Append the rejection reason inside the content block
                const content = el.querySelector('.file-content');
                if (content) {
                    let errHtml = '<div class="patch-block" style="border-top: 2px solid #f44336; padding-top: 10px;">';
                    errHtml += '<div class="error-msg"><strong>Rejected:</strong> ' + escapeHtml(details.reason) + '</div>';
                    if (details.failed_patch && details.failed_patch.current_line) {
                        errHtml += '<div class="hint-block"><strong>Matched Line Echo:</strong><pre>' + escapeHtml(details.failed_patch.current_line) + '</pre></div>';
                    }
                    errHtml += '</div>';
                    content.innerHTML += errHtml;
                }
            }
        }
    }

    // 3. Show Control Plane Buttons
    copyLedgerBtn.style.display = 'inline-block';
    if (data.successful_files_committed && data.successful_files_committed.length > 0) {
        retestBtn.style.display = 'inline-block';
    }
    
    // Disable Apply once handled
    applyBtn.disabled = true;
    applyBtn.classList.remove('ready');
}

function renderPreview(data) {
    if (!data.patches || Object.keys(data.patches).length === 0) {
        outputEl.innerHTML = "<em>No valid patches found in bundle.</em>";
        applyBtn.disabled = true;
        copyLedgerBtn.style.display = 'none';
        retestBtn.style.display = 'none';
        return;
    }

    let html = '';
    let readyCount = 0;
    
    for (const [file, patches] of Object.entries(data.patches)) {
        let fileHasError = patches.some(p => p.status === 'error');
        let fileHasIgnored = patches.some(p => p.status === 'ignored');
        
        if (!fileHasError && !fileHasIgnored) readyCount++;
        
        let statusClass = fileHasError ? 'status-error' : (fileHasIgnored ? 'status-ignored' : 'status-ok');
        let chipText = fileHasError ? 'ERROR' : (fileHasIgnored ? 'IGNORED' : 'OK');

        // Add ID for renderResult DOM targeting
        html += '<details id="file-block-' + escapeHtml(file) + '" class="file-block ' + statusClass + '" open>';
        
        // RHS Pinning via flex-space-between (handled in css .file-header)
                    // Fix for RHS pinning in <summary> which can break flexbox on some browsers
            html += '<summary class="file-header" style="display: flex; align-items: center;">';
            html += '<div style="flex: 1; display: flex; justify-content: space-between; align-items: center;">';
            html += '<strong>' + escapeHtml(file) + '</strong>';
            html += '<span class="status-badge ' + statusClass + '">' + chipText + '</span>';
            html += '</div></summary>';
            
            html += '<div class="file-content">';
            patches.forEach(p => {
                html += '<div class="patch-block">';
                
                // Header of patch block with Net Lines
                html += '<div style="display: flex; justify-content: space-between; margin-bottom: 8px;">';
                html += '<span class="status-badge status-' + p.status + '">' + p.status + '</span>';
                let delta = p.line_delta > 0 ? ('+' + p.line_delta) : p.line_delta;
                html += '<span style="font-size:11px; color:#888;">Net lines: ' + delta + '</span>';
                html += '</div>';

                if (p.message) html += '<div class="error-msg">' + escapeHtml(p.message) + '</div>';
                if (p.hint) html += '<div class="hint-block"><strong>Closest Match:</strong><pre>' + escapeHtml(p.hint) + '</pre></div>';
                if (p.advisory) html += '<div class="hint-block" style="color:#2196f3; border-left: 3px solid #2196f3;"><strong>Advisory:</strong><br>' + escapeHtml(p.advisory) + '</div>';
                
                // Diff style replacement preview
                if (p.replace !== undefined) {
                    html += '<div class="replace-block"><pre style="margin:0; white-space: pre-wrap; font-family: inherit;">' + escapeHtml(p.replace) + '</pre></div>';
                }
                
                html += '</div>';
            });
            html += '</div></details>';
        }
    
    outputEl.innerHTML = html;
    
    applyBtn.disabled = (readyCount === 0);
    if (readyCount > 0) applyBtn.classList.add('ready');
    else applyBtn.classList.remove('ready');

    if (data.path_fixes && Object.keys(data.path_fixes).length > 0) {
        fixPathsBtn.style.display = 'inline-block';
        window.pendingPathFixes = data.path_fixes;
    } else {
        fixPathsBtn.style.display = 'none';
    }
    
    // Hide post-action buttons during preview
    copyLedgerBtn.style.display = 'none';
    retestBtn.style.display = 'none';
}

function fixFilePaths() {
    if (!window.pendingPathFixes) return;
    let val = inputEl.value;
    for (const [oldPath, newPath] of Object.entries(window.pendingPathFixes)) {
        val = val.replace("filename: " + oldPath, "filename: " + newPath);
    }
    inputEl.value = val;
    window.pendingPathFixes = null;
    fixPathsBtn.style.display = 'none';
    debouncePreview();
}

async function copyResultLedger() {
    try {
        await navigator.clipboard.writeText(window.lastLedgerText || "No ledger available.");
        const originalText = copyLedgerBtn.innerText;
        copyLedgerBtn.innerText = "Copied!";
        setTimeout(() => copyLedgerBtn.innerText = originalText, 2000);
    } catch (err) {
        console.error("Failed to copy:", err);
    }
}

function escapeHtml(unsafe) {
    if (!unsafe) return "";
    return unsafe.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

async function runRetest() {
    if (!window.committedFiles || window.committedFiles.length === 0) return;
    
    // Extract unique package directories from the committed file paths
    const packages = [...new Set(window.committedFiles.map(f => {
        const parts = f.split('/');
        parts.pop();
        return "./" + (parts.length > 0 ? parts.join('/') : '.');
    }))];
    
    retestBtn.innerText = "Running Tests...";
    retestBtn.disabled = true;
    
    // Append a loading indicator without destroying the file blocks
    const loadingHtml = "<div id='testLoading' class='patch-block' style='margin-top: 15px; border-top: 1px solid #555; padding-top: 15px;'><em>Running tests for " + packages.join(', ') + " ...</em></div>";
    outputEl.innerHTML += loadingHtml;

    try {
        const res = await fetch('/api/retest', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ packages: packages })
        });
        const data = await res.json();
        
        const loader = document.getElementById('testLoading');
        if (loader) loader.style.display = 'none';
        
        let testOutput = "<div class='patch-block' style='margin-top: 15px; border-top: 1px solid #555; padding-top: 15px;'>";
        testOutput += "<strong>Test Execution Report</strong>";
        testOutput += "<pre style='background: #111; padding: 10px; border-radius: 4px; overflow-x: auto; font-family: monospace;'>" + escapeHtml(JSON.stringify(data, null, 2)) + "</pre>";
        testOutput += "</div>";
        
        outputEl.innerHTML += testOutput;
    } catch (err) {
        const loader = document.getElementById('testLoading');
        if (loader) loader.style.display = 'none';
        outputEl.innerHTML += "<div class='error' style='margin-top:15px;'>Test request failed: " + err.message + "</div>";
    } finally {
        retestBtn.innerText = "Retest Impacted";
        retestBtn.disabled = false;
    }
}

function cancelRetest() {
    // Note: Cancelling an in-flight server request requires AbortController & server support.
    // For now, this is just a UI stub for the Stop Tests button.
    console.log("Cancel retest not fully implemented on backend yet.");
}

// Initial state check
syncUIState();
`
