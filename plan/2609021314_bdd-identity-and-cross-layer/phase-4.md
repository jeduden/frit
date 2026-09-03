---
n: 4
title: "A reset window, a fenced release and a doc boundary: S62, S63, S65, S66 run real"
status: "🔲"
result: false
---
Convert four more rows from `@pending` declarations into passing
scenarios: S62, S63, S65 and S66. Phase 3's handoff split the six
remaining rows in two. S63 and S72 turn on two machines racing the
verbs. S62, S65, S66 and S74 need a fixture no phase has built yet.
This phase takes the four needing no second clone and no true
concurrency. S63 reduces to Phase 1's own S86, once a live session is
layered on. S62 and S65 recombine the stale-hold fixture S60 and S61
already share. S66 is the doc-boundary row S48 already set the
convention for. S72 and S74 wait for a later phase, needing
`cloneRepoIntoRoot` in `bdd_process_death_test.go` and a true race no
row has needed yet.

**Assumes.** Everything phase-1.md, phase-2.md and phase-3.md
assumed, including this file's own thirty-four steps and
`identityAndCrossLayerState`. Phase 1 gives the stale-hold fixture,
its two herdr fakes, and the claim-over-the-stale-hold step. Phases 1
and 3 give the live-lane and takeover pairs, bound and unbound, and
the lane's own release run and refusal check.

Beyond the step file:

- `claim.TokenPath` and `claim.ReadToken`
  ([token.go](../../internal/claim/token.go)) are the token's own
  file API, already used this way by
  `bdd_partitions_and_clocks_test.go`.
- `leaseMessage` ([lease.go](../../internal/claim/lease.go)) writes
  `lane:` as `opts.Lane` verbatim — a filesystem path, never a
  `host:path` pair, and S66's whole assertion.
- `claimRefusal` and `notMaturedReason`
  ([claim.go](../../cmd/frit/claim.go),
  [main.go](../../cmd/frit/main.go)) hold `claim`'s own "not takeable
  until the window matures" wording, shared with `start`.
- `standUpClaimWorktree` in `claim.go` shows `claim` itself calls
  herdr to stand a worktree up after minting — the seam S65 needs,
  since a herdr that answers but names nobody lets that succeed where
  S60 and S61's unreachable fake could not.

**Value.** A fourth identity row and three more cross-layer rows gain
their executable promise. A stale hold whose holder keeps pushing
while herdr cannot be reached is never taken over out from under it.
The window a claimant would take it on resets the moment the tip it
watched moves — exactly as a live lane's own progress should read. A
lane whose session herdr confirms alive is fenced out all the same
once a takeover has landed elsewhere: liveness only ever vetoes an
*incoming* takeover, never rescues a lane a CAS has already lost. A
herdr that answers but shows nobody standing — the shape a restart
leaves before panes reattach — is enough to let a stale hold's
takeover complete cleanly, not merely to spare it the live-session
veto. And a lane's own marker and token, read back after the fact,
show why an NFS-shared clone across hosts is out of scope: the
`lane:` trailer is a bare path, the token lives inside that path's
own git directory, and neither carries a host at all.

