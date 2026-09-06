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
`S15`, `S31`, `S3`, `S16`, `S18`), with a link to the id's feature file
at each file's first mention in the document. Every id was confirmed
against lease-protocol.md's matrix by grep before citing it.

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
