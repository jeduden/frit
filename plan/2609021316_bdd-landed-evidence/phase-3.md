---
n: 3
title: The Gather rows of landed evidence run for real
status: "✅"
result: false
---
Convert S80 and S87 from `@pending` into passing scenarios. Both are
decided by `Gather`, read through a read verb rather than a mutating
one.

S80 is `laggingDefaultBranch` naming a local default branch that fell
behind its own just-fetched remote-tracking ref. S87 is the global
`--fetch` flag. It controls whether a checkout unfetched since its
plan's lease was deleted upstream reads that plan as landed or still
held.

**Assumes.** Phase 2's handoff named exactly what carries over and
what does not. The CLI-level Given/root pattern in
`landedEvidenceState` carries over; the shared "a fleet-wide `reap
--go` runs" When does not, since both rows run over `board`, a read
verb, never `reap`.

- `laggingDefaultBranch` in
  [gather.go](../../internal/fleet/gather.go) is already unit-pinned
  by `TestGatherReportsALocalDefaultBranchLaggingItsFetchedRemote` in
  [gather_test.go](../../internal/fleet/gather_test.go): a repository
  whose local default branch is a fetched remote-tracking ref's
  strict ancestor gets a `Problem` naming the gap, using only the ref
  list `Gather` already reads.
- `landedDeletedClone(t, name, id)` in
  [main_test.go](../../cmd/frit/main_test.go) builds a clone whose
  origin has since squash-landed the plan and deleted its lease
  branch, while the clone's own `refs/remotes/origin/*` still carry
  the pre-land state. `TestFetchFlagReachesTheReadWalk` already
  proves S87's own claim over it once: `board`'s default fetch reads
  the plan as landed and off the board; `--no-fetch` reads the stale
  remote-tracking copy and the plan is still held.
- `report.BoardDoc.Problems` in
  [board.go](../../internal/report/board.go) is where S80's own
  evidence surfaces to a caller; `boardPlanByID` in
  [main_test.go](../../cmd/frit/main_test.go) is how S87's own Then
  already reads a plan's `Held` field back off a decoded `BoardDoc`.

**Value.** The read path is now proven end to end, not only at
`internal/fleet`'s own unit level: a fetched-but-unmerged local
default branch is a named problem on the board a person or an agent
actually sees, and `--fetch`/`--no-fetch` genuinely changes whether a
squash-landed, upstream-deleted lease reads as landed or held. A
regression in either fails the build.

**RED.** Drop `@pending` from S80 and S87 in
[landed-evidence.feature](../../features/landed-evidence.feature) and
write each one's Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S(80|87):'`: strict mode reports the new steps
undefined and the subtests fail. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S80, local default branch lags its own fetched remote-tracking ref.
  Given a repository whose origin's main has advanced past the local
  clone's own main — a second clone commits and pushes ahead, the
  first clone's own `refs/remotes/origin/main` stale or absent —
  when `board` runs (fetch on by default, so the fetch and the
  comparison both happen in this one run), then the report names the
  local default branch behind its own freshly fetched remote.
- S87, read verb reads landed evidence off a checkout unfetched since
  a PR merged. Given a checkout unfetched since its plan's lease was
  deleted upstream — `landedDeletedClone` — when `board` runs with the
  default fetch, then the plan reads as landed, off the board; when
  `board` runs with `--no-fetch` instead, then the same plan reads as
  held, off the stale local view — the same repository, judged twice,
  so the flag alone is what moves the answer.

**GREEN.** Extend `cmd/frit/bdd_landed_evidence_test.go`. Both rows
share a `board report.BoardDoc` field on `landedEvidenceState` and two
When steps — "board runs with the default fetch" and "board runs with
--no-fetch" — built over `emit`, the JSON-decoding helper
[json_test.go](../../cmd/frit/json_test.go) already provides; "board
runs" is a third step text bound to the same default-fetch function,
S80's own shorter phrasing. Their Then steps read `le.board.Problems`
and `boardPlanByID(le.board, ...)` directly — no new report parsing.
Every step function ships with a unit test of its own, per CLAUDE.md.

**Guard the edges.** S80's Then must match on the wording
`laggingDefaultBranch` actually writes ("commit(s) behind fetched"),
not merely "any problem present" — a repository with an unrelated
problem must not pass this row. S87's two Then steps must each read
`le.board` fresh off its own `emit` call, never a cached decode from
the other branch, or a bug that made `--no-fetch` a no-op could still
pass by comparing a document against itself. A scenario that only
passes by weakening an assertion is a finding, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(80|87):'` passes
with both reported PASS and none SKIP. `go test ./cmd/frit -run
TestFeatures` (every section landed so far) stays green. `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
clean.

Write the handoff to `phase-3.result.md`. Confirm S59 is the only row
left, over `frit ready`'s own `discovery.Ready`, with the
`internal/doctor` gap recorded rather than fixed — the plan's last
phase.
