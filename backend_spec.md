# Appy Backend / PatchEng Functional Specification

> **Status**: ACTIVE
> **Applies To**: `patcheng` library and Appy backend routers.

## 1. Core Philosophy: The Patch Airlock
The patching engine (`patcheng`) enforces a strict airlock between LLM output and the filesystem. It operates on a **File-Atomic** basis: patches are gathered into memory, validated against target files, and optionally run through a compiler pre-flight. If *any* patch block for a file fails, the entire file is rejected, preventing partial/corrupt application.

## 2. Patch Directives & Format
The backend consumes bundles separated by the `%%%` directive. 

### 2.1 The Directives
* `%%% filename: <path>`: Initiates a patch sequence for a specific file. Must be the first directive for a file.
* `%%% replace`, `%%% replace_anchored`, `%%% replace_symbol <name>`, `%%% replace_block <cond>`, `%%% replace_element <selector>`, `%%% replace_json_path <path>`: Opens a search or targeting block.
* `%%% with`: Closes the search block and opens the replacement block.
* `%%% overwrite`: Replaces the entire file. Must be the only directive for that file.
* `%%% ndcl_update`: Opens a state-mutation block for `.ndcl` checklists.
* `%%% meta_update`: Opens a state-mutation block for NeuroScript/SDI metadata.
* `%%% end`: Closes the current operation.

### 2.2 Whitespace Tolerance
The bundle parser is designed to be forgiving of LLM hallucination artifacts. It will automatically tolerate and trim leading standard spaces, tabs, and non-breaking spaces (`\u00A0`) appearing immediately before a `%%%` directive.

---

## 3. Patching Strategies

### 3.1 `fuzzy_patch_agnostic` (The Workhorse)
**Targeting:** Text blocks.
**Mechanism:** Extracts runes from the search block and the baseline file, completely ignoring all whitespace, tabs, and line breaks.
**Blind Spot (Comments):** For specific languages (like Go), this strategy *also* strips `//` and `/* */` comments from the comparison buffer. This prevents patches from failing if documentation has drifted, but it means **you cannot use this strategy to target or replace comments.**
**Elision:** Supports `...` on an isolated line within the search block to skip large chunks of unchanged code.

### 3.2 `replace_symbol` (AST Aware)
**Targeting:** Functions, Methods, Classes, Structs, Interfaces, Vars, and Consts. (Supports Go, JS, TS, Python, Java, C++).
**Mechanism:** Uses `go/ast` (for Go) or the `gotreesitter` polyglot engine (for others) to find the exact byte boundaries of a symbol and replaces its entirety. For receiver methods, the format is `Receiver.MethodName` (the parser is forgiving of missing `*` pointers).

### 3.3 `replace_block` (AST Aware)
**Targeting:** Control flow blocks (`if`, `for`, `switch`, `while`).
**Mechanism:** Uses `go/ast` or `gotreesitter` to find a control flow block matching a specific condition string.

### 3.4 `replace_element` (DOM Aware)
**Targeting:** HTML / Astro elements.
**Mechanism:** Uses `golang.org/x/net/html` Tokenizer to find byte boundaries of elements by `#id`, `.class`, or `tag`.

### 3.5 `replace_json_path`
**Targeting:** JSON keys.
**Mechanism:** Uses `tidwall/sjson` to surgically mutate or delete a specific key using dot-notation, preserving all surrounding formatting.

### 3.6 `match_line`
**Targeting:** Exact line matches.
**Mechanism:** A hybrid of fuzzy patching that expands the matched boundary to the nearest newline `\n` characters. Essential for replacing specific lines in Markdown or configuration files where surrounding context is sparse.

---

## 4. Structural Updaters

### 4.1 `ndcl_update`
Evaluates NDCL checklists.
* Updates item status via `#(id) <status>`. Enforces strict semantic vocabulary (`open`, `done`, `skipped`, `inprogress`, `blocked`, `question`).
* Appends new items via `addkey #(id) <Full text of item>`.
* Automatically triggers a topological bottom-up rollup of all `| |` automatic parent nodes in the file.

### 4.2 `meta_update`
Evaluates FDM/NeuroScript/SDI metadata blocks (`:: key: value`).
* Automatically handles Native (`:: key: value`) and Embedded (`// :: key: value`) formats.
* If the key exists, it mutates the value in-place.
* If the key does not exist, it appends it to the bottom of the discovered metadata block, respecting the prefix of the block.
* Fails safely if no existing metadata block is found.

---

## 5. LLM Interaction Directives

* *LLM-Optimized Feedback:* Appy surfaces specific "Fallback Hints" and "Matched Line Echoes" designed to be copied directly back to the LLM to facilitate immediate self-correction.
* *Auto-Pilot:* Operators can run the golden path (`Paste` -> `Unarmor` -> `Preview` -> `Apply` -> `Retest`) with a single click. The Auto-Pilot strictly enforces the airlock, halting immediately upon encountering any `ERROR` state.

:: filename: code/patcheng/backend_spec.md
