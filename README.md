# Appy: The Stateful Patch Console

Appy is a specialized, state-machine-driven web console designed to safely preview, validate, and commit LLM-generated code patches. It acts as an airlock between the raw output of an LLM and your local filesystem, ensuring that hallucinations, syntactic errors, and overlapping diffs are caught before they touch the disk.

## Core Philosophy: The Patch Airlock
Appy is not a text editor; it is a **patch-flight console**. 
1. **No Limping:** Code must pass language-specific syntax checks in-memory before it can be committed.
2. **File-Atomic Commits:** Partial failures are isolated to the file level. If `main.go` has an overlapping patch but `utils.go` is clean, `utils.go` can be applied while `main.go` is rejected.
3. **LLM-Optimized Feedback:** Appy surfaces specific "Fallback Hints" and "Matched Line Echoes" designed to be copied directly back to the LLM to facilitate immediate self-correction.

## Architecture

Appy is divided into a lightweight Go backend and an anti-layout-shift frontend. It relies on the `patcheng` (Patch Engine) library for its parsing and AST-aware replacement strategies.

### The UI State Machine
File stripes transition through a strict set of primary states, decoupled from secondary verification:
*   🟢 **READY (`OK`)**: Search block matches perfectly; file is ready to commit.
*   🔴 **ERROR (`ERROR`)**: Search block not found, ambiguous, or syntax failure.
*   ⚫ **APPLIED (`APPLIED`)**: File successfully written to disk; hash recorded in ledger.
*   🟠 **IGNORED (`IGNORED`)**: Safety skip (e.g., empty search on existing file).

**Secondary Decorators (RHS Sigils)**
*   🏅/⚠️ : Compiler Pass/Fail
*   🧪/💥 : Test Pass/Fail

### API Endpoints

#### `POST /api/preview`
Simulates the application of a patch bundle. Returns the exact line deltas, structural diffs, and closest-match hints without modifying the disk.
*   *Safety:* Protects against path traversal and validates file existence.

#### `POST /api/apply`
Executes a File-Atomic commit of all `READY` patches.
*   *Pre-Flight:* Runs `Check Compiler` (AST parsing) in-memory before writing.
*   *Ledger:* Records the SHA256 hashes of applied patches to `.appy_ledger.json` to prevent duplicate applications.

#### `POST /api/retest`
Triggers `go test` for the packages affected by the current batch of applied patches. Injects raw traces directly into the JSON payload for clipboard export.

## Patching Strategies (Powered by `patcheng`)
Appy supports escalating patching strategies, selected automatically by the LLM based on language profiles:
1.  **`symbol_replace`**: Replaces Go functions/types by their AST signature (e.g., `*Engine.Apply`).
2.  **`replace_block` / `replace_element`**: Replaces specific control-flow blocks (`if`, `for`) or DOM elements (`#nav`) using AST/Tokenizers.
3.  **`match_line`**: Expands fuzzy searches to exact line boundaries (excellent for Markdown/NDCL checklists).
4.  **`fuzzy_patch_agnostic`**: The workhorse. Replaces code while ignoring all whitespace, tabs, newlines, and language-specific comments.
5.  **`strict_full_file` / `overwrite`**: Replaces the entire file contents.

## Running Appy
Appy runs as a standalone Go binary.
```bash
go run . -port 8085
```
Navigate to `http://localhost:8085`. Appy will automatically watch its own executable for modifications and trigger a hot-reload if rebuilt.