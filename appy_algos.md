# Appy Architecture & Algorithms Specification

## Section 1: Invariants — What Appy Must and Must Not Do

Appy is the airlock between untrusted, non-deterministic LLM output and the physical host filesystem. Its sole reason for existence is to prevent silent drift, partial truncation, and logic rot.

### 1.1 Invariants: What Appy MUST Do

1. **Fail Loud and Fast (Closed-World Enforcement):**
  - Reject any patch block whose directive, syntax, or boundaries are not explicitly recognized.
  - If a search target does not resolve uniquely, reject immediately with candidate details rather than guessing.

2. **Strict Uniqueness Verification:**
  - In fuzzy or normalized text mode, a target block MUST match exactly once across the whole file.
  - If $count > 1$, require an explicit disambiguator (`%%% replace <N>`) or additional surrounding context lines.

3. **Whole-File Determinism & In-Memory Isolation:**
  - Apply all patch hunks for a file in memory before touching disk.
  - Run formatting (`goimports`) and compiler pre-flight validation on the in-memory buffer prior to commit.

4. **Atomic Disk Commits with Historical Snapshots:**
  - Snapshot target files to `.appy_history/tx_<id>` before writing.
  - Commit writes only after all in-memory verifications pass.
  - Update `.appy_ledger.json` idempotently to prevent duplicate re-application loops.

5. **Zero-Byte & Truncation Defense:**
  - Halt and roll back if a non-empty file on disk would be truncated to 0 bytes without an explicit `%%% delete_file CONFIRM` directive.
  - Halt and roll back if a file of $\ge 20$ lines loses $>50\%$ of its lines without an explicit replacement or deletion contract.

6. **Jail Boundary Enforcement:**
  - Guarantee all target paths resolve strictly within the configured root directory (`isPathSafe`), violently rejecting path traversals (`../`).

---

### 1.2 Invariants: What Appy MUST NOT Do

1. **NEVER Rely on Line Hints for Matching:**
  - Line numbers (`near <line>`) MUST NOT act as hard bounding boxes or vetoes.
  - LLM line estimates are notoriously noisy; rigid line-window filters cause false rejections of otherwise unique matches.

2. **NEVER Accommodate Ambiguous Input:**
  - Do not pick the "closest" match when multiple identical blocks exist in a file.
  - Do not guess caller intent when syntax is malformed.

3. **NEVER Overwrite Without Explicit Authority:**
  - `%%% create` MUST NOT overwrite an existing $>0$ byte file.
  - `%%% complete_replace` MUST NOT target a missing or $0$-byte file.
  - Fuzzy `%%% replace` with an empty search block MUST NOT wipe or overwrite an existing file.

4. **NEVER Leave Corrupt Files on Disk:**
  - If a compiler check fails or post-write verification detects a collapsed buffer, disk state MUST be restored immediately from backup.

5. **NEVER Bypass Idempotency Without Instruction:**
  - If a patch hunk's SHA-256 hash exists in the active ledger, skip it gracefully unless explicitly forgotten via `/api/forget`.