# Appy Bug Report: AST `replace_symbol` Fails on Go Method Receivers

**Tool:** Appy (v1.8.19)
**Language Target:** Go
**Strategy:** `replace_symbol`

## Issue Summary
When using the `replace_symbol` patching strategy, the Go AST matcher successfully locates standard top-level functions (e.g., `func InitializeBotrTools()`) but **fails to match** methods bound to a struct receiver (e.g., `func (ts *toolset) testProvider(...)`). 

When attempting to target a method receiver, Appy rejects the patch with:
`Issue: symbol 'testProvider' not found in baseline AST`

## Root Cause Speculation
The underlying AST parser (likely using `go/ast`) is either filtering out `*ast.FuncDecl` nodes where `Recv != nil`, or the symbol name matching logic is strictly expecting a global namespace match and failing to resolve the `FuncDecl.Name.Name` when a receiver block is present.

## Impact
This forces the LLM to fall back to `replace_block` (which is brittle for function signatures) or escalate to `strict_full_file` (`%%% overwrite`). Overwriting entire files circumvents Appy's surgical precision, wastes context window, and significantly increases the risk of code drift on larger files.

## Repro Example
**Baseline Go Code:**
```go
func (ts *toolset) testProvider(rt api.Runtime, args []any) (any, error) {
    // ...
}

Appy Patch:

Plaintext
%%% replace_symbol testProvider
%%% with
func (ts *toolset) testProvider(rt api.Runtime, args []any) (any, error) {
    // updated logic
}
%%% end
Expected Result: Successful AST swap of the method body and signature.
Actual Result: Rejected (symbol not found).