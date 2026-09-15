// Appy Builder: Clipboard Pasting & Ingestion
async function replaceTxtarCommand() {
   await pasteTxtarCommand(true);
}

async function pasteTxtarCommand(replace = false) {
   try {
       if (!navigator.clipboard || !navigator.clipboard.readText) {
           const manual = prompt("Paste paths or txtar command here:");
           if (!manual) return;
           applyPastedText(manual, replace);
           return;
       }
       const text = await navigator.clipboard.readText();
       applyPastedText(text, replace);
   } catch (err) {
       console.error("Paste failed", err);
       const manual = prompt("Clipboard read failed (" + err.message + "). Paste paths here manually:");
       if (manual) applyPastedText(manual, replace);
   }
}

function applyPastedText(text, replace = false) {
   let cleaned = (text || "").trim();
   if (!cleaned) return;
   cleaned = cleaned.replace(/^txtar\s+c\s+/i, '').replace(/^txtar\s+/i, '');
   cleaned = cleaned.replace(/>\s*[^\s]+$/, '');
   cleaned = cleaned.replace(/\\\r?\n/g, ' ');
   const paths = cleaned.split(/[\r\n\s]+/).map(p => p.trim()).filter(p => p.length > 0 && !p.startsWith('#') && p !== '--');
   if (paths.length === 0) return;

      let existing = replace ? [] : getTxtarPaths();
   paths.forEach(p => {
       if (!existing.includes(p)) existing.push(p);
   });
   setTxtarPaths(existing);
   saveTxtarState();
}

async function autoBundleFromClipboard() {
   const btn = document.getElementById('autoBundleClipBtn');
   const origText = btn ? btn.innerText : '⚡ Auto Bundle';
   if (btn) {
       btn.innerText = '⏳ Bundling...';
       btn.disabled = true;
   }
   try {
       const scratchRadio = document.getElementById('modeScratchpad');
       if (scratchRadio && !scratchRadio.checked) {
           scratchRadio.checked = true;
           if (typeof toggleBuilderMode === 'function') toggleBuilderMode();
       }
       await replaceTxtarCommand();
       if (typeof buildAndCopyTxtar === 'function') {
           await buildAndCopyTxtar();
       }
       if (btn) {
           btn.innerText = '✓ Copied!';
           setTimeout(() => {
               btn.innerText = origText;
               btn.disabled = false;
           }, 2000);
       }
   } catch (err) {
       alert('Auto bundle failed: ' + err.message);
       if (btn) {
           btn.innerText = origText;
           btn.disabled = false;
       }
   }
}