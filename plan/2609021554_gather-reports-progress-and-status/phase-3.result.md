---
n: 3
title: the gather status joins the report model, in table and JSON
status: "✅"
result: true
summary: >-
  Every gathering verb's report now carries a gather block — discovered,
  read, fetched, problems, and elapsed_ms under a fixed key — in --json
  and as a closing table footer, both drawn from one report.Gather so
  they cannot drift. A partial walk shows read < discovered in both
  renderings; Schema stays 1.
---
The gather's coverage now rides in the document a consumer reads, not
only on the stderr progress a terminal sees. `report.Gather` carries
the counts — discovered, read, fetched, problems — and the elapsed time
in milliseconds under a fixed key `elapsed_ms`, so the shape is stable
for a consumer and deterministic for a golden. `StatusLine` renders the
one line the table shows from that same struct, so the table footer and
the JSON block cannot drift.

**Field name and shape.** The block is keyed `gather`, carried by a
shared `gathered` embed placed right after `header` on each gathering
document, so it opens the document just beneath `schema` and `command`.
Every count is always present, even at zero — the JSON contract's first
rule — so a consumer indexes it without first testing for it. Elapsed
is an integer millisecond count, not a duration string, so it reads the
same at every scale.

**One projection, sixteen verbs.** `SetGather` is promoted onto every
gathering document from the one embed, and the cmd layer projects each
walk's `Summary` through `gatherStatus` and prints the footer through
`printGather` — one converter and one renderer, not sixteen. All
sixteen gathering verbs carry it: the read and discovery verbs (ready,
pick, find, next, show, phase, board, orphans, drift, reap) and the
mutate verbs (open, nudge, claim, yield, release, start), the latter
stamped once where each builds its document so every refusal and
success path carries the coverage. The eight non-gathering verbs —
version, repos, plans, stale, doctor, who, init, skills — are untouched,
so their documents carry no misleading zero block.

**Contract held.** `Schema` stays 1: a new key changes no meaning, and
a consumer reading by name is unaffected. The twenty-two gathering
goldens (verbs and their variants) each grew only the seven-line gather
block, with no other line moved and every schema still 1 — confirmed by
diffing the update. Lists stay `[]`. Under `--json` the block is
populated and stderr stays empty; the transient stderr progress from
Phase 2 is unchanged. The board `--width` test now treats the status
line as a caption beside the fitted table rather than one of its rows —
the footer is a status caption, not a width-fitted table row.

**Verified end to end.** A cmd test drives a partial walk — a fleet of
two readable repositories and one the walk steps over — and asserts both
renderings show `read < discovered` and the problem count. The built
`frit ready --json` over such a fleet shows `gather` with populated
counts and `schema` 1; the table shows
`gathered 2/3 repositories, 0 fetched, 1 problem(s), in 16ms` on stdout
with nothing on stderr.

## Handoff

The plan is complete. `Gather` returns a status `Summary` on every
`Result` by construction (Phase 1), the terminal renders it as a
transient line that clears to a closing status and never wraps (Phase
2), and the report model surfaces the coverage in both the table and
`--json` on every gathering verb (Phase 3). A gather can no longer run
in silence, and no consumer receives a gathered fleet's answer without
the coverage that answer reflects.
