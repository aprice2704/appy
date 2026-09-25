const CANDIDATE_PORTS = [8085, 8086, 8087, 8088, 8089];
let activeInstances = []; // [{ port, name }]
let isProbing = false;

async function probeInstances() {
 if (isProbing) return;
 isProbing = true;
 const found = [];
 for (const port of CANDIDATE_PORTS) {
   try {
     const controller = new AbortController();
     const id = setTimeout(() => controller.abort(), 250);
     const res = await fetch(`http://localhost:${port}/api/info`, { signal: controller.signal });
     clearTimeout(id);
     if (res.ok) {
       const info = await res.json();
       const projName = info.repo_name || `port:${port}`;
       found.push({ port, name: projName });
     } else {
       const fallbackRes = await fetch(`http://localhost:${port}/api/sets`);
       if (fallbackRes.ok) {
         found.push({ port, name: `port:${port}` });
       }
     }
   } catch (_) {}
 }
 isProbing = false;

 const currentKey = activeInstances.map(i => `${i.port}:${i.name}`).join('|');
 const newKey = found.map(i => `${i.port}:${i.name}`).join('|');
 if (currentKey !== newKey) {
   activeInstances = found;
   renderButtons();
 }
}

function isPatchPayload(text) {
 const trimmed = text.trim();
 return trimmed.includes("@@@%%%") || 
        trimmed.includes("@@@") || 
        trimmed.startsWith("%%% filename:") ||
        trimmed.includes("\n%%% filename:");
}

function scrollToBottom() {
 let scrolled = false;
 const chatApp = document.querySelector("chat-app");
 if (chatApp) {
   const roots = [chatApp];
   if (chatApp.shadowRoot) roots.push(chatApp.shadowRoot);
   for (const root of roots) {
     const candidates = root.querySelectorAll("*");
     for (const el of candidates) {
       if (el.scrollHeight > el.clientHeight + 30) {
         const ov = window.getComputedStyle(el).overflowY;
         if (ov === "auto" || ov === "scroll") {
           el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
           scrolled = true;
         }
       }
     }
   }
 }

 const selectors = [
   "chat-window",
   "ms-chat-window",
   ".chat-history",
   ".conversation-container",
   "[data-testid*='conversation']",
   "infinite-scroller",
   "main",
   ".scrollable-container"
 ];
 for (const sel of selectors) {
   const el = document.querySelector(sel);
   if (el && el.scrollHeight > el.clientHeight + 30) {
     el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
     scrolled = true;
   }
 }

 const inputRow = document.querySelector(".input-area, [contenteditable='true'], textarea, rich-textarea");
 if (inputRow) {
   let curr = inputRow.parentElement;
   while (curr && curr !== document.body) {
     const overflow = window.getComputedStyle(curr).overflowY;
     if ((overflow === 'auto' || overflow === 'scroll') && curr.scrollHeight > curr.clientHeight + 30) {
       curr.scrollTo({ top: curr.scrollHeight, behavior: "smooth" });
       scrolled = true;
     }
     curr = curr.parentElement;
   }
 }

 if (!scrolled) {
   if (document.scrollingElement) {
     document.scrollingElement.scrollTo({ top: document.scrollingElement.scrollHeight, behavior: "smooth" });
   }
   window.scrollTo({ top: document.body.scrollHeight || document.documentElement.scrollHeight, behavior: "smooth" });
 }
}

