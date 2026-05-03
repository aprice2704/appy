Team Grey say:

## Overall verdict

This is **much better than a vibes-spec**, but it still leaves too much wiggle room in exactly the places where dev can rotate the canid again. 🐕‍🦺🔩

The core model is good:

```text
input bundle
→ preview parse
→ file stripes with status
→ optional compiler check
→ apply OK files
→ optional retest
→ decorate applied stripes
→ copy machine-readable-ish reports
```

The spec correctly identifies the three main layout zones, the stripe-state model, the control buttons, status decorators, armor handling, preview details, and recovery reporting .

But I would harden it into a **state machine spec**, because right now it says what buttons do, but not enough about **when state mutates**, **what survives across actions**, and **what must never happen**.

---

# Biggest strengths

## 1. Stripe model is the right UI primitive

The file-block stripe idea is sound. It gives the operator a fast visual ledger:

| State     | Meaning                         |
| --------- | ------------------------------- |
| `OK`      | preview matched, safe candidate |
| `ERROR`   | parse/match/syntax failure      |
| `APPLIED` | written to disk                 |
| `IGNORED` | intentionally skipped           |

This is exactly right for appy. The UI is not a document editor; it is a **patch-flight console**.

## 2. “Check Compiler must not disable Apply” is excellent

This line is important:

> **Check Compiler** performs in-memory validation and **MUST NOT disable Apply to Disk**. 

Good. That prevents the classic failure where an optional diagnostic pass accidentally becomes a gatekeeper.

But see below: you need to clarify what happens if compiler check downgrades some files to `ERROR`.

## 3. Copy Preview Errors and Matched Line Echo are exactly the right recovery loop

The spec says preview failures should be copyable as markdown and include closest-match hints, plus a matched line echo for `%%% match_line` recovery . This is very appy-shaped.

That gives you the golden loop:

```text
LLM emits patch
→ appy rejects surgically
→ user copies precise failure report
→ LLM repairs patch
```

This should be treated as first-class, not auxiliary.

## 4. Retest results decorate stripes instead of dumping JSON

Very good. The spec explicitly says retest results must not be dumped as raw JSON, and should decorate applied file stripes instead . That is the correct operator UX.

Raw JSON at the bottom of the screen is the UI equivalent of finding a squid in the bread drawer.

---

# Main problems to fix

## 1. Define the state machine explicitly

Right now, state is implicit. Add a canonical state transition table.

Suggested:

| Action                       | Before             | After                                        |
| ---------------------------- | ------------------ | -------------------------------------------- |
| `/api/preview` success       | none/input changed | file status = `OK`                           |
| `/api/preview` patch failure | none/input changed | file status = `ERROR`                        |
| `/api/preview` safety skip   | none/input changed | file status = `IGNORED`                      |
| `Check Compiler` pass        | `OK`               | remains `OK`, add compiler pass decorator    |
| `Check Compiler` fail        | `OK`               | becomes `ERROR`, add compiler fail decorator |
| `Apply to Disk` success      | `OK`               | `APPLIED`                                    |
| `Apply to Disk` failure      | `OK`               | `ERROR`, show write failure                  |
| `Retest Impacted` pass       | `APPLIED`          | remains `APPLIED`, add test pass decorator   |
| `Retest Impacted` fail       | `APPLIED`          | remains `APPLIED`, add test fail decorator   |
| input changes                | any                | stale preview cleared or marked stale        |

That last one matters.

If the input changes after preview, the old stripes are now stale. The spec should say one of:

```text
Option A: clear preview immediately on input edit.
Option B: mark existing preview as STALE and disable Apply.
```

I recommend **Option A**. Simpler. Fewer undead UI states.

---

## 2. Separate primary status from secondary verification

The spec currently says compiler failure downgrades files to `ERROR`, while test failure decorates applied files with 💥 . That is plausible, but needs a clearer model.

I’d define:

```text
primary_status:
  READY | ERROR | APPLIED | IGNORED

compiler_status:
  NOT_RUN | PASS | FAIL

test_status:
  NOT_RUN | PASS | FAIL
```

