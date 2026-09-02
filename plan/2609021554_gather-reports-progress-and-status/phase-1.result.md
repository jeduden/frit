---
n: 1
title: A required reporter and a returned status on Gather
status: "✅"
result: true
summary: >-
  Gather takes a required Reporter and emits Start / Repo / Done as it
  walks; it returns a Summary (discovered, read, fetched, problems,
  elapsed) on every Result. The one production seam renders progress to
  a terminal stderr and stays silent under --json.
---
`fleet.Gather` now takes a `Reporter` as a required parameter, so no
caller can gather without one, and it emits into that reporter
unconditionally: a `Start` naming the walkable repositories, a `Repo`
per repository in walk order, and a `Done` carrying the status. It also
returns a `Summary` on every `Result` — repositories discovered, read
and fetched, problems met, and elapsed — computed by construction, so
no caller can hold a gathered fleet without its status.

**Progress fetch signal.** `fetchRemote` now reports whether it
actually refreshed a remote, so the summary's `Fetched` count reflects
real fetches rather than every repository walked with fetch on.
`gatherRepo` carries that bool up to the walk.

**Render gate.** `progressFor` in
[cmd/frit/progress.go](../../cmd/frit/progress.go) renders progress only
when stderr is a real terminal and the run is not `--json`; every other
run — a pipe, a file, a test buffer, a `--json` consumer — gets
`DiscardReporter`, keeping the JSON contract and the golden outputs
clean. Verified against a built `frit` over a three-repository fleet:
`[1/3] atlas … [3/3] zephyr` and a closing status line reach a terminal
stderr while stdout carries only the report; the same verb under
`--json` leaves stderr empty.

**Handoff.** The reporter seam is in place, so the two remaining phases
change only their own surface. Phase 2 makes the terminal rendering
transient — a redrawn line rather than one per repository — inside
[cmd/frit/progress.go](../../cmd/frit/progress.go) alone. Phase 3
projects `Result.Summary` into the report model so `frit <verb> --json`
and the table both surface the gather's status; the `Summary` fields it
needs are already populated.
