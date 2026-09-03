---
n: 1
title: "The herdr fake reaches a step: S48, S61, S64, S86 run real"
status: "✅"
result: false
---
Convert four rows from `@pending` declarations into passing scenarios:
S61, S86, S64 and S48. The herdr fake, the lane token and the verbs
can drive each of them today. This fixes three things the later phases
copy. First, the step file and its registration. Second, the world
that installs the herdr fake and runs a verb. Third, the convention
for an identity row the doc resolves by argument.

**Assumes.** `TestFeatures` in
[cmd/frit/bdd_test.go](../../cmd/frit/bdd_test.go) runs each tagged
scenario as its own subtest under godog's strict mode and skips a
`@pending` one. Steps bind through the `registrars` slice; a file
appends its registrar from `init`, as
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) does. The world
holds the scenario's own `*testing.T`. `withHerdr(t, runner)` in
[who_test.go](../../cmd/frit/who_test.go) swaps the package's herdr
runner and restores it in `t.Cleanup`; `herdrReturning(panes...)` cans
an agent list; `startHerdr()` in
[start_test.go](../../cmd/frit/start_test.go) answers start's handshake
and records calls. `heldLaneOwnedBy(t, root, holder, session)` builds
a held lane whose checkout carries the token its renewal persisted;
`remoteWorkTip` reads origin's work ref; `seedWindow` in
[claim_test.go](../../cmd/frit/claim_test.go) ages an observation. A
verb runs in-process through `run(args, out, err)`, and `t.Chdir(lane)`
puts a step inside the lane. `claim.ReadToken` and `claim.OwnAdvance`
are the token's own API. The refusal and success strings a scenario
matches are the ones the unit tests already match.

**Value.** The two sections stop being eighteen declarations and gain
their first four executable promises: an unreachable herdr neither
vetoes nor frees, a lane's raw commits are its own advance while a
takeover fences, a hand-moved branch fences its own lane, and a token
outranks the holder string. Any of those regressing fails the build.
The herdr fake is exercised from a step for the first time, and the
file the remaining fourteen rows join already exists.

**RED.** Drop `@pending` from S61 and S86 in
[cross-layer.feature](../../features/cross-layer.feature), from S64
there too, and from S48 in
[identity.feature](../../features/identity.feature). Write each one's
Given/When/Then. Run `go test ./cmd/frit -run TestFeatures/S61_`:
strict mode reports the new steps undefined and the subtest fails.
That is the red — commit it.

The scenarios, in the matrix's own terms:

- S61, herdr down at observation. Given "elsewhere" holds plan 7
  bound to a session, and the window has matured, and herdr is
  unreachable, when "box-b" claims, then a takeover at epoch 2 sits on
  the stale tip. The unreachable herdr cannot stand a worktree up, so
  a release marker sits above it; the assertion is that the veto never
  fired. Mirror `TestClaimTakesOverWhenHerdrCannotAnswer`: the fake is
  a runner returning a dial error, installed by a Given step.
- S86, a live lane's own raw commits advance the branch. Given this
  machine holds plan 7 in a lane with its token, and two raw commits
  are pushed on top, when the lane runs `claim` from inside, then it
  is resumed and the beat's parent is the raw tip. And when a takeover
  at a new epoch lands and the lane runs `release`, then it is refused
  and the takeover stands. Mirror the pair
  `TestReleaseRecognizesALaneWhoseOwnCommitsAdvancedTheTip` and
  `TestReleaseStillRefusesAGenuineTakeoverAfterItsOwnRenewal` in
  [release_test.go](../../cmd/frit/release_test.go). One scenario
  carries both halves, as the row does.
- S64, branch repurposed by hand. No unit test pins this row; its
  fixture is the phase's to set, and the handoff records it. Given
  this machine holds plan 7 in a lane with its token, when origin's
  work ref is moved by hand to a commit that does not descend from the
  token — delete the ref and mint a fresh lease as "elsewhere" from a
  second clone — then `release` from the lane is refused and origin's
  tip is untouched, and `claim` from the lane reports the plan already
  held and never "resumed". Mirror `TestReleaseRefusesALiveForeignHold`
  run from inside the lane. `claim.OwnAdvance` answering false for the
  token and the new tip is the seam; the scenario asserts the verbs.
- S48, hostname changes. Given a held lane whose marker names
  "elsewhere" as holder and whose checkout carries the token, and
  herdr shows no agent on it, when this machine runs `start --go`,
  then the plan is resumed and no takeover marker sits between the
  held tip and origin's tip. Mirror
  `TestStartResumesWhateverTheHolderStringSays` over `heldLaneOwnedBy`
  and `startHerdr`. This is the identity convention: the token proves
  the lease, the trailer only reports.

**GREEN.** Add `cmd/frit/bdd_identity_and_cross_layer_test.go`: a
world for the two sections holding the root, the repo, the lane, the
held tip and the last verb's output, the step functions, and an `init`
appending the registrar. Reuse every step `bdd_lease_test.go` already
defines; define only what the four rows add. A quoted machine name in
a step is checked against its role, as the lease world does, so a
scenario cannot pass by naming the wrong box. The herdr fake enters
through one Given per shape — "herdr is unreachable", "herdr shows no
agent on the lane" — each calling `withHerdr(w.t, ...)`, so the
scenario's cleanup restores the real runner. Every step function ships
with a unit test of its own, per CLAUDE.md.

**Guard the edges.** A step text `bdd_lease_test.go` already defines —
"holds the lease for plan", "takes the lease over", "the error
suggests yield" and the rest — must not be redefined: strict mode
reports it ambiguous. The world must refuse a machine the scenario
never introduced. A scenario must not pass with the real herdr runner
still installed; the unreachable shape is a fake, never a missing
socket on the build box. A scenario that only passes by weakening an
assertion is a finding for the handoff, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(48|61|64|86)_'`
passes with every one of the four reported PASS and none SKIP. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Name the fixture S64 settled
on. Name the rows that needed a step the lease world lacked. Record
any finding a row exposed. Say what the verb-level rows — S45, S46,
S47, S49, S60, S63, S72, S73, S76, S77 — will need from `startHerdr`,
the resume path and yield.
