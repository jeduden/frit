---
n: 1
title: First run becomes a getting-started page
status: "🔲"
result: false
---
Move the README's First run walkthrough to a focused `docs/`
getting-started page. The README keeps a two-line quickstart and a
pointer; the page carries the full init-to-first-lane steps. This
proves the extraction shape the Configuration and JSON sections copy.

**BDD coverage.** None applies. This phase moves documentation; its
gate is `mdsmith check` and resolving links, not a scenario.

**Assumes.** The README's First run section is the init-to-first-lane
walkthrough — `frit init`, writing a plan, `mdsmith check`, `doctor`,
`ready`, `claim`, `board`. The README already links focused pages from
a Documentation map, and [docs/commands.md](../../docs/commands.md) is
the most recent example of an extracted page plus a map row. The
300-line file cap (MDS022) is on every markdown file.

**Value.** The front page stops opening the door and walking the
newcomer through the house in one breath. A reader who wants to start
follows one link; a reader who wants the pitch is not scrolling past a
shell transcript to reach the Documentation map.

**RED.** No test turns red — this is prose moved between files. Guard
against a broken move instead: before editing, note that `mdsmith
check .` is clean, so a dangling link or a cap breach after the move
is caught.

**GREEN.** Create `docs/getting-started.md` with the First run
walkthrough, in the voice of the existing docs pages. In the README,
replace the section body with a minimal quickstart — point frit at a
root, `frit init`, then the one link — and a pointer to the new page.
Add a Documentation map row for it. Keep every internal link relative
and correct from each file's own directory.

**Guard the edges.** The moved page must not exceed the 300-line cap
either, and the README must drop under it — the point of the move.
Reuse the map's existing table shape rather than a new one. Do not
reword the walkthrough; moving it is the change.

**Gate.** `mdsmith check .` is clean, the README is under 300 lines,
and every link into and out of `docs/getting-started.md` resolves. The
README still reads as a front page: pitch, quickstart, map. `go test
./...` is green.
