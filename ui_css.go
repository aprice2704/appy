// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 2
// :: description: CSS styles for the Appy UI.
// :: filename: ui_css.go
// :: serialization: go

package main

const cssStyles = `
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen, Ubuntu, Cantarell, "Helvetica Neue", sans-serif; background: #0b0f19; color: #e2e8f0; margin: 0; padding: 20px; font-size: 15px; }
        .container { display: flex; flex-direction: column; height: 95vh; max-width: 1400px; margin: 0 auto; gap: 20px; }
        
        textarea { flex: 1; background: #121826; color: #a0aec0; border: 1px solid #2d3748; padding: 18px; font-family: 'Consolas', 'JetBrains Mono', monospace; font-size: 14px; resize: none; border-radius: 6px; outline: none; box-shadow: inset 0 2px 4px rgba(0,0,0,0.2); transition: border-color 0.2s; }
        textarea:focus { border-color: #3b82f6; box-shadow: 0 0 0 1px #3b82f6, inset 0 2px 4px rgba(0,0,0,0.2); }
        
        .controls { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; background: #121826; padding: 12px 18px; border-radius: 6px; border: 1px solid #2d3748; }
        
        button { padding: 10px 20px; border: none; border-radius: 4px; font-size: 14px; cursor: pointer; font-weight: 600; transition: all 0.2s ease; letter-spacing: 0.3px; display: flex; align-items: center; gap: 6px; }
        button:hover { filter: brightness(1.15); transform: translateY(-1px); }
        button:active { transform: translateY(0); }
        button:disabled { opacity: 0.4; cursor: not-allowed; transform: none; filter: none; }
        
        #clearPasteBtn { background: #3f3f46; color: white; border: 1px solid #52525b; }
        #checkBtn { background: #ca8a04; color: #fff; border: 1px solid #eab308; }
        #applyBtn { background: #2563eb; color: white; border: 1px solid #3b82f6; }
        #applyBtn.ready { background: #16a34a; border-color: #22c55e; box-shadow: 0 0 10px rgba(34, 197, 94, 0.3); }
        #unarmorBtn { background: #7c3aed; color: #fff; max-width: 0; padding-left: 0; padding-right: 0; opacity: 0; overflow: hidden; white-space: nowrap; transition: all 0.3s ease; display: none; }
        #unarmorBtn.show { max-width: 200px; padding-left: 20px; padding-right: 20px; opacity: 1; border: 1px solid #8b5cf6; }
        #fixPathsBtn { background: #0891b2; color: white; border: 1px solid #06b6d4; }
        #copyLedgerBtn { background: #475569; color: white; border: 1px solid #64748b; }
        #retestBtn { background: #0284c7; color: white; border: 1px solid #06b6d4; }
        #cancelRetestBtn { background: #dc2626; color: white; border: 1px solid #ef4444; }

        .output { flex: 1; background: #0f1420; border: 1px solid #2d3748; padding: 20px; overflow-y: auto; border-radius: 6px; font-family: 'Consolas', 'JetBrains Mono', monospace; font-size: 13.5px; line-height: 1.6; box-shadow: inset 0 2px 4px rgba(0,0,0,0.1); }
        
        .file-block { margin-bottom: 16px; border: 1px solid #334155; border-radius: 6px; overflow: hidden; background: #1e293b; transition: all 0.2s; box-shadow: 0 2px 5px rgba(0,0,0,0.15); }
        
        .file-block.status-ok { border-color: #16a34a; }
        .file-block.status-ok .file-header { background: rgba(22, 163, 74, 0.15); color: #bbf7d0; border-bottom: 1px solid rgba(22, 163, 74, 0.3); }
        
        .file-block.status-applied { border-color: #475569; opacity: 0.85; }
        .file-block.status-applied .file-header { background: rgba(71, 85, 105, 0.2); color: #cbd5e1; border-bottom: 1px solid rgba(71, 85, 105, 0.4); }
        
        .file-block.status-error { border-color: #ef4444; }
        .file-block.status-error .file-header { background: rgba(239, 68, 68, 0.15); color: #fecaca; border-bottom: 1px solid rgba(239, 68, 68, 0.3); }
        
        .file-block.status-ignored { border-color: #f59e0b; }
        .file-block.status-ignored .file-header { background: rgba(245, 158, 11, 0.15); color: #fef08a; border-bottom: 1px solid rgba(245, 158, 11, 0.3); }
        
        .file-header { background: #1e293b; padding: 10px 16px; cursor: pointer; display: flex; justify-content: space-between; align-items: center; user-select: none; transition: background 0.2s; font-family: -apple-system, BlinkMacSystemFont, sans-serif; letter-spacing: 0.2px; }
        .file-header:hover { filter: brightness(1.2); }
        
        .file-content { padding: 14px 16px; }
        .file-block[open] .file-content { display: block; }
        
        .patch-block { margin-bottom: 18px; padding-bottom: 18px; border-bottom: 1px dashed #475569; }
        .patch-block:last-child { border-bottom: none; margin-bottom: 0; padding-bottom: 0; }
        
        .status-badge { padding: 3px 10px; border-radius: 12px; font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.5px; }
        .status-ok { background: rgba(34, 197, 94, 0.2); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.3); }
        .status-error { background: rgba(239, 68, 68, 0.2); color: #f87171; border: 1px solid rgba(239, 68, 68, 0.3); }
        .status-ignored { background: rgba(245, 158, 11, 0.2); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
        .status-applied { background: rgba(100, 116, 139, 0.2); color: #94a3b8; border: 1px solid rgba(100, 116, 139, 0.3); }
        
        .error-msg { color: #f87171; margin-top: 8px; font-size: 13px; font-weight: 500; }
        
        .replace-block { color: #86efac; margin-top: 8px; white-space: pre-wrap; background: rgba(34, 197, 94, 0.08); padding: 10px 12px; border-left: 3px solid #22c55e; border-radius: 0 4px 4px 0; }
        .replace-block.applied { color: #94a3b8; background: rgba(100, 116, 139, 0.08); border-left: 3px solid #64748b; }
        
        .hint-block { color: #7dd3fc; margin-top: 8px; white-space: pre-wrap; background: rgba(6, 182, 212, 0.08); padding: 10px 12px; border-left: 3px dashed #06b6d4; border-radius: 0 4px 4px 0; }
        
        ::-webkit-scrollbar { width: 8px; height: 8px; }
        ::-webkit-scrollbar-track { background: #0f1420; border-radius: 4px; }
        ::-webkit-scrollbar-thumb { background: #334155; border-radius: 4px; }
        ::-webkit-scrollbar-thumb:hover { background: #475569; }
`
