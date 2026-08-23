---
id: 2608222201
title: Reap streams per-lane progress, so a long run isn't silent
status: "🔳"
summary: >-
  reap parks a rescue ref per landed lane with a network push each, so a
  fleet-wide teardown runs many seconds while printing nothing until the
  whole loop ends and it reads as hung. Stream one line to stderr as each
  lane is actually reaped, dropped, or refused, leaving the final table
  and the --json document untouched.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: stranded lanes stream as they are reaped
    status: "✅"
  - n: 2
    title: held drops, pruned worktrees, and refusals all stream
    status: "🔲"
---
# Reap streams per-lane progress

## Goal

A `frit reap --go` that acts on many lanes prints what it is doing as it
does it. Each lane torn down, hold dropped, or action refused emits one
line to stderr at the moment it happens, so a run whose cost is serial
network pushes is legible in flight rather than silent until the end.
The stdout table and the `--json` document are byte-for-byte unchanged.

## Context

reap's cost is network, not compute. `reapStranded` and `reapUnstaffed`
in [cmd/frit/reap.go](../cmd/frit/reap.go) call `parkBranch` →
`claim.ParkUnlanded`, which pushes a rescue ref to origin per lane; a
bare `git ls-remote origin` already costs ~1.3s here, so a 13-lane reap
spends ~15–18s in serial pushes. `Run` builds the whole `report.ReapDoc`
across every repository and only then calls `printReap(rt.stdout, doc)`.
Nothing reaches the terminal during the pushes, so the command reads as
frozen — the report of what happened arrives only once it has all
happened.

Reuse searched:

- **`printProblems(rt.stderr, doc.Problems)`** — already the pattern for
  writing to `rt.stderr` beside the stdout report. Progress reuses the
  same stream and the same `rt.stderr` field, not a new sink.
- **`printReap`** — renders the final document to stdout via tabwriter
  after the loop. It is kept exactly as is; progress is a second,
  in-flight channel, not a replacement, so the batch table and its
  golden-free stdout assertions do not move.
- **`rt.stderr`** — already threaded into every reap function through
  `rt *runtime` ([cmd/frit/main.go](../cmd/frit/main.go)), so the park
  and teardown sites can write progress without a new parameter on the
  runtime. What they lack is knowledge of `--json`; the JSON contract
  (nothing to stderr under `--json`) is honored by passing an explicit
  progress writer that `Run` sets to `io.Discard` when `c.JSON` is set.
- No existing streaming/progress helper — nothing in the codebase emits
  incremental per-item output, so a tiny writer-per-action is new, but
  bounded to one `Fprintf` at each existing append site.

Because the JSON path must stay clean, the mechanism is an explicit
`progress io.Writer` argument threaded into the reap functions, set to
`rt.stderr` on the human path and `io.Discard` under `--json`. That
keeps the guard in one place (`Run`) rather than testing `c.JSON` deep
in each loop.

## Tasks

1. Stream the stranded-lane teardown: `reapStranded` writes one progress
   line per reaped and per refused lane to an injected writer, proven by
   a stderr assertion, with `--json` leaving stderr empty.
2. (determined after Phase 1)

## Phase 1: Stranded lanes stream as they are reaped

**RED.** In [cmd/frit/reap_test.go](../cmd/frit/reap_test.go), add a test
that stands up two stranded, landed checkouts (reuse
`strandedCheckout` + an ordinary merge per branch, as the existing
`TestReapRemovesALandedCheckoutAndDeletesItsBranchWithGo` does) and runs
`run([]string{"reap", "--go", "--root", root}, &out, &errb)`. Assert
that `errb.String()` contains a progress line for each worktree naming
its branch — a stderr line matching `reaped` and the worktree name — so
the two lanes are announced as they are torn down, not only in the final
stdout table. Add a second test that runs the same fixture with
`--json` and asserts `errb.String()` is empty, pinning the JSON
contract. Both fail today: nothing is written to stderr mid-loop, and
there is no progress writer.

**GREEN.** Thread a `progress io.Writer` parameter into `reapStranded`.
At the point a lane is appended to `reaped`, and at each `refused`
append, `fmt.Fprintf(progress, ...)` one line naming the fate, worktree
and branch. In `Run`, compute the writer once —
`progress := io.Writer(rt.stderr)`; `if c.JSON { progress = io.Discard }`
— and pass it to `reapStranded`. Leave `reapUnstaffed` and `reapPruned`
signatures unchanged this phase; only stranded lanes stream. `printReap`
is untouched, so the final table and existing stdout assertions still
pass.

**Gate.** `go test ./cmd/frit -run TestReap` green; `go build ./...`;
`go vet ./...`; the stranded-progress test and the `--json`-silent test
both pass; `go test ./internal/report` still green (the JSON golden is
unchanged).

## Phase 2: Held drops, pruned worktrees, and refusals all stream

**RED.** Add tests that a `--go` reap of an unstaffed-but-abandoned hold
(reuse the abandonment fixtures already in `reap_test.go`) and of a
prunable/empty worktree each emit their own stderr progress line as they
act, and that a refused hold (a live lease, as in the current refusal
tests) emits a `refused` progress line naming the reason. Assert
`--json` still leaves stderr empty across all of these. They fail:
Phase 1 streamed only stranded lanes.

**GREEN.** Thread the same `progress io.Writer` into `reapUnstaffed` and
`reapPruned` and emit one line at each `dropped`, `pruned`, `refused`,
and `refusedPruned` append site, matching the wording `printReap` uses
for that fate so the stream and the final table agree. `Run` passes the
one writer it already computed to all three functions.

**Gate.** `go test ./cmd/frit -run TestReap` green; every fate
(reaped, dropped, pruned, refused) has a proven stderr line; `--json`
stderr stays empty; `go build ./...`, `go vet ./...`,
`go test ./internal/report` all green.

## Execution

| Phase | Work                                                                    | Tier   |
| ----- | ----------------------------------------------------------------------- | ------ |
| 1     | Proving slice: stranded lanes stream to an injected writer, JSON silent | sonnet |
| 2     | Extend the same writer to drops, prunes, and refusals                   | sonnet |

## Acceptance Criteria

- [ ] A `--go` reap prints one stderr line per lane at the moment it is
      reaped, dropped, pruned, or refused — not batched at the end.
- [ ] Each progress line's fate word and identifiers match the final
      `printReap` table for that lane.
- [ ] Under `--json`, stderr stays empty and the JSON document is
      byte-for-byte unchanged (the `internal/report` golden is untouched).
- [ ] The final stdout table is unchanged; existing reap stdout
      assertions still pass.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...`, and
      `mdsmith check .` all pass.
