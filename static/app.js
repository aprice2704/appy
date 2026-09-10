const bt = String.fromCharCode(96);
const tbt = bt + bt + bt;

let previewTimeout;
window.tracePayload = "";
let configSets = {};
let txtarStatsTimeout;
window.pendingBuilderPathFixes = null;
window.pendingPathFixes = null;

function switchTab(tabId) {
    document.querySelectorAll('.tab-btn').forEach(btn => btn.classList.remove('active'));
    document.getElementById('btn-' + tabId).classList.add('active');

    document.querySelectorAll('.tab-pane').forEach(pane => {
        pane.classList.remove('active');
        pane.style.display = 'none';
    });
    const activePane = document.getElementById(tabId);
    activePane.classList.add('active');
    activePane.style.display = tabId === 'tab-history' ? 'block' : 'flex';
    if (tabId === 'tab-history') {
        loadHistory();
    }
}

async function clearAndPaste() {
    const inputEl = document.getElementById('bundleInput');
    const outputEl = document.getElementById('output');
    inputEl.value = "";
    try {
        if (!navigator.clipboard || !navigator.clipboard.readText) {
            throw new Error("Clipboard API blocked by browser (requires localhost or HTTPS)");
        }
        const text = await navigator.clipboard.readText();
        inputEl.value = text;
        syncUIState();
        sendRequest('/api/preview');
    } catch (err) {
        console.error("Clipboard access denied:", err);
        outputEl.innerHTML = `<div class='error' style='padding: 15px; background: rgba(239, 68, 68, 0.1); border: 1px solid #ef4444; border-radius: 4px; margin-top: 10px;'><strong>Clipboard read failed:</strong> ${err.message}.<br><br>Please click inside the text box above and press <strong>Ctrl+V</strong> to paste manually.</div>`;
        inputEl.focus();
    }
}

function syncUIState() {
    const inputEl = document.getElementById('bundleInput');
    const checkBtn = document.getElementById('checkBtn');
    const hasContent = inputEl.value.trim().length > 0;
    checkBtn.disabled = !hasContent;

    const lines = inputEl.value.split('\n');
    let armorCount = 0;
    for (let i = 0; i < lines.length; i++) {
        if (lines[i].trim().startsWith('@@@')) {
            armorCount++;
        }
    }
    if (armorCount >= 2) {
        inputEl.value = inputEl.value.replace(/^@@@[ \u00A0]?/gm, '');
    }
}

function debouncePreview() {
    const outputEl = document.getElementById('output');
    const applyBtn = document.getElementById('applyBtn');
    const checkBtn = document.getElementById('checkBtn');
    const retestBtn = document.getElementById('retestBtn');
    const fixPathsBtn = document.getElementById('fixPathsBtn');

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

function setExportMode(label, severity) {
    const copyTraceBtn = document.getElementById('copyTraceBtn');
    if (label === 'none') {
        copyTraceBtn.style.display = 'none';
        return;
    }
    copyTraceBtn.style.display = 'inline-block';
    copyTraceBtn.className = '';
    if (severity === 'success') {
        copyTraceBtn.classList.add('trace-blue');
        copyTraceBtn.innerText = "✅ Copy " + label + " Report";
    } else if (severity === 'mixed') {
        copyTraceBtn.classList.add('trace-purple');
        copyTraceBtn.innerText = "⚠️ Copy " + label + " Report";
    } else if (severity === 'error') {
        copyTraceBtn.classList.add('trace-red');
        copyTraceBtn.innerText = "❌ Copy " + label + " Errors";
    }
}

async function copyTraceReport() {
    const copyTraceBtn = document.getElementById('copyTraceBtn');
    try {
        await navigator.clipboard.writeText(window.tracePayload || "No data available.");
        const originalText = copyTraceBtn.innerText;
        copyTraceBtn.innerText = "Copied!";
        setTimeout(() => copyTraceBtn.innerText = originalText, 2000);
    } catch (err) {
        console.error("Failed to copy:", err);
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

// Terminal / Clipboard Bash Runner
let lastBashOutput = "";

async function runBashFromClipboard() {
    const btn = document.getElementById('runBashClipBtn');
    let cmd = "";
    try {
        if (!navigator.clipboard || !navigator.clipboard.readText) {
            throw new Error("Clipboard access not available.");
        }
        cmd = await navigator.clipboard.readText();
    } catch (err) {
        alert("Clipboard read failed: " + err.message);
        return;
    }

    cmd = cmd.trim();
    if (cmd.startsWith('$ ')) {
        cmd = cmd.substring(2).trim();
    } else if (cmd.startsWith('% ')) {
        cmd = cmd.substring(2).trim();
    }

    if (!cmd) {
        alert("Clipboard is empty.");
        return;
    }

    btn.innerText = "⚡ Running...";
    btn.disabled = true;

    openBashDrawer();
    const outEl = document.getElementById('bashConsoleOutput');
    const statusChip = document.getElementById('bashStatusChip');
    const durEl = document.getElementById('bashDuration');
    const noticeEl = document.getElementById('bashClipNotice');

    outEl.innerText = "$ " + cmd + "\n\n⏳ Running command in " + (window.AppyRootDir || 'sandbox') + "...\n";
    statusChip.innerText = "RUNNING";
    statusChip.style.background = "rgba(59, 130, 246, 0.2)";
    statusChip.style.color = "#60a5fa";
    noticeEl.style.display = "none";
    durEl.innerText = "";

    try {
        const res = await fetch('/api/exec_bash', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ command: cmd, timeout: 120 })
        });
        const data = await res.json();

        const output = data.output || (data.error ? ("Error: " + data.error) : "No output produced.");
        lastBashOutput = output;
        outEl.innerText = "$ " + cmd + "\n\n" + output;
        durEl.innerText = `(${data.duration || ''})`;

        if (data.exit_code === 0) {
            statusChip.innerText = "EXIT 0";
            statusChip.style.background = "rgba(34, 197, 94, 0.2)";
            statusChip.style.color = "#4ade80";
        } else {
            statusChip.innerText = `EXIT ${data.exit_code}`;
            statusChip.style.background = "rgba(239, 68, 68, 0.2)";
            statusChip.style.color = "#f87171";
        }

        // Auto-copy output back into clipboard
        try {
            await navigator.clipboard.writeText(output);
            noticeEl.style.display = "inline";
        } catch (copyErr) {
            console.warn("Auto-copy output failed:", copyErr);
        }
    } catch (err) {
        outEl.innerText = "$ " + cmd + "\n\nError executing command: " + err.message;
        statusChip.innerText = "ERROR";
        statusChip.style.background = "rgba(239, 68, 68, 0.2)";
        statusChip.style.color = "#f87171";
    } finally {
        btn.innerText = "⚡ Run Clip";
        btn.disabled = false;
    }
}

function openBashDrawer() {
   const drawer = document.getElementById('bashDrawer');
   if (drawer) drawer.style.display = 'flex';
}

function closeBashDrawer() {
   const drawer = document.getElementById('bashDrawer');
   if (drawer) drawer.style.display = 'none';
}

async function copyBashOutput() {
   if (!lastBashOutput) return;
   try {
       await navigator.clipboard.writeText(lastBashOutput);
       const noticeEl = document.getElementById('bashClipNotice');
       if (noticeEl) {
           noticeEl.style.display = "inline";
           noticeEl.innerText = "✓ Copied!";
           setTimeout(() => { noticeEl.innerText = "✓ Output copied to clipboard!"; }, 2000);
       }
   } catch (e) {
       alert("Copy failed: " + e.message);
   }
}