async function executeRunClipAction(inst, btn) {
 const origText = btn.innerText;
 const origBg = btn.style.background;
 const origBorder = btn.style.borderColor;
 btn.disabled = true;

 try {
   let cmd = "";
   if (!navigator.clipboard || !navigator.clipboard.readText) {
     throw new Error("Clipboard access not available.");
   }
   cmd = await navigator.clipboard.readText();
   cmd = (cmd || "").trim();

   if (cmd.startsWith("$ ")) {
     cmd = cmd.substring(2).trim();
   } else if (cmd.startsWith("% ")) {
     cmd = cmd.substring(2).trim();
   }

   if (!cmd) {
     alert("Clipboard is empty.");
     btn.innerText = origText;
     btn.disabled = false;
     return;
   }

   btn.innerText = "⏳";
   const base = `http://localhost:${inst.port}`;
   const res = await fetch(`${base}/api/exec_bash`, {
     method: "POST",
     headers: { "Content-Type": "application/json" },
     body: JSON.stringify({ command: cmd, timeout: 120 })
   });
   const data = await res.json();

   const output = data.output || (data.error ? ("Error: " + data.error) : "No output produced.");
   try {
     await navigator.clipboard.writeText(output);
   } catch (copyErr) {
     console.warn("Auto-copy bash output failed:", copyErr);
   }

   if (data.exit_code === 0) {
     btn.innerText = "✓";
     btn.style.background = "#16a34a";
     btn.style.borderColor = "#4ade80";
     btn.style.boxShadow = "0 0 10px rgba(22, 163, 74, 0.7)";
   } else {
     btn.innerText = `🚨${data.exit_code}`;
     btn.style.background = "#dc2626";
     btn.style.borderColor = "#f87171";
     btn.style.boxShadow = "0 0 12px rgba(220, 38, 38, 0.8)";
   }

   setTimeout(() => {
     btn.innerText = origText;
     btn.style.background = origBg;
     btn.style.borderColor = origBorder;
     btn.style.boxShadow = "0 2px 6px rgba(0, 0, 0, 0.25)";
     btn.disabled = false;
   }, 3000);
 } catch (err) {
   alert(`Run Bash [${inst.name}] failed: ` + err.message);
   btn.innerText = origText;
   btn.style.background = origBg;
   btn.style.borderColor = origBorder;
   btn.style.boxShadow = "0 2px 6px rgba(0, 0, 0, 0.25)";
   btn.disabled = false;
 }
}

async function executeAppyAction(inst, btn) {
 const origText = btn.innerText;
 const origBg = btn.style.background;
 const origBorder = btn.style.borderColor;
 btn.disabled = true;

 try {
   const text = await navigator.clipboard.readText();
   const cleaned = (text || "").trim();

   if (!cleaned) {
     alert("Clipboard is empty.");
     btn.innerText = origText;
     btn.disabled = false;
     return;
   }

   const base = `http://localhost:${inst.port}`;

   if (isPatchPayload(cleaned)) {
     btn.innerText = "⏳";
     const res = await fetch(`${base}/api/apply`, {
       method: "POST",
       headers: { "Content-Type": "application/json" },
       body: JSON.stringify({ bundle: cleaned })
     });
     const data = await res.json();

     let hasErrors = !res.ok;
     let files = data.files || [];
     const failedFiles = files.filter(f => !f.applied);
     if (failedFiles.length > 0) {
       hasErrors = true;
     }

     let ledgerText = "";
     if (data.error) {
       ledgerText = `❌ Appy Patch Error [${inst.name}]:\n${data.error}`;
     } else if (failedFiles.length > 0) {
       ledgerText = `**Appy Result Ledger** [${inst.name}]\n\n`;
       const successFiles = files.filter(f => f.applied);
       if (successFiles.length > 0) {
         ledgerText += `Committed files:\n` + successFiles.map(f => `- \`${f.path}\``).join('\n') + `\n\n`;
       }
       ledgerText += `Rejected files:\n`;
       for (const f of failedFiles) {
         ledgerText += `- \`${f.path}\` (status: rejected)\n`;
         if (f.error) ledgerText += `  Issue: ${f.error}\n`;
         if (f.failed_patch) {
           if (f.failed_patch.current_line_echo) {
             ledgerText += `  Current line echo: \`${f.failed_patch.current_line_echo}\`\n`;
           }
           if (f.failed_patch.llm_fallback_hint) {
             ledgerText += `  Fallback Strategy: ${f.failed_patch.llm_fallback_hint}\n`;
           }
         }
       }
     } else if (data.ledger) {
       ledgerText = typeof data.ledger === "string" ? data.ledger : JSON.stringify(data.ledger, null, 2);
     } else if (data.output) {
       ledgerText = data.output;
     } else {
       ledgerText = `✓ Patch applied successfully to ${inst.name}.`;
     }

     await navigator.clipboard.writeText(ledgerText);

     if (hasErrors) {
       btn.innerText = "🚨 Error";
       btn.style.background = "#dc2626";
       btn.style.borderColor = "#f87171";
       btn.style.boxShadow = "0 0 10px rgba(220, 38, 38, 0.8)";
     } else {
       btn.innerText = "✓ Ok";
       btn.style.background = "#16a34a";
       btn.style.borderColor = "#4ade80";
       btn.style.boxShadow = "0 0 8px rgba(22, 163, 74, 0.7)";
     }

     setTimeout(() => {
       btn.innerText = origText;
       btn.style.background = origBg;
       btn.style.borderColor = origBorder;
       btn.style.boxShadow = "0 2px 6px rgba(0, 0, 0, 0.25)";
       btn.disabled = false;
     }, 3000);

   } else {
     btn.innerText = "⏳";
     let stripped = cleaned.replace(/^txtar\s+c\s+/i, '').replace(/^txtar\s+/i, '');
     stripped = stripped.replace(/>\s*[^\s]+$/, '');
     stripped = stripped.replace(/\\\r?\n/g, ' ');
     const paths = stripped.split(/[\r\n\s]+/).map(s => s.trim()).filter(s => s && !s.startsWith('#') && s !== '--');

     if (paths.length === 0) {
       alert("No valid file paths or patch found on clipboard.");
       btn.innerText = origText;
       btn.disabled = false;
       return;
     }

     const buildRes = await fetch(`${base}/api/txtar`, {
       method: "POST",
       headers: { "Content-Type": "application/json" },
       body: JSON.stringify({ paths, excludes: [".git", "vendor", "node_modules"] })
     });
     const buildData = await buildRes.json();
     if (buildData.error) throw new Error(buildData.error);

     const copyRes = await fetch(`${base}/api/txtar_copy`, {
       method: "POST",
       headers: { "Content-Type": "application/json" },
       body: JSON.stringify({ file_name: buildData.file_name })
     });
     const copyData = await copyRes.json();
     if (copyData.error) throw new Error(copyData.error);

     btn.innerText = "✓ Primed";
     btn.style.background = "#059669";
     btn.style.borderColor = "#34d399";
     btn.style.boxShadow = "0 0 8px rgba(5, 150, 105, 0.7)";

     setTimeout(() => {
       btn.innerText = origText;
       btn.style.background = origBg;
       btn.style.borderColor = origBorder;
       btn.style.boxShadow = "0 2px 6px rgba(0, 0, 0, 0.25)";
       btn.disabled = false;
     }, 2000);
   }
 } catch (err) {
   alert(`Appy [${inst.name}] failed: ` + err.message);
   btn.innerText = origText;
   btn.style.background = origBg;
   btn.style.borderColor = origBorder;
   btn.style.boxShadow = "0 2px 6px rgba(0, 0, 0, 0.25)";
   btn.disabled = false;
 }
}

