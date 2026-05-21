// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 4
// :: description: Appy v1.5.22 UI Layout matching anti-shift spec.
// :: filename: ui_html.go
// :: serialization: go

package main

const htmlTop = `<!DOCTYPE html>
<html lang="en">
<head>
   <meta charset="UTF-8">
       <link rel="icon" href='data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">🚀</text></svg>'>
   <title>{TITLE} | Appy</title>
   <style>`

const htmlMiddle = `</style>
</head>
<body>
   <div class="container">
       <div class="header-zone">
           <h2 style="margin: 0; display: flex; align-items: baseline; gap: 10px;">
               {TITLE} | Appy 
               <span style="font-size: 0.6em; color: #888; font-weight: normal;">{VERSION}</span>
           </h2>
           <div style="font-size: 13px; color: #64748b; margin-top: 5px; font-family: monospace;">
               Sandboxed to: {ROOT_DIR}
           </div>
       </div>
      
        <textarea id="bundleInput" placeholder="Paste the raw LLM output here (including the markdown code blocks and %%% syntax)..." oninput="debouncePreview()"></textarea>
       
       <div class="controls">
                               <div class="control-group prep-group">
              <button id="autoBtn" onclick="runAutoPilot()">🤖 Auto</button>
              <button id="clearPasteBtn" onclick="clearAndPaste()">📋 Paste</button>
              <button id="fixPathsBtn" onclick="fixFilePaths()" style="display: none;">🔧 Fix Paths</button>
              <button id="historyBtn" onclick="toggleHistory()">⏪ History</button>
          </div>
          
          <div class="control-group action-group">
              <button id="checkBtn" onclick="checkSyntax()" disabled>🧪 Check</button>
              <button id="applyBtn" onclick="applyBundle()" disabled>🚀 Apply</button>
              <button id="retestBtn" onclick="runRetest()" style="display: none;">🔄 Retest</button>
              <button id="cancelRetestBtn" onclick="cancelRetest()" style="display: none;">⏹️ Stop</button>
          </div>
          
          <div class="control-group export-group">
              <button id="copyTraceBtn" onclick="copyTraceReport()" style="display: none;">📋 Copy</button>
          </div>
       </div>
       
             <div id="output" class="output">
          <em>Waiting for input...</em>
      </div>
      <div id="historyOutput" class="output" style="display: none;"></div>
  </div>
   <script>`

const htmlBottom = `</script>
</body>
</html>`
