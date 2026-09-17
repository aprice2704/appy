// Appy Explorer: Bookmarks, Recent Dirs & Context Navigation
let contextTargetItem = null;

function getExplorerStorageKey(key) {
   const root = window.AppyRootDir || 'default';
   return `appy_${key}_${root}`;
}

function loadBookmarks() {
   try {
       const raw = localStorage.getItem(getExplorerStorageKey('bookmarks'));
       return raw ? JSON.parse(raw) : [];
   } catch (e) {
       return [];
   }
}

function saveBookmarks(list) {
   localStorage.setItem(getExplorerStorageKey('bookmarks'), JSON.stringify(list));
   renderBookmarks();
}

function loadRecentDirs() {
   try {
       const raw = localStorage.getItem(getExplorerStorageKey('recent_dirs'));
       return raw ? JSON.parse(raw) : [];
   } catch (e) {
       return [];
   }
}

function saveRecentDirs(list) {
   localStorage.setItem(getExplorerStorageKey('recent_dirs'), JSON.stringify(list));
   renderRecentDirs();
}

function addRecentDir(dirPath) {
   if (!dirPath || dirPath === '.') return;
   let list = loadRecentDirs().filter(d => d !== dirPath);
   list.push(dirPath);
   list.sort((a, b) => a.localeCompare(b));
   if (list.length > 10) {
       list = list.slice(list.length - 10);
   }
   saveRecentDirs(list);
}

function renderBookmarks() {
   const el = document.getElementById('explorerBookmarksList');
   if (!el) return;
   const items = loadBookmarks();
   if (items.length === 0) {
       el.innerHTML = '<em style="color: #64748b; font-size: 11px;">Right-click any file/dir to bookmark.</em>';
       return;
   }
   items.sort((a, b) => (a.path || a).localeCompare(b.path || b));
   let html = '';
   items.forEach((item, idx) => {
       const p = item.path || item;
       const isDir = Boolean(item.isDir);
       const icon = isDir ? '📁' : '📄';
       html += `<div class="nav-panel-item" onclick="revealExplorerPath('${escapeHtml(p)}')">
           <span class="nav-panel-label" title="${escapeHtml(p)}">${icon}${escapeHtml(p)}</span>
           <span class="nav-panel-del" onclick="removeBookmark(event, ${idx})" title="Remove bookmark">✕</span>
       </div>`;
   });
   el.innerHTML = html;
}

function renderRecentDirs() {
   const el = document.getElementById('explorerRecentDirsList');
   if (!el) return;
   const items = loadRecentDirs();
   if (items.length === 0) {
       el.innerHTML = '<em style="color: #64748b; font-size: 11px;">Check items in tree to record recent dirs.</em>';
       return;
   }
   let html = '';
   items.forEach((dir) => {
       html += `<div class="nav-panel-item" onclick="revealExplorerPath('${escapeHtml(dir)}')">
           <span class="nav-panel-label" title="${escapeHtml(dir)}">📁 ${escapeHtml(dir)}</span>
       </div>`;
   });
   el.innerHTML = html;
}

function removeBookmark(e, idx) {
   e.stopPropagation();
   let list = loadBookmarks();
   list.splice(idx, 1);
   saveBookmarks(list);
}

function clearBookmarks() {
   saveBookmarks([]);
}

function clearRecentDirs() {
   saveRecentDirs([]);
}

async function revealExplorerPath(targetPath) {
   if (!targetPath) return;
   const parts = targetPath.split('/');
   let cur = '';
   for (let i = 0; i < parts.length - 1; i++) {
       cur = cur ? `${cur}/${parts[i]}` : parts[i];
       if (!explorerTreeCache[cur]) {
           explorerTreeCache[cur] = await fetchTreeDir(cur);
       }
       explorerExpanded.add(cur);
   }
   if (explorerTreeCache[targetPath]) {
       explorerExpanded.add(targetPath);
   }
   renderExplorerTree();

   setTimeout(() => {
       const rowId = `tree-row-${targetPath.replace(/[^a-zA-Z0-9_-]/g, '_')}`;
       const rowEl = document.getElementById(rowId);
       if (rowEl) {
           rowEl.scrollIntoView({ block: 'center', behavior: 'smooth' });
           rowEl.classList.add('tree-highlight');
           setTimeout(() => rowEl.classList.remove('tree-highlight'), 1800);
       }
   }, 80);
}

