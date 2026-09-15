const CANDIDATE_PORTS = [8085, 8086, 8087, 8088, 8089];
let activeInstances = []; // [{ port, name }]

async function probeInstances() {
  const found = [];
  for (const port of CANDIDATE_PORTS) {
    try {
      const controller = new AbortController();
      const id = setTimeout(() => controller.abort(), 300);
      const res = await fetch(`http://localhost:${port}/api/info`, { signal: controller.signal });
      clearTimeout(id);
      if (res.ok) {
        const info = await res.json();
        const projName = info.repo_name || `port:${port}`;
        found.push({ port, name: projName });
      } else {
        // Fallback for older server instances without /api/info
        const fallbackRes = await fetch(`http://localhost:${port}/api/sets`);
        if (fallbackRes.ok) {
          found.push({ port, name: `port:${port}` });
        }
      }
    } catch (_) {}
  }

  // Update DOM only if instance count or identities changed
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

async function executeAppyAction(inst, btn) {
  const origText = btn.innerText;
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
      btn.innerText = "⏳ Patching...";
      const res = await fetch(`${base}/api/apply`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ bundle: cleaned })
      });
      const data = await res.json();
      
      let ledgerText = "";
      if (data.error) {
        ledgerText = `❌ Appy Patch Error [${inst.name}]:\n${data.error}`;
      } else if (data.ledger) {
        ledgerText = typeof data.ledger === "string" ? data.ledger : JSON.stringify(data.ledger, null, 2);
      } else if (data.output) {
        ledgerText = data.output;
      } else {
        ledgerText = `✓ Patch applied successfully to ${inst.name}.`;
      }

      await navigator.clipboard.writeText(ledgerText);
      btn.innerText = "✓ Log Copied!";
      setTimeout(() => {
        btn.innerText = origText;
        btn.disabled = false;
      }, 2000);

    } else {
      btn.innerText = "⏳ Bundling...";
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
        body: JSON.stringify({ paths, excludes: ["*_test.go", ".git", "vendor"] })
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

      btn.innerText = "✓ Primed!";
      setTimeout(() => {
        btn.innerText = origText;
        btn.disabled = false;
      }, 2000);
    }
  } catch (err) {
    alert(`Appy [${inst.name}] failed: ` + err.message);
    btn.innerText = origText;
    btn.disabled = false;
  }
}

function renderButtons() {
  const container = document.getElementById("appy-smart-container");
  if (!container) return;
  container.innerHTML = "";

  if (activeInstances.length === 0) {
    const none = document.createElement("div");
    none.style.cssText = "font-size: 11px; color: #ef4444; background: rgba(15,23,42,0.85); padding: 4px 8px; border-radius: 4px; border: 1px solid #334155;";
    none.innerText = "No Appy Found";
    container.appendChild(none);
    return;
  }

  activeInstances.forEach((inst, idx) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.innerText = `⚡ ${inst.name}`;
    btn.title = `Target port :${inst.port} (${inst.name}) - auto-detects patch vs bundle`;
    
    // Distinguish buttons slightly by color palette
    const colors = ["#0284c7", "#0d9488", "#6366f1", "#d97706"];
    const baseColor = colors[idx % colors.length];

    btn.style.cssText = `
      background: ${baseColor};
      color: #f8fafc;
      border: 1px solid rgba(255, 255, 255, 0.25);
      border-radius: 6px;
      padding: 6px 10px;
      font-size: 11.5px;
      font-weight: bold;
      cursor: pointer;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
      white-space: nowrap;
      transition: filter 0.15s, transform 0.1s;
      width: 100%;
      text-align: center;
    `;
    btn.onmouseover = () => { btn.style.filter = "brightness(1.15)"; };
    btn.onmouseout = () => { btn.style.filter = "none"; };
    btn.onmousedown = () => { btn.style.transform = "scale(0.97)"; };
    btn.onmouseup = () => { btn.style.transform = "scale(1)"; };
    btn.onclick = () => executeAppyAction(inst, btn);

    container.appendChild(btn);
  });
}

function injectControls() {
  if (document.getElementById("appy-smart-container")) return;

  const anchor = document.querySelector("chat-window, [role='region'], main, form") || document.body;
  const inputRow = anchor.querySelector(".input-area, [contenteditable='true'], textarea");
  if (!inputRow) return;

  const targetBox = inputRow.closest("form") || inputRow.parentElement;
  if (!targetBox) return;

  targetBox.style.position = "relative";

  const container = document.createElement("div");
  container.id = "appy-smart-container";
  container.style.cssText = `
    position: absolute;
    left: -130px;
    bottom: 8px;
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 6px;
    z-index: 1000;
  `;

  targetBox.appendChild(container);
  probeInstances();
}

const observer = new MutationObserver(injectControls);
observer.observe(document.body, { childList: true, subtree: true });
injectControls();
setInterval(probeInstances, 5000);