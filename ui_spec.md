# Appy UI Functional Specification v1.5.19

## 1. Top-Level Layout
The UI is contained within a fixed-height flex container (`95vh`) and divided into three primary functional zones:
*   **Input Zone**: A large, unformatted textarea for pasting multi-file patch bundles (includes markdown blocks and `%%%` syntax).
*   **Control Zone**: A horizontal button array for system actions.
*   **Output Zone**: A scrollable area rendering visual "File Block" stripes.

## 2. File Block Visualization (The Stripes)
Every file parsed from the bundle is rendered as an expandable `<details>` element. The visual state is driven by the patch status:

| State | Stripe Header Color | RHS Chip Text | Logic |
| :--- | :--- | :--- | :--- |
| **Ready** | Light Green | `OK` | Search block matches perfectly; file is ready to commit. |
| **Error** | Light Red | `ERROR` | Search block not found, ambiguous, or syntax failure. |
| **Applied** | Dark Grey | `APPLIED` | File successfully written to disk; hash recorded in ledger. |
| **Ignored** | Light Orange | `IGNORED` | Safety skip (e.g., empty search on existing file). |

### 2.1 RHS Status Chips & Decorators
Status chips MUST be pinned to the right-hand side of the stripe header using flex-space-between to ensure legibility during rapid scrolling.

**Secondary Decorators:**
As files pass through extended verification phases, secondary emoji decorators are appended next to the RHS chip:
*   **Compiler Verified (`Check Compiler`)**: Acquires a 🏅 (medal) or ✅ (checkmark) icon indicating the in-memory AST parsed successfully without syntax errors.
*   **Test Verified (`Retest Impacted`)**: Acquires a 🧪 (green pass) or 💥 (red explosion/fail) icon indicating post-application test suite results for that specific file/package.

## 3. Control Plane Action Registry
Buttons are enabled/disabled based on the presence of content and the results of the preview cycle.

*   **Clear & Paste**: Clears the input, reads from the clipboard, and triggers an immediate `/api/preview`.
*   **Remove @@@**: Unarmors the payload. Only appears if **every single line** of the input (ignoring empty lines) starts with the `@@@` armor prefix.
*   **Check Compiler**: Executes a full bundle application in memory and performs a language-specific syntax validation without committing to disk. **Crucially, this action MUST NOT disable the 'Apply to Disk' button.** Files that pass validation receive a 🏅 decorator. Files that fail validation are downgraded to the `ERROR` state.
*   **Apply to Disk**: Commits all `OK` files to the filesystem. Only available if at least one file is in the `OK` state (regardless of whether 'Check Compiler' was clicked). Must execute immediately without a modal confirmation box.
*   **Fix File Paths**: Resolves "missing" file errors by searching the repository for unique path suffixes.
*   **Copy Preview Errors**: Appears during the preview phase if any file patch fails. Copies a markdown-formatted report of the rejected files and their closest-match hints to easily paste back to the LLM.
*   **Copy Result Ledger**: Appears after application. Copies the "Team Grey" markdown summary of the committed files.
*   **Retest Impacted**: Executes tests for the packages affected by the applied patches. Results MUST NOT be dumped as raw JSON at the bottom of the screen. Instead, results dynamically decorate the applied file stripes.
*   **Copy Test Report**: Replaces or augments the "Copy Result Ledger" button after a retest, allowing the user to copy a markdown-formatted summary of the test outcomes to feed back to the LLM.

## 4. Code & Metadata Syntax
When Appy generates Go source files, metadata must be at the absolute top, one directive per line, followed by exactly one blank line. 

**The Armor Mechanism**: If the LLM wraps the code in `@@@` to protect it from markdown parsers, it MUST armor every single line. Appy will only offer to strip the armor if the entire payload is uniformly protected. Example of valid Appy output:

@@@// :: product: FDM/NS
@@@// :: majorVersion: 1
@@@// :: fileVersion: 24
@@@// :: description: v1.5.14
@@@// :: filename: code/cmd/appy/main.go
@@@// :: serialization: go
@@@package main
@@@...

## 5. Preview & Patch Details
Each patch block within a file must display:
*   **Net Lines**: A floating right-hand metric showing the line delta (e.g., `+3` or `-1`).
*   **Diff Preview**: The targeted replacement text rendered inside a `.replace-block` (light green background) to visualize what will be injected into the file.

## 6. Failure & Recovery Reporting
Rejected files must include a "Matched Line Echo" in the report to allow for easy operator recovery using `%%% match_line`.

## 7. Backlog & Future Capabilities
## AST-Aware HTML/Astro Patching
*   **Status**: DONE (v1.5.18).
*   **Implementation**: Utilizes `golang.org/x/net/html` Tokenizer for precision byte-offset localization. Resolves exact element boundaries to preserve all external whitespace without triggering a global DOM re-render/reformat.
*   **Syntax**: `%%% replace_element #id`, `%%% replace_element .class`, `%%% replace_element tag`. Supports `near <line>`.