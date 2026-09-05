function getTxtarPaths() {
    const inputs = document.querySelectorAll('.path-input');
    return Array.from(inputs).map(i => i.value).filter(v => v.trim() !== '');
}

function setTxtarPaths(paths) {
    const tbody = document.getElementById('txtarPathsBody');
    if (!tbody) return;
    tbody.innerHTML = '';
    if (!paths || paths.length === 0) {
        addPathRow('');
        return;
    }
    paths.forEach(p => addPathRow(p));
    scheduleTxtarStatsUpdate();
}

function addPathRow(val) {
   const tbody = document.getElementById('txtarPathsBody');
   if (!tbody) return;
   const tr = document.createElement('tr');
   tr.innerHTML = `
      <td class="stat-files" style="text-align: center; padding: 4px; color: #94a3b8;">-</td>
      <td class="stat-tokens" style="text-align: center; padding: 4px; color: #94a3b8;">-</td>
      <td style="padding: 4px;"><input type="text" class="path-input" value="${escapeHtml(val)}" oninput="saveTxtarState()" onkeydown="handlePathKeydown(event)" style="width: 100%; background: transparent; border: none; color: inherit; outline: none; font-family: monospace;"></td>
      <td style="text-align: center; padding: 4px;"><button onclick="this.closest('tr').remove(); saveTxtarState();"
style="background: transparent; padding: 0; min-width: 0; color: #64748b; border: none; height: auto; cursor: pointer; font-size: 16px;"
onmouseover="this.style.color='#ef4444'" onmouseout="this.style.color='#64748b'" title="Remove path">🗑️</button></td>
  `;
   tbody.appendChild(tr);
}

async function handlePathKeydown(e) {
   if (e.key === 'Tab') {
       e.preventDefault();
       const input = e.target;
       const val = input.value;
       try {
           const res = await fetch('/api/autocomplete_path?prefix=' + encodeURIComponent(val));
           const data = await res.json();
           if (data.suggestions && data.suggestions.length > 0) {
               if (data.suggestions.length === 1) {
                   input.value = data.suggestions[0];
               } else {
                   // Compute longest common prefix
                   let lcp = data.suggestions[0];
                   for (let s of data.suggestions) {
                       while (!s.startsWith(lcp) && lcp.length > 0) {
                           lcp = lcp.slice(0, -1);
                       }
                   }
                   if (lcp.length > val.length) {
                       input.value = lcp;
                   } else {
                       console.log("Completions: " + data.suggestions.join(", "));
                   }
               }
               saveTxtarState();
           }
       } catch (err) {
           console.error("Autocomplete failed:", err);
       }
   } else if (e.key === 'Enter') {
       e.preventDefault();
       addPathRow('');
       const inputs = document.querySelectorAll('.path-input');
       if (inputs.length > 0) {
           inputs[inputs.length - 1].focus();
       }
   }
}

function syncExcludeTestsCheckbox() {
   const chk = document.getElementById('chkExcludeTests');
   if (!chk) return;
   const excludesText = document.getElementById('txtarExcludes').value;
   const lines = excludesText.split('\n').map(l => l.trim());
   chk.checked = lines.includes('*_test.go');
}

function toggleExcludeTests(checked) {
   const textarea = document.getElementById('txtarExcludes');
   let lines = textarea.value.split('\n').map(l => l.trim()).filter(l => l !== '');
   if (checked) {
       if (!lines.includes('*_test.go')) {
           lines.unshift('*_test.go');
       }
   } else {
       lines = lines.filter(l => l !== '*_test.go');
   }
   textarea.value = lines.join('\n');
   saveTxtarState();
}

function onExcludesInput() {
   syncExcludeTestsCheckbox();
   saveTxtarState();
}

function toggleGlobHelp() {
    const helpDiv = document.getElementById('globHelp');
    helpDiv.style.display = helpDiv.style.display === 'none' ? 'block' : 'none';
}

async function replaceTxtarCommand() {
    setTxtarPaths([]);
    await pasteTxtarCommand();
}

async function pasteTxtarCommand() {
    try {
        const text = await navigator.clipboard.readText();
        let cleaned = text.trim();
        cleaned = cleaned.replace(/^txtar\s+c\s+/i, '').replace(/^txtar\s+/i, '');
        cleaned = cleaned.replace(/>\s*[^\s]+$/, '');
        cleaned = cleaned.replace(/\\\r?\n/g, ' ');
        const paths = cleaned.split(/\s+/).map(p => p.trim()).filter(p => p.length > 0);

        let existing = getTxtarPaths();
        paths.forEach(p => {
            if (!existing.includes(p)) existing.push(p);
        });
        setTxtarPaths(existing);
        saveTxtarState();
    } catch (err) {
        console.error("Paste failed", err);
        alert("Clipboard paste failed: " + err.message);
    }
}

function clearTxtarPaths() {
    setTxtarPaths([]);
    saveTxtarState();
}

