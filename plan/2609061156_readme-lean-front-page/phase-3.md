---
n: 3
title: Scripting with JSON shrinks to a pointer and one example
status: "✅"
result: false
---
Shrink the README's Scripting with JSON section: keep the one-line
claim and the one example command, drop the "Three rules" paragraph.
Point at [ux-principles.md's JSON
Contract](../../docs/ux-principles.md#the-json-contract), which
already states those rules. Unlike phases 1 and 2, this section gets
no new `docs/` page — the fact already lives in ux-principles.md, so
the fold is a cut, not a move.

**BDD coverage.** None applies. This phase moves documentation; its
gate is `mdsmith check` and resolving links, not a scenario.

**Assumes.** ux-principles.md's "The JSON contract" section already
states the same three rules — every key present, a list is `[]` and
never null, a repository frit could not read is carried in the
document — so the README's paragraph is pure duplication, not a
distinct fact.

**Value.** A rule stated once has one place to go stale. Today a
change to the JSON contract has to be kept in sync in both the README
and ux-principles.md by hand; folding removes the second copy.

**RED.** No test turns red — this is prose removed, not moved. Guard
against a broken fold instead: before editing, note that `mdsmith
check .` is clean, so a dangling link or a cap breach after the edit
is caught.

**GREEN.** In the README, keep the section's opening sentence and the
`frit orphans --json | jq ...` example, delete the "Three rules ..."
paragraph, and replace it with a single sentence pointing at
[ux-principles.md's JSON
Contract](../../docs/ux-principles.md#the-json-contract) for the
rules and the golden files that pin them.

**Guard the edges.** The README must stay under the 300-line cap.
This phase adds no new file. Do not reword the rules into the
pointer sentence — state that they live in ux-principles.md, not what
they are.

**Gate.** `mdsmith check .` is clean, the README is under 300 lines,
and the link into ux-principles.md#the-json-contract resolves. The
README still reads as a front page: pitch, quickstart, map. `go test
./...` is green.
