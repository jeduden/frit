---
n: 3
title: Scripting with JSON shrinks to a pointer and one example
status: "✅"
result: true
summary: >-
  Scripting with JSON keeps its one-line claim and one example; the
  duplicated "Three rules" paragraph is gone, replaced by a pointer to
  ux-principles.md's JSON contract.
---

## Handoff

The README's Scripting with JSON section now keeps only the opening
sentence and the `frit orphans --json | jq ...` example. The "Three
rules" paragraph — a near-verbatim duplicate of ux-principles.md's
"The JSON contract" section — is gone, replaced by one sentence
pointing there for the rules and the golden files that pin them. No
new `docs/` page: the fact already lived in ux-principles.md, so this
was a cut, not a move, unlike phases 1 and 2.

README dropped from 226 to 224 lines. `mdsmith check .` is clean (319
checked, 0 failures) and `go test ./...` is green.

This closes every section this plan's Context named — First run,
Configuration, Scripting with JSON — and every Acceptance Criterion.
Its close flips `plan.md`'s `status` to ✅ and ticks the plan's
remaining criteria.
