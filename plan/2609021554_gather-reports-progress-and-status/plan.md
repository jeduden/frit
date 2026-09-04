---
id: 2609021554
title: A fleet gather reports its progress and its status
status: "🔳"
summary: >-
  Gathering the fleet walks every repository under the root and fetches
  each one over the network, yet today it runs in total silence — a
  slow or hung fetch across a dozen repositories is a black box, and no
  caller can tell a gather that is working from one that has stalled.
  This plan makes progress and a status summary a structural obligation
  of the gather rather than an opt-in each of its ~20 callers could
  forget: `Gather` gains a required reporter it emits into as it walks,
  and it returns a status summary in its result by construction. The one
  production seam every verb funnels through renders the progress to
  stderr, and the status joins the report model so both the table and
  `--json` carry it.
model: sonnet
depends-on: []
---
# A fleet gather reports its progress and its status

## Goal

Every fleet gather emits progress as it walks the repositories. It
also returns a status summary of what it found. Both are enforced by
architecture. A caller cannot gather in silence, and cannot receive a
result without its status.

## Context

**The gap.** `Gather` in
[internal/fleet/gather.go](../../internal/fleet/gather.go) loops over
every repository `discover.Repos` finds, calling `gatherRepo` on each —
which fetches the remote, collects the plans, and reads the refs. The
fetch is network I/O, repeated once per repository. The whole loop
prints nothing until it returns, so a gather over a dozen repositories
with one slow or unreachable remote is indistinguishable from a hang.
Nor does the result say what the walk covered: a caller learns the
plans and the problems, but not how many repositories were discovered,
how many were read, or how long it took.

**The single seam.** Every read and mutate verb funnels through one
production call site: `gatherFleetOpts` in
[cmd/frit/main.go](../../cmd/frit/main.go), which is the only place
`fleet.Gather` is called outside tests. Wiring a reporter there gives
all ~20 callers progress at once, so none of them change. The runtime
already carries `stderr`; progress belongs there, never on stdout,
which the JSON contract owns
([internal/report/report.go](../../internal/report/report.go)).

**Ensuring it by architecture.** Progress is made unforgettable the
same way the git runners already are: `Gather` takes the reporter as a
required parameter, so the compiler rejects any new caller that omits
it, and `Gather` itself emits the events unconditionally. Whether a
given reporter renders them is the reporter's concern — the production
seam wires a rendering one, a test wires a discarding one. Status is
made unforgettable more strongly still: `Gather` always computes it and
returns it inside `Result`, so no caller can hold a result without its
status.

**Status belongs in the document, not only on stderr.** The JSON
Contract's rule is that a status a consumer branches on lives in the
document, and `report` builds the table and the JSON from one model. So
the summary — repositories discovered, read, and fetched, problems met,
and elapsed time — is carried on `Result` and projected into the report
model, surfacing in both renderings. Adding keys does not move the
schema version.

**Reuse first.** Two things the reporter tests need already live in
[internal/fleet/gather_test.go](../../internal/fleet/gather_test.go):
the recording-fake pattern for asserting a sequence of calls, and the
multi-repository fixture builders. The new tests reuse both rather than
standing up fresh scaffolding. The summary's counts are already in hand
on the walk — the repositories from `discover.Repos`, the problems
appended to `Result`. So the summary tallies what the gather already
knows, and re-derives nothing.

## Tasks

1. Phase 1 (proving slice): make the reporter a required parameter of
   `Gather`, emit start / per-repository / done events as it walks, and
   return a status `Summary` on `Result`. Wire the one production seam
   to a stderr reporter so a real run shows progress end to end. Drive
   it with a recording reporter asserting the event order and the
   summary counts.
2. Later, once the handoff shows the shape: make the stderr rendering
   transient and terminal-aware (a redrawn line on a TTY, plain lines
   under a pipe or test), and a final status line.
3. Later: carry the `Summary` into the report model so `frit <verb>
   --json` and the table both surface the gather's status.

## Execution

| Phase | Title                                                  | Tier   | Gate                                                                                                                                             |
| ----- | ------------------------------------------------------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | A required reporter and a returned status on `Gather`  | sonnet | new red tests in internal/fleet pass green; the built `frit` prints per-repository progress to stderr with stdout clean                          |
| 2     | The terminal progress is transient, plus a status line | sonnet | new `progress` tests pass — a redrawn line, a cleared close, a final status line; built `frit` shows one redrawing line with stdout clean        |
| 3     | The gather status joins the report model               | sonnet | new tests pass — a verb's `--json` and table both surface the gather status, every key present, `Schema` unchanged; built `frit --json` shows it |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                                                                         |
| --- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [A required reporter and a returned status on Gather](phase-1.md)                                                                                                                                                                                             |
|     | ↳      | Gather takes a required Reporter and emits Start / Repo / Done as it walks; it returns a Summary (discovered, read, fetched, problems, elapsed) on every Result. The one production seam renders progress to a terminal stderr and stays silent under --json. |
| 2   | 🔲     | [the terminal progress is transient, with a closing status line](phase-2.md)                                                                                                                                                                                  |
| 3   | 🔲     | [the gather status joins the report model, in table and JSON](phase-3.md)                                                                                                                                                                                     |
<?/catalog?>

## Acceptance Criteria

- [x] `fleet.Gather` cannot be called without a reporter: it is a
      required parameter, and the build fails if a caller omits it
- [x] `Gather` emits a start event, one event per repository as it
      walks, and a done event carrying the summary — verified by a
      recording reporter over a multi-repository fixture
- [x] `Gather` returns a status `Summary` on every `Result`, counting
      the repositories discovered, read, and fetched, the problems met,
      and the elapsed time
- [x] Running the built `frit` against a multi-repository root prints
      per-repository progress to stderr while it walks, and stdout
      still carries only the command's own output
- [ ] On a terminal, the progress is transient: the walk redraws a
      single line in place as it advances, then clears it and prints one
      closing status line, so a fast walk leaves one line behind
- [ ] The gather status joins the report model, so `frit <verb> --json`
      and the table both surface the counts — discovered, read, fetched,
      problems — with every key present and `Schema` unchanged
- [ ] A partial walk, where a fault stepped over a repository, renders
      the reduced counts (`Read < Discovered`) in both renderings
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
