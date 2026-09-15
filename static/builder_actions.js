// Appy Builder: Stats, Build, Copy & Configuration Sets
function saveTxtarState() {
   const root = window.AppyRootDir || 'default';
   const isScratchpad = !document.getElementById('modeScratchpad') || document.getElementById('modeScratchpad').checked;
   if (isScratchpad) {
       if (typeof getTxtarPaths === 'function') {
           localStorage.setItem('txtarPaths_' + root, getTxtarPaths().join('\n'));
       }
       const exEl = document.getElementById('txtarExcludes');
       if (exEl) localStorage.setItem('txtarExcludes_' + root, exEl.value);
       const anEl = document.getElementById('txtarAnchors');
       if (anEl) localStorage.setItem('txtarAnchors_' + root, anEl.value);
       const prefEl = document.getElementById('txtarPreface');
       if (prefEl) localStorage.setItem('txtarPreface_' + root, prefEl.value);
   }
   if (typeof scheduleTxtarStatsUpdate === 'function') scheduleTxtarStatsUpdate();
}

function loadTxtarState() {
   const root = window.AppyRootDir || 'default';
   const pathsStr = localStorage.getItem('txtarPaths_' + root);
   if (pathsStr !== null && typeof setTxtarPaths === 'function') {
       setTxtarPaths(pathsStr.split('\n'));
   } else if (typeof setTxtarPaths === 'function') {
       setTxtarPaths(['.']);
   }

   const exStr = localStorage.getItem('txtarExcludes_' + root);
   const exEl = document.getElementById('txtarExcludes');
   if (exStr !== null && exEl) exEl.value = exStr;
   
   if (typeof syncExcludeTestsCheckbox === 'function') syncExcludeTestsCheckbox();
   
   const anStr = localStorage.getItem('txtarAnchors_' + root);
   const anEl = document.getElementById('txtarAnchors');
   if (anStr !== null && anEl) anEl.value = anStr;

   const prefStr = localStorage.getItem('txtarPreface_' + root);
   const prefEl = document.getElementById('txtarPreface');
   if (prefStr !== null && prefEl) prefEl.value = prefStr;
   
   if (typeof scheduleTxtarStatsUpdate === 'function') scheduleTxtarStatsUpdate();
}

async function updateTxtarStats() {
   const paths = typeof getTxtarPaths === 'function' ? getTxtarPaths() : [];
   const excludesEl = document.getElementById('txtarExcludes');
   const excludes = excludesEl ? excludesEl.value.split('\n').map(l => l.trim()).filter(l => l.length > 0) : [];
   try {
       const res = await fetch('/api/txtar_stats', {
           method: 'POST',
           headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify({ paths: paths, excludes: excludes })
       });
       const data = await res.json();

       if (!data.error) {
           const totFiles = document.getElementById('totFiles');
           const totTokens = document.getElementById('totTokens');
           if (totFiles) totFiles.innerText = data.file_count;
           if (totTokens) totTokens.innerText = Math.round(data.tokens_est / 1000);

           const fixBtn = document.getElementById('builderFixPathsBtn');
           if (data.path_fixes && Object.keys(data.path_fixes).length > 0) {
               window.pendingBuilderPathFixes = data.path_fixes;
               if (fixBtn) fixBtn.style.display = 'inline-block';
           } else {
               window.pendingBuilderPathFixes = null;
               if (fixBtn) fixBtn.style.display = 'none';
           }

           const rows = document.querySelectorAll('#txtarPathsBody tr');
           rows.forEach(tr => {
               const input = tr.querySelector('.path-input');
               if (!input) return;
               const val = input.value.trim();
               const statFiles = tr.querySelector('.stat-files');
               const statTokens = tr.querySelector('.stat-tokens');

               tr.className = '';
               if (val && data.path_statuses) {
                   const status = data.path_statuses[val];
                   if (status === 'valid') tr.classList.add('hl-valid');
                   else if (status === 'not_found') tr.classList.add('hl-missing');
                   else if (status === 'zero_matches') tr.classList.add('hl-empty');
               }

               if (val && data.path_stats && data.path_stats[val]) {
                   statFiles.innerText = data.path_stats[val].files;
                   statTokens.innerText = Math.round(data.path_stats[val].tokens / 1000);
               } else {
                   statFiles.innerText = '-';
                   statTokens.innerText = '-';
               }
           });
       }
   } catch (e) {
       console.error("Stats fetch failed", e);
   }
}

function scheduleTxtarStatsUpdate() {
   clearTimeout(window.txtarStatsTimeout);
   window.txtarStatsTimeout = setTimeout(updateTxtarStats, 300);
}

