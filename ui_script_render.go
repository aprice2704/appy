// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: JS rendering logic for Appy UI.
// :: filename: code/cmd/appy/ui_script_render.go
// :: serialization: go

package main

const jsRender = `
function renderResult(data, isCheck) {
    if (isCheck) {
        if (data.successful_files_committed) {
            data.successful_files_committed.forEach(f => {
                const el = document.getElementById('file-block-' + f);
                if (el) addDecorator(el, '🏅');
            });
        }
        if (data.rejected_files) {
            for (const [file, details] of Object.entries(data.rejected_files)) {
                const el = document.getElementById('file-block-' + file);
                if (el) {
                    el.className = 'file-block status-error';
                    const badge = el.querySelector('.status-badge');
                    if (badge) {
                        badge.className = 'status-badge status-error';
                        badge.innerText = 'ERROR';
                    }
                    addDecorator(el, '❌');
                    
                    const content = el.querySelector('.file-content');
                    if (content && !content.querySelector('.error-msg.compiler-err')) {
                        let errHtml = '<div class="patch-block compiler-err" style="border-top: 2px solid #f44336; padding-top: 10px;">';
                        errHtml += '<div class="error-msg"><strong>Compiler Rejected:</strong> ' + escapeHtml(details.reason) + '</div>';
                        errHtml += '</div>';
                        content.innerHTML += errHtml;
                    }
                }
            }
        }
        
        const anyOk = document.querySelectorAll('.file-block.status-ok').length > 0;
        applyBtn.disabled = !anyOk;
        if (anyOk) applyBtn.classList.add('ready');
        else applyBtn.classList.remove('ready');
        
        return;
    }

    let ledger = "**Appy Result Ledger**\n\n";
    if (data.successful_files_committed && data.successful_files_committed.length > 0) {
        ledger += "Committed files:\n" + data.successful_files_committed.map(f => "- " + bt + f + bt).join('\n') + "\n\n";
    }
    if (data.rejected_files && Object.keys(data.rejected_files).length > 0) {
        ledger += "Rejected files:\n";
        for (const [file, details] of Object.entries(data.rejected_files)) {
            ledger += "- " + bt + file + bt + " (file_commit_status: rejected)\n";
            ledger += "  Issue: " + details.reason + "\n";
            if (details.failed_patch && details.failed_patch.current_line) {
                ledger += "  Current line echo: " + bt + details.failed_patch.current_line + bt + "\n";
            }
        }
    }
    window.lastLedgerText = ledger;
    window.committedFiles = data.successful_files_committed;

    if (data.successful_files_committed) {
        data.successful_files_committed.forEach(f => {
            const el = document.getElementById('file-block-' + f);
            if (el) {
                el.className = 'file-block status-applied';
                const badge = el.querySelector('.status-badge');
                if (badge) {
                    badge.className = 'status-badge status-applied';
                    badge.innerText = 'APPLIED';
                }
            }
        });
    }

    if (data.rejected_files) {
        for (const [file, details] of Object.entries(data.rejected_files)) {
            const el = document.getElementById('file-block-' + file);
            if (el) {
                el.className = 'file-block status-error';
                const badge = el.querySelector('.status-badge');
                if (badge) {
                    badge.className = 'status-badge status-error';
                    badge.innerText = 'ERROR';
                }
                const content = el.querySelector('.file-content');
                if (content) {
                    let errHtml = '<div class="patch-block" style="border-top: 2px solid #f44336; padding-top: 10px;">';
                    errHtml += '<div class="error-msg"><strong>Rejected:</strong> ' + escapeHtml(details.reason) + '</div>';
                    if (details.failed_patch && details.failed_patch.current_line) {
                        errHtml += '<div class="hint-block"><strong>Matched Line Echo:</strong><pre>' + escapeHtml(details.failed_patch.current_line) + '</pre></div>';
                    }
                    errHtml += '</div>';
                    content.innerHTML += errHtml;
                }
            }
        }
    }

    copyLedgerBtn.innerText = "✅ Copy Result Ledger";
    copyLedgerBtn.style.background = "";
    copyLedgerBtn.style.borderColor = "";
    copyLedgerBtn.style.display = 'inline-block';
    copyErrorsBtn.style.display = 'none';
    if (data.successful_files_committed && data.successful_files_committed.length > 0) {
        retestBtn.style.display = 'inline-block';
    }
    
    applyBtn.disabled = true;
    applyBtn.classList.remove('ready');
    applyBtn.innerText = "Applied!";
    setTimeout(() => { applyBtn.innerText = "🚀 Apply to Disk"; }, 2500);
}

function renderPreview(data) {
    if (!data.patches || Object.keys(data.patches).length === 0) {
        outputEl.innerHTML = "<em>No valid patches found in bundle.</em>";
        applyBtn.disabled = true;
        copyLedgerBtn.style.display = 'none';
        copyErrorsBtn.style.display = 'none';
        retestBtn.style.display = 'none';
        return;
    }

    let html = '';
    let readyCount = 0;
    let errorCount = 0;
    let errorReport = "**Appy Preview Errors**\n\nRejected files:\n";
    
    for (const [file, patches] of Object.entries(data.patches)) {
        let fileHasError = patches.some(p => p.status === 'error');
        let fileHasIgnored = patches.some(p => p.status === 'ignored');
        
        if (!fileHasError && !fileHasIgnored) {
            readyCount++;
        } else if (fileHasError) {
            errorCount++;
            errorReport += "- " + bt + file + bt + "\n";
        }
        
        let statusClass = fileHasError ? 'status-error' : (fileHasIgnored ? 'status-ignored' : 'status-ok');
        let chipText = fileHasError ? 'ERROR' : (fileHasIgnored ? 'IGNORED' : 'OK');
        
        html += '<details id="file-block-' + escapeHtml(file) + '" class="file-block ' + statusClass + '">';
        html += '<summary class="file-header" style="display: flex; align-items: center;">';
        html += '<div style="flex: 1; display: flex; justify-content: space-between; align-items: center;">';
        html += '<strong>' + escapeHtml(file) + '</strong>';
        html += '<span class="status-badge ' + statusClass + '">' + chipText + '</span>';
        html += '</div></summary>';
        
        html += '<div class="file-content">';
        patches.forEach(p => {
            if (p.status === 'error') {
                errorReport += "  Issue: " + (p.message || "Unknown error") + "\n";
                if (p.hint) {
                    errorReport += "  Hint/Closest Match:\n  " + tbt + "\n  " + p.hint.replace(/\n/g, "\n  ") + "\n  " + tbt + "\n";
                }
                if (p.advisory) {
                    errorReport += "  Advisory: " + p.advisory + "\n";
                }
            }

            html += '<div class="patch-block">';
            html += '<div style="display: flex; justify-content: space-between; margin-bottom: 8px;">';
            html += '<span class="status-badge status-' + p.status + '">' + p.status + '</span>';
            let delta = p.line_delta > 0 ? ('+' + p.line_delta) : p.line_delta;
            html += '<span style="font-size:11px; color:#888;">Net lines: ' + delta + '</span>';
            html += '</div>';

            if (p.message) html += '<div class="error-msg">' + escapeHtml(p.message) + '</div>';
            if (p.hint) html += '<div class="hint-block"><strong>Closest Match:</strong><pre>' + escapeHtml(p.hint) + '</pre></div>';
            if (p.advisory) html += '<div class="hint-block" style="color:#2196f3; border-left: 3px solid #2196f3;"><strong>Advisory:</strong><br>' + escapeHtml(p.advisory) + '</div>';
            
            if (p.replace !== undefined) {
                html += '<div class="replace-block"><pre style="margin:0; white-space: pre-wrap; font-family: inherit;">' + escapeHtml(p.replace) + '</pre></div>';
            }
            html += '</div>';
        });
        html += '</div></details>';
    }
    
    outputEl.innerHTML = html;
    
    applyBtn.disabled = (readyCount === 0);
    if (readyCount > 0) applyBtn.classList.add('ready');
    else applyBtn.classList.remove('ready');
    applyBtn.innerText = "🚀 Apply to Disk";

    if (data.path_fixes && Object.keys(data.path_fixes).length > 0) {
        fixPathsBtn.style.display = 'inline-block';
        window.pendingPathFixes = data.path_fixes;
    } else {
        fixPathsBtn.style.display = 'none';
    }
    
    copyLedgerBtn.style.display = 'none';
    retestBtn.style.display = 'none';

    if (errorCount > 0) {
        window.lastErrorText = errorReport;
        copyErrorsBtn.style.display = 'inline-block';
    } else {
        copyErrorsBtn.style.display = 'none';
    }
}
`
