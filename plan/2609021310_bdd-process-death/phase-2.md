---
n: 2
title: The verb-level rows S3, S4, S5 and S6 run for real
status: "✅"
result: false
---
Convert the four process-death rows that live above the lease API —
S3, S4, S5, S6 — from `@pending` declarations into passing scenarios.
Phase 1 drove `internal/claim` directly. These four drive `cmd/frit`'s
own verbs (`claim`, `board`) through `run` instead, because the
promise each row makes is a verb's, not the lease atom's: self-resume,
the observation-gated takeover, the board's rendering, and the veto
that a bound session either raises or cannot.

**Assumes.** `cmd/frit/claim.go` chains `resumeOwnLease` →
`resumeToken` → `ownToken` → `tokenProves` for `claim`'s self-resume.
`ownToken` needs two things. One: the calling directory is this exact
plan's own worktree (`inOwnLane`). Two: a persisted token
(`claim.ReadToken`/`TokenPath`, `internal/claim/token.go`) that either
equals origin's current tip, or is this lane's own advance beyond it
(`claim.OwnAdvance`).

`resumeToken` adds the live-session veto (`herdr.SessionLive`) on top.
`herdr.SessionLive` (`internal/herdr/session.go`) returns `false`
without ever calling herdr when the session is `""` or `"-"`. A lease
minted with no bound session can never veto a takeover, herdr fake or
not. It only walks the pane list when a session is actually recorded —
the `herdrReturning` fake's job (`who_test.go`): an empty agent list
answers that herdr was reachable and nobody is on it.

`emit` (`json_test.go`) runs a verb with `--json`. It decodes the
result into a document, which is how these four rows read a verb's
outcome — never stdout prose.

Two fields carry that outcome: `report.ClaimDoc`'s
`Claimed`/`Resumed`/`Refused` (`internal/report/dispatch.go`), and
`report.BoardDoc`'s per-plan `Held`/`Agent`
(`internal/report/board.go`).

Two tests in `cmd/frit/claim_test.go` already prove these two
mechanisms. These scenarios take their shape, through godog:

- `TestClaimResumesItsOwnLeaseFromThePersistedToken`: resume.
- `TestClaimTakesOverAStaleLease`: takeover.

**A finding, from reading the resume chain rather than assuming the
matrix's prose.** S4's outcome cell reads "RESUME on the same host;
elsewhere OBS→TAKE". But no code path in `claim.go` or `start.go`
special-cases "same host" at all. `ownToken` requires a real,
already-token-bearing worktree (`inOwnLane` plus `ReadToken`). The
reattach path, `laneTokenResumeTip` → `heldLaneMarker`, requires the
same token proof, read from the lane the hold marker names
(`m.HasLane()` plus `tokenProves`). A token is only ever persisted by
a lease transition run from inside a real worktree (`persistToken`,
gated on `TokenPath`'s `gitwt.GitDir` succeeding). A claim killed
before `standUpClaimWorktree` ever ran has minted no worktree, so it
persisted no token anywhere, on any host. Retried immediately, such a
claim is not resumed — it is refused exactly like any other live
hold, `HeldError` and all — and only becomes takeable once the
ordinary window matures. This scenario asserts that true, current
behavior instead of the matrix's more optimistic prose. The gap
between them is a possible future feature — an identity-free,
same-host retry shortcut — out of this plan's scope, which changes no
verb.

**Value.** `claim`'s two real doors — resume by token, takeover by a
matured window with no session to protect it — and `board`'s honest
"held, no session" both become regression-checked, and the matrix's
S4 cell is corrected against the tree rather than left to mislead the
next reader.

**RED.** Drop `@pending` from S3, S4, S5 and S6 in
[process-death.feature](../../features/process-death.feature) and
write each one's Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S3:'`: strict mode reports the new steps undefined and
the subtest fails. That is the red — commit it.

The scenarios, in the matrix's own terms and this phase's finding:

- S3, killed mid-push, server committed. Given a lane that already
  persisted a token from a prior renewal, when that same lane retries
  the claim, then it resumes instead of refusing or re-acquiring — no
  window consulted (`TestClaimResumesItsOwnLeaseFromThePersistedToken`'s
  own shape).
- S4, killed before worktree creation. Given a claim minted with no
  worktree ever stood up, when that same holder retries immediately,
  then it is refused, not resumed — the finding above, asserted as the
  real outcome. When the window then matures, another machine's claim
  takes the lease over, same as any other stale hold.
- S5, killed between worktree and agent start. Given a held plan with
  no session bound, when `board` runs, then it shows the plan held
  with no agent — `TestBoardRowShowsIdleForAHeldPlanWithNoAgent`'s
  assertion, reached through `report.BoardDoc` instead of the text
  column.
- S6, killed between agent start and prompt. Given a held plan whose
  marker names a session and herdr, positively asked, shows nobody on
  it, and the window has matured, when another machine claims, then
  the takeover proceeds — the veto's own query path runs and finds
  nothing to protect, distinct from S4's session-never-bound
  short-circuit.

**GREEN.** Add the four scenarios' steps to
`cmd/frit/bdd_process_death_test.go`, appended beside phase 1's; no
new registrar file, since this is the same section. New building
blocks: a Given that stands a real worktree up and persists a token
through it (S3), a step that runs `claim`/`board` through `run` and
decodes the JSON document into `deathState` (S3, S4, S5, S6), and a
Given that binds a session on the marker before seeding a matured
window (S6). `"([^"]+)" runs claim for plan (\d+)` and "the claim
takes the lease over" / "is refused, not resumed" are reused across
S4 and S6, since both close on the same `mintOrTakeOver` branch by
different roads.

**Guard the edges.** A step text this file or `bdd_lease_test.go`
already defines must not be redefined: strict mode reports it
ambiguous. The world must refuse a machine the scenario never
introduced. S4's refusal step must check `Refused != ""` and
`!Resumed`, not merely `!Claimed` — a takeover that failed for the
wrong reason must not read as this row's promise.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(1|2|3|4|5|6|7|10|11):'`
passes with all nine reported PASS and none SKIP. `go test
./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-2.result.md`. Name any row that needed
something the four together didn't cover. Say what the scavenge and
unwind rows, S8, S9, S12, S13, will need from `Scavenge` and the
landed-evidence fixtures.
