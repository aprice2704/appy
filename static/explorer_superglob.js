// Appy Explorer: Super-Glob Evaluation & Transfer
let sgEvalTimeout;
let evaluatedResultPaths = [];

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

function sendExplorerToBuilder(replace = false) {
   const raw = (document.getElementById('superGlobInput').value || '').trim();
   if (!raw && evaluatedResultPaths.length === 0) {
       alert("No paths selected in Explorer.");
       return;
   }

   let targetPaths = replace ? [] : getTxtarPaths();
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

   const btnId = replace ? 'sendExplorerReplaceBtn' : 'sendExplorerAddBtn';
   const sendBtn = document.getElementById(btnId);
   if (sendBtn) {
       const origText = sendBtn.innerText;
       const origBg = sendBtn.style.background;
       const origBorder = sendBtn.style.borderColor;
       sendBtn.innerText = replace ? `✓ Replaced (${addedCount})` : `✓ Added (${addedCount})`;
       sendBtn.style.background = "#16a34a";
       sendBtn.style.borderColor = "#22c55e";
       setTimeout(() => {
           sendBtn.innerText = origText;
           sendBtn.style.background = origBg;
           sendBtn.style.borderColor = origBorder;
       }, 1500);
   }
}