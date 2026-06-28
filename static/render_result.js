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
                    const rhs = el.querySelector('.rhs-chips');
                    if (rhs && !rhs.querySelector('.reset-stripe-btn')) {
                        const btn = document.createElement('button');
                        btn.type = 'button';
                        btn.className = 'reset-stripe-btn';
                        btn.onclick = (e) => forgetStripe(e, f.path);
                        btn.style.cssText = 'margin-right: 8px; font-size: 11px; padding: 2px 8px; height: 22px; background: transparent; border: 1px solid #64748b; color: #94a3b8; border-radius: 4px; cursor: pointer; transition: all 0.2s;';
                        btn.onmouseover = function () { this.style.color = '#f8fafc'; this.style.borderColor = '#94a3b8'; this.style.background = '#334155'; };
                        btn.onmouseout = function () { this.style.color = '#94a3b8'; this.style.borderColor = '#64748b'; this.style.background = 'transparent'; };
                        btn.title = "Forget this patch in the ledger so it can be applied again";
                        btn.innerText = '↺ Reset';
                        rhs.insertBefore(btn, badge);
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