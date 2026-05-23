function renderResult(data, isCheck) {
    const applyBtn = document.getElementById('applyBtn');
    const checkBtn = document.getElementById('checkBtn');
    const retestBtn = document.getElementById('retestBtn');

    if (isCheck) {
        if (!data.files) return;
        let compilerErrorsReport = "**Appy Compiler Pre-Flight Errors**\n\n";
        let hasFails = false;
        data.files.forEach(f => {
            const el = document.getElementById('file-block-' + escapeHtml(f.path));
            if (el) {
                if (f.compiler_status === 'PASS') {
                    addDecorator(el, '🏅');
                } else if (f.compiler_status === 'FAIL') {
                    hasFails = true;
                    el.className = 'file-block status-error';
                    const badge = el.querySelector('.status-badge');
                    if (badge) {
                        badge.className = 'status-badge status-error';
                        badge.innerText = 'ERROR';
                    }
                    addDecorator(el, '⚠️');
                    compilerErrorsReport += "- " + bt + f.path + bt + "\n";
                    if (f.raw_output) {
                        compilerErrorsReport += "  Compiler Trace:\n  " + tbt + "go\n  " + f.raw_output.replace(/\n/g, "\n  ") + "\n  " + tbt + "\n\n";
                    }
                }
            }
        });
        if (hasFails) {
            window.tracePayload = compilerErrorsReport;
            setExportMode('Compiler', 'error');
        }

        const anyOk = document.querySelectorAll('.file-block.status-ready').length > 0;
        applyBtn.disabled = !anyOk;
        if (anyOk) applyBtn.classList.add('ready');
        else applyBtn.classList.remove('ready');

        checkBtn.disabled = false;
        checkBtn.innerText = "🧪 Check";
        applyBtn.innerText = "🚀 Apply";
        return;
    }

      let appyVer = (data.__nd && data.__nd.appy_version) ? data.__nd.appy_version : "unknown";
  let ledger = "**Appy Result Ledger** (" + appyVer + ")\n\n";
    let successfulFiles = [];
    let rejectedFiles = [];
    if (data.files) {
        data.files.forEach(f => {
            if (f.applied) successfulFiles.push(f.path);
            else rejectedFiles.push(f);
        });
    }

    if (successfulFiles.length > 0) {
        ledger += "Committed files:\n" + successfulFiles.map(f => "- " + bt + f + bt).join('\n') + "\n\n";
    }
    if (rejectedFiles.length > 0) {
        ledger += "Rejected files:\n";
        rejectedFiles.forEach(f => {
            ledger += "- " + bt + f.path + bt + " (file_commit_status: rejected)\n";
            ledger += "  Issue: " + (f.error || "Unknown error") + "\n";
            if (f.failed_patch && f.failed_patch.current_line_echo) {
                ledger += "  Current line echo: " + bt + f.failed_patch.current_line_echo + bt + "\n";
            }
            if (f.failed_patch && f.failed_patch.llm_fallback_hint) {
                ledger += "  Fallback Strategy: " + f.failed_patch.llm_fallback_hint + "\n";
            }
        });
    }
    window.tracePayload = ledger;
    window.committedFiles = successfulFiles;
    if (data.files) {
        data.files.forEach(f => {
            const el = document.getElementById('file-block-' + escapeHtml(f.path));
            if (el) {
                if (f.applied) {
                    el.className = 'file-block status-applied';
                    const badge = el.querySelector('.status-badge');
                    if (badge) {
                        badge.className = 'status-badge status-applied';
                        badge.innerText = 'APPLIED';
                    }
                } else {
                    el.className = 'file-block status-error';
                    const badge = el.querySelector('.status-badge');
                    if (badge) {
                        badge.className = 'status-badge status-error';
                        badge.innerText = 'ERROR';
                    }
                    const content = el.querySelector('.file-content');
                    if (content) {
                        let errHtml = '<div class="patch-block" style="border-top: 2px solid #f44336; padding-top: 10px;">';
                        errHtml += '<div class="error-msg"><strong>Rejected:</strong> ' + escapeHtml(f.error) + '</div>';
                        if (f.failed_patch && f.failed_patch.current_line_echo) {
                            errHtml += '<div class="hint-block"><strong>Matched Line Echo:</strong><pre>' + escapeHtml(f.failed_patch.current_line_echo) + '</pre></div>';
                        }
                        if (f.failed_patch && f.failed_patch.llm_fallback_hint) {
                            errHtml += '<div class="hint-block" style="color:#2196f3; border-left: 3px solid #2196f3;"><strong>Advisory:</strong><br>' + escapeHtml(f.failed_patch.llm_fallback_hint) + '</div>';
                        }
                        errHtml += '</div>';
                        content.innerHTML += errHtml;
                    }
                }
            }
        });
    }

    if (rejectedFiles.length > 0 && successfulFiles.length > 0) {
        setExportMode('Apply', 'mixed');
    } else if (rejectedFiles.length > 0) {
        setExportMode('Apply', 'error');
    } else {
        setExportMode('Ledger', 'success');
    }

    if (successfulFiles.length > 0) {
        retestBtn.style.display = 'inline-block';
    }

    applyBtn.disabled = true;
    applyBtn.classList.remove('ready');
    applyBtn.innerText = "Applied!";
    setTimeout(() => { applyBtn.innerText = "🚀 Apply"; }, 2500);
}

