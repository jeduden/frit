---
n: 2
title: internal/claim reaches 100% line coverage and joins the gate
status: "✅"
result: false
---
Drive [internal/claim](../../internal/claim) to 100% line coverage —
90.5% today. Add it to `scripts/check-coverage.sh`'s CI call alongside
`internal/report`, so both packages are locked together.

**BDD coverage.** None applies. This phase changes no lease-protocol
behavior — it adds unit tests, one non-behavioral seam, and extends
the coverage gate. No cross-host claim, release, yield or takeover
behavior changes, so no `@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** `internal/claim` has no process boundary of its own.
Every git call already goes through the injectable `gitwt.Runner`.
`claim_test.go`/`lease_test.go` already fake it per-callsite —
wrapping `gitwt.Exec` and failing one named subcommand — to reach an
error path real git will not misbehave into on demand. That pattern
reaches every gap but one: `newNonce`'s `crypto/rand.Read` failure has
no seam today, and real entropy does not fail on command, so it is
untestable as written.

**Value.** internal/claim is the lease atom every dispatch verb calls
through; proving every line — including the marker, rescue and
takeover error paths a live race rarely exercises — is the second
largest slice of the gap and the one nearest the top of the call
stack's risk.

**RED.** Record the package's current coverage: `go test
./internal/claim -coverprofile`, then read the raw profile's
zero-count ranges (not just `go tool cover -func`'s per-function
percentages, which hide which branch of a partial function is the
gap). That range list is the worklist: 3 partial functions in
`claim.go` and 27 zero or partial spots across 21 functions in
`lease.go`, enumerated in this phase's result.

**GREEN, the seam.** Add `var randRead = rand.Read` in `lease.go` and
call it from `newNonce` in place of `rand.Read` directly, so a test can
swap it to force the failure `mintMarker` propagates. No other
behavior changes.

**GREEN, the tests.** Cover each zero-count range by its own shape.
Eight `Error()`/`Unwrap()` methods are invoked nowhere: call each
directly. A git-fault branch in `advance`, `pushClaimMarker`,
`mintMarker`, `Takeover`, `Scavenge`, `Yield`, `ParkUnlanded`,
`hasUnlanded`, `RescueRefs`, `AllRescueRefs`, `fetchedMarker` or
`commitMarker` gets a fake `gitwt.Runner` that fails one named
subcommand (`fetch`, `push`, `ls-remote`, `rev-parse` or `log`) and
passes every other call through to `gitwt.Exec`. `pushClaimMarker`'s
own lost-CAS branch — distinct from `Acquire`'s already-tested
pre-push lost race — needs a real two-clone race timed by a
once-empty fake `ls-remote`. `landedTip`'s ancestor-true branch needs
a real ordinary merge, not a squash, into base. The pure string
functions `rescuePlanID`, `markerSubject`, `freshBase` and `LiveHold`
each get a direct-input unit test. Re-run `go test ./internal/claim
-cover` until it reports 100%.

**GREEN, the gate.** Add `./internal/claim` to the
`scripts/check-coverage.sh` call in
[.github/workflows/ci.yml](../../.github/workflows/ci.yml), beside
`./internal/report`. Verify red the same way phase 1 did: a throwaway
untested branch drops the package below 100% and the script exits
non-zero; remove it once seen.

**Guard the edges.** No new exclusion list entry: the seam covers the
one spot that looked like a boundary. Branch coverage stays out of
scope.

**Gate.** `go test ./internal/claim -cover` reports 100%; the gate
call in CI covers both `./internal/report` and `./internal/claim` and
reddens on an added untested line in either; `go test ./...` and `go
tool -modfile=tools/go.mod golangci-lint run` are green.