**RED.** Drop `@pending` from S66 in
[identity.feature](../../features/identity.feature), and from S62,
S63 and S65 in
[cross-layer.feature](../../features/cross-layer.feature). Write each
one's Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S(62|63|65|66):'`: strict mode reports the new steps
undefined and the four subtests fail. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S62, host unreachable, agents pushing. Given "elsewhere" holds plan
  7 bound to a session, the window has matured, herdr is unreachable
  and the holder pushes a raw commit past the held tip, when "box-b"
  claims plan 7 over the stale hold, then claim refuses: already
  held, naming the window not yet matured. The pushed commit is the
  point: `claim`'s gather reads the advanced tip fresh,
  `discovery.Observe` ([stale.go](../../internal/discovery/stale.go))
  restarts the window at one sample, and `claimRefusal` reports the
  plan held but not stale before `mintOrTakeOver` ever runs. No veto,
  no takeover, the window alone governs. `bdd_process_death_test.go`'s
  own `observerResetsTheWindowOnTheNewTip` drives the reset directly;
  this row proves it end to end, through a real claimant's refusal.
- S63, pane alive, lease released. Given this machine holds plan 7 in
  a lane bound to a session, its token persisted, herdr confirms that
  session live, and a takeover bound to a session at a new epoch
  lands on plan 7, when the lane runs `release`, then it is refused
  and the takeover stands. The point is what does *not* change from
  Phase 1's own S86: fencing is the CAS, never identity, so a session
  herdr swears alive buys the lane nothing once a foreign takeover
  has moved the ref. `release` never consults herdr — the veto only
  guards an *incoming* takeover — so the fake here is proof by
  absence, the finding this phase records once the row is green.
- S65, herdr restarts, loses panes. Given "elsewhere" holds plan 7
  bound to a session, the window has matured and herdr shows no agent
  on the lane, when "box-b" claims plan 7 over the stale hold, then it
  takes over cleanly at the next epoch, and the veto never fired.
  `herdrShowsNoAgentOnTheLane` installs the same reachable, empty
  handshake S48 and S76 already use. Here it also answers the
  takeover's own worktree stand-up, so the claim completes instead of
  unwinding the way S61's dial-error fake forces it to: an
  unreachable herdr blocks the stand-up (S61); a reachable-but-empty
  one does not (S65).
- S66, NFS-shared clone across hosts. Given this machine holds plan 7
  in a lane with its token persisted, then the marker's lane trailer
  is a bare path naming no host, and the lane's token lives inside
  that path's git directory. No verb runs; this is the doc's own
  boundary, per the plan's Context: `leaseMessage` writes `lane:` as
  `opts.Lane` verbatim, never a `host:path` pair, and
  `claim.TokenPath` resolves entirely from that path's git directory
  — nowhere in either could a host be recorded.

**GREEN.** Extend `cmd/frit/bdd_identity_and_cross_layer_test.go`
with six new steps, each shipping its own unit test per CLAUDE.md:

- `herdrConfirmsTheLanesOwnSessionIsLive` (Given, S63): names the
  session the live-lane step already binds.
- `theHolderPushesARawCommitOnTopOfTheHeldTip` (Given, S62): a
  worktree add on the stale holder's own clone, an empty commit, a
  push — mirroring `twoRawCommitsArePushedOnTopOfTheLane`'s shape,
  for a lane that was never a live checkout.
- `theRefusalNamesTheWindowNotYetMatured` (Then, S62).
- `itTakesOverCleanlyAtTheNextEpoch` (Then, S65): reads the stale
  holder's own local branch back, exactly as
  `takeoverAtEpoch2SitsOnTheStaleTip` does, but for a plain takeover
  marker, not a release-wrapped failed stand-up.
- `theMarkersLaneTrailerIsABarePathNamingNoHost` (Then, S66).
- `theLanesTokenLivesInsideThatPathsGitDirectory` (Then, S66).

A `funlen` trip on `registerVerbLevelIdentityAndCrossLayer` splits a
sibling registrar off it, the way Phase 3's handoff already did once.

**Guard the edges.** Reuse these steps as-is, with no change to their
text — strict mode reports a redefinition as ambiguous:

- `holdsPlanBoundToASession`
- `theWindowHasMaturedForPlan`
- `herdrIsUnreachable`
- `herdrShowsNoAgentOnTheLane`
- `machineClaimsPlan`
- `theVetoNeverFired`
- `claimRefusesAlreadyHeld`
- `thisMachineHoldsPlanInALaneWithItsTokenPersisted`
- `thisMachineHoldsPlanInALaneBoundToASessionWithItsTokenPersisted`
- `aTakeoverBoundToASessionAtANewEpochLandsOnPlan`
- `theLaneRunsReleaseForPlan`
- `itIsRefusedAndTheTakeoverStands`

S62's pushed commit must land through a fresh `git worktree add` on
the stale holder's own clone. Never `t.Chdir`. That directory already
plays "another machine's own checkout" for `machineClaimsPlan`.
Chdir-ing into it would blur which side of the row is which.

S63's herdr fake must name the exact session the live-lane Given
already binds: `"wOld:p1"`, the session S77 already uses, not S61's
own `"wS:p9"`. Get this wrong and `herdr.SessionLive` reads the lane
as unattended, and the row proves nothing about liveness.

S66 asserts no verb output. Its two Then steps read the marker and
the token straight off the Given step's own fixture — the same
read-only shape S64's read-backs use.

**Gate.** `go test ./cmd/frit -run
'TestFeatures/S(45|46|47|48|49|60|61|62|63|64|65|66|73|76|77|86):'`
passes with every one of the sixteen reported PASS and none SKIP. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-4.result.md`. Record any finding a row
exposes — S63's proof-by-absence chief among them. Say what S72 and
S74, the two rows this plan still owes, need from `cloneRepoIntoRoot`
and a genuine two-process race the verbs have not required until now.
