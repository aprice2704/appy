let explorerTreeCache = {};
let explorerExpanded = new Set();
let explorerSelected = new Set();
let sgEvalTimeout;
let treeFilterTimeout;
let evaluatedResultPaths = [];

async function fetchTreeDir(dirPath) {
   const showHidden = document.getElementById('chkShowHidden') && document.getElementById('chkShowHidden').checked;
   const res = await fetch(`/api/fs/tree?dir=${encodeURIComponent(dirPath)}&hidden=${showHidden}`);
   const data = await res.json();
   return data.nodes || [];
}

async function reloadExplorerRoot() {
   explorerTreeCache = {};
   const container = document.getElementById('explorerTreeContainer');
   if (!container) return;
   container.innerHTML = "<em>Loading repository tree...</em>";
   const nodes = await fetchTreeDir('');
   explorerTreeCache[''] = nodes;
   renderExplorerTree();
}

function renderExplorerTree() {
   const container = document.getElementById('explorerTreeContainer');
   if (!container) return;
   const filter = (document.getElementById('explorerSearch').value || '').toLowerCase().trim();
   container.innerHTML = '';
   renderTreeBranch('', container, 0, filter);
}

function renderTreeBranch(dirPath, parentEl, depth, filter) {
    const nodes = explorerTreeCache[dirPath];
    if (!nodes) return;

    nodes.forEach(node => {
        const name = node.name || node.Name || '';
        const path = node.path || node.Path || '';
        const isDir = Boolean(node.is_dir !== undefined ? node.is_dir : node.IsDir);

        if (filter && !isDir && !path.toLowerCase().includes(filter) && !name.toLowerCase().includes(filter)) {
            return;
        }

        const row = document.createElement('div');
        row.className = 'tree-row';
        row.style.paddingLeft = (depth * 16 + 4) + 'px';

        const isExpanded = explorerExpanded.has(path);
        const isChecked = explorerSelected.has(path);

        let toggleHtml = '<span class="tree-toggle"></span>';
        let folderPrefix = '';
        if (isDir) {
            toggleHtml = `<span class="tree-toggle" onclick="toggleTreeDir(event, '${escapeHtml(path)}')">${isExpanded ? '▼' : '▶'}</span>`;
            folderPrefix = `<span class="tree-folder-marker">📁</span>`;
        } else {
            // Indent files slightly so they align under folder names without needing a leaf icon
            folderPrefix = `<span style="display: inline-block; width: 4px;"></span>`;
        }

        row.innerHTML = `
            ${toggleHtml}
            <input type="checkbox" ${isChecked ? 'checked' : ''} onchange="onTreeCheckboxChange('${escapeHtml(path)}',${isDir}, this.checked)">
            ${folderPrefix}
            <span class="tree-name ${isDir ? 'is-dir' : ''}">${escapeHtml(name)}</span>
        `;
        parentEl.appendChild(row);

        if (isDir && isExpanded) {
            const childContainer = document.createElement('div');
            childContainer.id = `branch-${path.replace(/[^a-zA-Z0-9_-]/g, '_')}`;
            parentEl.appendChild(childContainer);
            if (explorerTreeCache[path]) {
                renderTreeBranch(path, childContainer, depth + 1, filter);
            } else {
                fetchTreeDir(path).then(childNodes => {
                    explorerTreeCache[path] = childNodes;
                    renderTreeBranch(path, childContainer, depth + 1, filter);
                });
            }
        }
    });
}

async function toggleTreeDir(e, dirPath) {
   e.stopPropagation();
   if (explorerExpanded.has(dirPath)) {
       explorerExpanded.delete(dirPath);
   } else {
       explorerExpanded.add(dirPath);
       if (!explorerTreeCache[dirPath]) {
           explorerTreeCache[dirPath] = await fetchTreeDir(dirPath);
       }
   }
   renderExplorerTree();
}

function onTreeCheckboxChange(itemPath, isDir, checked) {
   if (checked) {
       explorerSelected.add(itemPath);
   } else {
       explorerSelected.delete(itemPath);
   }
   rebuildSuperGlobFromSelections();
}

