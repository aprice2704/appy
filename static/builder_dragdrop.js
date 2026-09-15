// Appy Builder: File Chooser & Drag/Drop
function initTableDragAndDrop() {
   const dropZone = document.querySelector('.table-container');
   if (!dropZone) return;

   ['dragenter', 'dragover'].forEach(eventName => {
       dropZone.addEventListener(eventName, (e) => {
           e.preventDefault();
           e.stopPropagation();
           dropZone.classList.add('drag-over');
       }, false);
   });

   ['dragleave', 'drop'].forEach(eventName => {
       dropZone.addEventListener(eventName, (e) => {
           e.preventDefault();
           e.stopPropagation();
           dropZone.classList.remove('drag-over');
       }, false);
   });

   dropZone.addEventListener('drop', async (e) => {
       const dt = e.dataTransfer;
       if (!dt) return;

       const droppedText = dt.getData('text/plain');
       if (droppedText && (!dt.files || dt.files.length === 0)) {
           let lines = droppedText.split(/[\r\n]+/).map(s => s.trim()).filter(s => s.length > 0);
           let current = getTxtarPaths();
           for (let line of lines) {
               if (!current.includes(line)) current.push(line);
           }
           if (typeof setTxtarPaths === 'function') setTxtarPaths(current);
           if (typeof saveTxtarState === 'function') saveTxtarState();
           return;
       }

       let addedPaths = [];
       if (dt.items && dt.items.length > 0) {
           for (let i = 0; i < dt.items.length; i++) {
               const item = dt.items[i];
               if (item.webkitGetAsEntry) {
                   const entry = item.webkitGetAsEntry();
                   if (entry) {
                       await traverseFileTree(entry, '', addedPaths);
                   }
               }
           }
       } else if (dt.files && dt.files.length > 0) {
           for (let f of dt.files) {
               let p = f.path || f.webkitRelativePath || f.name;
               addedPaths.push(p);
           }
       }

       if (addedPaths.length > 0) {
           let current = typeof getTxtarPaths === 'function' ? getTxtarPaths() : [];
           for (let rawPath of addedPaths) {
               try {
                   const res = await fetch('/api/resolve_path?name=' + encodeURIComponent(rawPath));
                   const data = await res.json();
                   const finalPath = data.path || rawPath;
                   if (!current.includes(finalPath)) {
                       current.push(finalPath);
                   }
               } catch (err) {
                   if (!current.includes(rawPath)) {
                       current.push(rawPath);
                   }
               }
           }
           if (typeof setTxtarPaths === 'function') setTxtarPaths(current);
           if (typeof saveTxtarState === 'function') saveTxtarState();
       }
   }, false);
}

async function traverseFileTree(item, path, collector) {
   path = path || "";
   if (item.isFile) {
       collector.push((path + item.name).replace(/^\//, ''));
   } else if (item.isDirectory) {
       const dirReader = item.createReader();
       const readEntries = () => new Promise((resolve) => {
           dirReader.readEntries((entries) => resolve(entries));
       });
       let entries;
       do {
           entries = await readEntries();
           for (let i = 0; i < entries.length; i++) {
               await traverseFileTree(entries[i], path + item.name + "/", collector);
           }
       } while (entries && entries.length > 0);
   }
}

if (document.readyState === 'loading') {
   document.addEventListener('DOMContentLoaded', initTableDragAndDrop);
} else {
   initTableDragAndDrop();
}

async function handleTxtarFileSelect(event, isDir) {
   let lines = typeof getTxtarPaths === 'function' ? getTxtarPaths() : [];
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
           let p = file.path || (file.webkitRelativePath ? file.webkitRelativePath : file.name);
           if (p) {
               try {
                   const res = await fetch('/api/resolve_path?name=' + encodeURIComponent(p));
                   const data = await res.json();
                   if (data.candidates && data.candidates.length > 0) {
                       p = data.candidates[0];
                   } else if (data.path) {
                       p = data.path;
                   }
               } catch (e) {
                   console.error("Path resolution failed", e);
               }
           }
           if (p && !lines.includes(p)) {
               lines.push(p);
           }
       }
   }
   if (typeof setTxtarPaths === 'function') setTxtarPaths(lines);
   if (typeof saveTxtarState === 'function') saveTxtarState();
   event.target.value = '';
}