function renderPreview(data) {
    const outputEl = document.getElementById('output');
    const applyBtn = document.getElementById('applyBtn');
    const retestBtn = document.getElementById('retestBtn');
    const fixPathsBtn = document.getElementById('fixPathsBtn');

    if (!data.files || data.files.length === 0) {
        outputEl.innerHTML = "<em>No valid patches found in bundle.</em>";
        applyBtn.disabled = true;
        setExportMode('none');
        retestBtn.style.display = 'none';
        return;
    }

    let html = '';
    let readyCount = 0;
    let errorCount = 0;
      let appyVer = (data.__nd && data.__nd.appy_version) ? data.__nd.appy_version : "unknown";
  let errorReport = "**Appy Preview Errors** (" + appyVer + ")\n\nRejected files:\n";
    data.files.forEach(fileObj => {
        const fileStatus = fileObj.status;

        if (fileStatus === 'ERROR') {
            errorCount++;
            errorReport += "- " + bt + fileObj.path + bt + "\n";
        } else if (fileStatus === 'READY') {
            readyCount++;
        }

        let statusClass = 'status-' + fileStatus.toLowerCase();
        let chipText = fileStatus === 'READY' ? 'OK' : fileStatus;
        let lineDeltaFmt = fileObj.net_lines > 0 ? ('+' + fileObj.net_lines) : fileObj.net_lines;
        let isOverwrite = fileObj.patches && fileObj.patches.some(p => p.is_overwrite);
        let isDelete = fileObj.patches && fileObj.patches.some(p => p.is_delete_file);
        let isAnchored = fileObj.patches && fileObj.patches.some(p => p.is_anchored);

        let fileTypeHtml = '';
        if (fileObj.file_type) {
            fileTypeHtml = '<span class="file-type-tag">' + escapeHtml(fileObj.file_icon) + ' ' + escapeHtml(fileObj.file_type) + '</span>';
        }

        html += '<details id="file-block-' + escapeHtml(fileObj.path) + '" class="file-block ' + statusClass + '">';
        html += '<summary class="file-header">';
        html += '<div style="display: flex; align-items: center;">';
        html += fileTypeHtml;
        html += '<strong>' + escapeHtml(fileObj.path) + '</strong>';
        html += '<span class="net-lines">' + lineDeltaFmt + '</span>';
        html += '</div>';
        html += '<div class="rhs-chips">';
        if (isOverwrite) {
            html += '<span class="decorator" style="font-size: 1.2em;">☢️</span>';
        }
        if (isDelete) {
            html += '<span class="decorator" style="font-size: 1.2em;">🗑️</span>';
        }
        if (isAnchored) {
            html += '<span class="decorator" style="font-size: 1.2em;">⚓</span>';
        }
        html += '<span class="status-badge ' + statusClass + '">' + chipText + '</span>';
        html += '</div></summary>';

        html += '<div class="file-content">';
        if (fileObj.patches) {
            fileObj.patches.forEach(p => {
                if (p.error) {
                    errorReport += "  Issue: " + p.error + "\n";
                    if (p.closest_match_hint) {
                        errorReport += "  Closest Match (Use this for match_line):\n  " + tbt + "\n  " + p.closest_match_hint.replace(/\n/g, "\n  ") + "\n  " + tbt + "\n";
                    }
                    if (p.llm_fallback_hint) {
                        errorReport += "  Fallback Strategy: " + p.llm_fallback_hint + "\n";
                    }
                }

                html += '<div class="patch-block">';
                if (p.error) html += '<div class="error-msg">' + escapeHtml(p.error) + '</div>';
                if (p.closest_match_hint) html += '<div class="hint-block"><strong>Closest Match:</strong><pre>' + escapeHtml(p.closest_match_hint) + '</pre></div>';
                if (p.llm_fallback_hint) html += '<div class="hint-block" style="color:#2196f3; border-left: 3px solid #2196f3;"><strong>Advisory:</strong><br>' + escapeHtml(p.llm_fallback_hint) + '</div>';
                if (p.search_block) {
                    html += '<details style="margin-bottom: 8px; cursor: pointer; color: #94a3b8;"><summary style="font-size: 12px; margin-bottom: 4px;">View Search Block (Old Text)</summary><pre style="margin:0; white-space: pre-wrap; font-family: inherit; background: rgba(0,0,0,0.2); padding: 10px; border-left: 3px solid #475569;">' + escapeHtml(p.search_block) + '</pre></details>';
                }
                if (p.is_delete_file) {
                    html += '<div class="replace-block" style="border-left: 3px solid #ef4444; background: rgba(239, 68, 68, 0.08); color: #fca5a5;"><strong>FILE MARKED FOR DELETION</strong></div>';
                } else if (p.replace_block !== undefined) {
                    html += '<div class="replace-block"><pre style="margin:0; white-space: pre-wrap; font-family: inherit;">' + escapeHtml(p.replace_block) + '</pre></div>';
                }
                html += '</div>';
            });
        }
        html += '</div></details>';
    });

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

    retestBtn.style.display = 'none';
    if (errorCount > 0) {
        window.tracePayload = errorReport;
        setExportMode('Preview', 'error');
    } else {
        setExportMode('none', 'none');
    }
}