Then render:

```text
READY + compiler PASS = OK 🏅
ERROR + compiler FAIL = ERROR 💥 or ERROR compiler-fail
APPLIED + test PASS = APPLIED 🧪
APPLIED + test FAIL = APPLIED 💥
```

Do **not** overload one state field with all meanings. Otherwise dev will build a state lasagna.

---

## 3. “Ready” vs `OK` naming is inconsistent

The table uses:

```text
State: Ready
Chip: OK
```

This is not fatal, but specs benefit from one canonical term. Pick one.

I’d use:

```text
internal enum: READY
chip text: OK
```

And state that explicitly.

Suggested addition:

```md
The internal status enum is READY, ERROR, APPLIED, IGNORED.
The READY state renders as an `OK` chip.
```

---

## 4. `Apply to Disk` semantics need more detail

Current text:

> Commits all `OK` files to the filesystem. Only available if at least one file is in the `OK` state. Must execute immediately without a modal confirmation box. 

Good, but incomplete.

Add:

```text
Apply to Disk applies only files currently in READY.
It MUST NOT apply ERROR or IGNORED files.
It MUST NOT require Check Compiler to have run.
It MUST record per-file result.
If some files succeed and others fail, successful files become APPLIED and failed files become ERROR.
Apply button is disabled once there are zero READY files.
```

Also: define whether application is **all-or-nothing** or **partial**.

I think appy wants **partial per-file application**, because that matches the stripe model. But the spec must say so.

---

## 5. Compiler check needs “in memory” semantics nailed down

The spec says compiler check applies bundle in memory and validates syntax without committing to disk . Good.

But define:

```text
- uses current filesystem as base
- overlays all READY patches into an in-memory workspace
- validates the resulting files/packages
- does not mutate source files
- does not create persistent temp files under repo root
- may create temp files under OS temp dir if cleaned
```

That last part matters because you just got bitten by file operation weirdness. Appy should not produce scratch confetti inside the repo unless explicitly intended.

---

## 6. Armor rule is good but too strict unless UX explains it

The spec says `Remove @@@` appears only if every non-empty line starts with `@@@` . I agree.

But add what happens when armor is partial:

```text
If some but not all non-empty lines are armored, Remove @@@ MUST NOT appear.
The UI SHOULD display a small warning: "Partial @@@ armor detected; refusing automatic strip."
```

Otherwise the button just mysteriously fails to exist.

Also, after unarmor:

```text
Remove @@@ strips exactly one leading @@@ from each non-empty line.
It must not strip @@@ occurring later in a line.
It must immediately trigger preview.
```

---

## 7. Missing: loading/busy/error behaviour

This is a common UI regression cave. Add a section.

For each async action:

| Action          | Busy behaviour                                              |
| --------------- | ----------------------------------------------------------- |
| Clear & Paste   | disable while clipboard read/preview is running             |
| Preview         | show preview pending, disable Apply                         |
| Check Compiler  | disable Check Compiler only, not Apply unless preview stale |
| Apply to Disk   | disable Apply during write                                  |
| Retest Impacted | disable Retest during test run                              |

Critical point:

```text
Busy states must not erase existing stripes unless the action succeeds with a new preview model.
```

Otherwise one failed API request can blank the UI. Appy’s goblin window should not have a trapdoor.

---

## 8. Missing: API response contracts

The UI spec references `/api/preview`, compiler check, apply, retest, reports, but does not define response shape.

At minimum, define conceptual payloads:

```text
PreviewResponse:
  files[]
    path
    status
    patches[]
    net_lines
    match_summary
    error
    closest_match
    matched_line_echo

ApplyResponse:
  files[]
    path
    applied
    hash_before
    hash_after
    ledger_entry
    error

CompilerCheckResponse:
  files[]
    path
    compiler_status
    diagnostics[]

RetestResponse:
  packages[]
  files[]
    path
    test_status
    package
    summary
    failure_excerpt
```

You do not need final JSON schema in `ui_spec.md`, but you do need enough that dev cannot invent incompatible blobs.

---

# Specific wording fixes

