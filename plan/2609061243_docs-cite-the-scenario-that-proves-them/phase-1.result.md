---
n: 1
title: claiming.md and the matrix point at each other
status: "✅"
result: true
summary: claiming.md cites its scenarios; lease-protocol.md links its feature files
---

## Handoff

claiming.md's six behavioral sections — two machines at once, staleness
and takeover, liveness veto, self-resume, fencing and yield, and when a
claim is refused — each now cite the matrix id that proves them (`S26`,
`S15`, `S35`, `S31`, `S3`, `S78`, `S18`), with a link to the id's
feature file at each file's first mention in the document. Every id
was confirmed against lease-protocol.md's matrix by grep before citing
it, and two early picks were swapped on review: `S16` for the
`orphans` rescue-ref sweep became `S78` (`S16` never exercises
`orphans`, while `S78`'s own scenario text says "orphans lists both
tips as rescued"), and the staleness-and-takeover paragraph's single
trailing `S15` grew a second citation, `S35`, once code review found
`S15`'s scenario proves only the matured-takeover mint (epoch E+1,
child of the stale tip) and never exercises the `k · T` backoff the
same paragraph's closing clause claims — `S35` is lease-protocol.md's
own citation for that backoff (`backoff damps it (S35)` in its
Residual risks section).

lease-protocol.md's nine `### `-level matrix sections (Process death
through Cross-layer, with Lifecycle anomalies linking both
lifecycle.feature and landed-evidence.feature) each now open with a
link to the feature file that holds their scenarios.

Both files were at or near mdsmith's hard caps already — claiming.md
sat exactly at the 300-line ceiling, lease-protocol.md near the 8000
heuristic-token budget — so citations had to be folded into existing
sentences (wider-wrapped where the ceiling was tight) and feature-file
links de-duplicated to their first mention per file, rather than
repeated at every citing sentence, to land the change without pushing
either file over. `go test ./internal/scenario` and `mdsmith check .`
are both green.

No later phase is planned; the plan's own Tasks note ux-principles.md,
commands.md, architecture.md and reaping.md as later work if picked up
separately, but this plan's Execution table has only phase 1.
