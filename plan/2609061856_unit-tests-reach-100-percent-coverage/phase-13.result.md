---
n: 13
title: cmd/frit/claim.go, dispatch.go and drift.go close the package, which joins the gate
status: "✅"
result: true
summary: >-
  claim.go, dispatch.go and drift.go reached 100% of their reachable
  lines — thirty-five new tests, no seam or exclusion needed — and
  cmd/frit joined the CI gate, completing plan task 2.
---
## Handoff

`go tool cover -func` filtered to these three files started this phase
with fifty zero-count ranges between them — the gap [phase 12](phase-12.md)'s
own result surfaced. Every gap closed as this phase's own RED/GREEN
worklist expected, with two findings worth carrying forward:

- **`resolveOwnLane`'s `herdr.Resolve` returning an empty root is
  reachable, but only with a call-counting stub**, not a real git
  state: `inOwnLane` already calls the identical `rev-parse
  --show-toplevel` internally and requires it to succeed, so the two
  calls only diverge when the injected runner is stateful. Unlike
  yield.go's own excluded git-fault call site (phase 9), this one
  needed no real corruption to reach — a deterministic
  fail-on-the-second-call stub sufficed, so it stayed a test rather
  than becoming a seventh exclusion.
- **`printDrift`, `landedLabel` and `lastPhaseLabel` had zero coverage
  beyond the empty-table case** — every existing `drift` test reads
  `--json`, and none ever exercised the table renderer at all. One
  direct call against two hand-built rows, covering every
  landed/last-phase combination, closed all three at once.

Closed by function group:

- `claim.go`: `Run`'s two gaps (R1, and `mintClaim`'s own git fault —
  an unreachable origin on a fresh claim, distinct from the
  already-tested lost-race path), `resumeOwnLease`'s push failure,
  `resolveOwnLane`'s and `inOwnLane`'s own guards, `unwindFailedStandUp`
  naming both a stand-up cause and a release failure together,
  `mintOrTakeOver`'s own lost-takeover-resets-the-window branch (a real
  competing takeover from a second clone, called directly),
  `scavengeGlyph`'s coordinate guard, `resetWindow`'s `observe.Path`
  failure, `vetoRefusal`'s and `claimRefusal`'s remaining branches, and
  `printClaim`'s rescue-and-warning rendering.
- `dispatch.go`: `openCmd.Run`, `nudgeCmd.Run` and `messageCmd.Run`
  each closed with R1, an unresolvable selector, and their own
  `herdr.Prompt`/`herdr.Focus` failures propagated from the shared
  `nudgeSend`/`messageSend` helpers; `printOpenNextStep` and
  `holdKindFor` closed with direct calls.
- `drift.go`: `driftCmd.Run`'s R1 and ambiguous-repo skip,
  `newDriftRepoContext`'s four error sources (the last, `allCommits`,
  via a stub that fails only `git log`) plus its non-branch-ref skip,
  `bucketByID`'s overflow guard, `allCommits`'s own malformed-line
  guard, `lastPhaseNumber`'s non-numeric-phase and no-integer-parses
  branches, and the table-renderer trio above.

`go tool cover -func` now reads 100.0% on every function across these
three files. `go test ./cmd/frit -coverprofile` reports the package at
100% except the six declared exclusions (`yield.go:65-67`,
`start.go:1226-1233`, `main.go:3068-3070`, `main.go:2670-2675`,
`progress.go:26`, `main.go:3094`). `scripts/check-coverage.sh
./internal/report ./internal/claim ./internal/fleet ./internal/observe
./internal/repocfg ./internal/herdr ./cmd/frit` passes, and
`.github/workflows/ci.yml`'s call now carries all seven packages. `go
test ./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
both green. No BDD scenario was needed — every gap is an existing path
fed an input its tests never constructed.

Plan task 2 is complete: every `internal/*` package this plan touched
plus `cmd/frit` now reads 100% of its reachable lines, gated. Task 3 —
adopting a branch-coverage tool and driving every reachable condition
both ways — remains unplanned; no phase file exists for it yet.
