---
n: 2
title: Configuration becomes a focused docs page
status: "✅"
result: false
---
Move the README's Configuration section — the `.frit.yml` key
reference — to a focused `docs/configuration.md` page. The precedence
order for frit's own settings already lives in
[CLAUDE.md](../../CLAUDE.md#configuration), with the why in
[docs/ux-principles.md](../../docs/ux-principles.md#two-kinds-of-setting);
the new page points at both rather than restating the order. The
README keeps a one-line pointer.

**BDD coverage.** None applies. This phase moves documentation; its
gate is `mdsmith check` and resolving links, not a scenario.

**Assumes.** The README's Configuration section has two parts: the
`.frit.yml` key reference (the only place that abbreviated reference
lives) and the four-step precedence list, which duplicates
[CLAUDE.md](../../CLAUDE.md#configuration) almost exactly. Phase 1's
`docs/getting-started.md` is the most recent example of an extracted
page plus a map row.

**Value.** The `.frit.yml` key reference is something a person returns
to when writing the file, not something a newcomer reads once on the
front page. Splitting it out, and pointing at CLAUDE.md for the
precedence order instead of restating it, removes a duplication the
next precedence change would otherwise have to keep in sync by hand.

**RED.** No test turns red — this is prose moved between files. Guard
against a broken move instead: before editing, note that `mdsmith
check .` is clean, so a dangling link or a cap breach after the move
is caught.

**GREEN.** Create `docs/configuration.md` with the `.frit.yml` key
reference, in the voice of the existing docs pages, pointing at
CLAUDE.md for the exact precedence order and at ux-principles.md for
why the two kinds of setting differ. In the README, replace the
Configuration section body with a one-line pointer to the new page.
Add a Documentation map row for it. Keep every internal link relative
and correct from each file's own directory.

**Guard the edges.** The moved page must not exceed the 300-line cap
either, and the README must stay under it. Reuse the map's existing
table shape rather than a new one. Do not reword the `.frit.yml`
reference; moving it is the change.

**Gate.** `mdsmith check .` is clean, the README is under 300 lines,
and every link into and out of `docs/configuration.md` resolves. The
README still reads as a front page: pitch, quickstart, map. `go test
./...` is green.
