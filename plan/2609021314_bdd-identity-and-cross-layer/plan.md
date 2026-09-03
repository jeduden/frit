---
id: 2609021314
title: The identity and cross-layer scenarios run under godog
status: "🔳"
summary: >-
  The lease-protocol matrix's "Identity anomalies" section (S45..S49,
  S66) and its "Cross-layer: herdr and frit disagree" section (S60..S65,
  S72..S74, S76, S77, S86) are declared in features/identity.feature
  and features/cross-layer.feature, but every row is still @pending:
  declared, skipped, proving nothing. This plan writes each of the
  eighteen as a real Given/When/Then over the herdr fake, the lane
  token and the cmd/frit verbs, bound in one step file, so a regression
  in what the doc promises when herdr and frit disagree, or when a
  machine's name lies, fails the build. It is the first plan to wire
  the herdr fake into a step. It stands alone: no other conversion plan
  is a prerequisite, and none waits on it.
model: sonnet
depends-on: []
---
# The identity and cross-layer scenarios run under godog

## Goal

Every row of the matrix's identity section (S45..S49, S66) and its
cross-layer section (S60..S65, S72..S74, S76, S77, S86) is a passing
godog scenario. None is tagged `@pending`. A regression in any promise
the two sections make — a live session vetoing a takeover, a token
outranking a holder string, a hand-moved branch fencing its own lane —
fails `go test ./...`.

## Context

**The gap.** Plan 2609012000 stood the harness up: S16 runs for real,
and [identity.feature](../../features/identity.feature) and
[cross-layer.feature](../../features/cross-layer.feature) declare their
eighteen rows with `@pending`. `TestFeatures` skips each one. Its
handoff names the next gap in so many words: the herdr fake is real
but no step has ever called it, and a session-liveness row is the
first to wire it in. Today nothing executes a single row of either
section.

**What already exists, and is reused.** The herdr fake lives in
[who_test.go](../../cmd/frit/who_test.go): `withHerdr` swaps the
package's herdr runner for one test and restores it on cleanup, and
`herdrReturning` cans a pane list. `startHerdr` in
[start_test.go](../../cmd/frit/start_test.go) answers start's whole
handshake — worktree, pane, agent list — and records every call. An
unreachable herdr is a runner that returns an error, as
[claim_test.go](../../cmd/frit/claim_test.go) already does for S61.
Liveness is one call, `herdr.SessionLive` in
[session.go](../../internal/herdr/session.go), behind claim's veto
(`VetoError` in [lease.go](../../internal/claim/lease.go)). The token
is `claim.TokenPath` and `claim.ReadToken` in
[token.go](../../internal/claim/token.go); `claim.OwnAdvance` in
`lease.go` tells a lane's own advance from a foreign move. The verbs
read that proof through `ownToken` and `tokenProves` in
[claim.go](../../cmd/frit/claim.go); start's resume path is
`startResume` and `laneTokenResumeTip` in
[start.go](../../cmd/frit/start.go). The holder string a verb writes is
`hostname()` in [main.go](../../cmd/frit/main.go); no verb reads it
back as identity. The fixtures the rows need already exist:
`heldLaneOwnedBy`, `dropToken` and `remoteWorkTip` in `start_test.go`,
`seedWindow` in `claim_test.go`, `deadHold` in
[reap_test.go](../../cmd/frit/reap_test.go), and `claimableRepo` and
`cloneAgain` from the lease vocabulary. Unit tests already pin many
rows by number — `grep -rn "S86\b" cmd/frit` finds three — and each
scenario mirrors its unit test's fixture rather than inventing one.

**The rows, triaged.** Three shapes, and the phase order follows them.

- Drivable now, with the herdr fake, the token and the verbs, each
  backed by a unit test or by an existing fixture: S61 (an unreachable
  herdr neither vetoes nor frees; the window governs), S86 (the lane's
  own raw commits are its own advance; a new-epoch takeover fences),
  S64 (a branch moved by hand no longer descends from the token, so
  the lane's verbs refuse to act as holder), S48 (the token resumes a
  lease whose holder trailer names another machine).
