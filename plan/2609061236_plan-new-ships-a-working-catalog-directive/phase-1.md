---
n: 1
title: plan-new shows a catalog directive that does not double
status: "✅"
result: false
---
Ship the working Phases catalog directive as a literal an author
copies, and warn that `phase-N.result.md` matches `phase-*.md`. An
author then reaches for the shipped form, not the obvious one that
gains a second row when a phase closes.

**BDD coverage.** None applies. This phase ships skill or template
text; its gate is the built mdsmith run over a real folder plan, not a
scenario.

**Assumes.** The canonical text is plan-new's asset under
[internal/skills/assets](../../internal/skills/assets); `frit skills`
regenerates the dogfood copy, guarded by
`TestDogfoodCopiesMatchCanonical`. The `skill` kind caps the file at
650 heuristic tokens. [plan/proto.md](../proto.md) describes the
catalog in an HTML comment but ships no literal directive. Every real
folder plan already carries the working one — globbing both
`phase-*.md` and `phase-*.result.md` with a `row-expr` that branches
on `result`. mdsmith does not read a directive inside a fenced code
block, so a fence shows it verbatim.

**Value.** The close-out commit an author least re-reads stops
silently doubling a row. The one form that duplicates — a plain
`phase-*.md` glob — is named as the trap, and the form that does not
is there to copy.

**RED.** No test turns red — this ships prose. Guard against a false
green: build a scratch folder plan whose Phases catalog uses the
naive `phase-*.md` glob with a plain row, add a `phase-1.result.md`,
run the built mdsmith, and confirm it renders two identical rows. That
reproduces #155 and is the behaviour the shipped directive must avoid.

**GREEN.** Put the working directive where an author copies it — in
plan-new's canonical asset inside a fenced block, or in
[plan/proto.md](../proto.md) with the skill pointing at it, whichever
the 650-token budget allows. Include the both-globs list, the
branching `row-expr`, and one line warning that `phase-N.result.md`
matches `phase-*.md`. Regenerate the dogfood copy with `frit skills
--via "go run ./cmd/frit"`; never hand-edit the copy.

**Guard the edges.** The literal directive must sit in a fence, or
mdsmith runs it instead of showing it — the "unshowable" half of the
issue. Keep the skill within its token budget; if it is tight, the
full literal lives in `proto.md` and the skill points there. Do not
change how a catalog renders — the branching form already works.

**Gate.** A folder plan built from the shipped directive renders one
row per open phase and, after a `phase-N.result.md` is added, no
duplicate row — confirmed against the built mdsmith. The naive glob
still doubles, proving the warning earns its place.
`TestDogfoodCopiesMatchCanonical`, `mdsmith check .` and `go test
./...` are green.
