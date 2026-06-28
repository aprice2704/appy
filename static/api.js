async function checkSyntax() {
    const checkBtn = document.getElementById('checkBtn');
    const applyBtn = document.getElementById('applyBtn');
    checkBtn.innerText = "⏳ Checking...";
    applyBtn.disabled = true;
    checkBtn.disabled = true;
    sendRequest('/api/apply', false, true);
}

async function applyBundle() {
    const applyBtn = document.getElementById('applyBtn');
    const checkBtn = document.getElementById('checkBtn');
    applyBtn.innerText = "⏳ Applying...";
    applyBtn.disabled = true;
    checkBtn.disabled = true;
    sendRequest('/api/apply', true, false);
}

async function sendRequest(endpoint, skipCompiler = false, checkOnly = false) {
    const inputEl = document.getElementById('bundleInput');
    const outputEl = document.getElementById('output');
    const applyBtn = document.getElementById('applyBtn');
    const checkBtn = document.getElementById('checkBtn');
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
            checkBtn.innerText = "🧪 Check";
            applyBtn.innerText = "🚀 Apply";
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
        checkBtn.innerText = "🧪 Check";
        applyBtn.innerText = "🚀 Apply";
    }
}

function fixFilePaths() {
    const inputEl = document.getElementById('bundleInput');
    const fixPathsBtn = document.getElementById('fixPathsBtn');
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

async function forgetStripe(event, path) {
    event.preventDefault();
    event.stopPropagation();
    if (!confirm("Remove this file's patches from the applied ledger? This will allow you to re-apply them.")) {
        return;
    }
    const inputEl = document.getElementById('bundleInput');
    const bundle = inputEl.value;
    try {
        const res = await fetch('/api/forget', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ bundle: bundle, path: path })
        });
        const data = await res.json();
        if (data.error) {
            alert("Failed to reset: " + data.error);
        } else {
            debouncePreview();
        }
    } catch (err) {
        alert("Error resetting patch: " + err.message);
    }
}

async function runAutoPilot() {
    const inputEl = document.getElementById('bundleInput');
    const autoBtn = document.getElementById('autoBtn');
    const applyBtn = document.getElementById('applyBtn');

    autoBtn.disabled = true;
    autoBtn.innerText = "🤖 Auto...";
    try {
        if (!navigator.clipboard || !navigator.clipboard.readText) {
            throw new Error("Clipboard API not available. Auto-pilot requires clipboard read permissions.");
        }
        inputEl.value = await navigator.clipboard.readText();
        syncUIState();

        inputEl.value = inputEl.value.replace(/^@@@[ \u00A0]?/gm, '');
        syncUIState();
        if (!inputEl.value.trim()) throw new Error("Clipboard empty");
        const previewRes = await fetch('/api/preview', {
            method: 'POST', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ bundle: inputEl.value, skip_compiler: false, check_only: false })
        });
        const previewData = await previewRes.json();
        renderPreview(previewData);

        if (applyBtn.disabled) throw new Error("Auto-Pilot halted: Preview yielded errors or no ready files.");
        const applyRes = await fetch('/api/apply', {
            method: 'POST', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ bundle: inputEl.value, skip_compiler: false, check_only: false })
        });
        const applyData = await applyRes.json();
        renderResult(applyData, false);

        if (!window.committedFiles || window.committedFiles.length === 0 || applyData.files.some(f => !f.applied)) {
            throw new Error("Auto-Pilot halted: Errors occurred during disk application.");
        }

        await runRetest();
    } catch (err) {
        console.warn(err);
        alert(err.message);
    } finally {
        autoBtn.disabled = false;
        autoBtn.innerText = "🤖 Auto";
    }
}