function fixBuilderPaths() {
   const fixBtn = document.getElementById('builderFixPathsBtn');
   if (!window.pendingBuilderPathFixes) return;

   let lines = typeof getTxtarPaths === 'function' ? getTxtarPaths() : [];
   let updated = false;
   for (let i = 0; i < lines.length; i++) {
       let p = lines[i].trim();
       if (window.pendingBuilderPathFixes[p]) {
           lines[i] = window.pendingBuilderPathFixes[p];
           updated = true;
       }
   }
   if (updated) {
       if (typeof setTxtarPaths === 'function') setTxtarPaths(lines);
       if (typeof saveTxtarState === 'function') saveTxtarState();
   }

   window.pendingBuilderPathFixes = null;
   if (fixBtn) fixBtn.style.display = 'none';
}

async function buildTxtar(overridePaths = null, overrideFilename = null) {
   const btn = document.getElementById('buildTxtarBtn');
   if (btn) {
       btn.innerText = "⏳ Building...";
       btn.disabled = true;
   }
   
   const paths = overridePaths || (typeof getTxtarPaths === 'function' ? getTxtarPaths() : []);
   const exEl = document.getElementById('txtarExcludes');
   const excludes = exEl ? exEl.value.split('\n').map(l => l.trim()).filter(l => l.length > 0) : [];
   const anEl = document.getElementById('txtarAnchors');
   const anchors = anEl ? anEl.value.split('\n').map(l => l.trim()).filter(l => l.length > 0) : [];
   const prefEl = document.getElementById('txtarPreface');
   const preface = prefEl ? prefEl.value : "";
   const fnEl = document.getElementById('txtarFilename');
   const filename = overrideFilename || (fnEl ? fnEl.value.trim() : "");

   if (!overridePaths && typeof saveTxtarState === 'function') saveTxtarState();
   try {
       const res = await fetch('/api/txtar', {
           method: 'POST',
           headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify({ paths: paths, excludes: excludes, anchors: anchors, preface: preface, file_name: filename })
       });
       const data = await res.json();

       if (data.error) {
           alert("Build failed: " + data.error);
           return null;
       }

       window.lastBuiltTxtar = data;
       const resDiv = document.getElementById('txtarResult');
       if (resDiv) resDiv.style.display = 'flex';
       
       const statEl = document.getElementById('txtarResultStats');
       if (statEl) statEl.innerText = `Bundled ${data.file_count} files into${data.file_name}`;
       
       const downloadUrl = data.file_url;
       const absUrl = window.location.origin + downloadUrl;
       const link = document.getElementById('txtarDownloadLink');
       if (link) {
           link.href = downloadUrl;
           link.download = data.file_name;
           link.ondragstart = (e) => {
               e.dataTransfer.setData('DownloadURL', 'application/octet-stream:' + data.file_name + ':' + absUrl);
           };
       }
       return data;
   } catch (err) {
       alert("Network error: " + err.message);
       return null;
   } finally {
       if (btn) {
           btn.innerText = "📦 Build Txtar";
           btn.disabled = false;
       }
   }
}

async function buildAndCopyTxtar() {
   const copyBtn = document.getElementById('copyTxtarBtn');
   const originalText = copyBtn ? copyBtn.innerText : "📋 Copy";
   if (copyBtn) {
       copyBtn.innerText = "⏳ Copying...";
       copyBtn.disabled = true;
   }
   try {
       const buildData = await buildTxtar();
       if (!buildData || !buildData.file_name) return;

       const res = await fetch('/api/txtar_copy', {
           method: 'POST',
           headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify({ file_name: buildData.file_name })
       });
       const data = await res.json();
       if (data.error) {
           alert("Clipboard copy failed: " + data.error);
           return;
       }
       if (copyBtn) {
           copyBtn.innerText = "✓ Copied!";
           setTimeout(() => {
               copyBtn.innerText = originalText;
               copyBtn.disabled = false;
           }, 2000);
       }
   } catch (err) {
       alert("Copy failed: " + err.message);
       if (copyBtn) {
           copyBtn.innerText = originalText;
           copyBtn.disabled = false;
       }
   }
}

async function fetchSets() {
   try {
       const res = await fetch('/api/sets');
       configSets = await res.json();
       if (typeof updateSetDropdown === 'function') updateSetDropdown();
   } catch (err) {
       console.error("Failed to fetch config sets", err);
   }
}