- Verb-level, over start, claim, release and yield: S45 (a second
  agent on one host is refused by the live-session veto), S46 (a
  reused path carries no token, so it waits the window like any
  claimant), S47 (a failed handoff leaves a release marker and names
  the path), S49 (an equal holder string proves nothing; the token
  serializes), S60 (a claim with herdr down keeps its lease and stands
  up later), S63 (a released lease fences the still-live agent's next
  transition), S72 (one winner; the loser's refusal names the winning
  lane), S73 (a failed prompt releases and fences the agent), S76 (a
  dead session resumes on the marker's lane and its token; no token,
  no shortcut), S77 (a deserted lane on its own host refuses an
  unpushed suffix and names yield).
- Observation and boundary rows: S62 (a tip that advances resets the
  observation; no takeover), S65 (a herdr that lost its panes lapses
  the veto to the window), S74 (two repos, one id; lanes key on the
  repo and pane names carry it), S66 (a shared clone across hosts is
  unsupported; a lane is one host's path).

**Convention for a row the doc resolves by argument.** S48 says
"identity is machine-id; hostname is decoration". In the code the
identity is the token, and the holder trailer is reporting only. The
scenario asserts the observable: a hold whose trailer names another
machine is resumed, not seized, when the lane's token matches. S66
says "unsupported, documented". Its scenario asserts the boundary the
doc states — the marker's lane trailer is a bare path with no host,
and the token lives inside that path's git dir — never a comment.
Every such row becomes an assertion about what a verb, a marker or
origin shows.

**Where the steps go.** The two sections' steps live in one new
`cmd/frit/bdd_identity_and_cross_layer_test.go`, appended to the step
registry from `init` exactly as `bdd_lease_test.go` is. This plan never
edits `bdd_test.go`, and touches no feature file but its own two. The
sibling conversion plans — process death, host death and races,
partitions and clocks, storage, the two lifecycle halves — each own
their file the same way, so all land in any order. A step text this
file defines that another already has fails as ambiguous under godog's
strict mode; the fix is to reuse the existing step. The section's
world holds the scenario's own `*testing.T`, so a Given step installs
the herdr fake with `withHerdr(w.t, runner)` and the subtest's cleanup
restores the real runner before the next scenario runs.

**Out of scope.** No change to the lease protocol or to any verb. A
scenario that cannot be made to pass without changing behaviour is a
finding, parked in the handoff with the row it concerns, not a fix
made here.

## Tasks

1. Phase 1 (proving slice): the four drivable rows — S61, S86, S64,
   S48 — written and passing in
   `bdd_identity_and_cross_layer_test.go`, the file registered, the
   herdr fake reached from a step for the first time, and the identity
   convention set by S48. Driven red by dropping `@pending`: strict
   mode fails the undefined steps.
2. Later phases, shaped by Phase 1's handoff: the verb-level rows S45,
   S46, S47, S49, S60, S63, S72, S73, S76, S77 over start, claim,
   release, yield and the resume path; then the observation and
   boundary rows S62, S65, S74, S66.

## Execution

| Phase | Title                                                        | Tier   | Gate                                                                                                                                                                     |
| ----- | ------------------------------------------------------------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | The herdr fake reaches a step: S48, S61, S64, S86 run real   | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(48\|61\|64\|86)_'` passes with no SKIP; the bijection gate stays green; `go test ./...` and golangci-lint clean                 |
| 2     | Reachable without a new fixture: S45, S49, S60, S73 run real | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(45\|48\|49\|60\|61\|64\|73\|86):'` passes with no SKIP; the bijection gate stays green; `go test ./...` and golangci-lint clean |
| 3     | Resume-from-outside and yield: S46, S47, S76, S77 run real   | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(45\|46\|47\|48\|49\|60\|61\|64\|73\|76\|77\|86):'` passes, no SKIP; bijection green; `go test ./...` and lint clean             |
| 4     | S62, S63, S65, S66 run real                                  | sonnet | `S(62\|63\|65\|66):` PASS, no SKIP; lint clean                                                                                                                           |
| 5     | S72, S74 run real                                            | sonnet | `S(72\|74):` PASS, no SKIP; lint clean                                                                                                                                   |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
| --- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [The herdr fake reaches a step: S48, S61, S64, S86 run real](phase-1.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
|     | ↳      | S48, S61, S64 and S86 drop `@pending` and run as real scenarios in the new `cmd/frit/bdd_identity_and_cross_layer_test.go`: a stale hold bound to a session survives an unreachable herdr's failed worktree stand-up as a takeover at epoch 2 on the observed stale tip, never a live-session veto; a lane whose own raw commits outrun its persisted token still resumes on `claim`, CASing from the raw tip, while a genuine takeover after that resume still fences `release`; a branch deleted and reminted by hand fences both `release` (held live) and `claim` (already held, never resumed) from the original lane, since neither its ancestry nor its epoch/holder governs the fresh history; and a held lane whose marker names another machine as holder, but whose checkout carries the token that machine's own renewal persisted, resumes clean under `start --go` with no takeover marker between the held tip and origin's — the token proves the lease, the holder trailer only reports. The file registers itself the way `bdd_lease_test.go` does and keeps its own state in an `identityAndCrossLayerState` reached through `section[T]` rather than a field on `world`; it defines no step text any other section already owns. |
| 2   | ✅     | [Reachable without a new fixture: S45, S49, S60, S73 run real](phase-2.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
|     | ↳      | S45, S49, S60 and S73 drop `@pending` and run as real scenarios, each a fresh combination of Phase 1's own fixtures: a live bound session vetoes a second agent's `start --go`, renewing the holder's own lease rather than losing it to a seizure; a holder string equal to this very host proves nothing once its token is gone, so `start` refuses the hold as unprovable exactly as it would a stranger's, never resuming it; an unreachable herdr at claim time releases the fresh lease its own worktree stand-up could not complete, and the next claim, herdr healthy again, mints clean at the following epoch with no takeover window waited; and an agent that starts before its own prompt call fails is still torn down by the failed handoff's unwind, a release marker landing on the branch it never got to work. `identityAndCrossLayerState` gains a `rec *herdrCalls` field so S45 and S73 can prove what herdr was actually asked, not just read the verb's own text; the one existing step touched is `holdsPlanBoundToASession`, additively, to also record the root S45 needs to run `start --go` against.                                                                                                                    |
| 3   | ✅     | [The resume-from-outside path and yield reach a step: S46, S47, S76, S77 run real](phase-3.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
|     | ↳      | S46, S47, S76 and S77 drop `@pending` and run as real scenarios: a claim run from a directory that never carried plan 7's token — the shape a reused worktree path leaves — is refused as an ordinary claimant, and origin's ref is left untouched; a teardown that itself fails still releases the lease, its own error naming the worktree and pane it could not clean up; an unbound hold nobody attends resumes on its token alone with no window waited, the sharpest reading of "pane gone"; and a lane whose own token a foreign takeover has superseded refuses to reattach from inside itself, naming `yield` rather than orphaning what it never pushed. `identityAndCrossLayerState` gains no new field; S77 needed a session-bound sibling of two of Phase 1's own steps, since `deadSession` reads the session on the marker at the *current* tip and an unbound one gives herdr nothing to confirm gone.                                                                                                                                                                                                                                                                                                                               |
| 4   | ✅     | [A reset window, a fenced release and a doc boundary: S62, S63, S65, S66 run real](phase-4.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
|     | ↳      | S62, S63, S65 and S66 drop `@pending` and run as real scenarios: a stale hold's own holder pushing while herdr is unreachable resets the observation window, so a claimant over it waits again rather than taking over; a lane whose session herdr swears is live is still fenced out once a foreign takeover has moved the ref, proving liveness only ever vetoes an incoming takeover and never rescues a lane a CAS has already lost; a herdr that answers but names nobody lets a stale hold's takeover complete cleanly, unlike an unreachable herdr's own failed stand-up; and a lane's marker and token, read back after the fact, carry no host anywhere — the documented boundary an NFS-shared clone runs into. All sixteen rows this plan has landed so far stay green together.                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| 5   | 🔳     | [A genuine two-process race and two repos sharing one id: S72, S74 run real](phase-5.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
<?/catalog?>

## Acceptance Criteria

- [ ] No scenario in `features/identity.feature` or
      `features/cross-layer.feature` carries `@pending`; `go test
      ./cmd/frit -run TestFeatures/S` reports S45..S49, S66, S60..S65,
      S72..S74, S76, S77 and S86 as PASS, none as SKIP
- [ ] Every step is bound in
      `cmd/frit/bdd_identity_and_cross_layer_test.go` or reused from
      `bdd_lease_test.go`; `bdd_test.go` is untouched
- [ ] At least one scenario installs the herdr fake through `withHerdr`
      from a step, and the real runner is back before the next scenario
- [ ] Each scenario asserts an observable — a verb's result, a marker's
      trailer, a refusal naming a lane, origin's refs — never a comment
- [ ] A finding a row exposes is recorded in the handoff with its row
      id, not fixed silently
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
