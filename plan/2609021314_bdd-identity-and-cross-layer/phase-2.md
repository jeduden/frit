---
n: 2
title: "Reachable without a new fixture: S45, S49, S60, S73 run real"
status: "🔳"
result: false
---
Convert four more rows from `@pending` declarations into passing
scenarios: S45, S49, S60 and S73. Phase 1's handoff named these as
reachable with no new fixture — each is a fresh combination of the
herdr fake, the token and the verbs Phase 1 already proved from a
step, mirroring a unit test that already pins the row.

**Assumes.** Everything phase-1.md assumed, plus
`cmd/frit/bdd_identity_and_cross_layer_test.go`'s own sixteen steps
and `identityAndCrossLayerState`. `herdrCalls` and its `verb`/`hasArg`
readers, in
[dispatch_test.go](../../cmd/frit/dispatch_test.go), record every
herdr call a run makes; S45 and S73 both read them, the first rows in
this file to do so. `heldLaneOwnedBy` and `dropToken`
([start_test.go](../../cmd/frit/start_test.go)) build a held lane and
strip its token.

**Value.** Two more identity rows and two more cross-layer rows gain
their executable promise. A live bound session vetoes a second
agent's start rather than losing its own lease to it. A holder string
equal to this very host proves nothing without the token behind it,
so a hostname collision serializes on the token exactly as a
hostname change does. An unreachable herdr at claim time neither
strands a naked hold nor blocks the next attempt, since the failed
stand-up releases what it minted. And an agent already started before
its prompt fails still gets torn down, fenced by the release marker
its own lane pushes.

**RED.** Drop `@pending` from S45 and S49 in
[identity.feature](../../features/identity.feature), and from S60 and
S73 in
[cross-layer.feature](../../features/cross-layer.feature). Write each
one's Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S(45|49|60|73):'`: strict mode reports the new steps
undefined and the four subtests fail. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S45, two agents, one plan, one host. Given "elsewhere" holds plan 7
  bound to a session, and the window has matured, and herdr
  positively confirms that session live, when this machine runs
  `start --go`, then start refuses naming the live agent session, and
  the holder's own lease is renewed — a beat CASed from the held
  tip, naming "elsewhere" — never seized. Mirror
  `TestStartRefusesATakeoverVetoedByALiveSession`: the fixture is
  exactly Phase 1's own `holdsPlanBoundToASession` and
  `theWindowHasMaturedForPlan`, reused as-is; only the herdr fake and
  the two `Then` reads are new.
- S49, hostname collides. Given a held lane whose marker names this
  very host as holder, but whose checkout carries no token — the
  fixture is `heldLaneOwnedBy` plus `dropToken`, the shape a cloned
  machine-id or a reused path leaves — and herdr shows no agent on
  it, when this machine runs `start --go`, then it refuses: already
  held, not takeable until the window matures, and the plan is never
  resumed. Mirror `TestStartDoesNotResumeALaneWhoseTokenIsGone`: an
  equal holder string is exactly as unproven as a foreign one once
  the token is gone, so the row reads as S48's photographic negative.
- S60, herdr down at claim time. Given plan 7 is unclaimed and herdr
  is unreachable, when this machine claims plan 7, then the claim is
  refused: worktree not stood up (Phase 1's own S61 assertion, reused
  as-is), and the lease is released, not left standing — the ref
  still exists, its tip a release marker, never a delete. When herdr
  becomes reachable and this machine claims plan 7 again, then it
  claims clean at the next epoch, no takeover window waited. Mirror
  the pair `TestClaimReleasesTheLeaseWhenTheWorktreeStandUpFails` and
  `TestClaimAfterAFailedStandUpAcquiresAtOnce` in
  [claim_test.go](../../cmd/frit/claim_test.go), the same single-repo
  shape S61 already uses (no second clone; this run's own repo is
  what advances the branch, so a local `rev-parse` reads it safely).
- S73, prompt fails after agent start. Given plan 7 is unclaimed and
  the agent starts but its own prompt call fails, when this machine
  runs `start --go`, then start fails and a release marker sits on
  the branch, the agent was started before the failure, and the
  worktree stood up for it is torn down. No unit test pins this exact
  row — every existing failed-handoff test fails `agent start` itself,
  never the prompt after it — so its fixture is this phase's own to
  set: `TestStartUnwindTearsDownTheLaneOnAFailedHandoff`'s own herdr
  fake, with `agent`+`prompt` erroring instead of `agent`+`start`, is
  the nearest existing shape and the one this file's own new Given
  step layers onto `startHerdr`'s working handshake. `herdrCalls`
  proves the agent really started (its own verb ran) before the
  prompt failed, not just that the run exited non-zero.

**GREEN.** Extend `cmd/frit/bdd_identity_and_cross_layer_test.go`
with eleven new steps. `identityAndCrossLayerState` gains a `rec
*herdrCalls` field for S73's dispatch check.
`holdsPlanBoundToASession` (Phase 1's own Given) additionally records
`st.root`, so this phase's `thisMachineRunsStartGoForPlan` reuse has
a root to run against. That is the one change to an existing step,
additive only, and S61's own scenario is untouched by it. Every step
function ships with a unit test of its own, per CLAUDE.md.

**Guard the edges.** Reuse `herdrIsUnreachable`,
`theClaimIsRefusedWorktreeNotStoodUp`, `herdrShowsNoAgentOnTheLane`
and `thisMachineRunsStartGoForPlan` from Phase 1. Do not redefine
their text. Strict mode reports a redefinition as ambiguous. S49's
fixture must actually drop the token, not merely name a different
holder, or it collapses into a second S48. S73's assertion on
`herdrCalls` must find `agent start` recorded, not just
infer it from the exit code, so a future regression that skips
straight to the prompt failure is still caught.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(45|48|49|60|61|64|
73|86):'` passes with every one of the eight reported PASS and none
SKIP. `go test ./internal/scenario` stays green. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-2.result.md`. Record any finding a row
exposes. Say what the remaining rows — S46, S47, S63, S72, S76, S77,
then S62, S65, S66, S74 — still need from the resume path and yield
that this phase did not touch.
