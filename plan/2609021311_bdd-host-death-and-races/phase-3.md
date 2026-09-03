---
n: 3
title: S29, the release-vs-loser's-read race, runs for real
status: "🔳"
result: false
---
Convert S29 from `@pending` into a passing scenario. It is the one
race row a sequential test cannot produce by sequencing two real
machines alone. A fresh claimant's Acquire only ever attempts a push
when its own first read finds the ref absent or carrying a release
marker. A claimant that correctly reads a live claim returns
`HeldError` straight off that first read, with no push and so no
reconciliation read to race anything into. Racing a release into that
reconciliation read therefore needs an injected side effect, not two
more `attemptsClaim` calls.

**Assumes.** The CAS-loss path, traced through
[internal/claim/lease.go](../../internal/claim/lease.go):

- `Acquire` reads the ref's current tip once
  (`remoteHolder`/`remoteHolderErr`, a single `ls-remote <remote>
  <ref>` through the injected `gitwt.Runner`). An absent tip goes
  straight to `pushClaimMarker(ref, "", 1, run)` — a fresh claim.
- `pushClaimMarker` mints a claim marker locally
  (`rev-parse`/`commit-tree`, no network) and calls `casPush`: one
  `push --force-with-lease=<ref>:<expected>`. A push whose expected
  value no longer matches the remote fails for real; `casPush` then
  re-reads the ref (`remoteHolderErr` again — the loser's
  reconciliation read) to classify the loss.
- `heldError` reads whatever marker sits at that re-read tip, fetching
  it if the object is not yet local, and copies its `Kind`, `Holder`
  and `Epoch` verbatim into the `HeldError` it returns — a release
  marker's tip reads exactly as easily as a claim marker's, and
  nothing here tells the two apart. That is the row: a `HeldError`
  built off a release marker still names the releasing holder, as if
  they still held it.
- `gitwt.Runner` (`internal/gitwt/git.go`) is a plain function type
  threaded as an explicit parameter through every lease call, never a
  package var — a caller substitutes a wrapping closure for one
  specific call with no seam added to `internal/claim` itself.

**Value.** S26 through S28 already prove the ordinary shapes of a
claim race — a live rival, a rename that never reaches the ref, a
human's deleted ref. S29 is the shape those miss: the ref does not go
quiet during a race, it goes free — and if a CAS loser's own
reconciliation read lands in that exact instant, the *fact* it reads
is a release, not a hold, yet nothing downstream tells it apart from
one. A regression that lets a real bug hide behind that
misclassification — a retry that spins, an epoch that skips — would
otherwise ship unnoticed, because no scenario forces this exact
window today.

**RED.** Drop `@pending` from S29 in
[races.feature](../../features/races.feature) and write its
Given/When/Then:

```gherkin
@S29
Scenario: release races a loser's read
  Given "box-a" holds the lease for plan 7
  When "box-b" claims plan 7, racing "box-a"'s release into the read
  Then "box-b"'s claim loses, naming "box-a" at epoch 1
  When "box-b" retries plan 7
  Then "box-b"'s retry acquires at epoch 2
  And origin carries one work ref for plan 7
```

Every step but one already exists, reused verbatim from the races
vocabulary `bdd_lease_test.go` and `bdd_host_death_and_races_test.go`
built:

- `holdsTheLease` for the Given.
- `attemptsClaim` for both the losing claim and the retry.
- `claimLosesNaming`, `retryAcquiresAtEpoch` and
  `originCarriesOneWorkRef` for the three Then steps.

Run `go test ./cmd/frit -run TestFeatures -v`. The new subtest fails
on the one undefined step — that is the red. Commit it.

**GREEN.** Extend `cmd/frit/bdd_host_death_and_races_test.go` with the
one new step and its wrapper:

- `^"([^"]+)" claims plan (\d+), racing "([^"]+)"'s release into the
  read$` — a fresh clone for the holder (`cloneAs`, same as
  `attemptsClaim`'s first-time path), then `claim.Acquire` called with
  a wrapping `gitwt.Runner` instead of `gitwt.Exec`. The wrapper:
  - answers the very first `ls-remote` call with an empty result
    (simulating a claimant whose own read is stale-absent, from before
    the race), letting `Acquire` take the fresh-claim branch and
    attempt a real push with `expected=""` against a ref that
    genuinely already carries the releasing holder's live claim — so
    the push fails for real, no faking needed there;
  - on that push's failure, synchronously runs `claim.Release` for the
    releasing holder — for real, against the real remote, using the
    holder's own clone and its own recorded lease tip as `from` —
    before returning control to `casPush`;
  - passes every other call (the `rev-parse`s, `commit-tree`, the
    failed push itself, and the reconciliation `ls-remote` that
    follows) straight to `gitwt.Exec` unmodified, so that
    reconciliation read is a real read of the state the injected
    release just left.
- Store the losing attempt in `racesState.attempts`, exactly as
  `attemptsClaim` does, so `claimLosesNaming` and the reused retry
  steps read it back with no changes of their own.

**Guard the edges.** Two traps:

- The wrapper's "first `ls-remote` only" rule must key on call
  identity (a counter, flipped once), not on the ref or remote
  argument — `casPush`'s reconciliation read uses the identical
  `ls-remote <remote> <ref>` invocation, and must not be faked.
- The injected release must fire exactly once, only on the specific
  push this scenario's own claim attempt makes — never on a push
  another step in the same scenario might issue — so it is gated on
  "not yet released" as well as "this call is a push that just
  failed."

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S29:'` passes,
reported PASS, not SKIP. `go test ./internal/scenario` (the bijection
gate) stays green. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean.

Write the handoff to `phase-3.result.md`. Confirm this design against
what the test run actually showed. Restate what S32 still needs: the
stateful herdr fake reflecting a first `start --go` call's own freshly
created worktree as live to a second, same-host call. One wrinkle to
carry forward — herdr's `worktree create` RPC never creates a real
git worktree itself in production, so the fake must perform a real
`git clone --branch <plan-branch> <origin> <--cwd path>` as a side
effect the moment it intercepts that call, or the second call's
presence read has no real checkout to resolve.
