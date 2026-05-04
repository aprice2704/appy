// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 2
// :: description: JS retest logic matched to new schema and DOM isolation rules.
// :: filename: ui_script_retest.go
// :: serialization: go

package main

const jsRetest = `
async function runRetest() {
    if (!window.committedFiles || window.committedFiles.length === 0) return;
    
    const packages = [...new Set(window.committedFiles.map(f => {
        const parts = f.split('/');
        parts.pop();
        return "./" + (parts.length > 0 ? parts.join('/') : '.');
    }))];
    
    retestBtn.innerText = "Running Tests...";
    retestBtn.disabled = true;
    
    const loadingHtml = "<div id='testLoading' class='patch-block' style='margin-top: 15px; border-top: 1px solid #555; padding-top: 15px;'><em>Running tests for " + packages.join(', ') + " ...</em></div>";
    outputEl.innerHTML += loadingHtml;

    try {
        const res = await fetch('/api/retest', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ packages: packages })
        });
        const data = await res.json();
        
        const loader = document.getElementById('testLoading');
        if (loader) loader.style.display = 'none';

        let reportMd = "**Appy Retest Report**\n\n";
        
        const hasFails = data.files && data.files.some(f => f.test_status === 'FAIL');
        reportMd += "Success: " + (!hasFails ? "✅" : "❌") + "\n\n";

        if (window.committedFiles) {
            window.committedFiles.forEach(f => {
                const el = document.getElementById('file-block-' + escapeHtml(f));
                if (el) {
                    let failItem = null;
                    if (hasFails && data.files) {
                        const parts = f.split('/');
                        parts.pop();
                        const folder = parts.length > 0 ? parts.join('/') : '';
                        failItem = data.files.find(fail => {
                            return fail.test_status === 'FAIL' && (folder === '' || (fail.package && fail.package.endsWith(folder)));
                        });
                    }
                    addDecorator(el, failItem ? '💥' : '🧪');
                }
            });
        }

        if (hasFails) {
            reportMd += "Hard Fails:\n";
            data.files.forEach(fail => {
                if (fail.test_status === 'FAIL') {
                    const pkg = fail.package || 'Unknown';
                    reportMd += "- **" + pkg + "**:\n  " + tbt + "text\n  " + (fail.raw_output ? fail.raw_output.trim().replace(/\n/g, "\n  ") : 'Failed') + "\n  " + tbt + "\n\n";
                }
            });
        }

        window.tracePayload = reportMd;
        setExportMode('test');
        
    } catch (err) {
        const loader = document.getElementById('testLoading');
        if (loader) loader.style.display = 'none';
        outputEl.innerHTML += "<div class='error' style='margin-top:15px;'>Test request failed: " + err.message + "</div>";
    } finally {
        retestBtn.innerText = "🔄 Retest Impacted";
        retestBtn.disabled = false;
    }
}

function cancelRetest() {
    console.log("Cancel retest not fully implemented on backend yet.");
}
`
