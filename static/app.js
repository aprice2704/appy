const bt = String.fromCharCode(96);
const tbt = bt + bt + bt;

let previewTimeout;
window.tracePayload = "";
let configSets = {};

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

function escapeHtml(unsafe) {
   if (!unsafe) return "";
   return unsafe.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

async function fetchSets() {
   try {
       const res = await fetch('/api/sets');
       configSets = await res.json();
       updateSetDropdown();
   } catch (err) {
       console.error("Failed to fetch config sets", err);
   }
}

function updateSetDropdown() {
   const select = document.getElementById('setSelect');
   const currentVal = select.value;
   select.innerHTML = '<option value="">-- Default Scratchpad --</option>';
   for (const name in configSets) {
       const opt = document.createElement('option');
       opt.value = name;
       opt.textContent = name;
       select.appendChild(opt);
   }
   if (configSets[currentVal]) {
       select.value = currentVal;
   } else {
       select.value = '';
   }
}

function loadSelectedSet() {
   const name = document.getElementById('setSelect').value;
   if (!name) {
       loadTxtarState();
       return;
   }
   const set = configSets[name];
   if (set) {
       document.getElementById('txtarPaths').value = set.paths ? set.paths.join('\n') : '';
       document.getElementById('txtarExcludes').value = set.excludes ? set.excludes.join('\n') : '';
       document.getElementById('txtarAnchors').value = set.anchors ? set.anchors.join('\n') : '';
       document.getElementById('txtarPreface').value = set.preface || '';
       document.getElementById('txtarFilename').value = set.file_name || '';
       scheduleTxtarStatsUpdate();
   }
}

async function saveCurrentSet() {
   let name = document.getElementById('setSelect').value;
   const newName = document.getElementById('newSetName').value.trim();
   if (newName) {
       name = newName;
   }
   if (!name) {
       alert("Please select or enter a name for the configuration set.");
       return;
   }
   const payload = {
       paths: document.getElementById('txtarPaths').value.split('\n').map(l=>l.trim()).filter(l=>l),
       excludes: document.getElementById('txtarExcludes').value.split('\n').map(l=>l.trim()).filter(l=>l),
       anchors: document.getElementById('txtarAnchors').value.split('\n').map(l=>l.trim()).filter(l=>l),
       preface: document.getElementById('txtarPreface').value,
       file_name: document.getElementById('txtarFilename').value.trim()
   };
   configSets[name] = payload;
   try {
       await fetch('/api/sets', {
           method: 'POST',
           headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify(configSets)
       });
       document.getElementById('newSetName').value = '';
       updateSetDropdown();
       document.getElementById('setSelect').value = name;
   } catch (err) {
       alert("Failed to save set: " + err.message);
   }
}

async function deleteCurrentSet() {
   const name = document.getElementById('setSelect').value;
   if (!name) return;
   if (!confirm("Delete configuration set '" + name + "'?")) return;
   delete configSets[name];
   try {
       await fetch('/api/sets', {
           method: 'POST',
           headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify(configSets)
       });
       updateSetDropdown();
       loadTxtarState();
   } catch (err) {
       alert("Failed to delete set: " + err.message);
   }
}

async function pasteTxtarCommand() {
  try {
      const text = await navigator.clipboard.readText();
      let cleaned = text.trim();
      
      cleaned = cleaned.replace(/^txtar\s+c\s+/i, '').replace(/^txtar\s+/i, '');
      cleaned = cleaned.replace(/>\s*[^\s]+$/, '');
      cleaned = cleaned.replace(/\\\r?\n/g, ' ');
      
      const paths = cleaned.split(/\s+/).map(p => p.trim()).filter(p => p.length > 0);
  
      const el = document.getElementById('txtarPaths');
      let existing = el.value.split('\n').map(l => l.trim()).filter(l => l.length > 0);
      
      paths.forEach(p => {
          if (!existing.includes(p)) existing.push(p);
      });
      
      el.value = existing.join('\n');
      saveTxtarState();
  } catch(err) {
      console.error("Paste failed", err);
      alert("Clipboard paste failed: " + err.message);
  }
}

function clearTxtarPaths() {
   document.getElementById('txtarPaths').value = '';
   saveTxtarState();
}

async function handleTxtarFileSelect(event, isDir) {
 const el = document.getElementById('txtarPaths');
 let lines = el.value.split('\n').map(l => l.trim()).filter(l => l.length > 0);
 if (isDir && event.target.files.length > 0) {
     let firstFile = event.target.files[0];
     let pathStr = firstFile.path || firstFile.webkitRelativePath.split('/')[0];
     
     if (firstFile.path && firstFile.webkitRelativePath) {
         let relLen = firstFile.webkitRelativePath.length;
         let absDir = firstFile.path.substring(0, firstFile.path.length - relLen);
         pathStr = absDir + firstFile.webkitRelativePath.split('/')[0];
         pathStr = pathStr.replace(/\\/g, '/').replace(/\/\//g, '/');
     }
     
     let p = pathStr + "/**";
     if (pathStr && !lines.includes(p)) {
         lines.push(p);
     }
 } else {
     for (let file of event.target.files) {
         let p = file.path || file.name;
         if (!file.path) {
             try {
                 const res = await fetch('/api/resolve_path?name=' + encodeURIComponent(p));
                 const data = await res.json();
                 if (data.path) p = data.path;
             } catch (e) {
                 console.error("Path resolution failed", e);
             }
         }
         if (p && !lines.includes(p)) {
             lines.push(p);
         }
     }
 }
 el.value = lines.join('\n');
 saveTxtarState();
 event.target.value = '';
}

function saveTxtarState() {
  const root = window.AppyRootDir || 'default';
  if (!document.getElementById('setSelect').value) {
      localStorage.setItem('txtarPaths_' + root, document.getElementById('txtarPaths').value);
      localStorage.setItem('txtarExcludes_' + root, document.getElementById('txtarExcludes').value);
      localStorage.setItem('txtarAnchors_' + root, document.getElementById('txtarAnchors').value);
      localStorage.setItem('txtarPreface_' + root, document.getElementById('txtarPreface').value);
  }
  scheduleTxtarStatsUpdate();
}

function loadTxtarState() {
  const root = window.AppyRootDir || 'default';
  if (localStorage.getItem('txtarPaths_' + root) !== null) {
      document.getElementById('txtarPaths').value = localStorage.getItem('txtarPaths_' + root);
  }
  if (localStorage.getItem('txtarExcludes_' + root) !== null) {
      document.getElementById('txtarExcludes').value = localStorage.getItem('txtarExcludes_' + root);
  }
  if (localStorage.getItem('txtarAnchors_' + root) !== null) {
      document.getElementById('txtarAnchors').value = localStorage.getItem('txtarAnchors_' + root);
  }
  if (localStorage.getItem('txtarPreface_' + root) !== null) {
      document.getElementById('txtarPreface').value = localStorage.getItem('txtarPreface_' + root);
  }
  scheduleTxtarStatsUpdate();
}

let txtarStatsTimeout;
window.pendingBuilderPathFixes = null;

async function updateTxtarStats() {
  const paths = document.getElementById('txtarPaths').value.split('\n').map(l => l.trim()).filter(l => l.length > 0);
  const excludes = document.getElementById('txtarExcludes').value.split('\n').map(l => l.trim()).filter(l => l.length > 0);
  try {
      const res = await fetch('/api/txtar_stats', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ paths: paths, excludes: excludes })
      });
      const data = await res.json();
      if (!data.error) {
          const statsEl = document.getElementById('txtarLiveStats');
          if (statsEl) {
              statsEl.innerHTML = `<strong>Files:</strong> ${data.file_count} &nbsp;|&nbsp; <strong>Size:</strong> ${data.size_kb} KB &nbsp;|&nbsp; <strong>Tokens:</strong> ~${data.tokens_est}`;
          }
          const fixBtn = document.getElementById('builderFixPathsBtn');
          if (data.path_fixes && Object.keys(data.path_fixes).length > 0) {
              window.pendingBuilderPathFixes = data.path_fixes;
              if (fixBtn) fixBtn.style.display = 'inline-block';
          } else {
              window.pendingBuilderPathFixes = null;
              if (fixBtn) fixBtn.style.display = 'none';
          }
      }
  } catch (e) {
      console.error("Stats fetch failed", e);
  }
}

