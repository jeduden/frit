---
id: 2609011806
title: One runner per lane — a dispatched phase is never started twice
status: "✅"
summary: >-
  frit pick --go and start --go both claim the top plan and dispatch its
  phase prompt into a fresh pane, reporting prompt_dispatched: true. But
  nothing stops that same phase running a second time in the same lane:
  the operator (or a skill flow) re-runs /plan-phase in the picking
  session, or a live-but-unbound session already sits on the lane. Two
  runners then share one worktree, racing on commits. Two guards close
  both doors. A dispatch-time refusal: before a fresh acquire stands the
  lane up, if herdr already shows a live agent on the plan's lane,
  refuse loudly and carry it in --json rather than dispatch a duplicate
  — generalizing the leftover-worktree check to any live lane, so a
  live-but-unbound session (the very bind-fence case plan 2609011611
  addressed) no longer slips the claim veto. And the plan-pick and
  plan-phase skills learn that prompt_dispatched: true means the phase
  is already running in that pane: report it, never re-run it. A
  re-typed /plan-phase is not a frit verb frit can intercept, so the
  skill is the only guard for that vector. A persistent per-phase lock
  is a heavier follow-up A and B make unnecessary. Closes #126.
model: sonnet
depends-on: []
---
# One runner per lane — a dispatched phase is never started twice

## Goal

Close the two ways [issue #126][issue] lands two runners in one shared
worktree. `frit pick --go` and `frit start --go` never dispatch a
second runner onto a lane a live session already holds. And the
`plan-pick` and `plan-phase` skills treat a dispatched phase as already
running, so a caller never re-runs it.

[issue]: https://github.com/jeduden/frit/issues/126

## Context

**The confirmed failure.** `pick --go` and `start --go` share one path,
`buildStart` in [cmd/frit/start.go](../../cmd/frit/start.go): they mint
the claim, stand the lane up, dispatch the phase prompt into a fresh
pane, and report `prompt_dispatched: true` with the pane in `--json`
(the `report.StartDoc` field, true exactly when the prompt was sent).
Nothing then stops that same phase running a second time in the same
lane. The two ways it happens: the operator — or a skill flow — re-runs
`/plan-phase <id>` in the picking session, reasonable when the pick just
claimed the plan and it is not obvious a pane already runs it; or a
second live session was already on the lane's worktree. Both leave two
runners racing on commits in one checkout, untangled only by
`git rebase --onto` and a blob-hash check.

**Why the hold model does not catch it.** The claim is per plan, one
lane, and both runners share it — same worktree, same branch, same
machine — so frit sees one legitimately held lane. The atomic claim
stops two *different* lanes starting the same plan; it does nothing
about two runners inside *one* lane.

**What already half-covers it, and the gap each leaves.** The table
`printStart` already closes with "do not run it here". But a skill reads
`--json`. And [plan-pick](../../internal/skills/assets/plan-pick/SKILL.md)
never mentions that `--go` dispatches, so a caller re-runs the phase.
`reconcileLeftoverWorktree` in [start.go](../../cmd/frit/start.go)
refuses a fresh acquire when a live herdr pane sits on a *registered*
leftover worktree. But it fires only when git already lists a worktree
on the branch. The claim's takeover veto skips a plan whose lease
carries a *bound, live* session. But a bind that never landed leaves a
live lane the veto cannot see — the self-fence plan
[2609011611](../2609011611_bind-renews-from-the-current-tip/plan.md)
fixed, and any that still fails. So a live-but-unbound lane slips all
three and takes a duplicate runner.

**Reuse first, and where the fix goes.** `liveLaneFor` in
[cmd/frit/dispatch.go](../../cmd/frit/dispatch.go) already answers "is a
live agent on one of this plan's hold branches, in this repo". It is the
join `open` and `nudge` dispatch against, matched by branch and
repository, so an identically named branch elsewhere is never mistaken
for the lane. The dispatch-time guard reuses that read at
`pick`/`start --go`'s pre-flight rather than a second liveness probe. A
fresh acquire whose lane already shows a live agent is refused before
anything stands up. `SessionLive` and `RenewToBind` in
[internal/claim/lease.go](../../internal/claim/lease.go) and
[internal/herdr/session.go](../../internal/herdr/session.go) stay the
takeover veto's own machinery, untouched.

**Out of scope.** A persistent per-phase lock — a marker in the lane's
`<gitdir>/frit/` beside [token.go](../../internal/claim/token.go)'s
per-worktree store, that a second `/plan-phase` in the same worktree
refuses on — is the issue's third proposed guard. It is heavier: a new
lock lifecycle the `plan-phase` skill must acquire and release, with its
own staleness path for a crashed runner. The two guards here cover both
described vectors without it, so it is recorded as a deferred follow-up,
not built. The takeover veto, the board rendering, and the resume path
are untouched.

## Tasks

1. Phase 1 (proving slice): a dispatch-time refusal — a fresh
   `pick --go` / `start --go` whose plan's lane already shows a live
   herdr agent refuses and dispatches nothing, the refusal carried in
   `--json`; a lane with no live agent still starts. Driven red at the
   cmd level against the herdr fake the dispatch tests already use.
2. Phase 2: the `plan-pick` and `plan-phase` skills treat
   `prompt_dispatched: true` as "the phase is already running in that
   pane — report it, never run `/plan-phase` yourself," regenerated into
   the dogfood copies and proven against the built `frit`.

## Execution

| Phase | Title                                                  | Tier   | Gate                                                                                                                                                                      |
| ----- | ------------------------------------------------------ | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | Dispatch refuses a second runner on a live lane        | sonnet | a herdr fake showing a live agent on the lane makes `pick --go` and `start --go` refuse, sending nothing; refusal in `--json`; a clear lane starts; `go test ./...` green |
| 2     | The skills treat a dispatched phase as already running | sonnet | the built `frit`'s `prompt_dispatched` matches the skill text, `frit skills` regenerates the copies, `TestDogfoodCopiesMatchCanonical` and `mdsmith check .` pass         |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "!phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row: "| {n} | {status} | [{title}](phase-{n}.md) |"
footer: |

?>

| #   | Status | Phase                                                                |
| --- | ------ | -------------------------------------------------------------------- |
| 1   | ✅     | [Dispatch refuses a second runner on a live lane](phase-1.md)        |
| 2   | ✅     | [The skills treat a dispatched phase as already running](phase-2.md) |
<?/catalog?>

## Acceptance Criteria

- [x] A fresh `frit pick --go` / `frit start --go` whose plan's lane
      already shows a live herdr agent refuses and dispatches nothing,
      the refusal carried in `--json`
- [x] A plan whose lane shows no live agent still starts unchanged
- [x] The guard fires on a live-but-unbound lane the takeover veto does
      not catch, not only on a registered leftover worktree
- [x] `plan-pick` and `plan-phase` tell a caller that
      `prompt_dispatched: true` means the phase is already running in
      that pane — report it, never re-run it
- [x] The dogfood skill copies regenerate and
      `TestDogfoodCopiesMatchCanonical` passes
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
- [x] `mdsmith check .` is clean
