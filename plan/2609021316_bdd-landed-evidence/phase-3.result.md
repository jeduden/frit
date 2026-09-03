---
n: 3
title: The Gather rows of landed evidence run for real
status: "✅"
result: true
summary: >-
  S80 and S87 drop `@pending` and run for real. S80 builds a
  repository whose origin's main has advanced past a clone's own
  local main, and confirms `board`'s own report names
  `laggingDefaultBranch`'s exact wording, not merely any problem. S87
  reuses `landedDeletedClone` as-is and reads the same repository
  shape twice, once under the default `--fetch` and once under
  `--no-fetch`, confirming the flag alone moves the plan between
  landed-and-off-the-board and held-off-the-stale-view. A `rebuild`
  recipe on `landedEvidenceState`, called fresh on every board read,
  keeps the two reads independent — a fetch mutates a clone's refs on
  disk for good, so S87's second read would otherwise find the first
  read's own already-refreshed view instead of the stale one the row
  exists to prove.
---
## Handoff

**The one design addition this phase made.** `boardFleetRoot` reads a
`rebuild func() string` off `landedEvidenceState` when one is set,
calling it fresh on every read-verb When rather than reusing a fixed
`root`. S80's Given never sets it — one root, one read, reused as
`bdd_lease_test.go`'s own conventions already do elsewhere. S87's
Given sets it, since its own scenario reads the same repository shape
under two different flags and the first read's fetch would otherwise
silently erase the staleness the second read is meant to find — the
exact hazard `TestFetchFlagReachesTheReadWalk`'s own comment already
named, reproduced here as a real failure before the fix
(`board runs with the default fetch` passed, `... with --no-fetch`
failed: "plan 87 is off the board" — the first run's own prune had
already run). `TestBoardFleetRootRebuildsFreshEveryCall` pins that the
recipe is never memoized.

**No other finding.** `theReportNamesTheLocalDefaultBranchLagging`
matches `laggingDefaultBranch`'s own wording, not "any problem
present" — pinned by
`TestTheReportNamesTheLocalDefaultBranchLaggingWantsTheWording` against
an unrelated fault. S87's two Then steps each read `le.board` fresh
off their own `emit` call, never a value carried over from the other
branch. Every new step function carries its own unit test, per
CLAUDE.md.

**S59 is the last row.** `frit ready` in
[main.go](../../cmd/frit/main.go), backed by `discovery.Ready` in
[ready.go](../../internal/discovery/ready.go), is the read verb. Phase
2's own handoff already confirmed `doneByRepo` there marks a plan done
purely by its file's own `status: "✅"`, with no landed-evidence check,
so a dependent naming a hand-flipped plan in `depends-on` already
reads as ready today; `internal/doctor` still carries no early-✅
check. Nothing in phase 3 changed either fact. S59's own scenario
needs a Given building two plans in one repository — an upstream
hand-flipped to ✅ with no real merge behind it, and a dependent naming
it — a When running `frit ready`, and a Then confirming the dependent
is listed, with the `doctor` gap named in the handoff rather than
fixed, closing the plan.

All tests are green: `go test ./cmd/frit -run 'TestFeatures/S(80|87):'`
reports two PASS, none SKIP; `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are clean.
