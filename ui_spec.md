# Appy UI Functional Specification v1.6.4

## 1. Top-Level Layout
The UI is contained within a fixed-height flex container (`95vh`) and divided into four primary functional zones:
* **Header Zone**: The top of the page MUST display:
* The browser tab text and the primary page title set to the last element of the directory path from which Appy is run.
* The Appy name and current version number explicitly displayed in a smaller font next to the title.
* A subtitle indicating the absolute file root to which all operations are sandboxed.
* **Tab Bar**: A horizontal list of tabs (Patch, Builder, History) allowing navigation between functional areas.
* **Input Zone**: A large, unformatted textarea for pasting multi-file patch bundles (includes markdown blocks and `\%%%` syntax).
* **Control Zone**: A horizontal button array for system actions, strictly grouped to prevent layout shift.
* **Output Zone**: A scrollable area rendering visual "File Block" stripes.

### 1.1 Builder Tab Constraints
* **Smart Paste**: The Builder MUST support smart extraction of shell commands (stripping `txtar c`, line continuations, and output redirects) via a dedicated Paste action.
* **Absolute Paths & Directory Globbing**: The Builder MUST accept absolute file paths from anywhere on the filesystem, automatically appending `/**` for directories added via the picker. The Builder MUST allow files from anywhere on the disk, though the patcher remains restricted to the sandbox.
* **Config Sets**: The Builder MUST support saving and loading reusable configuration sets (Includes, Excludes, Anchors, Preface, Output Filename) via a dropdown interface, persisting to `.appy_sets.json`.
* **File Size Defenses**: The backend MUST inject a prominent warning marker (`⚠️ APPY NOTE: This file is overly large...`) below the filename in the generated `txtar` block for files exceeding the configured line threshold (default 350) to discourage LLM truncation and massive unmodified rewrites.

---

## 2. UI Behaviour Invariants
These rules are non-negotiable and dictate the core safety of the application:
1. The UI MUST never apply a file in `ERROR` or `IGNORED` state.
2. The UI MUST never require `Check Compiler` to be run before `Apply to Disk`.
3. `Check Compiler` MUST NOT write to repository files or create temp files in the repo root.
4. `Retest Impacted` MUST NOT alter primary file status; it only adds test decorators/reports.
5. Any input edit after preview MUST invalidate the current preview and disable Apply until preview is recomputed.
6. Status chips MUST remain visible at the RHS of each stripe header regardless of horizontal content length.
7. Copy Trace MUST contain enough information for an LLM to repair the patch without seeing the whole UI.
8. The Result Ledger (both the visual report and the internal `.appy_ledger.json` hash store) MUST only record/report files actually written to disk. It MUST NOT record hashes for files that failed pre-flight checks, even if other files in the same batch succeeded.
9. Partial success during Apply MUST be represented per file, not collapsed into one global success/failure banner.
10. Raw JSON responses MUST never be rendered directly in the main output zone.

---

## 3. File Block State Machine (The Stripes)
Every file parsed from the bundle is rendered as an expandable `<details>` element. Status is strictly decoupled into primary state and secondary verification decorators.

### 3.1 Stripe Header Layout
The visual header of the stripe MUST contain four elements in this exact order (left-to-right):
1.  **File Type Tag**: A visual indicator of the language profile (e.g., `🐹 Go`) as determined by the patching engine.
2.  **Filename**: The targeted file path.
3.  **Net Lines Aggregate**: The total line delta for the file (e.g., `<span class="net-lines">+4</span>`), placed immediately after the filename for at-a-glance sanity checking.
4.  **RHS Flex Container**: A right-aligned container holding the secondary decorators and the primary status chip.

### 3.2 Primary Status
| Internal State | Stripe Header Color | RHS Chip Text | Logic |
| :--- | :--- | :--- | :--- |
| **READY** | Light Green | `OK` | Search block matches perfectly; file is ready to commit. |
| **ERROR** | Light Red | `ERROR` | Search block not found, ambiguous, or syntax failure. |
| **APPLIED** | Dark Grey | `APPLIED` | File successfully written to disk; hash recorded in ledger. |
| **IGNORED** | Light Orange | `IGNORED` | Safety skip (e.g., empty search on existing file). |

### 3.3 Secondary Verification Decorators
Verification passes and structural modifiers append specific emoji decorators.
**These sigils MUST be placed on the RHS of the stripe, immediately to the left of the primary status chip.**
* **Overwrite Indicator**: ☢️ (Appears if the patch strategy is a full file overwrite).
* **Anchored Indicator**: ⚓ (Appears if the patch used semantic coordinates).
* **Compiler Verified (`Check Compiler`)**: 🏅 PASS / ⚠️ FAIL
* **Test Verified (`Retest Impacted`)**: 🧪 PASS / 💥 FAIL

---

## 4. Stale Preview Handling
The preview model is valid **only** for the exact input string and repository snapshot used to produce it.
* **If the input textarea changes:**
* All existing preview/apply buttons are disabled immediately.
* Existing stripes are cleared or marked STALE.
* Applying stale preview data is strictly forbidden.
* **If the repository changes between preview and apply:**
* The backend MUST re-check search block matches before writing.
* Any mismatch becomes `ERROR` and is not written.

---

## 5. Control Plane Button Matrix (Anti-Layout Shift)
To prevent frustrating UI jumping when states change, buttons MUST be rendered in strict, stable left-to-right logical groups. Mutually exclusive buttons must occupy the same spatial slot.