## Current

> **Test Verified (`Retest Impacted`)**: Acquires a 🧪 (green pass) or 💥 (red explosion/fail) icon

Problem: 🧪 is not inherently green. Emoji color is platform-dependent.

Better:

```md
Test pass renders as `🧪 PASS`; test fail renders as `💥 FAIL`.
Do not rely on emoji color alone.
```

## Current

> **Compiler Verified**: Acquires a 🏅 or ✅ icon

Pick one. UI specs should not use “or” for visual contract unless it is intentional configurability.

I’d use:

```text
compiler pass: 🏅
compiler fail: ⚠️
test pass: 🧪
test fail: 💥
```

Then the RHS might render:

```text
OK 🏅
APPLIED 🧪
APPLIED 💥
ERROR ⚠️
```

## Current

> Diff Preview: The targeted replacement text rendered inside a `.replace-block`

This sounds like it shows only replacement text. But for operator confidence, you probably want:

```text
search block / old text
replacement block / new text
net lines
```

If space is tight, collapse search by default, but make it inspectable.

Otherwise the user sees what will be inserted but not what it matched. That weakens trust.

---

# Suggested added section: invariants

I’d add this verbatim-ish:

```md
## UI Behaviour Invariants

1. The UI MUST never apply a file in ERROR or IGNORED state.
2. The UI MUST never require `Check Compiler` before `Apply to Disk`.
3. `Check Compiler` MUST NOT write to repository files.
4. `Retest Impacted` MUST NOT alter primary file status; it only adds test decorators/reports.
5. Any input edit after preview MUST invalidate the current preview and disable Apply until preview is recomputed.
6. Status chips MUST remain visible at the RHS of each stripe header regardless of horizontal content length.
7. Copy Preview Errors MUST contain enough information for an LLM to repair the patch without seeing the whole UI.
8. Copy Result Ledger MUST only report files actually written to disk.
9. Partial success during Apply MUST be represented per file, not collapsed into one global success/failure banner.
10. Raw JSON responses MUST never be rendered directly in the main output zone.
```

This is the kind of section that stops future footguns from breeding in the walls.

---

# Suggested added section: button visibility matrix

Add a small table:

| Button              | Visible when                         | Enabled when                   |
| ------------------- | ------------------------------------ | ------------------------------ |
| Clear & Paste       | always                               | clipboard available / not busy |
| Remove @@@          | uniformly armored input              | not busy                       |
| Check Compiler      | preview exists                       | at least one `READY` file      |
| Apply to Disk       | preview exists                       | at least one `READY` file      |
| Fix File Paths      | preview has missing-file errors      | not busy                       |
| Copy Preview Errors | preview has errors                   | not busy                       |
| Copy Result Ledger  | apply completed with ≥1 applied file | not busy                       |
| Retest Impacted     | apply completed with ≥1 applied file | not busy                       |
| Copy Test Report    | retest completed                     | not busy                       |

This is boring but powerful. Boring here is load-bearing.

---

# Suggested added section: stale preview

Very important:

```md
## Stale Preview Handling

The preview model is valid only for the exact input string and repository snapshot used to produce it.

If the input textarea changes after preview:
- all existing preview/apply buttons are disabled immediately;
- existing stripes are cleared or marked STALE;
- applying stale preview data is forbidden.

If the repository changes between preview and apply:
- the backend must re-check search block matches before writing;
- any mismatch becomes ERROR and is not written.
```

That last point is essential. Preview is not a write lock.

---

# My recommended v1.5.20 delta

If you want this to be “nail down behaviour” grade, I’d revise around these pillars:

```text
1. canonical state model
2. button visibility/enabled matrix
3. backend response contracts
4. stale preview rules
5. apply partial-success semantics
6. no raw JSON invariant
7. no repo-root temp/build artifacts
```

## Bottom line

The spec is directionally strong and captures the operator experience well. But it is still too prose-shaped. The next version should be more like:

```text
state enum
transition table
button matrix
response contracts
invariants
```

That will give dev less room to “creatively comply” and turn appy into a haunted toaster.
