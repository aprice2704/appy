// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: JS retest logic for Appy UI.
// :: filename: code/cmd/appy/ui_script_retest.go
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
        reportMd += "Success: " + (data.success ? "✅" : "❌") + "\n\n";

        if (window.committedFiles) {
            window.committedFiles.forEach(f => {
                const el = document.getElementById('file-block-' + f);
                if (el) {
                    let failed = false;
                    if (!data.success && data.hard_fails) {
                        const parts = f.split('/');
                        parts.pop();
                        const folder = parts.length > 0 ? parts[parts.length - 1] : '';
                                                failed = data.hard_fails.some(fail => {
                            return folder === '' || (fail.Task && fail.Task.Package && fail.Task.Package.endsWith(folder));
                        });
                    }
                    addDecorator(el, failed ? '💥' : '🧪');
                }
            });
        }

        if (data.hard_fails && data.hard_fails.length > 0) {
            reportMd += "Hard Fails:\n";
            data.hard_fails.forEach(fail => {
                const pkg = fail.Task ? fail.Task.Package : 'Unknown';
                const testName = (fail.Task && fail.Task.Test) ? fail.Task.Test : 'Package';
                reportMd += "- **" + pkg + "** (" + testName + "):\n  " + tbt + "\n  " + (fail.Output ? fail.Output.trim() : 'Failed') + "\n  " + tbt + "\n";
            });
        }
        if (data.heisenfails && data.heisenfails.length > 0) {
            reportMd += "\nHeisenfails (Passed on rerun):\n";
            data.heisenfails.forEach(fail => {
                const pkg = fail.Task ? fail.Task.Package : 'Unknown';
                const testName = (fail.Task && fail.Task.Test) ? fail.Task.Test : 'Package';
                reportMd += "- **" + pkg + "** (" + testName + ")\n";
            });
        }

        window.lastLedgerText = reportMd;
        copyLedgerBtn.innerText = "📋 Copy Test Report";
        copyLedgerBtn.style.background = data.success ? "#16a34a" : "#dc2626";
        copyLedgerBtn.style.borderColor = data.success ? "#22c55e" : "#ef4444";
        copyLedgerBtn.style.display = 'inline-block';
        
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
