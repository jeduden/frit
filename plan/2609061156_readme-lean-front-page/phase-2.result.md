---
n: 2
title: Configuration becomes a focused docs page
status: "✅"
result: true
summary: >-
  Configuration moved to docs/configuration.md, pointing at CLAUDE.md
  and ux-principles.md for the precedence order instead of restating
  it; the README keeps a one-line pointer plus a Documentation map
  row.
---

## Handoff

`docs/configuration.md` now carries the `.frit.yml` key reference
moved verbatim from the README's Configuration section (`plan-dir`,
`holds`, `remote`, `takeover-window`, `sample-gap`, `base`). Rather
than restating the four-step precedence list, the page points at
[CLAUDE.md](../../CLAUDE.md#configuration) for the exact order and
[ux-principles.md](../../docs/ux-principles.md#two-kinds-of-setting)
for why the two kinds of setting resolve differently — the
duplication the plan's Context flagged is gone. The README's
Configuration section is now two sentences plus a pointer, and the
Documentation map gained a row for it.

README dropped from 242 to 226 lines; `docs/configuration.md` is 21
lines. `mdsmith check .` is clean (317 checked, 0 failures) and `go
test ./...` is green.

The only section left on the plan's list is Scripting with JSON:
shrink it to a pointer plus the one example, since it mostly restates
the JSON contract already in [ux-principles.md's JSON
Contract](../../docs/ux-principles.md#the-json-contract). No phase
file exists for it yet; write `phase-3.md` before starting.
Its close is the one that ticks the plan's last Acceptance Criteria
and flips `plan.md`'s `status` to ✅.
