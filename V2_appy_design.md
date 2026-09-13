# V2 Appy & PatchEng Design Specification

This document defines the V2 architecture for `patcheng` and the Appy console backend. It addresses systemic friction—manual import rejection loops, prose/Markdown match degradation, syntax rigidity, and obscured failure telemetry—by eliminating punitive tripwires in favor of downstream canonical formatting, semantic AST locators, and unredacted receipts.

---

## 1. Architectural Invariants

### Invariant A: Language Pipeline Decoupling (Compilation vs. Prose)
* **Code:** Files with formal grammars (Go, TypeScript/JavaScript, Python) decouple logic mutations from syntax formatting. Localized search-and-replace targets code bodies; whitespace, indentation, and import blocks are delegated downstream to canonical toolchains (`goimports`, `gofmt`).
* **Prose:** Documents without compilers (`.md`, `.ndcl`, `.txt`) operate on a **block-normalized line stream**. Matching normalizes runs of horizontal whitespace, soft line breaks, and leading markdown list symbols (`*`, `-`, `+`) rather than enforcing byte-for-byte exactness.

### Invariant B: Import Non-Interference
* *Code patches mutate logic; toolchains govern imports.*
* The manual import ban tripwire (`REJECTED: Manual patching of Go imports via fuzzy search is forbidden`) is abolished.
* If a patch targeting a Go file contains `import (...)` or `import "..."`:
 1. The engine extracts the package paths (and optional aliases) into `ImportDirectives`.
 2. The import statements are stripped from the active search block so the search matches the surrounding declarations cleanly.
 3. Extracted imports are applied via `astutil.AddImport` / `astutil.AddNamedImport`, and canonicalized via `goimports -w` during post-flight.

### Invariant C: Unified AST Locator (`replace_ast`)
* The caller should not be forced to guess the parser's internal grammar category (symbol vs. block vs. statement).
* `replace_ast <target> [near <line>]` follows a deterministic resolution waterfall:
 1. **Symbol:** Declaration match (func, method, type, struct, interface).
 2. **Block:** Control flow condition match (`if`, `for`, `switch`, `select`).
 3. **Statement:** Statement-level match (assignments, returns, calls, var declarations).
 4. **Agnostic Fallback:** If AST locators do not resolve, fallback to whitespace/comment-agnostic fuzzy matching.
* On all header-targeted directives (`replace_ast`, `replace_symbol`, etc.), the `\%%% with` delimiter is **purely optional**.

### Invariant D: Explicit Lifecycle Verbs (`create` vs. `complete_replace`)
* **`create`**: Asserts the file must **NOT** exist (or has 0 bytes). Rejects if the file already exists, preventing accidental overwrite of existing logic.
* **`complete_replace [~lines <approx>]`**: Asserts the file **MUST** already exist (> 0 bytes). Rejects if missing, preventing accidental orphan creation due to typos in paths.
* **`overwrite`**: Unchecked blind write, retained for legacy compatibility.

### Invariant E: Truncation Defense & Parser Lifecycle
* **Localized Edits (`replace_ast`, `replace`)**: Support **implicit closure**. Encountering a subsequent directive (`\%%% filename:`, `\%%% replace*`, etc.) or reaching EOF cleanly closes the open block without demanding a clerical `\%%% end`.
* **Whole-File Rewrites (`create`, `complete_replace`)**: Mandate an explicit `\%%% end` handshake. If EOF or closing code fence is reached without `\%%% end`, the engine halts with a truncation warning.
* **Advisory Line Hints (`~lines <N>`)**: For large rewrites, an optional approximate line count hint provides an active corridor ($\pm 30\%$). If the emitted payload falls drastically short, the engine halts before writing to disk.

### Invariant F: Zero-Elision Telemetry ("Show the Receipts")
* Per the FDM truth discipline (`⟦ show the receipts ⟧`), failure artifacts must never be redacted.
* Dual logging channels:
 * **`.appy_failures.jsonl` (Ring buffer: Last 50 Rich Failures)**: Stores unredacted `search_block`, `replace_block`, error diagnostics, and the ±10 line candidate baseline snippet.
 * **`.appy_activity.jsonl` (Ring buffer: Last 200 Telemetry Records)**: High-level operational ledger tracking method, file, line delta, and success/failure status.

---

## 2. V2 Directive Grammar

```text
# 1. Whole-File Operations (No search block, no \%%% with, requires explicit \%%% end)
%%% create [~lines <N>]
%%% complete_replace [~lines <N>]
%%% overwrite
%%% delete_file [CONFIRM]

# 2. Semantic AST Operations (Target in header; \%%% with is OPTIONAL; implicit closure supported)
%%% replace_ast <target> [near <line>]
%%% replace_symbol <symbol>
%%% replace_block <condition> [near <line>]
%%% replace_statement <statement> [near <line>]

# 3. Text / Prose Operations (Target in body; \%%% with is MANDATORY delimiter)
%%% replace [near <line> | <index>]
%%% replace_anchored <coord>

# 4. Specialized Metadata & Checklists
%%% ndcl_update
%%% meta_update
```

---

## 3. Implementation Plan

- [x] **Ablate Import Rejection:** Remove `ValidateFuzzySearchBlock` import tripwire in `utils.go`.
- [x] **Dual Ring-Buffer Telemetry:** Implement 50-rich / 200-activity logs in `failure_logger.go`.
- [ ] **Implement `replace_ast` in `patcheng`:** Unified waterfall resolver in `strategy_ast.go`.
- [ ] **Add `create` & `complete_replace`:** Add existence validation in `bundle.go` and `api_patch.go`.
- [ ] **Make `\%%% with` Optional for Headers:** Auto-transition parser state on first content line.
- [ ] **Implicit Closure for Localized Edits:** Allow next directive / EOF to close `replace_ast` blocks.
- [ ] **Truncation Defense:** Validate `~lines <N>` bounds and enforce explicit `\%%% end` on whole-file rewrites.
