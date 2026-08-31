---
id: 2608310355
title: Raise phase-spec token budget for detailed proving slices
status: "🔲"
summary: >-
  The `phase-spec` kind's `token-budget` is 800 heuristic tokens,
  calibrated to the largest phase body written so far (~660). A
  genuinely detailed proving-slice spec — Assumes, Value, RED, a
  file-by-file GREEN list and a multi-break Gate — runs 900–1700+
  heuristic tokens, so it either fails the budget or is forced back
  inline into plan.md, defeating the per-phase resume the folder shape
  exists for. Raise the phase-spec (and phase-record) budget to fit a
  genuinely detailed phase with headroom.
model: sonnet
depends-on: []
---
# Raise phase-spec token budget for detailed proving slices

## Goal

Raise the `phase-spec` and `phase-record` `token-budget` in
[.mdsmith.yml](../.mdsmith.yml). A genuinely detailed proving-slice
spec then passes the linter, rather than being pushed back inline into
`plan.md`.

## Context

Issue #113. [.mdsmith.yml](../.mdsmith.yml) sets the `phase-spec`
kind's `token-budget.max` to 800 heuristic tokens, commented as sitting
above the largest phase body written so far (~660) with headroom. A
detailed proving-slice spec — Assumes + Value + RED + a file-by-file
GREEN list + a multi-break Gate — was observed at 900–1700+ heuristic
tokens. At 800 such a spec fails the budget, so the author is forced to
keep the phase inline in `plan.md`, defeating the per-phase resume the
folder shape is for. `phase-record` (currently 900) is a living record
that grows the same way.

The owner chose to raise the budget (not document a split-signal) and
consented to this `.mdsmith.yml` edit for this issue.

**Reuse first.** The budget lives only in the two kind definitions in
[.mdsmith.yml](../.mdsmith.yml); no Go change is involved. mdsmith
itself enforces it, so the gate is `mdsmith check` on a representative
detailed phase-spec fixture — failing at 800, passing at the raised
value. The fixture must sit at a `plan/*/phase-*.md` path so the
`phase-spec` kind is assigned to it.

## Tasks

1. Raise `phase-spec` `token-budget.max` from 800 to a value that fits
   a detailed proving-slice spec with headroom (target 1800, above the
   observed ~1700 ceiling); raise `phase-record` from 900 to match.
   Update the explanatory comments to reflect the new calibration and
   the reason (detailed proving-slice specs run 900–1700+).
2. Do not weaken any other rule; change only the two `max:` values and
   their comments.

## Execution

| Phase | Title                                        | Tier   | Gate                                                                                                                                                       |
| ----- | -------------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | Raise the phase-spec and phase-record budget | sonnet | A ~1500-token phase-spec fixture at `plan/*/phase-*.md` fails `mdsmith check` before the edit (MDS token-budget) and passes after; `mdsmith check .` clean |

## Acceptance Criteria

- [ ] `phase-spec` `token-budget.max` raised to fit a detailed
      proving-slice spec (>=1700), `phase-record` raised to match
- [ ] A representative detailed phase-spec (~1500 heuristic tokens)
      fails `mdsmith check` at the old budget and passes at the new one
- [ ] No other lint rule is relaxed; only the two budgets and their
      comments change
- [ ] `mdsmith check .` is clean
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
