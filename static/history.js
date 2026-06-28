async function loadHistory() {
    const histEl = document.getElementById('historyContent');
    histEl.innerHTML = "<em>Loading history...</em>";
    try {
        const res = await fetch('/api/history');
        const data = await res.json();
        if (!data.history || data.history.length === 0) {
            histEl.innerHTML = "<em>No history available.</em>";
            return;
        }
        let html = "";
        data.history.forEach(tx => {
            const d = new Date(tx.timestamp * 1000).toLocaleString();
            html += '<div class="file-block status-applied" style="padding: 15px; margin-bottom: 10px;">';
            html += '<div style="font-weight: bold; margin-bottom: 8px;">' + escapeHtml(d) + ' <span style="font-size: 11px; color: #94a3b8; font-weight: normal;">(' + escapeHtml(tx.tx_id) + ')</span></div>';
            tx.files.forEach(f => {
                html += '<div style="font-family: monospace; font-size: 12px; color: #cbd5e1; margin-bottom: 4px;">';
                html += (f.existed ? '<span style="color: #fbbf24;">[MOD/DEL]</span> ' : '<span style="color: #4ade80;">[CREATE]</span> ') + escapeHtml(f.path);
                html += '</div>';
            });
            html += '<button onclick="revertTx(\'' + escapeHtml(tx.tx_id) + '\')" style="margin-top: 10px; height: 30px; min-width: 100px; background: #dc2626; color: #f8fafc; border: 1px solid #ef4444; border-radius: 4px; cursor: pointer;">Revert This Patch</button>';
            html += '</div>';
        });
        histEl.innerHTML = html;
    } catch (err) {
        histEl.innerHTML = "<div class='error'>Failed to load history: " + err.message + "</div>";
    }
}

async function revertTx(txId) {
    if (!confirm("Are you sure you want to revert " + txId + "? This will restore original file contents and CANNOT be undone.")) return;
    try {
        const res = await fetch('/api/revert', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ tx_id: txId })
        });
        const data = await res.json();
        if (data.error) {
            alert("Revert failed: " + data.error);
            return;
        }
        alert("Revert successful. Reloading history.");
        loadHistory();
    } catch (err) {
        alert("Revert failed: " + err.message);
    }
}