---
n: 3
title: "The resume-from-outside path and yield reach a step: S46, S47, S76, S77 run real"
status: "🔳"
result: false
---
Convert four more rows from `@pending` declarations into passing
scenarios: S46, S47, S76 and S77. Phase 2's handoff named these,
alongside S63 and S72, as the rows still needing the resume-from-outside
path or yield. This phase takes the four that need neither a second
machine nor a new race fixture. S63 and S72 wait for a later phase.

**Assumes.** Everything phase-2.md assumed. Also:

- `cmd/frit/bdd_identity_and_cross_layer_test.go`'s own twenty-seven
  steps and its `identityAndCrossLayerState`
- [start_test.go](../../cmd/frit/start_test.go)'s `heldLaneOwnedBy`
  and `dropToken`, which build a held lane and strip its token —
  Phase 2 already used both
- [lease.go](../../internal/claim/lease.go)'s `claim.Takeover`,
  called directly here the same way Phase 1's own step for S86 calls
  it

**Value.** One more identity row and one more cross-layer row for
each half of this plan's remaining verb-level set gain their
executable promise. A claim run from a directory that never carried
plan 7's token — the shape a reused worktree path or a cloned lane
leaves — is refused exactly as any other claimant's would be, never
mistaken for the holder. A teardown that itself fails still releases
the lease, and its own error names the worktree and pane it could not
clean up, so `frit orphans` has something to find. A held lane nobody
attends and whose window has not matured still resumes on its token
alone — no window is waited when there is nothing to take from anyone.
And a lane whose own token has been superseded by somebody else's
takeover refuses to reattach from inside itself, naming `yield` rather
than orphaning whatever it never got to push.

**RED.** Drop `@pending` from S46 and S47 in
[identity.feature](../../features/identity.feature), and from S76 and
S77 in
[cross-layer.feature](../../features/cross-layer.feature). Write each
one's Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S(46|47|76|77):'`: strict mode reports the new steps
undefined and the four subtests fail. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S46, worktree path reused. Given "elsewhere" holds plan 7 with its
  lane's token persisted, when this machine runs `claim` for plan 7
  from an unrelated directory, then claim refuses: already held, and
  the plan 7 ref is unchanged. Mirror
  `TestResumeIgnoresATokenFromAnotherLane`: a directory that never
  carried this plan's token — whether it once served a different
  lease or never served one at all — gets no shortcut from `ownToken`'s
  `inOwnLane` check, and the ordinary "already held" door is the only
  one open.
- S47, worktree debris fails the handoff. Given plan 7 is unclaimed
  and the agent fails to start and its own teardown leaves debris
  behind, when this machine runs `start --go` for plan 7, then start
  fails and a release marker sits on the branch, and the error names
  the worktree and pane left behind. Mirror
  `TestStartUnwindNamesWhatTeardownLeftBehindWhenItFails`: the closest
  sibling to Phase 2's own S73 — the difference is the teardown itself
  erroring (`worktree`+`remove` failing) rather than succeeding
  quietly, so the release marker's presence is not new ground but
  naming what the failed teardown left behind is.
- S76, pane gone before the window matures. Given a held lane holding
  plan 7 whose marker names "elsewhere" as holder and names no
  session, and herdr shows no agent on the lane, when this machine
  runs `start --go` for plan 7, then the plan is resumed, and no
  takeover marker sits between the held tip and origin's tip. Mirror
  `TestStartResumesAnUnboundHoldOnItsToken`: an unbound hold — one
  whose marker never named a session in the first place, the sharpest
  reading of "pane gone" — needs no session to be confirmed dead;
  herdr showing nobody anywhere on the lane's checkout is the whole of
  the liveness question, and the token resumes it with no window
  waited.
- S77, deserted lane on its own host. Given this machine holds plan 7
  in a lane with its token persisted, and a takeover at a new epoch
  lands on plan 7, and herdr shows no agent on the lane, when the lane
  runs `start --go` for plan 7, then start refuses and names yield,
  and it is refused and the takeover stands. Mirror
  `TestStartNamesYieldForADesertedLaneOnThisHost`: run from the lane's
  own checkout, its token no longer proves anything against the
  branch a foreign takeover moved past it, so self-resume cannot
  recover it — start refuses rather than silently seizing its own
  dead lane and orphaning whatever it never pushed.

**GREEN.** Extend `cmd/frit/bdd_identity_and_cross_layer_test.go` with
seven new steps. Two are Given: S46's held-lane-with-no-session-bound
variant, reused for S76 too and saving one step; S47's
failing-teardown herdr fake. Two are When: S46's claim from an
unrelated directory; S77's start run from the lane. Three are Then:
S46's already-held refusal and unchanged ref, two steps; S47's error
naming the debris; S77's refusal naming yield. Every step function
ships with a unit test of its own, per CLAUDE.md.

**Guard the edges.** Reuse these steps as-is, with no change to their
text — strict mode reports a redefinition as ambiguous:

- `herdrShowsNoAgentOnTheLane`
- `thisMachineRunsStartGoForPlan`
- `thePlanIsResumed`
- `noTakeoverMarkerSitsBetweenTheHeldTipAndOriginsTip`
- `planIsUnclaimed`
- `startFailsAndAReleaseMarkerSitsOnTheBranch`
- `thisMachineHoldsPlanInALaneWithItsTokenPersisted`
- `aTakeoverAtANewEpochLandsOnPlan`
- `itIsRefusedAndTheTakeoverStands`

S46's fixture must never `t.Chdir` into the lane it builds. Otherwise
the row collapses into a resume it is meant to prove does not happen.
S77's takeover step mints its foreign lease as `"elsewhere"`, the same
identity Phase 1's S86 already uses for the role. No new machine name
is introduced.

**Gate.** `go test ./cmd/frit -run
'TestFeatures/S(45|46|47|48|49|60|61|64|73|76|77|86):'` passes with
every one of the twelve reported PASS and none SKIP. `go test
./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-3.result.md`. Record any finding a row
exposes. Say what S63 and S72 — the two rows this phase still leaves
untouched — need from a second machine racing the verbs, the shape
`cloneRepoIntoRoot` in
[bdd_process_death_test.go](../../cmd/frit/bdd_process_death_test.go)
already gives sibling sections, and what S62, S65, S66 and S74 still
need beyond that.
