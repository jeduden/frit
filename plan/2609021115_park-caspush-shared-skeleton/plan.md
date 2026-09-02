---
id: 2609021115
title: casPush and park share their push-then-classify skeleton
status: "🔲"
summary: >-
  casPush, park, and (after plan 2609021114) Scavenge's delete
  confirmation each independently attempt a git push and, on failure,
  re-read the remote with remoteHolderErr to classify what actually
  happened. A post-close review of plan 2609012210 flagged the
  casPush/park duplication as real but judged it a design task, not a
  rushed extraction: the three switches diverge in what they push,
  what success does, and what shape each failure returns. This plan
  extracts the narrow shared part — one push, one classify-on-failure
  read — into a single helper so a future fix to that read can't be
  applied to one site and forgotten in the others.
model: opus
depends-on: [2609021114]
---
# casPush and park share their push-then-classify skeleton

## Goal

`casPush`, `park`, and Scavenge's delete confirmation run one push and
classify a failure by the same shared read, in one place. Today each
carries its own independently written copy of it.

## Context

**The gap.** A post-close review of plan 2609012210 found that
`park`'s rewrite gave it the same push-then-classify shape `casPush`
already had: attempt a git push, and on a failure, read the remote
with `remoteHolderErr` rather than trust git's stderr. The reviewer
judged this a real duplication risk. A future fix to the
classification logic could land in one function and be forgotten in
the other. It recommended a separate design pass over folding the
extraction into that plan, though, since the two switches already
diverge in what they push and what each outcome returns.

**Why now, and why after 2609021114.** Plan 2609021114 gives Scavenge's
delete confirmation this same shape, which turns a two-site
duplication into a three-site one — a stronger case for a shared
helper, and enough real call sites to draw its boundary from evidence
rather than guess it from two. This plan depends on that one landing
first.

**Reuse first.** `remoteHolderErr` is exactly the primitive both the
current code and the new helper build on; nothing about it changes.
The extraction is scoped to the narrow shared part — one push, one
classify-on-failure read — not to the full switch each caller runs
afterward, which stays inline per call site. See phase 1's own
**Design** note for why: the three callers' success side effects and
failure-shape returns differ enough that forcing them through one
callback-shaped function would trade one duplication for a worse one.

**Considered and not taken.** Widening the helper to also cover
Scavenge's top-of-function read and Yield's still-held read (a bare
read-and-classify, no push involved) is left out. Those two are
already a simpler, already-shared shape between themselves, and mixing
a "push then classify" helper with a "just classify a read" one would
blur what each is for.

## Tasks

1. Phase 1 (proving slice): design the shared helper's boundary from
   the three real call sites, extract it, and rewire `casPush`, `park`,
   and Scavenge's delete confirmation onto it with no behavior change.

## Execution

| Phase | Title                                                                    | Tier | Gate                                                                               |
| ----- | ------------------------------------------------------------------------ | ---- | ---------------------------------------------------------------------------------- |
| 1     | casPush, park and Scavenge's delete share one classify-on-failure helper | opus | go test ./internal/claim unchanged and green before and after; full suite and lint |

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

| #   | Status | Phase                                                                                  |
| --- | ------ | -------------------------------------------------------------------------------------- |
| 1   | 🔲     | [casPush, park and Scavenge's delete share one classify-on-failure helper](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] `casPush`, `park`, and Scavenge's delete confirmation classify a
      failed push through one shared helper, not three independent
      copies
- [ ] No typed error, message wording, or exported signature changes
      for any of the three
- [ ] Every existing test on `casPush`, `park`, and `Scavenge` passes
      unchanged
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