async function handleTxtarFileSelect(event, isDir) {
    let lines = getTxtarPaths();
    if (isDir && event.target.files.length > 0) {
        let firstFile = event.target.files[0];
        let pathStr = firstFile.path || firstFile.webkitRelativePath.split('/')[0];

        if (firstFile.path && firstFile.webkitRelativePath) {
            let relLen = firstFile.webkitRelativePath.length;
            let absDir = firstFile.path.substring(0, firstFile.path.length - relLen);
            pathStr = absDir + firstFile.webkitRelativePath.split('/')[0];
            pathStr = pathStr.replace(/\\/g, '/').replace(/\/\//g, '/');
        }

        try {
            const res = await fetch('/api/resolve_path?name=' + encodeURIComponent(pathStr));
            const data = await res.json();
            if (data.path) pathStr = data.path;
        } catch (e) {
            console.error("Path resolution failed", e);
        }

        let p = pathStr + "/**";
        if (pathStr && !lines.includes(p)) {
            lines.push(p);
        }
    } else {
        for (let file of event.target.files) {
            let p = file.path || file.name;
            if (p) {
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
    setTxtarPaths(lines);
    saveTxtarState();
    event.target.value = '';
}

function saveTxtarState() {
    const root = window.AppyRootDir || 'default';
    const isScratchpad = !document.getElementById('modeScratchpad') || document.getElementById('modeScratchpad').checked;
    if (isScratchpad) {
        localStorage.setItem('txtarPaths_' + root, getTxtarPaths().join('\n'));
        localStorage.setItem('txtarExcludes_' + root, document.getElementById('txtarExcludes').value);
        localStorage.setItem('txtarAnchors_' + root, document.getElementById('txtarAnchors').value);
        localStorage.setItem('txtarPreface_' + root, document.getElementById('txtarPreface').value);
    }
    scheduleTxtarStatsUpdate();
}

function loadTxtarState() {
    const root = window.AppyRootDir || 'default';
    if (localStorage.getItem('txtarPaths_' + root) !== null) {
        setTxtarPaths(localStorage.getItem('txtarPaths_' + root).split('\n'));
    } else {
        setTxtarPaths(['.']);
    }
       if (localStorage.getItem('txtarExcludes_' + root) !== null) {
       document.getElementById('txtarExcludes').value = localStorage.getItem('txtarExcludes_' + root);
   }
   syncExcludeTestsCheckbox();
    if (localStorage.getItem('txtarAnchors_' + root) !== null) {
        document.getElementById('txtarAnchors').value = localStorage.getItem('txtarAnchors_' + root);
    }
    if (localStorage.getItem('txtarPreface_' + root) !== null) {
        document.getElementById('txtarPreface').value = localStorage.getItem('txtarPreface_' + root);
    }
    scheduleTxtarStatsUpdate();
}

async function updateTxtarStats() {
    const paths = getTxtarPaths();
    const excludes = document.getElementById('txtarExcludes').value.split('\n').map(l => l.trim()).filter(l => l.length > 0);
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
    clearTimeout(txtarStatsTimeout);
    txtarStatsTimeout = setTimeout(updateTxtarStats, 300);
}

function fixBuilderPaths() {
    const fixBtn = document.getElementById('builderFixPathsBtn');
    if (!window.pendingBuilderPathFixes) return;

    let lines = getTxtarPaths();
    let updated = false;
    for (let i = 0; i < lines.length; i++) {
        let p = lines[i].trim();
        if (window.pendingBuilderPathFixes[p]) {
            lines[i] = window.pendingBuilderPathFixes[p];
            updated = true;
        }
    }
    if (updated) {
        setTxtarPaths(lines);
        saveTxtarState();
    }

    window.pendingBuilderPathFixes = null;
    if (fixBtn) fixBtn.style.display = 'none';
}

async function buildTxtar(overridePaths = null, overrideFilename = null) {
    const btn = document.getElementById('buildTxtarBtn');
    btn.innerText = "⏳ Building...";
    btn.disabled = true;
    const paths = overridePaths || getTxtarPaths();
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
        btn.innerText = "📦 Build Txtar";
        btn.disabled = false;
    }
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
    const name = document.getElementById('setSelect').value;
    if (!name) {
        return;
    }
    const set = configSets[name];
    if (set) {
        setTxtarPaths(set.paths || []);
               document.getElementById('txtarExcludes').value = set.excludes ? set.excludes.join('\n') : '';
       syncExcludeTestsCheckbox();
        document.getElementById('txtarAnchors').value = set.anchors ? set.anchors.join('\n') : '';
        document.getElementById('txtarPreface').value = set.preface || '';
        if (document.getElementById('txtarFilename')) {
            document.getElementById('txtarFilename').value = set.file_name || '';
        }
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
        paths: getTxtarPaths(),
        excludes: document.getElementById('txtarExcludes').value.split('\n').map(l => l.trim()).filter(l => l),
        anchors: document.getElementById('txtarAnchors').value.split('\n').map(l => l.trim()).filter(l => l),
        preface: document.getElementById('txtarPreface').value,
        file_name: document.getElementById('txtarFilename') ? document.getElementById('txtarFilename').value.trim() : ""
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
        if (Object.keys(configSets).length === 0) {
            document.getElementById('modeScratchpad').click();
        } else {
            loadSelectedSet();
        }
    } catch (err) {
        alert("Failed to delete set: " + err.message);
    }
}

function toggleBuilderMode() {
   const isScratchpad = document.getElementById('modeScratchpad').checked;
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
       savedControls.style.display = 'none';
       document.getElementById('txtarFilename').value = '';
       loadTxtarState();
   } else {
       if (pane) {
           pane.classList.remove('mode-scratchpad');
           pane.classList.add('mode-savedset');
       }
       if (lblScratchpad) lblScratchpad.style.color = '#94a3b8';
       if (lblSavedSet) lblSavedSet.style.color = '#f59e0b';
       saveTxtarState();
       savedControls.style.display = 'flex';
       loadSelectedSet();
   }
}