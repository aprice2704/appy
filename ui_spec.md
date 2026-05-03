# Appy UI Functional Specification v1.5.15

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

### 2.1 RHS Status Chips
Status chips MUST be pinned to the right-hand side of the stripe header using flex-space-between to ensure legibility during rapid scrolling.

## 3. Control Plane Action Registry
Buttons are enabled/disabled based on the presence of content and the results of the preview cycle.

*   **Clear & Paste**: Clears the input, reads from the clipboard, and triggers an immediate `/api/preview`.
*   **Check Compiler**: Executes a full bundle application in memory and performs a language-specific syntax validation without committing to disk.
*   **Apply to Disk**: Commits all `OK` files to the filesystem. Only available if at least one file is in the `OK` state. Must execute immediately without a modal confirmation box for rapid iteration.
*   **Fix File Paths**: Resolves "missing" file errors by searching the repository for unique path suffixes.
*   **Copy Result Ledger**: Restored button that copies the "Team Grey" markdown summary of the last operation.

## 4. Code & Metadata Syntax
When Appy generates Go source files, metadata must be at the absolute top, one directive per line, followed by exactly one blank line[cite: 5].

Example of valid Appy output:

@@@// :: product: FDM/NS
@@@// :: majorVersion: 1
@@@// :: fileVersion: 24
@@@// :: description: v1.5.14
@@@// :: filename: code/cmd/appy/main.go
@@@// :: serialization: go
@@@
@@@package main
@@@...


## 5. Preview & Patch Details
Each patch block within a file must display:
*   **Net Lines**: A floating right-hand metric showing the line delta (e.g., `+3` or `-1`).
*   **Diff Preview**: The targeted replacement text rendered inside a `.replace-block` (light green background) to visualize what will be injected into the file.

## 6. Failure & Recovery Reporting
Rejected files must include a "Matched Line Echo" in the report to allow for easy operator recovery using `%%% match_line`.

## 7. Backlog & Future Capabilities
*   **AST HTML Patching**: `replace_element` and `replace_html_node` directives are planned to bypass fuzzy lexical search on highly structured XML/HTML/Astro documents.
```