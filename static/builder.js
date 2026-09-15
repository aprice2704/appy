// Appy Builder: Core Table State & Path Management
// Appy Builder: Core Table State & Path Management

function safeEscapeHtml(unsafe) {
   if (!unsafe) return "";
   return String(unsafe).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function getTxtarPaths() {
   const inputs = document.querySelectorAll('.path-input');
   return Array.from(inputs).map(i => i.value).filter(v => v.trim() !== '');
}

function setTxtarPaths(paths) {
   const tbody = document.getElementById('txtarPathsBody');
   if (!tbody) return;
   tbody.innerHTML = '';
   const cleanPaths = (paths || []).map(p => (p || '').trim()).filter(p => p !== '');
   if (cleanPaths.length === 0) {
       addPathRow('.');
       return;
   }
   cleanPaths.forEach(p => addPathRow(p));
   if (typeof scheduleTxtarStatsUpdate === 'function') {
       scheduleTxtarStatsUpdate();
   }
}

function addPathRow(val) {
   const tbody = document.getElementById('txtarPathsBody');
   if (!tbody) return;
   const rowVal = val !== undefined && val !== null ? val : '';
   const tr = document.createElement('tr');
   tr.innerHTML = `
     <td class="stat-files" style="text-align: center; padding: 4px; color: #94a3b8;">-</td>
     <td class="stat-tokens" style="text-align: center; padding: 4px; color: #94a3b8;">-</td>
     <td style="padding: 4px;"><input type="text" class="path-input" value="${safeEscapeHtml(rowVal)}" oninput="if(typeof saveTxtarState==='function')saveTxtarState()" onkeydown="handlePathKeydown(event)" style="width: 100%; background: transparent; border: none; color: inherit; outline: none; font-family: monospace;"></td>
     <td style="text-align: center; padding: 4px;"><button onclick="this.closest('tr').remove(); if(typeof saveTxtarState==='function')saveTxtarState();"
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
                   let lcp = data.suggestions[0];
                   for (let s of data.suggestions) {
                       while (!s.startsWith(lcp) && lcp.length > 0) {
                           lcp = lcp.slice(0, -1);
                       }
                   }
                   if (lcp.length > val.length) {
                       input.value = lcp;
                   }
               }
               if (typeof saveTxtarState === 'function') saveTxtarState();
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
   const excludesEl = document.getElementById('txtarExcludes');
   if (!excludesEl) return;
   const lines = excludesEl.value.split('\n').map(l => l.trim());
   chk.checked = lines.includes('*_test.go');
}

function toggleExcludeTests(checked) {
   const textarea = document.getElementById('txtarExcludes');
   if (!textarea) return;
   let lines = textarea.value.split('\n').map(l => l.trim()).filter(l => l !== '');
   if (checked) {
       if (!lines.includes('*_test.go')) {
           lines.unshift('*_test.go');
       }
   } else {
       lines = lines.filter(l => l !== '*_test.go');
   }
   textarea.value = lines.join('\n');
   if (typeof saveTxtarState === 'function') saveTxtarState();
}

function onExcludesInput() {
   syncExcludeTestsCheckbox();
   if (typeof saveTxtarState === 'function') saveTxtarState();
}

function toggleGlobHelp() {
   const helpDiv = document.getElementById('globHelp');
   if (helpDiv) {
       helpDiv.style.display = helpDiv.style.display === 'none' ? 'block' : 'none';
   }
}

function clearTxtarPaths() {
   setTxtarPaths([]);
   if (typeof saveTxtarState === 'function') saveTxtarState();
}