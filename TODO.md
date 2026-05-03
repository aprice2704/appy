// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: Appy feature requests and backlog.
// :: filename: /home/aprice/dev/appy/TODO.md
// :: serialization: md

# Appy TODO & Backlog

## AST-Aware HTML/Astro Patching
*   **Status**: DONE (v1.5.18).
*   **Implementation**: Utilizes `golang.org/x/net/html` Tokenizer for precision byte-offset localization. Resolves exact element boundaries to preserve all external whitespace without triggering a global DOM re-render/reformat.
*   **Syntax**: `%%% replace_element #id`, `%%% replace_element .class`, `%%% replace_element tag`. Supports `near <line>`.