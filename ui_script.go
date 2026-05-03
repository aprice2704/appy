// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 9
// :: description: Core JS logic and state management for Appy UI.
// :: filename: code/cmd/appy/ui_script.go
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
const retestBtn = document.getElementById('retestBtn');
let previewTimeout;
window.lastLedgerText = "";
window.lastErrorText = "";

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
    inputEl.value = inputEl.value.replace(/^@@@/gm, '');
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
            renderResult(data, checkOnly);
        } else {
            renderPreview(data);
        }
    } catch (err) {
        outputEl.innerHTML = "<div class='error'>Error: " + err.message + "</div>";
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

async function copyPreviewErrors() {
    try {
        await navigator.clipboard.writeText(window.lastErrorText || "No errors to report.");
        const originalText = copyErrorsBtn.innerText;
        copyErrorsBtn.innerText = "Copied!";
        setTimeout(() => copyErrorsBtn.innerText = originalText, 2000);
    } catch (err) {
        console.error("Failed to copy errors:", err);
    }
}

function addDecorator(el, emoji) {
    let header = el.querySelector('summary > div');
    if (header) {
        let dec = header.querySelector('.decorator');
        if (!dec) {
            dec = document.createElement('span');
            dec.className = 'decorator';
            dec.style.marginRight = '8px';
            dec.style.fontSize = '1.2em';
            const badge = header.querySelector('.status-badge');
            header.insertBefore(dec, badge);
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
