// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 4
// :: description: v1.5.11 - Atomic Patching and Structured Text rules.
// :: filename: /home/aprice/dev/appy/agent_patching.md
// :: serialization: md

# Agent Patching & Code Emission

### Code Emission Gate (HARD)

**COMPILER PRE-FLIGHT (NO LIMPING):** The `appy` patch engine runs a strict Go compiler check on all patched files *before* committing them to disk. If your patch introduces syntax errors, the entire batch is rejected.

**FILE-ATOMIC PATCHING (MANDATORY):** Patch application is **file-atomic**. If any patch block for a file fails to match, **no changes** for that file are committed, even if other blocks matched in memory. Rejection reports explicitly state: `file_commit_status: rejected`.

**THE PATCH LEDGER (BLUNT FEEDBACK):** If application fails, the engine returns a blunt ledger of "Committed" vs "Rejected" files. For rejected files, the report includes a **Matched Line Echo** showing the actual current state of the code to aid in recovery.

**Do NOT re-emit or adjust patches for files in the successful committed ledger.** Focus ONLY on fixing the specific rejected file or the specific failed block.

**STRATEGY BY FILE TYPE (LLM NUDGE):**
*   **Go**: Prefer `%%% replace_symbol` or `%%% replace_block`.
*   **Structured Text (.astro, .html, .css, .md, .typ)**: 
    *   **Prefer** `%%% match_line`.
    *   **Prefer** small, unique-line replacements.
    *   **Avoid** large fuzzy blocks or replacing multi-line literals unless context is exact.

### The `ndcodepatch` Format
... (rest of spec remains)