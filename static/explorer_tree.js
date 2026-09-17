// Appy Explorer: Directory Tree & Selections
// Appy Explorer: Directory Tree & Selections
let explorerTreeCache = {};
let explorerExpanded = new Set();
let explorerSelected = new Set();
let treeFilterTimeout;
const treeSortModes = ['alpha', 'recent', 'largest', 'smallest'];
let currentTreeSortIdx = 0;

function cycleTreeSortMode() {
   currentTreeSortIdx = (currentTreeSortIdx + 1) % treeSortModes.length;
   const mode = treeSortModes[currentTreeSortIdx];
   const labels = {
       alpha: 'Sort: Alpha',
       recent: 'Sort: Recent',
       largest: 'Sort: Largest',
       smallest: 'Sort: Smallest'
   };
   const btn = document.getElementById('btnTreeSortMode');
   if (btn) btn.innerText = labels[mode];
   renderExplorerTree();
}

function formatDeltaMinutes(modTimeSec) {
   if (!modTimeSec) return '';
   const diffSec = Math.max(0, Math.floor(Date.now() / 1000) - modTimeSec);
   const mins = Math.floor(diffSec / 60);
   if (mins < 60) return `${mins}m`;
   const hrs = Math.floor(mins / 60);
   if (hrs < 24) return `${hrs}h`;
   const days = Math.floor(hrs / 24);
   return `${days}d`;
}

function formatSizeKb(bytes) {
   if (!bytes || bytes <= 0) return '0k';
   const kb = Math.round(bytes / 1024);
   return `${kb || 1}k`;
}

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
   const rawNodes = explorerTreeCache[dirPath];
   if (!rawNodes) return;

   const dirs = [];
   const files = [];
   rawNodes.forEach(node => {
       const isDir = Boolean(node.is_dir !== undefined ? node.is_dir : node.IsDir);
       if (isDir) dirs.push(node);
       else files.push(node);
   });

   dirs.sort((a, b) => (a.name || a.Name || '').localeCompare(b.name || b.Name || ''));

   const sortMode = treeSortModes[currentTreeSortIdx];
   files.sort((a, b) => {
       const nameA = a.name || a.Name || '';
       const nameB = b.name || b.Name || '';
       if (sortMode === 'recent') {
           return (b.mod_time || b.ModTime || 0) - (a.mod_time || a.ModTime || 0);
       } else if (sortMode === 'largest') {
           return (b.size || b.Size || 0) - (a.size || a.Size || 0);
       } else if (sortMode === 'smallest') {
           return (a.size || a.Size || 0) - (b.size || b.Size || 0);
       }
       return nameA.localeCompare(nameB);
   });

   const nodes = [...dirs, ...files];

   nodes.forEach(node => {
       const name = node.name || node.Name || '';
       const path = node.path || node.Path || '';
       const isDir = Boolean(node.is_dir !== undefined ? node.is_dir : node.IsDir);
       const size = node.size || node.Size || 0;
       const modTime = node.mod_time || node.ModTime || 0;

       if (filter && !isDir && !path.toLowerCase().includes(filter) && !name.toLowerCase().includes(filter)) {
           return;
       }

       const row = document.createElement('div');
       row.className = 'tree-row';
       row.style.paddingLeft = (depth * 16 + 4) + 'px';
       row.id = `tree-row-${path.replace(/[^a-zA-Z0-9_-]/g, '_')}`;
       row.oncontextmenu = (e) => showTreeContextMenu(e, path, isDir);

       const isExpanded = explorerExpanded.has(path);
       const isChecked = explorerSelected.has(path);

       let toggleHtml = '<span class="tree-toggle"></span>';
       let folderPrefix = '';
       let rhsMeta = '';
       if (isDir) {
           toggleHtml = `<span class="tree-toggle" onclick="toggleTreeDir(event, '${escapeHtml(path)}')">${isExpanded ? '▼' : '▶'}</span>`;
           folderPrefix = `<span class="tree-folder-marker">📁</span>`;
           row.ondblclick = (e) => toggleTreeDir(e, path);
       } else {
           folderPrefix = `<span style="display: inline-block; width: 4px;"></span>`;
           row.ondblclick = (e) => {
               const nextVal = !explorerSelected.has(path);
               const chk = row.querySelector('input[type="checkbox"]');
               if (chk) chk.checked = nextVal;
               onTreeCheckboxChange(path, false, nextVal);
           };
           const delta = formatDeltaMinutes(modTime);
           const sz = formatSizeKb(size);
           rhsMeta = `<span style="margin-left: auto; font-size: 11px; color: #64748b; font-family: monospace; display: flex; gap: 8px; padding-right: 6px;">
               <span>${delta}</span>
               <span style="min-width: 32px; text-align: right;">${sz}</span>
           </span>`;
       }

       row.innerHTML = `
           ${toggleHtml}
           <input type="checkbox" ${isChecked ? 'checked' : ''} onchange="onTreeCheckboxChange('${escapeHtml(path)}',${isDir}, this.checked)">
           ${folderPrefix}
           <span class="tree-name ${isDir ? 'is-dir' : ''}">${escapeHtml(name)}</span>
           ${rhsMeta}
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
       const dirToAdd = isDir ? itemPath : (itemPath.includes('/') ? itemPath.substring(0, itemPath.lastIndexOf('/')) : '');
       if (dirToAdd) addRecentDir(dirToAdd);
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