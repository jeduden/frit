---
id: 2609061236
title: plan-new ships a working Phases catalog directive an author can copy
status: "🔲"
summary: >-
  plan-new describes the Phases catalog in prose but ships no working
  directive, so an author reaches for the obvious shape — glob
  phase-*.md with a plain row — which silently duplicates a row the
  moment a phase closes, because phase-N.result.md also matches
  phase-*.md and carries the same n. mdsmith fix regenerates the
  doubled table happily, so nothing goes red, and the corruption lands
  in the close-out commit least likely to be re-read. The drift gate
  stops an author writing the fix back into the installed skill. The
  fix is to ship the working directive itself: the same catalog real
  plans already use — globbing both phase-*.md and phase-*.result.md
  with a row-expr that branches on result — shown as a literal an
  author copies, inside a fence so mdsmith renders it rather than runs
  it. Then note that phase-N.result.md matches phase-*.md, the trap the
  naive glob falls into.
model: sonnet
depends-on: []
---
# plan-new ships a working Phases catalog directive an author can copy

## Goal

An author following plan-new copies a working catalog directive
for the Phases table, rather than inventing the obvious one that
duplicates a row when a phase closes. The skill shows the directive as
a literal, and warns that `phase-N.result.md` matches `phase-*.md`.

## Context

**The bug, from issue #155.** plan-new (v0.11.0) tells the author to
build the Phases table as a catalog directive "over both `phase-*.md` and
`phase-*.result.md` with a `row-expr` interleaving each spec row with
its result's summary row", but gives no working directive. The shape an
author naturally reaches for —

renders one row per phase while the plan is open. When a phase closes,
`phase-N.result.md` is created — and it also matches `phase-*.md` and
also carries `n`, so the table gains a second, identical row. `mdsmith
fix` regenerates it that way without complaint, so nothing reddens, and
the doubling lands in the close-out commit an author least re-reads.

**Why it cannot be fixed downstream.** `frit skills --force` owns the
installed `.claude/skills/plan-*`, and a repo may gate on the copies
being byte-identical to what the pinned frit ships. So an author who
works out the right directive cannot write it into the installed skill;
the next re-install drops it. The fix has to ship in the skill.

**Reuse first — the working directive already exists.** Every real
folder plan already carries the catalog that does not double:
[plan/2609050854](../2609050854_claim-lane-carries-its-token/plan.md)
globs both `phase-*.md` and `phase-*.result.md`, sorts numerically, and
uses a `row-expr` that branches on `result` — a spec row for a
`phase-N.md`, an indented summary row for its `phase-N.result.md`. That
directive is proven by every closed-out plan in `plan/`. This plan
lifts it into the skill verbatim rather than inventing a new one. The
issue's own one-line alternative, `glob: ["phase-*.md",
"!phase-*.result.md"]`, drops the summary rows; the branching form
keeps them, matching what plans already render, so it is the one to
ship.

**The "unshowable" half.** A literal catalog directive written into the
skill markdown would be run by mdsmith, not shown. A fenced code block
shows it verbatim — mdsmith does not read a directive inside a fence,
the same rule that lets the matrix quote a pipe row in a fence without
its being a table row. So the directive ships inside a fence.

**Where it ships.** The canonical text is plan-new's asset under
[internal/skills/assets](../../internal/skills/assets); `frit skills`
regenerates the dogfood copy, guarded by
`TestDogfoodCopiesMatchCanonical`. The `skill` kind caps the file at
650 heuristic tokens, so the directive must earn its space —
[plan/proto.md](../proto.md) may be the better home for the full
literal, with the skill pointing at it, if the budget is tight.

**Out of scope.** No change to mdsmith or to how a catalog renders.
The branching directive already works; this plan only makes it
copyable and warns about the `result.md` match.

## Tasks

1. Phase 1 (proving slice): ship the working catalog directive as
   a literal an author copies — in plan-new's canonical asset, or in
   `proto.md` with the skill pointing at it — and warn that
   `phase-N.result.md` matches `phase-*.md`. Prove a plan built from it
   renders one row per phase both before and after a phase closes.

## Execution

| Phase | Title                                                   | Tier   | Gate                                                                                                                 |
| ----- | ------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------- |
| 1     | plan-new shows a catalog directive that does not double | sonnet | a plan from the shipped directive shows no duplicate row when a phase closes, against the built mdsmith; tests green |

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

| #   | Status | Phase                                                                 |
| --- | ------ | --------------------------------------------------------------------- |
| 1   | 🔲     | [plan-new shows a catalog directive that does not double](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] plan-new ships the working catalog directive as a literal
      an author can copy, inside a fence so mdsmith shows it
- [ ] The skill or `proto.md` warns that `phase-N.result.md` matches
      `phase-*.md`, the trap the naive glob falls into
- [ ] A folder plan built from the shipped directive renders one row
      per open phase and no duplicate after a phase closes, confirmed
      against the built mdsmith
- [ ] The dogfood copy is regenerated, not hand-edited, and
      `TestDogfoodCopiesMatchCanonical` is green
- [ ] Every touched skill stays within its 650-token budget
- [ ] `mdsmith check .` and `go test ./...` are green
