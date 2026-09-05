---
id: 2609050143
title: the survey reads a live lane the way dispatch does
status: "🔳"
summary: >-
  Plan 2609032048 made board, ready, pick and find offer `frit message`
  for a held lane whose bound session is gone but whose branch a live
  pane attends. Its code review found two ways the survey and the verb
  disagree about which lane is live. The survey keys live lanes by
  branch name alone, while dispatch also requires the lane's repository
  to match, so a live pane on the same branch name in another
  repository clears a dead marker and offers an ask that `message` then
  refuses. And the survey offers the ask while a configured host went
  unread, a state in which `message` refuses on presence unknown before
  reaching the pane. Both are pre-existing joins outside that plan's
  diff. This plan makes the survey read presence exactly as dispatch
  does — one repository-aware join, one presence-complete rule — so a
  remedy the survey names is one the verb will take.
model: sonnet
depends-on: []
---
# the survey reads a live lane the way dispatch does

## Goal

Whatever the survey says about a lane's live pane, the dispatch verbs
agree with. A board or discovery row clears `dead` or names an `ask`
only for a pane `frit message` would find and send to. That means a
pane in the plan's own repository, with presence fully read.

## Context

**The two findings, from the review of PR #161.** Plan 2609032048's
code review raised them and skipped both as outside its diff.

1. *Cross-repository branch keying.*
   [`liveByBranch`](../../cmd/frit/main.go) keys every staffed lane by
   its branch name and nothing else, and
   [`laneFor`](../../cmd/frit/main.go) walks a plan's `Holds` against
   that map. A hold branch name is repository-local — a plan id is
   only unique within a repository — so two repositories in one fleet
   holding plan 7 both name `plan/7`. If only repository B's lane is
   live and A's bound session is dead, A's row reads the live pane as
   its own: its `dead` is cleared and, since 2609032048, it carries an
   ask. Running that ask, `Resolve` refuses the bare id as ambiguous,
   and disambiguated by slug
   [`liveLaneFor`](../../cmd/frit/dispatch.go) — which already checks
   `fleet.RepoName(lane.Root, …) == p.Repo` for exactly this reason —
   rejects B's lane and refuses "no live lane". The dead-clearing half
   predates 2609032048: it came in with 2609031939.
2. *Unread-host presence.* `liveByBranch` returns the host problems
   [`fleetPresence`](../../cmd/frit/main.go) collected, and the survey
   carries them in `problems[]`, but reads presence as complete
   regardless. [`presenceUnknown`](../../cmd/frit/dispatch.go) is the
   rule `open`, `nudge` and `message` apply: a configured host that
   answered with neither a live read nor a cache leaves a lane
   possible behind the gap, so the verb refuses rather than act. The
   survey offers an ask in that state that the verb refuses with
   "presence unknown: a configured host went unread" before it ever
   reaches the pane herdr did show.

**Why fix the survey, not the verbs.** Both verb-side rules are
deliberate and pinned: the repository check is "the one error this
whole join exists to prevent", and presence-unknown is the difference
between an absent lane and one frit could not see. The survey is the
side that guessed. Making it read the same facts the verb reads is
one change at one site, and every consumer of `board`, `ready`, `pick`
and `find` inherits it.

**What is reused.** The repository resolution is already written once,
inline in `liveLaneFor`: `fleet.RepoName` over the lane's worktree
root through the host's own git. Phase 1 lifts it into one helper
both joins call, rather than a second copy. `presenceUnknown` is
already a pure function of the read outcomes; Phase 2 feeds the
survey's own `hostProbs` through it, unchanged. The `attended`
callback [`report`](../../internal/report/discovery.go)'s `cardsOf`
and `BoardDoc.AddPlan` take is the seam both phases work behind, so
the report package needs no new field — only the string the callback
answers changes.

**What was looked at and not reused.** `herdr.Lane.Repo`, set in
[`Join`](../../internal/herdr/resolve.go) as the basename of the
resolved worktree root, is not the repository's name for a linked
worktree — that is the lane directory's own name — which is why
`liveLaneFor` resolves through `fleet.RepoName` instead. `who` renders
`Lane.Repo` as a column and is left alone; this plan does not redefine
it. A fleet-wide "presence complete" flag on the gather result was
considered and set aside: the per-host problems already carry the
`noPresence` bit `presenceUnknown` reads.

**Out of scope.** The verbs' own rules do not move. `who` is
unchanged. No new herdr query: the join reads the same `agent list`
it reads today.

## Tasks

1. Phase 1: key the survey's live lanes by repository and branch,
   through the repository resolution `liveLaneFor` already uses, so a
   same-named branch in another repository neither clears `dead` nor
   earns an ask.
2. Phase 2: withhold the ask — never the dead-clearing — when the
   presence read was incomplete by `presenceUnknown`'s own rule, so
   the survey offers only what the verb will take.

## Execution

| Phase | Title                                           | Tier   | Gate                                                                                                                                                              |
| ----- | ----------------------------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | the survey keys live lanes by repository        | sonnet | Unit test: two repos holding plan 7 on `plan/7`, one live; the dead repo's rows stay dead with no ask and no agent; `liveLaneFor` unchanged; suite and lint clean |
| 2     | the survey withholds the ask on unread presence | sonnet | Unit test: a host with `noPresence` and a live pane elsewhere; rows clear dead but carry `ask: ""`; a cached host still offers the ask; suite and lint clean      |

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

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                 |
| --- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [the survey keys live lanes by repository](phase-1.md)                                                                                                                                                                                                                                                                                |
|     | ↳      | liveByBranch now keys every staffed lane by `repoBranch{repo, branch}` rather than branch alone, through a new `laneRepo` helper in `cmd/frit/main.go` that `liveLaneFor` in `cmd/frit/dispatch.go` now calls too. Two repositories holding the same plan id on the same branch name no longer collide in board, ready, pick or find. |
| 2   | 🔲     | [the survey withholds the ask on unread presence](phase-2.md)                                                                                                                                                                                                                                                                         |
<?/catalog?>

## Acceptance Criteria

- [x] With two repositories each holding the same plan id on the same
      branch name and only one of them live, `board`, `ready`, `pick`
      and `find` clear `dead` and carry an `ask` only on the live
      repository's row; the other stays dead, ask empty, agent empty
- [x] The repository resolution the survey uses is the one
      `liveLaneFor` uses, written once and called from both joins
- [ ] When a configured host went unread with no cache, every survey
      row carries `ask: ""` and the host rides in `problems[]`; a pane
      herdr did show still clears `dead`
- [ ] A host served from stale cache does not withhold the ask,
      matching `presenceUnknown`
- [ ] `open`, `nudge` and `message` are unchanged: their tests pass
      without edits
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
