---
n: 1
title: A claim-only lane persists its token once it stands
status: "🔲"
result: false
---
Make `frit claim` leave the proof it mints. Once herdr reports the
worktree created, the minted tip is written as that lane's token. A
claim-only lane then behaves as a lane `start` created: it releases
from inside itself and resumes through `start` with no window.

**Assumes.** `claimCmd.Run` in
[cmd/frit/claim.go](../../cmd/frit/claim.go) mints through
`mintClaim`, then calls `standUpClaimWorktree`, which returns after
`herdr.WorktreeCreate` succeeds and records the path with
`doc.Stood`. `persistToken` in
[internal/claim/token.go](../../internal/claim/token.go) is
unexported and takes `LeaseOptions` plus a tip; it resolves the git
directory of the lane and skips silently when it cannot. `ownToken`
and `resumeToken` in claim.go read that file back. The claim tests
script `startHerdr`. That fake answers `worktree create` with canned
JSON and touches no disk. The fake that runs `git worktree add` at the
path frit supplies is `liveLaneHerdr`, in
[the host-death BDD file](../../cmd/frit/bdd_host_death_and_races_test.go).
The release tests run from inside a lane fixture.

**Value.** The one lane frit stands up without an agent stops being a
lane frit cannot prove. `release` and `start` from inside it work the
same way they work for a lane `start` created, with no new transition
and no new proof. Every later phase is about the lanes that still lack
a token; this phase makes that set the legacy set alone.

**RED.** In [cmd/frit/claim_test.go](../../cmd/frit/claim_test.go),
against `liveLaneHerdr`, lifted into a shared fixture if the BDD file
will not lend it:

- `TestClaimPersistsTheTokenOnceTheWorktreeStands`: after
  `frit claim <id>` reports the worktree, the token file under that
  worktree's git directory holds the minted tip.
- `TestClaimLeavesNoTokenWhenTheStandUpFails`: a fake that refuses
  `worktree.create` unwinds the lease as today, and no token file
  exists anywhere under the repository.

In [cmd/frit/start_test.go](../../cmd/frit/start_test.go):

- `TestStartPersistsTheTokenBeforeTheSessionBinds`: a fake whose pane
  session lookup fails still leaves the token file under the worktree,
  since `bindSession` returns before renewing when the session is
  empty.

In [cmd/frit/release_test.go](../../cmd/frit/release_test.go):

- `TestReleaseEndsALaneClaimAloneStoodUp`: claim, then `release` run
  from inside the stood-up lane, pushes a release marker; the JSON
  document reports `released: true` and an empty `refused`.

In [features/lifecycle.feature](../../features/lifecycle.feature), a
new `@S92` scenario: a plan claimed by `frit claim` on this machine,
then released from its own lane, ends the lease. Add row S92 to the
Lifecycle section of
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md)
so the matrix and the feature stay in bijection.

**GREEN.** Export a token writer from `internal/claim` that takes a
lane path, a plan id and a tip, shared with `persistToken` rather than
copied. Call it once `WorktreeCreate` returns, from
`standUpClaimWorktree` with `minted.Tip` and from `laneStandUpPane` in
[cmd/frit/start.go](../../cmd/frit/start.go) with the lease tip, so
neither verb's token depends on a later renewal. A failed write is
reported as a warning on the claim document, never an error: the lease
is on the remote and the CAS is the fence. Update the doc comment on
`standUpClaimWorktree`, and the matching comment in claim_test.go,
which say no lane ever persisted a token for a claim.

**Guard the edges.** `bindSession` persists again on renewal, but only
when herdr answered the best-effort session lookup, which is why the
stand-up write comes first; the second write of the same tip is a
no-op. A claim that resumes an existing lease through `claim.Resume`
already persists into a lane that exists, and is untouched. The unwind
path writes nothing.

**Gate.** Against the built frit in a scratch fleet with a bare remote:
`frit claim <id>`, then `frit release` from inside the new worktree,
prints `released` and the remote's tip is a release marker. The token
file exists before the release. `go test ./cmd/frit -run
'TestFeatures/^S92:'` runs rather than skips. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
