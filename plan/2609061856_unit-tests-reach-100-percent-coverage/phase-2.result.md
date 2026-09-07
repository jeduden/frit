---
n: 2
title: internal/claim reaches 100% line coverage and joins the gate
status: "✅"
result: true
summary: >-
  internal/claim reached 100% line coverage — one seam, thirty-odd new
  tests — and joined the hard CI gate beside internal/report.
---
## Handoff

`go test ./internal/claim -coverprofile` started this phase at 90.5%
of statements. The raw profile's zero-count ranges — not `go tool
cover -func`'s per-function percentages, which hide which branch of a
partial function is the gap — gave the real worklist: 3 partial spots
in `claim.go` and 27 across 21 functions in `lease.go`.

Every gap but one fit the pattern `claim_test.go`/`lease_test.go`
already used: a fake `gitwt.Runner` wrapping `gitwt.Exec` and failing
one named subcommand, or a direct call against a real two-clone git
fixture. That reached the eight untested `Error()`/`Unwrap()` methods,
every git-fault branch in `advance`, `pushClaimMarker`, `mintMarker`,
`Takeover`, `Scavenge`, `Yield`, `ParkUnlanded`, `hasUnlanded`,
`RescueRefs`, `AllRescueRefs`, `fetchedMarker` and `commitMarker`, the
pure string functions `rescuePlanID`, `markerSubject`, `freshBase` and
`LiveHold`, `landedTip`'s ancestor-true branch (a real ordinary merge,
not a squash, into base), `refuseDivergingLocalBranch`'s
ancestor-true branch (a local `plan/<id>` branch minted exactly at
base), and `OwnAdvance`'s not-itself-a-marker branch. One branch —
`pushClaimMarker`'s own lost-CAS path, distinct from `Acquire`'s
already-tested pre-push lost race — needed a real two-clone race timed
by a fake `ls-remote` that answers empty exactly once, so the second
caller takes the fresh-claim path and only discovers the real occupant
when its own push is rejected and it re-reads for real.

The one gap outside that pattern was `newNonce`'s `crypto/rand.Read`
failure: real entropy does not fail on command, so it had no seam.
Added `var randRead = rand.Read` in `lease.go` and read through it
instead of calling `rand.Read` directly — no other behavior changed —
so `TestNewNonceSurfacesAnEntropyFault` and
`TestMintMarkerSurfacesANonceFault` can force it. `go test
./internal/claim -cover` now reports 100.0% of statements, confirmed
clean under `-race -count=2` too.

`scripts/check-coverage.sh`'s CI call now covers `./internal/report
./internal/claim` together. Verified red the same way phase 1 did: a
throwaway untested branch in `claim.go` dropped the package to 99.8%
and the script exited 1; reverting it restored the exit-0, 100.0%
state. No BDD scenario was needed — no lease-protocol behavior
changed, only tests, the one seam, and the gate.

`go test ./...`, `go vet ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are all green.

**Inherited by the next line-coverage phase:** the same pattern — a
fake `gitwt.Runner` failing one named subcommand for a git-fault
branch, a direct call for a pure function or an error method, one more
package argument to `scripts/check-coverage.sh`'s CI call — plus the
seam-when-cheap precedent for a defensive branch no fake can reach.
`internal/fleet` is next per plan.md's task list.
