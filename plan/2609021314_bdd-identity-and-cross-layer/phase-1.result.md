---
n: 1
title: "The herdr fake reaches a step: S48, S61, S64, S86 run real"
status: "✅"
result: true
summary: >-
  S48, S61, S64 and S86 drop `@pending` and run as real scenarios in
  the new `cmd/frit/bdd_identity_and_cross_layer_test.go`: a stale hold
  bound to a session survives an unreachable herdr's failed worktree
  stand-up as a takeover at epoch 2 on the observed stale tip, never a
  live-session veto; a lane whose own raw commits outrun its persisted
  token still resumes on `claim`, CASing from the raw tip, while a
  genuine takeover after that resume still fences `release`; a branch
  deleted and reminted by hand fences both `release` (held live) and
  `claim` (already held, never resumed) from the original lane, since
  neither its ancestry nor its epoch/holder governs the fresh history;
  and a held lane whose marker names another machine as holder, but
  whose checkout carries the token that machine's own renewal
  persisted, resumes clean under `start --go` with no takeover marker
  between the held tip and origin's — the token proves the lease, the
  holder trailer only reports. The file registers itself the way
  `bdd_lease_test.go` does and keeps its own state in an
  `identityAndCrossLayerState` reached through `section[T]` rather
  than a field on `world`; it defines no step text any other section
  already owns.
---
## Handoff

**Fixture S64 settled on.** No unit test pins this row, as the phase
spec anticipated. "Repurposed by hand" is: delete origin's work ref
outright (`git push origin --delete plan/7`), then mint a wholly fresh
lease as `"elsewhere"` from a second clone (`cloneAgain` +
`claim.Acquire`) — `claim.Acquire` mints at epoch 1 with no parent the
instant the ref reads empty, so the new history shares no ancestor with
the original lane's token at all. That is the sharpest reading of
"does not descend from the token": `claim.OwnAdvance`'s `isAncestor`
check fails outright rather than merely landing on the wrong
epoch/holder, and `release`/`claim` both fall through to their
ordinary foreign-hold paths (`"held live by another lane"`,
`"already held ... not takeable until the window matures"`).

**The one real bug this row caught, in the scenario itself, not the
product.** The first draft asserted origin's post-release tip by
reading `st.repo`'s own local `refs/heads/plan/7` via `git rev-parse`
— the pattern every sibling row and the unit tests it mirrors use
safely, because in every one of those, the same physical clone (or a
worktree sharing its refs) is what advances the branch. S64 is the one
row where a *second*, independent clone moves the ref on origin; the
first clone's local branch ref never learns about it, so `rev-parse`
kept reading the stale pre-repurpose tip and the "untouched" assertion
compared the wrong two values. `claim.RemoteTip(repo, "origin",
planID, gitwt.Exec)` — the same `ls-remote`-backed read `ownToken` and
`tokenProves` themselves use to decide these very rows — reads origin
fresh regardless of which clone last touched it, and the row now
compares against reality rather than a cache.

**Rows needing a step the lease world lacked.** All four did — none of
`bdd_lease_test.go`'s eight steps survives contact with a real herdr
fake, a real token, or a repurposed branch, so all sixteen of this
file's own steps are new. `withHerdr` reaches a step for the first
time here, through two Given shapes: "herdr is unreachable" (S61, a
dial-error runner) and "herdr shows no agent on the lane" (S48,
`startHerdr()`'s own handshake fake, whose canned agent list never
names the session a `heldLaneOwnedBy` marker binds). S86's own
`section` state (`identityAndCrossLayerState`) tracks the raw and
takeover tips a raw-commit row and a takeover-after-resume row both
need to check against, alongside the last verb's captured output —
none of it borrowed from `deathState`, kept private to this file per
CLAUDE.md's one-file-per-section rule.

**The gate command in this plan's own phase-1.md does not match**,
for the same reason plan 2609021310's own phase 1 handoff already
found: `TestFeatures` names each subtest `"<id>: <title>"`
(`bdd_test.go`), so the real subtest is `S48:_hostname_changes`, not
`S48_hostname...`, and `-run`'s unanchored regex never finds a bare
`_` straight after the digits. `go test ./cmd/frit -run
'TestFeatures/S(48|61|64|86):'` is the working form — colon, not
underscore — and is what the Acceptance Criteria below were verified
against. Left as written in `plan.md`'s Execution table and this
phase's own gate line, per the sibling plan's own precedent of
recording the discrepancy in the handoff rather than editing history.

**No other finding.** Every assertion in all four rows passed on the
first shape tried once the RemoteTip fix above landed; nothing was
weakened to reach green.

**What the verb-level rows will need.** S45, S46, S47, S49, S60, S63,
S72, S73, S76 and S77 each turn on `startHerdr`, the resume path or
yield, none of which this phase exercised:

- `startHerdr()` and `startHerdrUnreachableList()` (`start_test.go`)
  are the two shapes already proven here and ready to reuse as-is:
  a full handshake (worktree create/open, pane current, agent list)
  and the same with `agent list` itself failing. S45 (a second agent
  refused by the live-session veto) and S73 (a failed prompt releases
  and fences) both need `startHerdr`'s recorded calls
  (`herdrCalls.calls`) to assert *what* was dispatched, not just the
  verb's own text output — this file never reads that struct, since
  none of its four rows needed to.
- The resume path itself — `startResume`, `laneTokenResumeTip`,
  `resumeToken`'s added veto — is untouched by this phase; S46 (a
  reused path carrying no token) and S76 (a dead session resumes on
  the marker's lane and its token) both turn on it directly, and
  `dropToken` (`start_test.go`) is the fixture S46 needs that this
  phase never called on.
- `yield` is not exercised anywhere in this file. S77 (a deserted lane
  refusing an unpushed suffix, naming yield) is the first row in
  either section to need it; `claim.Yield` and the rescue-ref read
  `yieldParks` (`bdd_lease_test.go`) already demonstrates are the
  pattern to mirror, but through the `yield` verb rather than the raw
  API, since S77's promise is what the CLI reports.
- S49 (an equal holder string proves nothing; the token serializes)
  and S60 (herdr down at claim time, not observation) are both
  reachable the same way S61 and S48 were here — no new fixture, just
  a new combination of the ones already proven — but were left for a
  later phase since the plan scoped this one to exactly S48/S61/S64/S86.
- S63 and S72 both turn on two machines racing through the verbs
  rather than the lease API directly (`runsClaimForPlan`'s
  `cloneRepoIntoRoot` pattern from `bdd_process_death_test.go` is the
  nearest existing shape, though this file borrows nothing from it),
  and neither needed anything this phase built.

All tests are green: `go test ./cmd/frit -run
'TestFeatures/S(48|61|64|86):'` reports four PASS, none SKIP; `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
clean; `go test ./internal/scenario` (the bijection gate) stays green.
