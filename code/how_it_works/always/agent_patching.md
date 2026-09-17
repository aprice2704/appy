// :: product: FDM/NS
// :: majorVersion: 2
// :: fileVersion: 5
// :: description: Agent Patching and Code Emission specification for Appy/PatchEng V2.4.0.
// :: filename: code/how_it_works/always/agent_patching.md
// :: serialization: md

# Agent Patching & Code Emission (V2.4 Architecture)

The Appy patching engine (`patcheng`) enforces a strict airlock between LLM output and the filesystem. This document defines the formatting, directives, and protocols required to emit zero-drift code.

> **ARCHITECTURAL LAW:** Long files invite patch failure and logic rot[cite: 1]. If you see files >250 lines or nested arrow code, **REFACTOR AND SPLIT IMMEDIATELY**[cite: 1]. Do not wait or ask[cite: 1].

---

## Section 0: Reconnaissance Protocol (`piranha` vs. `rg`)

Never guess types, struct fields, imports, or syntax signatures[cite: 1]. Execute targeted reconnaissance before proposing any patch[cite: 1]:

* **Use `rg` (ripgrep) exclusively for raw text searches:**[cite: 1]
 - Searching text strings, markdown headings, CSS classes, comments, or JSON keys[cite: 1].
 - Example: `rg -n "analysis:run_id"` or `rg "button-primary"`[cite: 1].
* **Use `piranha` exclusively for Go AST exploration:**[cite: 1]
 - Extracting exported signatures: `piranha -s <SymbolName>` (e.g. `piranha -s UpsertGoPackage`)[cite: 1].
 - Finding node schema & publicgraph signatures: `piranha -n <nodetype>` (e.g. `piranha -n code_file`)[cite: 1].
 - Locating call sites: `piranha --calls "<Symbol>" "code/analysis/**"`[cite: 1].
 - Inspecting whole package surfaces: `piranha -p code/publicgraph`[cite: 1].
 - Token-light structural map: `piranha -t "code/patcheng/**"`[cite: 1].

### Proactive File Retrieval Protocol
When you need full file contents to edit, output the filenames in a single plaintext code box[cite: 1]:
```text
code/path/to/file1.go
code/path/to/file2.go
```
Do NOT output `go run code/cmd/txtar/main.go` or txtar commands[cite: 1].

---

## Section 1: The Armored Envelope

All patches must be wrapped in the **Armored Content Rule**[cite: 1]:
1. **Single Plaintext Code Fence:** Wrap the entire payload in ONE 4-backtick code fence labeled `plaintext` (````plaintext). This ensures a clean one-click copy button renders in UI interfaces. Put ALL `\%%% filename:` blocks for that workspace in the same fence[cite: 1].
2. **Multi-Repo Isolation:** Appy sandboxes to a single repository root[cite: 1]. If editing files across multiple repositories (e.g. `fdm` and `appy`), you **MUST** split them into separate 4-backtick code fences per repository and clearly label each[cite: 1].
3. **Line Armoring:** Prepend `@@@` to **EVERY SINGLE LINE** inside the code fence, including empty lines[cite: 1].
4. **Three Percent Signs:** Directives use exactly three percent signs `\%%%`[cite: 1]. In prose or documentation, example directives MUST be escaped as `\%%%` to prevent parser collisions.
5. **Bundle Pragmas:** Global execution controls like `\%%% compiler: skip` can be declared anywhere in the bundle (or between files) to bypass pre-flight compiler gates during cross-package refactoring[cite: 1].

---

## Section 2: V2 Directive Taxonomy & Decision Matrix

### The Decision Matrix (When to Patch vs. Overwrite)
| Condition | Action | Directive |
| :--- | :--- | :--- |
| **File < 100 lines** | Always overwrite | `\%%% complete_replace`[cite: 1, 2] |
| **Delta > 30% of file** | Always overwrite | `\%%% complete_replace`[cite: 1, 2] |
| **File > 250 lines** | Refactor, flatten, and split | Split into files < 150 lines[cite: 1] |
| **Previous patch rejected** | Immediate escalation | `\%%% complete_replace`[cite: 1, 2] |
| **Small targeted change (<30%)** | Single Super-Reliable Method | `replace_symbol` or Anchored `replace`[cite: 1, 2] |

