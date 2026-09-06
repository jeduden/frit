---
n: 1
title: First run becomes a getting-started page
status: "✅"
result: true
summary: >-
  First run moved to docs/getting-started.md; the README keeps a
  two-line quickstart and a pointer, plus a Documentation map row.
---

## Handoff

`docs/getting-started.md` now carries the full init-to-first-lane
walkthrough, moved verbatim from the README's First run section
(`export FRIT_ROOT`, `frit init --mdsmith`, writing a plan, `mdsmith
check`, `doctor`, `ready`, `claim`, `board`). The README's First run
section is now a two-line quickstart — `export FRIT_ROOT` and `frit
init` — plus a pointer to the new page, and the Documentation map
gained a row for it.

README dropped from 258 to 242 lines; `docs/getting-started.md` is 26
lines. `mdsmith check .` is clean (314 checked, 0 failures) and `go
test ./...` is green.

This proves the extraction shape: a focused `docs/` page plus a
map row, README section shrunk to a pointer. The next phase repeats it
for Configuration (fold toward CLAUDE.md/ux-principles' existing
precedence text rather than a new page) and Scripting with JSON
(shrink to a pointer plus the one example) — neither has a phase file
yet; write `phase-2.md` before starting.
