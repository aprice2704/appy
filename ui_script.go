// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 12
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
const autoBtn = document.getElementById('autoBtn');
const clearPasteBtn = document.getElementById('clearPasteBtn');

const fixPathsBtn = document.getElementById('fixPathsBtn');
const historyBtn = document.getElementById('historyBtn');
const copyTraceBtn = document.getElementById('copyTraceBtn');
const retestBtn = document.getElementById('retestBtn');
let previewTimeout;
window.tracePayload = "";
// Unified clipboard payload for current state

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
  let armorCount = 0;
  for (let i = 0; i < lines.length; i++) {
      if (lines[i].trim().startsWith('@@@')) {
          armorCount++;
      }
  }
   if (armorCount >= 2) {
     // Auto-unarmor
     // Consume exactly ONE optional standard/non-breaking space left behind by LLMs
     // to preserve the structural indentation necessary for checklists and code.
     // CRITICAL: We do NOT consume \t here, because that destroys Makefile command indentation.
     inputEl.value = inputEl.value.replace(/^@@@[ \u00A0]?/gm, '');
 }
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

function setExportMode(label, severity) {
   if (label === 'none') {
       copyTraceBtn.style.display = 'none';
       return;
   }
   copyTraceBtn.style.display = 'inline-block';
   copyTraceBtn.className = '';
   if (severity === 'success') {
       copyTraceBtn.classList.add('trace-blue');
       copyTraceBtn.innerText = "✅ " + label + " Log";
   } else if (severity === 'mixed') {
       copyTraceBtn.classList.add('trace-purple');
       copyTraceBtn.innerText = "⚠️ " + label + " Log";
   } else if (severity === 'error') {
       copyTraceBtn.classList.add('trace-red');
       copyTraceBtn.innerText = "❌ " + label + " Errors";
   }
}

async function checkSyntax() {
   checkBtn.innerText = "⏳ Checking...";
   applyBtn.disabled = true;
   checkBtn.disabled = true;
   sendRequest('/api/apply', false, true);
}

async function applyBundle() {
   applyBtn.innerText = "⏳ Applying...";
   applyBtn.disabled = true;
   checkBtn.disabled = true;
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

async function copyTraceReport() {
   try {
       await navigator.clipboard.writeText(window.tracePayload || "No data available.");
       const originalText = copyTraceBtn.innerText;
       copyTraceBtn.innerText = "Copied!";
       setTimeout(() => copyTraceBtn.innerText = originalText, 2000);
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

async function runAutoPilot() {
   autoBtn.disabled = true;
     autoBtn.innerText = "🤖 Auto...";
   try {
       // 1. Paste
       inputEl.value = await navigator.clipboard.readText();
       syncUIState();
       
       // 2. Unarmor
             // 2. Unarmor (Handled automatically by syncUIState, but ensuring it here just in case)
      inputEl.value = inputEl.value.replace(/^@@@[ \u00A0]?/gm, '');
      syncUIState();
      
      if (!inputEl.value.trim()) throw new Error("Clipboard empty");
       // 3. Preview
       const previewRes = await fetch('/api/preview', {
           method: 'POST', headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify({ bundle: inputEl.value, skip_compiler: false, check_only: false })
       });
       const previewData = await previewRes.json();
       renderPreview(previewData);
       
       // Halt if preview found errors or had no ready files
       if (applyBtn.disabled) throw new Error("Auto-Pilot halted: Preview yielded errors or no ready files.");
       // 4. Apply
       const applyRes = await fetch('/api/apply', {
           method: 'POST', headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify({ bundle: inputEl.value, skip_compiler: false, check_only: false })
       });
       const applyData = await applyRes.json();
       renderResult(applyData, false);
       
       // Halt if apply failed on any file
       if (!window.committedFiles || window.committedFiles.length === 0 || applyData.files.some(f => !f.applied)) {
           throw new Error("Auto-Pilot halted: Errors occurred during disk application.");
       }
       
       // 5. Retest
       await retestImpacted();
   } catch (err) {
       console.warn(err);
   } finally {
       autoBtn.disabled = false;
             autoBtn.innerText = "🤖 Auto";
   }
}

function escapeHtml(unsafe) {
  if (!unsafe) return "";
  return unsafe.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

let showingHistory = false;
async function toggleHistory() {
   showingHistory = !showingHistory;
   if (showingHistory) {
       outputEl.style.display = 'none';
       document.getElementById('historyOutput').style.display = 'block';
       historyBtn.innerText = "🔙 Back";
       await loadHistory();
   } else {
       outputEl.style.display = 'block';
       document.getElementById('historyOutput').style.display = 'none';
       historyBtn.innerText = "⏪ History";
   }
}

async function loadHistory() {
   const histEl = document.getElementById('historyOutput');
   histEl.innerHTML = "<em>Loading history...</em>";
   try {
       const res = await fetch('/api/history');
       const data = await res.json();
       if (!data.history || data.history.length === 0) {
           histEl.innerHTML = "<em>No history available.</em>";
           return;
       }
       let html = "";
       data.history.forEach(tx => {
           const d = new Date(tx.timestamp * 1000).toLocaleString();
           html += '<div class="file-block status-applied" style="padding: 15px; margin-bottom: 10px;">';
           html += '<div style="font-weight: bold; margin-bottom: 8px;">' + escapeHtml(d) + ' <span style="font-size: 11px; color: #94a3b8; font-weight: normal;">(' + escapeHtml(tx.tx_id) + ')</span></div>';
           tx.files.forEach(f => {
               html += '<div style="font-family: monospace; font-size: 12px; color: #cbd5e1; margin-bottom: 4px;">';
               html += (f.existed ? '<span style="color: #fbbf24;">[MOD/DEL]</span> ' : '<span style="color: #4ade80;">[CREATE]</span> ') + escapeHtml(f.path);
               html += '</div>';
           });
           html += '<button onclick="revertTx(\'' + escapeHtml(tx.tx_id) + '\')" style="margin-top: 10px; height: 30px; min-width: 100px; background: #dc2626; color: white; border: 1px solid #ef4444; border-radius: 4px; cursor: pointer;">Revert This Patch</button>';
           html += '</div>';
       });
       histEl.innerHTML = html;
   } catch(err) {
       histEl.innerHTML = "<div class='error'>Failed to load history: " + err.message + "</div>";
   }
}

async function revertTx(txId) {
   if (!confirm("Are you sure you want to revert " + txId + "? This will restore original file contents and CANNOT be undone.")) return;
   try {
       const res = await fetch('/api/revert', {
           method: 'POST',
           headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify({ tx_id: txId })
       });
       const data = await res.json();
       if (data.error) {
           alert("Revert failed: " + data.error);
           return;
       }
       alert("Revert successful. Reloading history.");
       loadHistory();
   } catch (err) {
       alert("Revert failed: " + err.message);
   }
}
`
