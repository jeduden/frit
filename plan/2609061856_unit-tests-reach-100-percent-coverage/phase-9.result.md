---
n: 9
title: cmd/frit/yield.go reaches 100% of its reachable lines, and check-coverage.sh gains exclusions
status: "✅"
result: true
summary: >-
  cmd/frit/yield.go reached 100% of its reachable lines — seven new
  tests and one listed exclusion — and scripts/check-coverage.sh now
  understands a declared, justified exclusion list, proven red then
  green against a scratch fixture before the real entry was added.
---
## Handoff

`go test ./cmd/frit -coverprofile` started this phase with `yield.go`
at 90.0% of statements, spread across `Run`, `localRef`, `tearDownLane`
and `renderYield`. Every gap closed as the phase spec expected:

- `Run`'s three early returns (`gatherFleet` error, `resolveSelector`
  error, the `ambiguousRepo` refusal) each closed with a direct CLI
  fixture, the same shapes phases 7 and 8 already established.
- `localRef`'s real-fault branch closed by calling the function
  directly with a stub runner returning a plain error.
- `tearDownLane`'s `herdr.CurrentPane` and `herdr.WorktreeRemove`
  error branches both closed by calling the function directly against
  a hand-built `report.YieldDoc` — the first needs no repository at
  all, the second reuses the claim-then-fence-then-checkout fixture
  `TestYieldParksAFencedLaneAndTearsItDown` already established, just
  calling `tearDownLane` in place of the full CLI.
- `renderYield`'s JSON branch closed with the existing `emit` helper.
- `Run`'s outer call site at `localRef` (65-67) is the one exclusion
  the phase's own Assumes section anticipated: a real git fault after
  the ref is already confirmed present, not reachable without
  corrupting a live object store mid-run.

`go tool cover -func` filtered to `yield.go` reads 100.0% on every
line but 65-67.

**The exclusion mechanism.** `scripts/check-coverage.sh` was rewritten
to read `go test $pkg -coverprofile` directly rather than parse its
printed summary line, filtering out any statement block that falls
inside a declared range in the new `scripts/coverage-exclude.txt`
(`path/to/file.go:startLine-endLine  # reason`, one entry per
excluded range) before computing the percentage from the filtered
profile's own statement and count totals. A package with no matching
entries reads exactly as before — verified by re-running the six
already-gated packages, still 100.0% each.

Proven red then green against a throwaway scratch package
(`internal/scratchcov`, removed once proven): the *old* script still
failed it even with an exclusion entry present, since it had no way
to read one; the *new* script passed it once the exclusion existed,
and still failed a package with no exclusion covering its gap. The
scratch package left no trace in the tree.

The real entry, `cmd/frit/yield.go:65-67`, is now the sole line in
`scripts/coverage-exclude.txt`. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are both green. No BDD
scenario was needed — every gap is an existing path fed an input its
tests never constructed, and the gate change is tooling, not
lease-protocol behavior.

`./cmd/frit` is not yet added to `scripts/check-coverage.sh`'s CI
call — `start.go` and `main.go`/`progress.go` still carry gaps, and
the whole-package check only makes sense once every file in it is
closed.

**Inherited by phase 10:** `start.go` is next. It carries its own
judgment call, already resolved in the phase's own spec —
`editInEditor`'s `WriteString`/`Close` pair on an already-open temp
file is a genuine OS-level boundary, the same category this phase's
own `yield.go:65-67` fell into, and gets its own entry in
`scripts/coverage-exclude.txt` rather than a forced test.
