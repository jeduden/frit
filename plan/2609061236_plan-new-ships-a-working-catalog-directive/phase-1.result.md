---
n: 1
title: plan-new shows a catalog directive that does not double
status: "✅"
result: true
summary: >-
  `plan/proto.md` now carries the working `## Phases` catalog
  directive as a literal, fenced so mdsmith shows it rather
  than runs it, globbing both `phase-*.md` and `phase-*.result.md`
  with a `row-expr` that branches on `result` — the same form every
  closed-out plan already uses. A trailing paragraph names the trap: a
  closed phase's `phase-N.result.md` also matches `phase-*.md` and
  carries the same `n`, so the naive single-glob form renders that
  phase's row twice. `internal/skills/assets/plan-new/SKILL.md` points
  its "Write `plan.md`" step at that literal instead of re-describing
  it in prose, since the skill was already at 612 of its 650-token
  budget with no room for the ~20-line directive itself; the dogfood
  copy is regenerated via `frit skills --via "go run ./cmd/frit"`, not
  hand-edited. A scratch folder plan built from the naive glob
  reproduced issue #155 (a doubled row once `phase-1.result.md`
  appeared); the same plan built from the shipped directive rendered
  one row before the phase closed and one spec row plus one indented
  summary row after, with no duplicate.
---
# plan-new shows a catalog directive that does not double

## Handoff

The Phases catalog trap from issue #155 is closed: an author copying
plan-new's instructions now lands on the branching, both-globs
directive because it sits in `plan/proto.md` as copyable text, not
because they reconstructed it correctly by hand. Nothing else in this
plan remains — it was a single-phase plan and this closes it.

Verified directly against the built `mdsmith` (not just `go test`):

- A scratch plan using the naive `phase-*.md` glob with a plain row
  rendered one row, then two identical rows once a
  `phase-1.result.md` was added — reproducing #155.
- The same scratch plan rebuilt with the shipped directive rendered
  one row before the close, and one spec row plus one `↳` summary row
  after — never doubled.
- `mdsmith check .` (311 files) and `go test ./...` are green;
  `TestDogfoodCopiesMatchCanonical` passed on the regenerated
  `.claude/skills/plan-new/SKILL.md`.
- `go run ./cmd/frit doctor --json` reports no findings.

No follow-up parked; the branching directive's rendering behavior was
out of scope and untouched.
