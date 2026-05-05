// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 11
// :: description: Core JS logic and state management for Appy UI.
// :: filename: ui_script.go
// :: serialization: go

package main

const jsCore = `
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
const copyErrorsBtn = document.getElementById('copyErrorsBtn');
const copyTestReportBtn = document.getElementById('copyTestReportBtn');
const retestBtn = document.getElementById('retestBtn');
let previewTimeout;
window.tracePayload = ""; // Unified clipboard payload for current state

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
    
    const lines = inputEl.value.split('\n');
    let hasArmor = false;
    if (lines.length > 0) {
hasArmor = lines.every(line => {
const trimmed = line.trim();
// Tolerate markdown code fences so they don't break the armor check
if (trimmed.startsWith(tbt)) return true;
return trimmed === '' || trimmed.startsWith('@@@');
});
if (hasArmor) {
hasArmor = lines.some(line => line.trim().startsWith('@@@'));
}
}

    if (hasArmor) {
        unarmorBtn.classList.add('show');
        unarmorBtn.style.display = 'inline-block';
    } else {
        unarmorBtn.classList.remove('show');
        unarmorBtn.style.display = 'none';
    }
}

function unarmorText() {
    // Consume optional standard/non-breaking spaces left behind by LLMs
    inputEl.value = inputEl.value.replace(/^@@@[ \t\u00A0]*/gm, '');
    syncUIState();
    sendRequest('/api/preview');
}

function debouncePreview() {
    outputEl.innerHTML = "<em style='color: #64748b;'>Waiting for input... (Stale preview cleared)</em>";
    applyBtn.disabled = true;
    applyBtn.classList.remove('ready');
    checkBtn.disabled = true;
    setExportMode('none');
    retestBtn.style.display = 'none';
    fixPathsBtn.style.display = 'none';
    
    syncUIState();
    clearTimeout(previewTimeout);
    previewTimeout = setTimeout(() => {
        sendRequest('/api/preview');
    }, 500);
}

function setExportMode(mode) {
    copyErrorsBtn.style.display = mode === 'errors' ? 'inline-block' : 'none';
    copyLedgerBtn.style.display = mode === 'ledger' ? 'inline-block' : 'none';
    copyTestReportBtn.style.display = mode === 'test' ? 'inline-block' : 'none';
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
            window.tracePayload = "**Appy Server Error**\n\n" + data.error;
            setExportMode('errors');
            applyBtn.disabled = true;
            checkBtn.disabled = true;
            return;
        }

        if (endpoint === '/api/apply') {
            renderResult(data, checkOnly);
        } else {
            renderPreview(data);
        }
    } catch (err) {
        outputEl.innerHTML = "<div class='error'>Error: " + err.message + "</div>";
        window.tracePayload = "**Appy Network/Client Error**\n\n" + err.message;
        setExportMode('errors');
    }
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

async function copyTraceReport(mode) {
    try {
        await navigator.clipboard.writeText(window.tracePayload || "No data available.");
        let btn = copyLedgerBtn;
        if (mode === 'errors') btn = copyErrorsBtn;
        if (mode === 'test') btn = copyTestReportBtn;
        
        const originalText = btn.innerText;
        btn.innerText = "Copied!";
        setTimeout(() => btn.innerText = originalText, 2000);
    } catch (err) {
        console.error("Failed to copy:", err);
    }
}

async function retestImpacted() {
    const originalText = retestBtn.innerText;
    retestBtn.innerText = "Running Tests...";
    retestBtn.disabled = true;
    
    try {
        const res = await fetch('/api/retest', { method: 'POST' });
        const data = await res.json();
        renderRetest(data);
    } catch (err) {
        outputEl.innerHTML += "<div class='error'>Retest Error: " + err.message + "</div>";
    } finally {
        retestBtn.innerText = originalText;
        retestBtn.disabled = false;
    }
}

function addDecorator(el, emoji) {
    let rhs = el.querySelector('.rhs-chips');
    if (rhs) {
        let dec = rhs.querySelector('.decorator');
        if (!dec) {
            dec = document.createElement('span');
            dec.className = 'decorator';
            dec.style.fontSize = '1.2em';
            rhs.insertBefore(dec, rhs.firstChild);
        }
        if (!dec.innerText.includes(emoji)) {
            dec.innerText += emoji;
        }
    }
}

function escapeHtml(unsafe) {
    if (!unsafe) return "";
    return unsafe.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}
`