function renderButtons() {
 const container = document.getElementById("appy-smart-container");
 if (!container) return;
 container.innerHTML = "";

 const scrollBtn = document.createElement("button");
 scrollBtn.type = "button";
 scrollBtn.innerText = "⬇ Bottom";
 scrollBtn.title = "Scroll conversation to bottom";
 scrollBtn.style.cssText = `
   background: #334155;
   color: #f8fafc;
   border: 1px solid rgba(255, 255, 255, 0.2);
   border-radius: 5px;
   padding: 4px 6px;
   height: 25px;
   line-height: 15px;
   font-size: 10.5px;
   font-weight: bold;
   cursor: pointer;
   box-shadow: 0 2px 5px rgba(0, 0, 0, 0.25);
   white-space: nowrap;
   text-align: center;
   transition: filter 0.15s, transform 0.1s;
   width: 100%;
   box-sizing: border-box;
 `;
 scrollBtn.onmouseover = () => { scrollBtn.style.filter = "brightness(1.2)"; };
 scrollBtn.onmouseout = () => { scrollBtn.style.filter = "none"; };
 scrollBtn.onmousedown = () => { scrollBtn.style.transform = "scale(0.97)"; };
 scrollBtn.onmouseup = () => { scrollBtn.style.transform = "scale(1)"; };
 scrollBtn.onclick = scrollToBottom;
 container.appendChild(scrollBtn);

 if (activeInstances.length === 0) {
   const none = document.createElement("div");
   none.style.cssText = "font-size: 10px; color: #ef4444; background: rgba(15,23,42,0.85); padding: 4px 6px; border-radius: 4px; border: 1px solid #334155; text-align: center;";
   none.innerText = "No Appy Found";
   container.appendChild(none);
   return;
 }

 activeInstances.forEach((inst, idx) => {
   const row = document.createElement("div");
   row.style.cssText = "display: flex; gap: 4px; align-items: center; width: 100%;";

   const clipBtn = document.createElement("button");
   clipBtn.type = "button";
   clipBtn.innerText = "🐚 Bash";
   clipBtn.title = `Run clipboard bash command on ${inst.name} (:${inst.port}) and copy output`;
   clipBtn.style.cssText = `
     background: #475569;
     color: #f8fafc;
     border: 1px solid rgba(255, 255, 255, 0.25);
     border-radius: 5px;
     padding: 4px 4px;
     height: 26px;
     line-height: 16px;
     font-size: 10.5px;
     font-weight: bold;
     cursor: pointer;
     box-shadow: 0 2px 5px rgba(0, 0, 0, 0.25);
     white-space: nowrap;
     transition: filter 0.15s, transform 0.1s;
     flex: 1;
     min-width: 0;
     text-align: center;
     box-sizing: border-box;
   `;
   clipBtn.onmouseover = () => { clipBtn.style.filter = "brightness(1.2)"; };
   clipBtn.onmouseout = () => { clipBtn.style.filter = "none"; };
   clipBtn.onmousedown = () => { clipBtn.style.transform = "scale(0.97)"; };
   clipBtn.onmouseup = () => { clipBtn.style.transform = "scale(1)"; };
   clipBtn.onclick = () => executeRunClipAction(inst, clipBtn);

   const repoBtn = document.createElement("button");
   repoBtn.type = "button";
   repoBtn.innerText = `⚡ ${inst.name}`;
   repoBtn.title = `Target port :${inst.port} (${inst.name}) - auto-detects patch vs bundle`;

   const colors = ["#0284c7", "#0d9488", "#6366f1", "#d97706"];
   const baseColor = colors[idx % colors.length];

   repoBtn.style.cssText = `
     background: ${baseColor};
     color: #f8fafc;
     border: 1px solid rgba(255, 255, 255, 0.25);
     border-radius: 5px;
     padding: 4px 2px;
     height: 26px;
     line-height: 16px;
     font-size: 10.5px;
     font-weight: bold;
     cursor: pointer;
     box-shadow: 0 2px 5px rgba(0, 0, 0, 0.25);
     white-space: nowrap;
     transition: filter 0.15s, transform 0.1s;
     flex: 0 0 50%;
     width: 50%;
     max-width: 50%;
     min-width: 0;
     overflow: hidden;
     text-overflow: ellipsis;
     text-align: center;
     box-sizing: border-box;
   `;
   repoBtn.onmouseover = () => { repoBtn.style.filter = "brightness(1.15)"; };
   repoBtn.onmouseout = () => { repoBtn.style.filter = "none"; };
   repoBtn.onmousedown = () => { repoBtn.style.transform = "scale(0.97)"; };
   repoBtn.onmouseup = () => { repoBtn.style.transform = "scale(1)"; };
   repoBtn.onclick = () => executeAppyAction(inst, repoBtn);

   row.appendChild(clipBtn);
   row.appendChild(repoBtn);
   container.appendChild(row);
 });
}

