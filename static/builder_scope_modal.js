// Appy Builder: Scoped Directory Modal Chooser
let scopedModalFiles = [];

async function triggerScopeDirChooser() {
   if (window.showDirectoryPicker) {
       try {
           const dirHandle = await window.showDirectoryPicker();
           scopedModalFiles = [];
           await readDirectoryHandle(dirHandle, '');
           
           const samples = scopedModalFiles.slice(0, 5).map(f => f.path);
           let truePrefix = dirHandle.name;

           try {
               const res = await fetch('/api/resolve_directory', {
                   method: 'POST',
                   headers: { 'Content-Type': 'application/json' },
                   body: JSON.stringify({ dir_name: dirHandle.name, sample_files: samples })
               });
               const data = await res.json();
               if (data.path) {
                   truePrefix = data.path;
               }
           } catch (err) {
               console.error("Directory resolution failed:", err);
           }

           scopedModalFiles.forEach(item => {
               item.path = truePrefix + '/' + item.path;
           });

           openScopedModal(truePrefix);
           return;
       } catch (err) {
           if (err.name === 'AbortError') return;
           console.warn("showDirectoryPicker failed, falling back to input:", err);
       }
   }
   document.getElementById('txtarScopeDirInput').click();
}

async function readDirectoryHandle(dirHandle, pathPrefix) {
   for await (const entry of dirHandle.values()) {
       if (entry.name.startsWith('.') || entry.name === 'node_modules' || entry.name === 'vendor') continue;
       const entryPath = pathPrefix ? `${pathPrefix}/${entry.name}` : entry.name;
       if (entry.kind === 'file') {
           scopedModalFiles.push({ path: entryPath, selected: true });
       } else if (entry.kind === 'directory') {
           await readDirectoryHandle(entry, entryPath);
       }
   }
}

async function handleScopeDirSelect(event) {
   const files = event.target.files;
   if (!files || files.length === 0) return;

   let dirName = "";
   scopedModalFiles = [];

   for (let f of files) {
       let rel = f.webkitRelativePath || f.name;
       if (!dirName && f.webkitRelativePath) {
           dirName = f.webkitRelativePath.split('/')[0];
       }
       scopedModalFiles.push({
           path: rel.replace(/\\/g, '/'),
           selected: true
       });
   }

   if (dirName) {
       try {
           const res = await fetch('/api/resolve_path?name=' + encodeURIComponent(dirName));
           const data = await res.json();
           if (data.path && data.path !== dirName) {
               const prefix = data.path.replace(/\/\*\*$/, '');
               scopedModalFiles.forEach(item => {
                   const rest = item.path.substring(dirName.length);
                   item.path = (prefix + rest).replace(/^\//, '');
               });
               dirName = prefix;
           }
       } catch (err) {
           console.error("Path resolution failed:", err);
       }
   }

   openScopedModal(dirName);
   event.target.value = '';
}

function openScopedModal(dirName) {
   const modal = document.getElementById('scopedFileModal');
   const sub = document.getElementById('scopedModalSubtitle');
   const filterInput = document.getElementById('scopedModalFilter');
   if (sub) sub.innerText = dirName ? `Directory: ${dirName}` : '';
   if (filterInput) filterInput.value = '';
   renderScopedModalList();
   if (modal) modal.style.display = 'flex';
}

function closeScopedModal() {
   const modal = document.getElementById('scopedFileModal');
   if (modal) modal.style.display = 'none';
}

function renderScopedModalList() {
   const listContainer = document.getElementById('scopedModalList');
   if (!listContainer) return;
   listContainer.innerHTML = '';

   scopedModalFiles.sort((a, b) => a.path.localeCompare(b.path));

   scopedModalFiles.forEach((item, idx) => {
       const row = document.createElement('label');
       row.className = 'appy-modal-item';
       row.id = `scoped-item-${idx}`;
       row.innerHTML = `
           <input type="checkbox" id="chk-scoped-${idx}" ${item.selected ? 'checked' : ''} onchange="onScopedCheckboxChange(${idx}, this.checked)">
           <span class="item-path">${safeEscapeHtml(item.path)}</span>
       `;
       listContainer.appendChild(row);
   });
   updateScopedModalCount();
}

function onScopedCheckboxChange(idx, checked) {
   if (scopedModalFiles[idx]) {
       scopedModalFiles[idx].selected = checked;
   }
   updateScopedModalCount();
}

function filterScopedModalList() {
   const q = (document.getElementById('scopedModalFilter').value || '').toLowerCase().trim();
   scopedModalFiles.forEach((item, idx) => {
       const el = document.getElementById(`scoped-item-${idx}`);
       if (el) {
           if (!q || item.path.toLowerCase().includes(q)) {
               el.classList.remove('hidden');
           } else {
               el.classList.add('hidden');
           }
       }
   });
}

function toggleSelectAllScoped(val) {
   const q = (document.getElementById('scopedModalFilter').value || '').toLowerCase().trim();
   scopedModalFiles.forEach((item, idx) => {
       if (!q || item.path.toLowerCase().includes(q)) {
           item.selected = val;
           const chk = document.getElementById(`chk-scoped-${idx}`);
           if (chk) chk.checked = val;
       }
   });
   updateScopedModalCount();
}

function updateScopedModalCount() {
   const cnt = scopedModalFiles.filter(i => i.selected).length;
   const lbl = document.getElementById('scopedModalCount');
   if (lbl) lbl.innerText = `${cnt} of${scopedModalFiles.length} selected`;
}

function confirmScopedModal() {
   let existing = getTxtarPaths();
   scopedModalFiles.filter(i => i.selected).forEach(item => {
       if (!existing.includes(item.path)) {
           existing.push(item.path);
       }
   });
   setTxtarPaths(existing);
   saveTxtarState();
   closeScopedModal();
}