| Group | Buttons (Strict Left-to-Right Order) | Visibility / Layout Rules |
| :--- | :--- | :--- |
| **1. Prep** | `Auto-Pilot`, `Clear & Paste`, `Remove @@@`, `Fix File Paths` | Always grouped left. `Remove @@@` only visible if ≥ 2 lines are armored. |
| **2. Action** | `Check Compiler`, `Apply to Disk`, `Retest Impacted` | Center grouped. `Retest` appears only after successful application. |
| **3. Export** | `Copy Trace` (Dynamic) | Grouped right. The button dynamically updates its color (Blue/Purple/Red) and label (`Copy Preview Errors`, `Copy Test Log`, etc.) based on the severity of the current active trace payload. |

*Busy states MUST NOT erase existing stripes unless the action succeeds with a new preview model. Disable buttons rather than hiding them during processing.*

---

## 6. Action Definitions & Semantics

### 6.1 Apply to Disk Semantics
* Operates **only** on files currently in `READY`. It MUST NOT apply `ERROR` or `IGNORED` files.
* Application is **partial/per-file**. If some files succeed and others fail, successful files become `APPLIED` and failed files become `ERROR`.
* Records per-file results in the ledger.
* If all patches for a given file are already present in the ledger (previously applied), the backend MUST gracefully skip the file and report it as `APPLIED` without attempting to re-modify the disk or throwing an error.

### 6.2 Check Compiler Semantics
* Uses the current filesystem as a base.
* Overlays all `READY` patches into an in-memory workspace.
* Validates the resulting files/packages.
* Must NOT mutate source files or leave scratch files in the repo.

### 6.3 Auto-Unarmor Semantics
* If the input contains 2 or more instances of `@@@` at the start of a line, Appy will automatically strip the armor.
* Strips exactly **one** leading `@@@` per line and immediately triggers a preview.
* Operates safely on partially armored text, only stripping from lines that begin with the prefix.

---

## 7. Preview & Patch Details
To ensure operator trust, each patch block within an expanded file stripe MUST display:
* **Diff Preview**: Shows both the **Search Block** (old text, collapsible but inspectable) and the **Replacement Block** (new text, light green background). Showing only the replacement text weakens trust.

*(Note: The `Net Lines` metric is aggregated and displayed at the File Stripe Header level, not buried inside the patch blocks).*

---

## 8. Failure & Recovery Reporting
When a patch fails during preview, application, compiler checks, or testing, the UI MUST surface detailed, LLM-friendly diagnostic data:
1.  **Matched Line Echo**: Reports must echo the exact state of the target line(s) to allow easy recovery using `\%%% match_line`.
2.  **LLM Hints & Fallbacks**: The error feedback MUST include explicit instructions delivering the suggested fallback patching sequence for the targeted language profile so the LLM knows how to recover.
3.  **Trace Output (Clipboard Only)**: If a failure occurs during `Check Compiler` or `Retest Impacted`, the exact standard output/error trace MUST NOT be injected into the visual DOM stripes (to prevent UI bloat/lag). Instead, it MUST be appended exclusively to the clipboard payload of the `Copy Trace` button.
4.  **Global Error Routing**: Top-level server or network errors (e.g., malformed bundle parsing failures) MUST NOT bypass the export mechanisms. They must populate the clipboard payload and force the `Copy Trace` button to appear.

---

## 9. API Response Contracts
The backend MUST adhere to these conceptual payload shapes to prevent UI spaghetti:

**PreviewResponse:**
```yaml
files:
- path: string
status: string (READY|ERROR|IGNORED)
net_lines: int
file_type: string
file_icon: string
patches:
  - search_block: string
    replace_block: string
    is_overwrite: bool
    error: string
    closest_match_hint: string
    llm_fallback_hint: string
```

**ApplyResponse:**
```yaml
files:
- path: string
applied: bool
net_lines: int
file_type: string
file_icon: string
hash_before: string
hash_after: string
ledger_entry: string
error: string
failed_patch:
  current_line_echo: string
  llm_fallback_hint: string
```

**CompilerCheckResponse:**
```yaml
files:
- path: string
compiler_status: string (PASS|FAIL)
diagnostics: []string
raw_output: string
```

**RetestResponse:**
```yaml
packages: []string
files:
- path: string
test_status: string (PASS|FAIL)
package: string
summary: string
failure_excerpt: string
raw_output: string
```

**Builder Responses (`/api/sets`, `/api/txtar`, `/api/txtar_stats`):**
```yaml
ConfigSetsResponse:
 <set_name>:
   paths: []string
   excludes: []string
   anchors: []string
   preface: string
   file_name: string

TxtarStatsResponse:
 file_count: int
 size_kb: int
 tokens_est: int

TxtarBuildResponse:
 success: bool
 file_url: string
 file_name: string
 file_count: int
```

---

## 10. Code & Metadata Syntax
When Appy generates Go source files, metadata must be at the absolute top, one directive per line, followed by exactly one blank line. The backend parser and frontend unarmor logic MUST tolerate optional leading whitespace (including non-breaking spaces) before patch directives (`\%%%`) to gracefully handle LLM formatting artifacts.

:: product: FDM/NS
:: majorVersion: 1
:: fileVersion: 32
:: description: Documented Builder API response contracts following backend modularization.
:: filename: ui_spec.md
:: serialization: md
:: latestChange: Added /api/sets, /api/txtar, and /api/txtar_stats response contracts.
