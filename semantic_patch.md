**To: The Appy / Patcher Tooling Team**
**Subject: Feature Request: AST-Aware Patching (or higher-tolerance fuzzy matching)**

**The Problem:**
Currently, `appy` fails with `agnostic fuzzy search block not found` when the `%%% replace` block is logically identical to the target code but contains trivial lexical differences. 

In our recent pipeline refactor, the target Go code contained redundant type casts—specifically, wrapping an already-typed constant in a string cast and then casting it back to its original type:
`p.GetField(ix.FieldName(string(nodes.FieldStructureStabilityAssessmentSubject)))`

The LLM-generated patch attempted to match the cleaned, canonically correct version of that line:
`p.GetField(nodes.FieldStructureStabilityAssessmentSubject)`

Because `appy` performs strict string/fuzzy matching rather than Abstract Syntax Tree (AST) aware diffing, it completely choked on the redundant `string()` cast. The entire multi-file patch was aborted.

**Impact:**
This brittleness forces the LLM to hallucinate or perfectly memorize the exact, unformatted, or technically flawed state of the user's local files just to get a patch to apply. It turns code patching into a frustrating game of exact-string-matching whack-a-mole.

**Recommendations:**
1. **Semantic (AST-Level) Patching:** For supported languages (like Go), `appy` should parse the `replace` block and the target file into ASTs and match on semantic equivalence, completely ignoring formatting, redundant parens, or trivial explicit casts.
2. **Interactive Conflict Resolution:** If the fuzzy search confidence is high but not perfect, `appy` should surface the diff to the user for a Y/N approval rather than hard-failing the entire batch.
3. **Partial Application:** If one file in a multi-file patch fails the search block, `appy` should still apply the successful patches to the other files rather than rolling back the entire transaction.

***
