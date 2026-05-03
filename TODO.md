// :: product: FDM/NS
// :: majorVersion: 1
// :: fileVersion: 1
// :: description: Appy feature requests and backlog.
// :: filename: code/cmd/appy/TODO.md
// :: serialization: md

# Appy TODO & Backlog

## AST-Aware HTML/Astro Patching
*   **Request**: Implement `replace_element` or `replace_html_node` directive.
*   **Reasoning**: "This failure mode argues for a replace_element / replace_html_node directive someday. Target by id='principles-title' or by enclosing <section id='principles'>, then replace the AST-ish HTML node. The fuzzy text matcher is being asked to do surgery wearing boxing gloves." (via Team Grey)
*   **Target Specs**: Should leverage `x/net/html` or similar to locate nodes by tag/ID/class instead of relying purely on lexical fuzzy matching.