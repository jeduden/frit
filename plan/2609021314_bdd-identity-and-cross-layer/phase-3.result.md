---
n: 3
title: "Resume-from-outside and yield reach a step: S46, S47, S76, S77 run real"
status: "✅"
result: true
summary: >-
  S46, S47, S76 and S77 drop `@pending` and run as real scenarios: a
  claim run from a directory that never carried plan 7's token — the
  shape a reused worktree path leaves — is refused as an ordinary
  claimant, and origin's ref is left untouched; a teardown that itself
  fails still releases the lease, its own error naming the worktree
  and pane it could not clean up; an unbound hold nobody attends
  resumes on its token alone with no window waited, the sharpest
  reading of "pane gone"; and a lane whose own token a foreign
  takeover has superseded refuses to reattach from inside itself,
  naming `yield` rather than orphaning what it never pushed.
  `identityAndCrossLayerState` gains no new field; S77 needed a
  session-bound sibling of two of Phase 1's own steps, since
  `deadSession` reads the session on the marker at the *current* tip
  and an unbound one gives herdr nothing to confirm gone.
---
## Handoff

**S46, S47 and S76 landed exactly as phase-3.md predicted.** Each
mirrors its named unit test with no new mechanism: `heldLaneOwnedBy`
without a `t.Chdir` (S46), a herdr fake whose own `worktree`+`remove`
errors on top of Phase 2's own failed-`agent`+`start` shape (S47), and
`heldLaneOwnedBy` with an empty session in place of Phase 1's own
bound one (S76, one new Given step, four reused).

**The one deviation: S77 could not reuse Phase 1's `this machine holds
plan 7 in a lane with its token persisted` or `a takeover at a new
epoch lands on plan 7`.** The phase spec named both as reusable
as-is. Running the row with them first refused on the *ordinary*
already-held door ("not takeable until the window matures"), never on
`desertedRefusal`'s own wording. The cause: `deadSession`
(`cmd/frit/main.go`) reads the marker at origin's *current* tip — after
the takeover, that is the takeover's own marker, not the original
hold's — and calls `herdr.SessionDeadIn(panes, m.Session)`, which
answers `false` outright for an empty session (`internal/herdr/session.go`:
`if session == "" { return false }`). Neither shared step ever binds
a session, so `plan.Dead` never turns true and `desertedRefusal` never
fires. The unit test this row mirrors,
`TestStartNamesYieldForADesertedLaneOnThisHost`, binds `"wOld:p1"` on
the original hold and `"wGhost:p1"` on the takeover for exactly this
reason — a detail phase-3.md's own summary of the mirror did not carry
over. Two new Given steps supply it:
`thisMachineHoldsPlanInALaneBoundToASessionWithItsTokenPersisted` and
`aTakeoverBoundToASessionAtANewEpochLandsOnPlan`, each a session-bound
copy of Phase 1's own step, `claim.Branch`/`leaseFor` and all. Phase
1's two originals are untouched and still serve S64 and S86 unchanged.

**A second, mechanical finding: the registrar and two of the section's
own unit tests outgrew golangci-lint's `funlen` once Phase 3's steps
landed.** `registerIdentityAndCrossLayer` split into itself (Phase 1's
rows) and a new `registerVerbLevelIdentityAndCrossLayer` (Phase 2 and
Phase 3's) — the same split-by-group pattern
[a sibling section](../../cmd/frit/bdd_partitions_and_clocks_test.go)
already uses for one file registering several groups.
`TestIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition` and
`TestIdentityAndCrossLayerReadBacksWantTheirExactShape` each split off
a `TestPhase3...` sibling carrying only this phase's own four and
three cases. No behavior changed; `go tool -modfile=tools/go.mod
golangci-lint run` went from two `funlen` findings to zero.

**No finding against the product.** Every assertion in all four rows
passed once the S77 fixture supplied a session; nothing was weakened
to reach green.

**What the remaining rows will still need.** S63, S72, S62, S65, S66
and S74 are untouched by this phase:

- S63 (a released lease fences the still-live agent's next transition)
  and S72 (claim and start race on one host — one winner, the loser's
  refusal names the winning lane) both turn on two machines racing the
  verbs rather than the lease API directly.
  `cloneRepoIntoRoot` and `runClaimFrom`
  ([bdd_process_death_test.go](../../cmd/frit/bdd_process_death_test.go))
  already give a sibling section this exact shape — a second clone
  under its own `--root` — and nothing in this file has borrowed it
  yet.
- S62 (a tip that advances resets the observation window; no
  takeover), S65 (a herdr that lost its panes lapses the veto to the
  window) and S66 (a shared clone across hosts is unsupported, a lane
  is one host's path) are the observation-and-boundary rows Phase 1
  scoped out entirely. `resetWindow`
  (`bdd_process_death_test.go`'s own S7) is the nearest existing
  pattern for S62; S65 and S66 need no fixture named in either phase's
  handoff so far and are each this plan's own to design.
- S74 (same plan id in two repos — lanes key on host:repo:id, pane
  names carry the repo) needs two `claimableRepo`s under different
  repository names sharing one plan id, a combination no row in this
  file has built yet.

All tests are green: `go test ./cmd/frit -run
'TestFeatures/S(45|46|47|48|49|60|61|64|73|76|77|86):'` reports twelve
PASS, none SKIP; `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean; `go test ./internal/scenario` (the
bijection gate) stays green.