function showTreeContextMenu(e, path, isDir) {
   e.preventDefault();
   e.stopPropagation();
   contextTargetItem = { path, isDir };
   const menu = document.getElementById('treeContextMenu');
   if (!menu) return;

   const bookmarks = loadBookmarks();
   const isBookmarked = bookmarks.some(b => (b.path || b) === path);
   const actionEl = document.getElementById('ctxMenuBookmarkAction');
   if (actionEl) {
       actionEl.innerText = isBookmarked ? `❌ Remove Bookmark` : `⭐ Bookmark ${isDir ? 'Directory' : 'File'}`;
   }

   const selTypeEl = document.getElementById('ctxMenuSelectTypeAction');
   const deselTypeEl = document.getElementById('ctxMenuDeselectTypeAction');
   if (!isDir && path.includes('.')) {
       const ext = '.' + path.split('.').pop();
       if (selTypeEl) {
           selTypeEl.innerText = `☑️ Select all *${ext}`;
           selTypeEl.style.display = 'block';
       }
       if (deselTypeEl) {
           deselTypeEl.innerText = `☐ Deselect all *${ext}`;
           deselTypeEl.style.display = 'block';
       }
   } else {
       if (selTypeEl) selTypeEl.style.display = 'none';
       if (deselTypeEl) deselTypeEl.style.display = 'none';
   }

   menu.style.left = `${e.clientX}px`;
   menu.style.top = `${e.clientY}px`;
   menu.style.display = 'block';
}

function onContextSelectTypeClick(select) {
   if (!contextTargetItem || contextTargetItem.isDir) return;
   const path = contextTargetItem.path;
   if (!path.includes('.')) return;
   const ext = '.' + path.split('.').pop().toLowerCase();

   // Determine the directory scope of the clicked item
   const dir = path.includes('/') ? path.substring(0, path.lastIndexOf('/')) : '';
   const pattern = dir ? `${dir}/{*${ext},**/*${ext}}` : `*${ext}`;

   if (select) {
       // Compact into a glob pattern instead of populating individual files
       appendSuperGlobToken(pattern);
       // Select matching cached files in the tree view for visual feedback
       for (const d in explorerTreeCache) {
           (explorerTreeCache[d] || []).forEach(n => {
               const isNodeDir = Boolean(n.is_dir !== undefined ? n.is_dir : n.IsDir);
               const p = n.path || n.Path || '';
               if (!isNodeDir && p.toLowerCase().endsWith(ext) && (!dir || p.startsWith(dir + '/'))) {
                   explorerSelected.add(p);
               }
           });
       }
   } else {
       // Remove pattern from input and deselect matching files
       const superGlobEl = document.getElementById('superGlobInput');
       if (superGlobEl) {
           const lines = superGlobEl.value.split('\n').filter(l => {
               const t = l.trim();
               return t !== pattern && t !== `${dir}/**/*${ext}` && t !== `*${ext}` && t !== `**/*${ext}`;
           });
           superGlobEl.value = lines.join('\n');
       }
       for (const d in explorerTreeCache) {
           (explorerTreeCache[d] || []).forEach(n => {
               const isNodeDir = Boolean(n.is_dir !== undefined ? n.is_dir : n.IsDir);
               const p = n.path || n.Path || '';
               if (!isNodeDir && p.toLowerCase().endsWith(ext) && (!dir || p.startsWith(dir + '/'))) {
                   explorerSelected.delete(p);
               }
           });
       }
       debounceSuperGlobEval();
   }
   renderExplorerTree();
   hideTreeContextMenu();
}

function hideTreeContextMenu() {
   const menu = document.getElementById('treeContextMenu');
   if (menu) menu.style.display = 'none';
   contextTargetItem = null;
}

function onContextBookmarkClick() {
   if (!contextTargetItem) return;
   const { path, isDir } = contextTargetItem;
   let bookmarks = loadBookmarks();
   const idx = bookmarks.findIndex(b => (b.path || b) === path);
   if (idx >= 0) {
       bookmarks.splice(idx, 1);
   } else {
       bookmarks.push({ path, isDir });
   }
   saveBookmarks(bookmarks);
   hideTreeContextMenu();
}

document.addEventListener('click', hideTreeContextMenu);

const origSwitchTab = window.switchTab;
window.switchTab = function(tabId) {
   if (typeof origSwitchTab === 'function') {
       origSwitchTab(tabId);
   }
   if (tabId === 'tab-explorer') {
       renderBookmarks();
       renderRecentDirs();
       if (Object.keys(explorerTreeCache).length === 0) {
           reloadExplorerRoot();
       }
   }
};