function scheduleTxtarStatsUpdate() {
  clearTimeout(txtarStatsTimeout);
  txtarStatsTimeout = setTimeout(updateTxtarStats, 300);
}

function fixBuilderPaths() {
   const inputEl = document.getElementById('txtarPaths');
   const fixBtn = document.getElementById('builderFixPathsBtn');
   if (!window.pendingBuilderPathFixes) return;

   let lines = inputEl.value.split('\n');
   let updated = false;
   for (let i = 0; i < lines.length; i++) {
       let p = lines[i].trim();
       if (window.pendingBuilderPathFixes[p]) {
           lines[i] = window.pendingBuilderPathFixes[p];
           updated = true;
       }
   }
   if (updated) {
       inputEl.value = lines.join('\n');
       saveTxtarState();
   }

   window.pendingBuilderPathFixes = null;
   if (fixBtn) fixBtn.style.display = 'none';
}

async function buildTxtar(overridePaths = null, overrideFilename = null) {
  const btn = document.getElementById('buildTxtarBtn');
  btn.innerText = "⏳ Building...";
  btn.disabled = true;
  const paths = overridePaths || document.getElementById('txtarPaths').value.split('\n').map(l => l.trim()).filter(l => l.length > 0);
  const excludes = document.getElementById('txtarExcludes').value.split('\n').map(l => l.trim()).filter(l => l.length > 0);
  const anchors = document.getElementById('txtarAnchors').value.split('\n').map(l => l.trim()).filter(l => l.length > 0);
  const preface = document.getElementById('txtarPreface').value;
  const filename = overrideFilename || (document.getElementById('txtarFilename') ? document.getElementById('txtarFilename').value.trim() : "");

  if (!overridePaths) saveTxtarState();
  try {
      const res = await fetch('/api/txtar', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ paths: paths, excludes: excludes, anchors: anchors, preface: preface, file_name: filename })
      });
      const data = await res.json();

      if (data.error) {
          alert("Build failed: " + data.error);
          return;
      }

      const resDiv = document.getElementById('txtarResult');
      resDiv.style.display = 'flex';
      document.getElementById('txtarResultStats').innerText = `Bundled ${data.file_count} files into ${data.file_name}`;

      const downloadUrl = data.file_url;
      const absUrl = window.location.origin + downloadUrl;
      const link = document.getElementById('txtarDownloadLink');
      link.href = downloadUrl;
      link.download = data.file_name;
      link.ondragstart = (e) => {
          e.dataTransfer.setData('DownloadURL', 'application/octet-stream:' + data.file_name + ':' + absUrl);
      };

  } catch (err) {
      alert("Network error: " + err.message);
  } finally {
      btn.innerText = "Build Txtar";
      btn.disabled = false;
  }
}

async function loadHistory() {
   const histEl = document.getElementById('tab-history');
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
   } catch (err) {
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