function updateSetDropdown() {
   const select = document.getElementById('setSelect');
   if (!select) return;
   const currentVal = select.value;
   select.innerHTML = '';
   for (const name in configSets) {
       const opt = document.createElement('option');
       opt.value = name;
       opt.textContent = name;
       select.appendChild(opt);
   }
   if (configSets[currentVal]) {
       select.value = currentVal;
   } else if (select.options.length > 0) {
       select.value = select.options[0].value;
   } else {
       select.value = '';
   }
}

function loadSelectedSet() {
   const select = document.getElementById('setSelect');
   if (!select) return;
   const name = select.value;
   if (!name) return;
   const set = configSets[name];
   if (set) {
       if (typeof setTxtarPaths === 'function') setTxtarPaths(set.paths || []);
       const exEl = document.getElementById('txtarExcludes');
       if (exEl) exEl.value = set.excludes ? set.excludes.join('\n') : '';
       if (typeof syncExcludeTestsCheckbox === 'function') syncExcludeTestsCheckbox();
       const anEl = document.getElementById('txtarAnchors');
       if (anEl) anEl.value = set.anchors ? set.anchors.join('\n') : '';
       const prefEl = document.getElementById('txtarPreface');
       if (prefEl) prefEl.value = set.preface || '';
       const fnEl = document.getElementById('txtarFilename');
       if (fnEl) fnEl.value = set.file_name || '';
       if (typeof scheduleTxtarStatsUpdate === 'function') scheduleTxtarStatsUpdate();
   }
}

async function saveCurrentSet() {
   const select = document.getElementById('setSelect');
   if (!select) return;
   let name = select.value;
   const newNameEl = document.getElementById('newSetName');
   const newName = newNameEl ? newNameEl.value.trim() : '';
   if (newName) name = newName;
   if (!name) {
       alert("Please select or enter a name for the configuration set.");
       return;
   }
   
   const exEl = document.getElementById('txtarExcludes');
   const anEl = document.getElementById('txtarAnchors');
   const prefEl = document.getElementById('txtarPreface');
   const fnEl = document.getElementById('txtarFilename');
   
   const payload = {
       paths: typeof getTxtarPaths === 'function' ? getTxtarPaths() : [],
       excludes: exEl ? exEl.value.split('\n').map(l => l.trim()).filter(l => l) : [],
       anchors: anEl ? anEl.value.split('\n').map(l => l.trim()).filter(l => l) : [],
       preface: prefEl ? prefEl.value : "",
       file_name: fnEl ? fnEl.value.trim() : ""
   };
   configSets[name] = payload;
   try {
       await fetch('/api/sets', {
           method: 'POST',
           headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify(configSets)
       });
       if (newNameEl) newNameEl.value = '';
       updateSetDropdown();
       select.value = name;
   } catch (err) {
       alert("Failed to save set: " + err.message);
   }
}

async function deleteCurrentSet() {
   const select = document.getElementById('setSelect');
   if (!select) return;
   const name = select.value;
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
       if (Object.keys(configSets).length === 0) {
           const scratch = document.getElementById('modeScratchpad');
           if (scratch) scratch.click();
       } else {
           loadSelectedSet();
       }
   } catch (err) {
       alert("Failed to delete set: " + err.message);
   }
}

function toggleBuilderMode() {
   const scratchEl = document.getElementById('modeScratchpad');
   const isScratchpad = !scratchEl || scratchEl.checked;
   const savedControls = document.getElementById('savedSetControls');
   const pane = document.getElementById('tab-bundle');
   const lblScratchpad = document.getElementById('lblScratchpad');
   const lblSavedSet = document.getElementById('lblSavedSet');

   if (isScratchpad) {
       if (pane) {
           pane.classList.remove('mode-savedset');
           pane.classList.add('mode-scratchpad');
       }
       if (lblScratchpad) lblScratchpad.style.color = '#38bdf8';
       if (lblSavedSet) lblSavedSet.style.color = '#94a3b8';
       if (savedControls) savedControls.style.display = 'none';
       const fnEl = document.getElementById('txtarFilename');
       if (fnEl) fnEl.value = '';
       if (typeof loadTxtarState === 'function') loadTxtarState();
   } else {
       if (pane) {
           pane.classList.remove('mode-scratchpad');
           pane.classList.add('mode-savedset');
       }
       if (lblScratchpad) lblScratchpad.style.color = '#94a3b8';
       if (lblSavedSet) lblSavedSet.style.color = '#f59e0b';
       if (typeof saveTxtarState === 'function') saveTxtarState();
       if (savedControls) savedControls.style.display = 'flex';
       if (typeof loadSelectedSet === 'function') loadSelectedSet();
   }
}