---
id: 2609021114
title: Scavenge and Yield stop reading an unreadable remote as gone
status: "🔳"
summary: >-
  Scavenge's post-delete confirmation and Yield's still-held check both
  classify a read with the fold-to-absent `remoteHolder`, whose
  contract folds a failed read into "absent" — the exact bug plan
  2609012210 fixed in `park`'s push classification. A dropped
  connection after Scavenge's delete push makes it silently delete the
  local ref and report success for a delete that was never confirmed;
  the same fault in Yield's still-held check makes it park a lease that
  may still be live instead of refusing. Both switch to
  `remoteHolderErr` and return a typed, honest error on a read fault
  instead of guessing.
model: sonnet
depends-on: []
---
# Scavenge and Yield stop reading an unreadable remote as gone

## Goal

An unreadable remote at Scavenge's delete confirmation or Yield's
still-held check is reported as unconfirmed. It is never silently
folded into "gone" or "not held".

## Context

**The gap.** Plan 2609012210 fixed this same class of bug in `park`'s
push classification. A post-close review of that plan flagged two
sibling call sites still using the fold-to-absent `remoteHolder`:

- `Scavenge` in
  [internal/claim/lease.go](../../internal/claim/lease.go), after its
  delete push fails, reads `remoteHolder(...) != ""` to decide whether
  the delete actually landed. An unreadable remote reads as `""`, the
  check is false, and Scavenge deletes the local ref and reports
  success for a delete it never confirmed.
- `Yield`, before parking a fenced lane's work, reads
  `remoteHolder(...) == local` to refuse a still-live holder
  (`StillHeldError`). An unreadable remote again reads as `""`, so
  `"" == local` is false, and Yield proceeds to park a lease that may
  in fact still be held live.

**The cause.** Both sites predate `remoteHolderErr`. It was minted for
exactly this "gone and unreadable must not fold together" distinction,
and `park`, `casPush`, and Scavenge's own top-of-function read
(`TestScavengeErrsWhenTheRemoteCannotBeRead` pins that one) already
use it. These two call sites were missed when `remoteHolderErr` was
introduced, and again when plan 2609012210 swept `park`.

**Reuse first.** `remoteHolderErr` already exists and needs no
changes. The scripted-`gitwt.Runner` test pattern, established in
[internal/claim/caspush_test.go](../../internal/claim/caspush_test.go)
and reused by plan 2609012210 in
[internal/claim/park_test.go](../../internal/claim/park_test.go),
drives both new tests. `UnconfirmedPushError`'s shape (a `PlanID`,
wrapped `Err`, and an `Unwrap`) is the template each new error type
follows. Neither is reused verbatim, though: its wording ("push
failed … it may have landed") is specific to a landing push and would
misdescribe a delete or a still-held check.

**Considered and not taken.** Consolidating `park`, `casPush`,
Scavenge's delete and Yield's still-held check onto one shared
push-then-classify helper is left to a separate plan (2609021115).
That extraction needs its own design pass once this plan's two fixes
exist, so the helper's shape is drawn from four real call sites, not
two. Mixing a behavior fix with a structural refactor in one plan
would also make either harder to verify in isolation.

## Tasks

1. Phase 1 (proving slice): Scavenge's delete-confirmation read
   switches to `remoteHolderErr`; a read fault returns a typed,
   unconfirmed-delete error and leaves the local ref untouched.
2. Phase 2: Yield's still-held read switches to `remoteHolderErr`; a
   read fault refuses with a typed error before `park` is ever called.

## Execution

| Phase | Title                                                                | Tier   | Gate                                                                                 |
| ----- | -------------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------ |
| 1     | Scavenge's post-delete confirmation stops reading unreadable as gone | sonnet | new red tests in internal/claim pass green; every existing Scavenge test stays green |
| 2     | Yield refuses rather than guess when the still-held read fails       | sonnet | new red test in internal/claim passes green; every existing Yield test stays green   |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                                                                                         |
| --- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [Scavenge's post-delete confirmation stops reading unreadable as gone](phase-1.md)                                                                                                                                                                                            |
|     | ↳      | Scavenge's delete-confirmation read switches from remoteHolder to remoteHolderErr; a read fault now returns a typed UnconfirmedDeleteError wrapping both the delete's and the confirmation read's faults and leaves the local ref untouched, instead of silently deleting it. |
| 2   | 🔲     | [Yield refuses rather than guess when the still-held read fails](phase-2.md)                                                                                                                                                                                                  |
<?/catalog?>

## Acceptance Criteria

- [x] A Scavenge whose delete push fails and whose confirmation read
      also fails reports an unconfirmed delete and leaves the local
      ref untouched
- [x] A Scavenge whose delete push fails and whose confirmation read
      confirms the ref still present keeps today's behavior unchanged
- [ ] A Yield whose still-held read fails refuses with a typed error
      and never calls park
- [ ] A Yield whose still-held read confirms or denies the still-held
      case keeps today's behavior unchanged
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