### Directives
```text
# 1. Whole-File Operations (Baseline default; requires explicit \%%% end)
%%% create [~lines <N>]
%%% complete_replace [~lines <N>]
%%% overwrite
%%% delete_file

# 2. Semantic Declarations (Target in header; replaces entire symbol)
%%% replace_symbol <symbol>
%%% replace_ast <target>

# 3. Text & Prose Operations (Target in body; \%%% with is MANDATORY; requires 3 lines context above/below)
%%% replace

# 4. Pragmas & Execution Controls
%%% compiler: skip

# 5. Specialized Checklists & Metadata
%%% ndcl_update
%%% meta_update
```

---

## Section 3: Semantic AST Directives (`replace_symbol`)

Prefer declaration-level replacement over fuzzy text matching[cite: 1]:
1. **`replace_symbol <SymbolName>`:** Replaces an entire function, method, struct, or type declaration by identifier alone[cite: 1, 2, 3]. Zero line numbers or offsets required.
2. **`replace_ast <target>`:** Matches declaration or top-level node[cite: 1]. Line hints (`near <line>`) are deprecated and ignored to prevent false rejections[cite: 1, 2, 3].

> **THE TWO-STRIKE MANDATE:** If a patch fails or produces ambiguity on Turn 1, do not attempt to adjust search lines[cite: 1, 2]. Switch immediately to `\%%% complete_replace`[cite: 1, 2].

---

## Section 4: File Lifecycle Directives

Never guess with whole-file rewrites[cite: 1]. Use the appropriate verb[cite: 1]:

* **`\%%% create [~lines <N>]`**: Creates a **brand new file**[cite: 1].
 * Fails immediately if the file already exists or has >0 bytes[cite: 1, 2].
* **`\%%% complete_replace [~lines <N>]`**: Replaces the **entire content of an existing file**[cite: 1].
 * Fails immediately if the target file is missing or has 0 bytes[cite: 1, 2].
* **`\%%% overwrite`**: Unchecked write[cite: 1]. Retained for recovery and backward compatibility[cite: 1].
* **`\%%% delete_file`**: Deletes the file[cite: 1]. Requires the exact word `CONFIRM` on a line by itself[cite: 1].

---

## Section 5: Text & Prose Matching (`%%% replace`)

Use `\%%% replace` for raw text or non-compiled files[cite: 1].

* **Mandatory Surrounding Context:**
 Search blocks must provide at least 3 lines of unchanged context above and 3 lines below the change to guarantee uniqueness.
* **Strict Uniqueness:**
 Matches must be unique across the entire file[cite: 1, 2, 3]. If a block matches more than once or zero times, it fails immediately[cite: 2, 3]. Do not guess with line offsets.
* **Automatic Import Handling:**
 You do not need to manually calculate Go `import (...)` blocks[cite: 1]. Downstream `goimports` handles import organization[cite: 1, 2].

---

## Section 6: Pragmas & Cross-Package Refactoring (`\%%% compiler: skip`)

When executing cross-package refactorings where intermediate packages temporarily fail Go compilation before reciprocal changes land, use the bundle-wide compiler skip pragma[cite: 1]:

```text
%%% compiler: skip
```

* **Scope**: Applies globally across all files included in the patch bundle[cite: 1].
* **Behavior**: Bypasses the sandbox pre-flight compilation check[cite: 1]. Validated files are applied and written to disk without triggering a compiler lockup[cite: 1].
* **Audit Trail**: Appy outputs an explicit audit line into the result ledger[cite: 1].

---

## Section 7: Checklist Mutations (`\%%% ndcl_update`)

To update `.ndcl` checklists safely without risking syntax drift[cite: 1]:
* Reference items via exact IDs: `#(id)`[cite: 1].
* Use semantic status verbs: `open`, `done`, `skipped`, `inprogress`, `blocked`, `question`[cite: 1].
* To add an ID to an un-anchored task: `addkey #(id) <existing text>`[cite: 1].
* To inject a task: `insert_after #(anchor_id) - [ ] <new text> #(new_id)`[cite: 1].