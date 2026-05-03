// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: CSS styles for the Appy UI.
// :: filename: code/cmd/appy/ui_css.go
// :: serialization: go

package main

const cssStyles = `
       body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #1e1e1e; color: #d4d4d4; margin: 0; padding: 20px; }
       .container { display: flex; flex-direction: column; height: 95vh; max-width: 1200px; margin: 0 auto; gap: 20px; }
       textarea { flex: 1; background: #252526; color: #d4d4d4; border: 1px solid #3c3c3c; padding: 15px; font-family: 'Consolas', 'Courier New', monospace; font-size: 14px; resize: none; border-radius: 4px; outline: none; }
       textarea:focus { border-color: #007acc; }
       .controls { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
       button { padding: 10px 20px; border: none; border-radius: 4px; font-size: 14px; cursor: pointer; font-weight: 500; transition: opacity 0.2s; }
       button:hover { opacity: 0.9; }
       button:disabled { opacity: 0.5; cursor: not-allowed; }
       #applyBtn { background: #007acc; color: white; }
       #applyBtn.ready { background: #4caf50; }
       .output { flex: 1; background: #1e1e1e; border: 1px solid #3c3c3c; padding: 15px; overflow-y: auto; border-radius: 4px; font-family: 'Consolas', 'Courier New', monospace; font-size: 13px; line-height: 1.5; }
       .file-block { margin-bottom: 15px; border: 1px solid #333; border-radius: 4px; overflow: hidden; background: #1e1e1e; transition: all 0.2s; }
       .file-block.status-ok { border-color: #4caf50; }
       .file-block.status-ok .file-header { background: rgba(76, 175, 80, 0.15); color: #e8f5e9; }
       .file-block.status-applied { border-color: #555; }
       .file-block.status-applied .file-header { background: rgba(136, 136, 136, 0.15); color: #aaa; }
       .file-block.status-error { border-color: #f44336; }
       .file-block.status-error .file-header { background: rgba(244, 67, 54, 0.15); color: #ffebee; }
       .file-block.status-ignored { border-color: #ff9800; }
       .file-block.status-ignored .file-header { background: rgba(255, 152, 0, 0.15); color: #fff3e0; }
       .file-header { background: #2d2d2d; padding: 8px 12px; cursor: pointer; display: flex; justify-content: space-between; align-items: center; user-select: none; transition: background 0.2s; }
       .file-header:hover { filter: brightness(1.2); }
       .file-content { padding: 10px; display: none; }
       .file-block[open] .file-content { display: block; }
       .patch-block { margin-bottom: 15px; padding-bottom: 15px; border-bottom: 1px dashed #444; }
       .patch-block:last-child { border-bottom: none; margin-bottom: 0; padding-bottom: 0; }
       .status-badge { padding: 2px 8px; border-radius: 12px; font-size: 11px; font-weight: bold; text-transform: uppercase; }
       .status-ok { background: rgba(76, 175, 80, 0.2); color: #4caf50; }
       .status-error { background: rgba(244, 67, 54, 0.2); color: #f44336; }
       .status-ignored { background: rgba(255, 152, 0, 0.2); color: #ff9800; }
       .status-applied { background: rgba(136, 136, 136, 0.2); color: #888; }
       .error-msg { color: #f44336; margin-top: 5px; font-size: 12px; }
       .replace-block { color: #81c995; margin-top: 5px; white-space: pre-wrap; background: rgba(129, 201, 149, 0.1); padding: 5px; border-left: 3px solid #81c995;}
       .replace-block.applied { color: #888; background: rgba(136, 136, 136, 0.1); border-left: 3px solid #888;}
       .hint-block { color: #9cdcfe; margin-top: 5px; white-space: pre-wrap; background: rgba(156, 220, 254, 0.1); padding: 5px; border-left: 3px dashed #9cdcfe;}
       .success { color: #4caf50; font-weight: bold; }
       .error { color: #f44336; }
       .warning { color: #ff9800; }
       #unarmorBtn { background: #9c27b0; color: #fff; max-width: 0; padding-left: 0; padding-right: 0; opacity: 0; overflow: hidden; white-space: nowrap; transition: all 0.3s ease; display: none; }
       #unarmorBtn.show { max-width: 200px; padding-left: 20px; padding-right: 20px; opacity: 1; }
`