function injectControls() {
 const existing = document.getElementById("appy-smart-container");
 if (existing && existing.isConnected) {
   return;
 }
 if (existing && !existing.isConnected) {
   existing.remove();
 }

 const inputRow = document.querySelector(".input-area, [contenteditable='true'], textarea, rich-textarea");
 if (!inputRow) return;

 const targetBox = inputRow.closest("form") || inputRow.closest(".input-area") || inputRow.parentElement;
 if (!targetBox) return;

 if (window.getComputedStyle(targetBox).position === "static") {
   targetBox.style.position = "relative";
 }

 const container = document.createElement("div");
 container.id = "appy-smart-container";
 container.style.cssText = `
   position: absolute;
   left: -172px;
   bottom: 70px;
   display: flex;
   flex-direction: column;
   align-items: stretch;
   gap: 5px;
   z-index: 9999;
   width: 140px;
 `;

 targetBox.appendChild(container);
 renderButtons();
}

let injectDebounceTimer = null;
const observer = new MutationObserver(() => {
 const existing = document.getElementById("appy-smart-container");
 if (existing && existing.isConnected) {
   return;
 }
 if (injectDebounceTimer) clearTimeout(injectDebounceTimer);
 injectDebounceTimer = setTimeout(injectControls, 250);
});

observer.observe(document.body, { childList: true, subtree: true });
injectControls();
probeInstances();
setInterval(probeInstances, 5000);