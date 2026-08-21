---
id: 2608211210
title: pick --go selects the top plan and starts it, so a skill needs one verb
status: "🔳"
summary: >-
  plan-pick spends most of its tokens making the agent re-run frit's own
  ranking by hand — pick, board, show, next, then start. frit already
  owns each step. Give pick a --go that selects the top candidate,
  resume fallback included, and runs start's claim-and-stand-up path,
  advancing past a lost race on its own, so the skill shrinks to one
  verb.
model: sonnet
depends-on: [2608192045]
---
# pick --go selects the top plan and starts it, so a skill needs one verb

## Goal

Give `frit pick` a `--go` that selects the top-ranked startable plan and
runs `start`'s claim-and-stand-up path on it. Selection falls back to an
in-progress plan nobody holds when nothing fresh is startable, and
advances to the next candidate when a claim loses its race. The
`plan-pick` skill then drops its by-hand `pick → board → show → next →
start` procedure for a single verb, shedding the tokens it costs.

## Context

The `plan-pick` skill is a control loop the agent runs in prose: gather
evidence from `pick`, `board`, `show`, `next`, reason over it, then feed
an id to `start`. Every step already lives in frit, so the agent is
re-deriving frit's own reasoning at ~850 tokens a pick.

Selection is pure and already built.
[discovery.Pick](../internal/discovery/pick.go) ranks the fresh
startable plans by how much each unblocks;
[discovery.Ready](../internal/discovery/ready.go) is the startable set it
draws from. The resume case — an in-progress plan nobody holds, whose
lane merged away — is already blessed for claiming by `claimRefusal` in
[cmd/frit/claim.go](../cmd/frit/claim.go), which lets an unheld
`InProgress()` plan through. What no pure function yet returns is the two
in one ordered list.

`start` already collapses the rest. [startCmd.Run](../cmd/frit/start.go)
resolves a selector, refuses an unstartable or held plan, composes the
escalation, and with `--go` mints the claim and stands the lane up
through herdr — dry-run by default, `ErrLostRace` carried as a refusal.
It is missing only the selection: it takes an explicit id, where
`pick --go` must choose one.

### Reuse

`pick --go` is `start` with the id chosen for it. It reuses
`startContextOf`, `composeStart`, `startExecute` and `renderStart` from
[start.go](../cmd/frit/start.go) verbatim — both are package `main`, so
the actor is a selection loop wrapped around the existing execute path,
not a second copy of it. Selection reuses `discovery.Pick`; the only new
pure code is the resume tail appended to it. On the report side,
`pick --go` emits the existing [report.StartDoc](../internal/report),
because its answer is "what I started", not the ranked list a bare `pick`
emits.

## Non-goals

- No new mutation. The only ref written is the claim `start` already
  mints; `pick --go` chooses which plan, it does not invent a push.
- No landed-work grep. The skill's `grep -rn "<symbol>"` guard is
  dropped, trusting status and deps; folding a landed-work heuristic into
  frit is deferred until an agent is shown re-picking landed work.
- Bare `pick` is unchanged. Without `--go` it stays the ranked lister it
  is today, emitting a `PickDoc`.

## Phase 1: pick --go starts the top candidate, end to end

The proving slice. Add `Go bool` to `pickCmd`. Bare `pick` is the ranked
list it is today — the safe look. With `--go`, select the top of
`discovery.Pick(res.Plans, 0)` and run it through the same `composeStart`
→ `startExecute` → `renderStart` path `start --go` uses, standing the
lane up through herdr and emitting a `StartDoc`. The `--go` is the same
explicit opt-in to the mutation `start` requires. When `pick` finds
nothing startable, `pick --go` prints the same empty answer `pick`
gives and mutates nothing. This establishes the whole seam — select,
reuse start's execute, emit a `StartDoc` — that phases 2+ extend. It ends
in sign-off on that seam and the output shape.

Phase 1 is a fresh pick only: the resume fallback and lost-race
advance are Phase 2. The phases below were appended at Phase 1 sign-off.

## Phase 2: the resume fallback and the lost-race advance

Two refinements to selection, each red-first. First, a pure
`discovery.Candidates(plans)`. It returns `Pick`'s ranked fresh plans
followed by the resume tail — in-progress, unheld — ranked the same way.
So `pick --go` resumes a merged-away lane when nothing fresh is
startable, as the skill's step 1 describes. Second, the actor iterates
those candidates. When `startExecute` carries an `ErrLostRace` refusal,
it advances to the next candidate rather than handing the race back. That
is the retry the skill currently spells out by hand.

## Phase 3: the skill leans on the one verb

Rewrite the shipped `plan-pick` skill around `frit pick` to look and
`frit pick --go` to claim-and-start, dropping the by-hand `board`,
`show`, `next`, `claim` procedure and the landed-work grep. Regenerate
frit's own `.claude/skills` from the bundle with `frit skills`, and keep
the skill under the kind's line cap. Re-record any JSON golden the new
`pick --go` output touches and read the diff.

## Tasks

1. Phase 1 — `frit pick --go` selects the top fresh candidate and runs
   start's claim-and-stand-up path, emitting a `StartDoc`; nothing
   startable prints the empty answer. Done.
2. Phase 2 — `discovery.Candidates` appends the resume tail so `pick
   --go` resumes an unheld in-progress plan; the actor advances past an
   `ErrLostRace` to the next candidate. Done.
3. Phase 3 — the `plan-pick` skill drops its by-hand procedure for the
   one verb; re-dogfood `frit skills`; goldens re-recorded.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase             | Design | Implement | Gate that catches a wrong answer                                               |
| ----------------- | ------ | --------- | ------------------------------------------------------------------------------ |
| 1 pick --go slice | opus   | sonnet    | test: `pick --go` starts the top pick under the start fakes, dry-run + `--go`  |
| 2 resume & retry  | sonnet | sonnet    | test: `Candidates` orders fresh then resume; a lost-race claim advances one on |
| 3 skill leans     | sonnet | sonnet    | shipped skill drops the procedure, stays under the cap, goldens re-recorded    |

## Acceptance Criteria

- [x] `frit pick --go` mints the claim and stands the lane up through
      herdr for the top-ranked startable plan, the same path
      `frit start <id> --go` runs
- [x] `frit pick --go` when nothing is startable prints the same empty
      answer bare `pick` gives, and mutates nothing
- [x] With no fresh plan startable but an in-progress plan nobody holds,
      `frit pick --go` resumes that plan
- [x] A claim that loses its race advances `pick --go` to the next
      candidate instead of surfacing the race to the caller
- [x] Bare `frit pick` is unchanged: the ranked list, no mutation
- [ ] The `plan-pick` skill drives one verb to claim-and-start, drops
      the landed-work grep, and stays under the skill kind's line cap
- [ ] The JSON goldens `pick --go` touches are re-recorded and the diff
      read
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
