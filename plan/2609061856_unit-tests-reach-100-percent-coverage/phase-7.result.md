---
n: 7
title: cmd/frit/release.go reaches 100% line coverage
status: "✅"
result: true
summary: >-
  cmd/frit/release.go reached 100% line coverage — six new tests, no
  seam and no exclusion needed — proving the fixture-reuse approach
  extends cleanly into cmd/frit's first file.
---
## Handoff

`go test ./cmd/frit -coverprofile` started this phase with
`release.go` at 87.5% of statements — `Run`, `releaseHeld` and
`printRelease` each partial. Every gap closed as the phase spec
expected, no seam and no exclusion:

- `Run`'s three early returns — `gatherFleet`'s error, `resolveSelector`'s
  error, and the `ambiguousRepo` refusal — each closed with a direct
  CLI fixture: a missing `--root`, an empty selector run outside any
  held checkout, and two same-named checkouts under root (the same
  fixture `TestClaimRefusesAnAmbiguousRepoName` already established).
- `releaseHeld`'s CAS push-failure branch closed by calling the
  unexported function directly with a hand-built `runtime`, bypassing
  `Run`'s unconditional `rt.git` reassignment — the workaround the
  phase's own Assumes section anticipated. One surprise: `claim.Release`'s
  `casPush` cannot tell an unrelated push failure from a genuine lost
  CAS — any push rejection where the remote ref still resolves to a
  real marker reads as fenced, naming whichever holder's marker sits
  there (in this case the test's own, since nothing else moved the
  ref). The test asserts the failure is reported (`doc.Warning`
  contains `"release:"`) and the lease is left untouched, rather than
  asserting the literal injected error string.
- `printRelease`'s `Rescue`/`Warning` rendering branches closed with
  two direct calls against a hand-built `report.ReleaseDoc`, no CLI
  needed.

`go test ./cmd/frit -coverprofile` now reports `release.go` at 100.0%
via `go tool cover -func`. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are both green. No BDD
scenario was needed — no lease-protocol behavior changed, only tests.
`./cmd/frit` is not yet added to `scripts/check-coverage.sh`'s CI
call; that waits for [phase 12](phase-12.md), the last file in the
package.

**Inherited by phase 8:** `reap.go` is next. Its own spec already
names two branches as structurally dead rather than undertested
(`Run`'s second `gatherFleet` error check, and `repoRemoteBase`'s
error-surfacing check) — this phase's own experience supports treating
that as read rather than re-litigating it: a coverage gap is not
always a missing test.
