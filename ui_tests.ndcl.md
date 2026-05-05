# Appy UI Verification Checklist

This checklist locks the implementation of Appy to the `ui_spec.md` behavioral contracts. These tests MUST pass to certify an Appy release.

- | | 1. Top-Level Layout #(test-layout-group)
  - [ ] Verify browser tab text matches the last element of the sandbox path. #(t-lay-01)
  - [ ] Verify Appy version is explicitly displayed next to the title. #(t-lay-02)
  - [ ] Verify sandbox root subtitle is displayed. #(t-lay-03)

- | | 2. UI Behaviour Invariants #(test-invariants-group)
  - [ ] Verify Apply to Disk is disabled when NO files are in the READY state. #(t-inv-01)
  - [ ] Verify Apply to Disk completely ignores files in ERROR or IGNORED states. #(t-inv-02)
  - [ ] Verify Apply to Disk can be clicked BEFORE Check Compiler is run. #(t-inv-03)
  - [ ] Verify Check Compiler does not write to the filesystem or leave temp files in the repo root. #(t-inv-04)
  - [ ] Verify Retest Impacted does not alter the primary file status (APPLIED). #(t-inv-05)
  - [ ] Verify Retest Impacted correctly adds test decorators (🧪 or 💥) to the RHS of applied stripes. #(t-inv-06)
  - [ ] Verify raw JSON responses are never rendered directly in the main output zone. #(t-inv-07)

- | | 3. Stale Preview Handling #(test-stale-group)
  - [ ] Verify editing the input textarea immediately disables Apply and Check Compiler buttons. #(t-sta-01)
  - [ ] Verify editing the input textarea clears existing stripes to prevent applying stale data. #(t-sta-02)
  - [ ] Verify an out-of-band repository change between preview and apply triggers a backend safety rejection (ERROR state). #(t-sta-03)

- | | 4. Stripe Visualization & Layout #(test-stripe-group)
  - [ ] Verify the stripe header order is exactly: Filename -> Net Lines -> RHS Flex Container. #(t-str-01)
  - [ ] Verify Net Lines aggregate is visible at the header level. #(t-str-02)
  - [ ] Verify primary status chips (OK, ERROR, APPLIED, IGNORED) are pinned to the RHS. #(t-str-03)
  - [ ] Verify secondary decorators (🏅, ⚠️, 🧪, 💥) appear immediately to the left of the primary status chip in the RHS container. #(t-str-04)
  - [ ] Verify the Diff Preview shows both the old Search Block (collapsible) and the new Replacement Block. #(t-str-05)
  - [ ] Verify the ☢️ icon appears on the stripe RHS when a file is patched using the full file overwrite strategy. #(t-str-06)

- | | 5. Control Plane Button Matrix (Anti-Shift) #(test-matrix-group)
  - [ ] Verify buttons are logically grouped into Prep (Left), Action (Center), and Export (Right). #(t-mat-01)
  - [ ] Verify "Copy Preview Errors", "Copy Result Ledger", and "Copy Test Report" occupy the same spatial slot and swap seamlessly without shifting action buttons. #(t-mat-02)
  - [ ] Verify "Remove @@@" ONLY appears if the input is uniformly armored (every non-empty line starts with @@@). #(t-mat-03)
  - [ ] Verify "Remove @@@" correctly strips exactly one level of @@@ per line and triggers a preview. #(t-mat-04)

- | | 6. Failure & Recovery Reporting #(test-recovery-group)
  - [ ] Verify a failed preview populates the "Copy Preview Errors" payload with a Matched Line Echo. #(t-rec-01)
  - [ ] Verify a failed preview populates the "Copy Preview Errors" payload with LLM Fallback Hints based on LanguageProfile. #(t-rec-02)
  - [ ] Verify a failed Check Compiler run appends the raw compiler trace to the clipboard payload, NOT the DOM stripe. #(t-rec-03)
  - [ ] Verify a failed Retest Impacted run appends the raw test trace to the clipboard payload, NOT the DOM stripe. #(t-rec-04)
  - [ ] Verify top-level server errors (e.g., malformed bundle `unexpected EOF`) display the "Copy Preview Errors" button and populate the clipboard payload. #(t-rec-05)

- | | 7. File-Atomic Apply Semantics #(test-atomic-group)
  - [ ] Verify that in a multi-file bundle, a syntax error in file A correctly rejects file A but allows the successful application of file B. #(t-atm-01)
  - [ ] Verify that partial application surfaces distinct APPLIED and ERROR stripes simultaneously. #(t-atm-02)
  - [ ] Verify the Copy Result Ledger accurately reports ONLY the files that were written to disk. #(t-atm-03)

- | | 8. API Response Contracts #(test-api-group)
  - [ ] Verify `/api/preview` returns the strictly nested `files` -> `patches` array structure. #(t-api-01)
  - [ ] Verify `/api/apply` returns file-level applied status, net lines, and properly structured `failed_patch` blocks. #(t-api-02)
  - [ ] Verify `/api/retest` isolates error traces to the `raw_output` field of the `files` array. #(t-api-03)

- | | 9. Code & Metadata Syntax #(test-meta-group)
  - [ ] Verify generated Go files place metadata at the absolute top followed by one blank line. #(t-meta-01)
  - [ ] Verify the UI correctly identifies uniformly armored input with `@@@` prefixes and refuses partial armor. #(t-meta-02)
  - [ ] Verify the backend and frontend correctly strip leading whitespace from patch directives. #(t-meta-03)

- | | 10. Interactive Edge Cases & Micro-Interactions #(test-edge-group)
  - [ ] Verify pasting junk/empty text gracefully displays "No valid patches found" and disables action buttons. #(t-edg-01)
  - [ ] Verify the "Fix File Paths" button successfully rewrites the textarea and triggers a re-preview. #(t-edg-02)
  - [ ] Verify Export buttons temporarily change text to "Copied!" to provide visual feedback. #(t-edg-03)
  - [ ] Verify the "Retest" button disables and shows "Running Tests..." during execution. #(t-edg-04)


:: product: FDM/NS
:: majorVersion: 1
:: fileVersion: 2
:: description: E2E and Unit Test definitions derived strictly from the Appy UI Spec. Added API Contract and Metadata checks.
:: filename: ui_tests.ndcl.md
:: serialization: md