function rebuildSuperGlobFromSelections() {
   const items = Array.from(explorerSelected);
   const paths = items.map(p => {
       const isDir = explorerTreeCache[p] || p.endsWith('/') || !p.includes('.');
       return isDir ? `${p}/**` : p;
   });
   const superGlobEl = document.getElementById('superGlobInput');
   if (superGlobEl) {
       superGlobEl.value = paths.join('\n');
   }
   debounceSuperGlobEval();
}

function debounceFilterTree() {
   clearTimeout(treeFilterTimeout);
   treeFilterTimeout = setTimeout(renderExplorerTree, 250);
}

function appendSuperGlobToken(token) {
   const el = document.getElementById('superGlobInput');
   if (!el) return;
   let val = el.value.trim();
   if (val && !val.endsWith('\n')) val += '\n';
   val += token;
   el.value = val;
   debounceSuperGlobEval();
}

function clearSuperGlob() {
   const el = document.getElementById('superGlobInput');
   if (el) el.value = '';
   explorerSelected.clear();
   renderExplorerTree();
   debounceSuperGlobEval();
}

function debounceSuperGlobEval() {
   clearTimeout(sgEvalTimeout);
   sgEvalTimeout = setTimeout(runSuperGlobEval, 300);
}

async function runSuperGlobEval() {
   const raw = (document.getElementById('superGlobInput').value || '').trim();
   const countEl = document.getElementById('sgFileCount');
   const tokEl = document.getElementById('sgTokens');
   const manifestEl = document.getElementById('superGlobManifest');

   if (!raw) {
       if (countEl) countEl.innerText = '0';
       if (tokEl) tokEl.innerText = '0';
       if (manifestEl) manifestEl.innerHTML = '<em style="color: #64748b;">No files matched.</em>';
       evaluatedResultPaths = [];
       return;
   }

   try {
       const res = await fetch('/api/superglob/eval', {
           method: 'POST',
           headers: { 'Content-Type': 'application/json' },
           body: JSON.stringify({ expression: raw })
       });
       const data = await res.json();
       if (data.files) {
           evaluatedResultPaths = data.files.map(f => f.path);
           if (countEl) countEl.innerText = data.file_count;
           if (tokEl) tokEl.innerText = Math.round(data.tokens_est / 1000);
           if (manifestEl) {
               let html = '';
               data.files.forEach(f => {
                   html += `<div style="padding: 2px 0; display: flex; justify-content: space-between;">
                       <span style="color: #4ade80;">${escapeHtml(f.path)}</span>
                       <span style="color: #64748b;">${Math.round(f.tokens)} tok</span>
                   </div>`;
               });
               manifestEl.innerHTML = html || '<em style="color: #64748b;">Zero files matched expression.</em>';
           }
       }
   } catch (err) {
       console.error("SuperGlob evaluation failed:", err);
   }
}

function sendExplorerToBuilder() {
    const raw = (document.getElementById('superGlobInput').value || '').trim();
    if (!raw && evaluatedResultPaths.length === 0) {
        alert("No paths selected in Explorer.");
        return;
    }

    let targetPaths = getTxtarPaths();
    let addedCount = 0;

    raw.split('\n').forEach(line => {
        line = line.trim();
        if (line && !targetPaths.includes(line)) {
            targetPaths.push(line);
            addedCount++;
        }
    });

    setTxtarPaths(targetPaths);
    saveTxtarState();

    const sendBtn = document.querySelector('button[onclick="sendExplorerToBuilder()"]');
    if (sendBtn) {
        const origText = sendBtn.innerText;
        sendBtn.innerText = `✓ Added (${addedCount})`;
        sendBtn.style.background = "#16a34a";
        sendBtn.style.borderColor = "#22c55e";
        setTimeout(() => {
            sendBtn.innerText = origText;
            sendBtn.style.background = "#2563eb";
            sendBtn.style.borderColor = "#3b82f6";
        }, 1500);
    }
}

// Hook tab-switch to load tree if opening tab-explorer
const origSwitchTab = window.switchTab;
window.switchTab = function(tabId) {
   if (typeof origSwitchTab === 'function') {
       origSwitchTab(tabId);
   }
   if (tabId === 'tab-explorer') {
       if (Object.keys(explorerTreeCache).length === 0) {
           reloadExplorerRoot();
       }
   }
};
