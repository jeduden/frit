---
id: 2610132034
title: frit runs a lane on a host without herdr
status: "✅"
summary: >-
  Today every lane verb refuses on a host with no herdr: claim,
  start --go and pick --go stop before the mint, and open, nudge,
  message and yield have no pane to act on. This plan lets such a
  host run a lane end to end. frit stands the worktree up itself
  with git, hands the agent's prompt back rather than dispatching
  it, tears the worktree down with git, and lets a cloud session
  hold a plan from its own checkout.
model: sonnet
depends-on: []
---
# frit runs a lane on a host without herdr

## Goal

A host with no herdr installed can claim, start, resume, release and
yield a lane. frit does in git what herdr did, and hands back
whatever it could only have done in a pane.

## Context

**Where herdr is load-bearing today.** A survey of `cmd/frit` and
[internal/herdr](../../internal/herdr) found four uses, each with a
different fate here:

- *Presence* (`agent list`) already degrades. board shows `?`, who
  prints no live agents, and a takeover veto fails open to the
  staleness window (S61). Nothing changes.
- *The worktree* (`worktree create|open|remove`) is the hard
  dependency. claim and `start --go` hand it to herdr, and plan
  2609302340 phase 1 made them refuse when herdr is not installed
  (C15, C16). A worktree is plain git, though: reap and start's
  leftover cleanup already remove one with `git worktree remove`.
- *The pane* (`agent start`, `agent prompt`, `agent focus`) has no
  git stand-in. Without herdr there is nothing to dispatch into, so
  the composed prompt goes back to the caller, as a dry-run `start`
  already prints it.
- *The session binding* (`pane current`) yields no session. The hold
  then carries none, so only the window protects it. That is the
  shape S61 already accepts when herdr is down at observation.

**The switch.** A missing herdr is certain, and an unreachable one
is not. Plan 2609302340 settled this and it holds: the herdr-less
path is taken only when the binary is not on `$PATH`, the probe phase
1 of that plan added. An installed herdr whose socket refuses keeps
the mint-and-unwind S60 pins. An explicit opt-out for a host that has
herdr installed is a later task, decided once the herdr-less path
exists.

**Reuse.** These pieces were found and reused:

- the missing-herdr probe;
- C15's harness, which runs the built frit under a `$PATH` holding
  git alone;
- the lane path claim already computes;
- the token persist both claim and start reach after a stand-up;
- reap's `git worktree remove`.

No `Runner` fake is written for the git stand-in, because the
proving scenario runs the real binary against real git. Phase 1
adds no lane-backend interface either. Two backends, herdr and git,
share only `create` and `remove`, so a branch on the probe at those
two sites is enough until a third backend appears.

**What this reverses.** C15 and C16 pin a refusal. Once each verb
can finish without herdr, its refusal row is rewritten to the
success, never deleted. The probe stays the switch. The
[README](../../README.md)'s line that herdr must run for every lane
verb is narrowed as each verb lands. So is
[architecture.md](../../docs/architecture.md)'s "herdr owns
worktrees".

**Absorbs plan 2609302340 phase 2.** That phase's herdr-less lane,
held from a cloud session's own checkout, becomes Task 4 here. That
phase is marked ⛔ in the same commit. Its five open decisions moved
with it.

**Task 4 decided: the lane stays a lane.** Task 4's five questions
rest on one decision: where a harness session's work lives on origin
relative to the lease. A lease that only ever carries markers, with
the work on the harness branch, breaks the rule that work rides the
lease ref:

- the tip stops moving while the agent works, so other hosts see an
  attended lease go stale;
- a takeover no longer fences the old session's pushes;
- landing falls back to the glyph and a window;
- a park cannot see the work.

The owner chose to keep the rule. The session claims as any headless
host does, works in the git-stood lane on `plan/<id>`, and pushes
there. It then pushes the same tip to its harness branch, which only
carries the PR. Recognition, landing, renewal and teardown stay as
they are, and no `--here` flag is needed. Two alternatives were
weighed and set aside. frit could relay a harness-branch tip onto
the lease, but harness pushes would sit outside the fence. A `work:`
trailer pointing at the harness branch would make the observer,
scavenge and park all learn a second ref.

## Tasks

1. Phase 1: claim on a host without herdr stands the lane's worktree
   up with git, persists its token, and releases from it.
2. `start --go` and `pick --go` without herdr: claim, stand up with
   git, and report the composed prompt and the worktree as handed
   back, with `prompt_dispatched` false. A resume reattaches with no
   `worktree open`. C16 is rewritten to the hand-back. `open` is
   unchanged: it keeps its presence-unknown answer (phase 3).
3. Teardown and the pane verbs: yield removes a herdr-less lane with
   `git worktree remove`. start's leftover cleanup treats a missing
   herdr as proof that no pane sits on the leftover. nudge and
   message refuse, naming herdr not installed rather than
   unreachable. The refusal still prints the text to deliver.
4. A session whose branch the harness fixes works its plan in the
   headless lane and copies the lane's tip to its harness branch for
   the PR. The skills teach the copy, and a command scenario proves
   the plan lands when that branch merges.
5. A host with herdr installed may opt out of it, through frit's
   four-place settings chain. Owner's decision: it must be allowed.
6. On a headless host, a leftover worktree a fresh start meets is
   parked and left standing for its owner, never removed. Owner's
   decision, from the third review.

Each task ships its own skill edits and README lines in the same
change as the verb. That is CLAUDE.md's Shipping Skills rule. The
skills that touch herdr are `plan-pick`, `plan-start`, `plan-drive`,
`plan-tidy` and `plan-phase`.

## Execution

| Phase | Title                                                                  | Tier   | Gate                                                                                     |
| ----- | ---------------------------------------------------------------------- | ------ | ---------------------------------------------------------------------------------------- |
| 1     | claim stands its own worktree up without herdr                         | opus   | C15 on the built frit, git-only `$PATH`: claim, then release from the lane               |
| 2     | start and pick hand the prompt back without herdr                      | sonnet | C16 and C18, git-only `$PATH`: handed back, then resumed                                 |
| 3     | yield tears a headless lane down, and the pane verbs say why not       | sonnet | C19 and C20, git-only `$PATH`: refusal wording; yield removes the lane                   |
| 4     | a harness session publishes its lane through its own branch            | sonnet | C21, git-only `$PATH`: merging the harness branch reads the plan unheld                  |
| 5     | a headless start parks a leftover worktree and leaves it for the owner | sonnet | C23, git-only `$PATH`: parked, left standing, refusal names reap; `reap --go` removes it |
| 6     | a host with herdr installed can opt out with --no-herdr                | sonnet | C24: a recording herdr on `$PATH` is never called; the lane is git's                     |

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

| #   | Status | Phase                                                                                                                                                                                                                                                                           |
| --- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [claim stands its own worktree up without herdr](phase-1.md)                                                                                                                                                                                                                    |
|     | ↳      | On a host with no herdr installed, claim mints the lease and stands the lane up with git on the minted tip, its token kept and no session bound; release from that lane ends it. C15 now pins the success rather than the refusal.                                              |
| 2   | ✅     | [start and pick hand the prompt back without herdr](phase-2.md)                                                                                                                                                                                                                 |
|     | ↳      | On a host with no herdr installed, start --go and pick --go claim, stand the lane up with git and hand the prompt back as handoff handed-back; a start from inside that lane resumes on its token and hands back again. C16 is rewritten to the hand-back and C18 added.        |
| 3   | ✅     | [yield tears a headless lane down, and the pane verbs say why not](phase-3.md)                                                                                                                                                                                                  |
|     | ↳      | On a host with no herdr installed, yield removes a fenced lane's worktree with git from the primary checkout, never the checkout itself; nudge and message refuse naming herdr not installed. C19 and C20 cover it.                                                             |
| 4   | ✅     | [a harness session publishes its lane through its own branch](phase-4.md)                                                                                                                                                                                                       |
|     | ↳      | A session whose harness fixes its branch works its plan in the headless lane on plan/<id>, then copies that tip to its own branch for the PR; once that branch merges, the plan reads landed through existing evidence. No protocol change; C21 proves it, the skills teach it. |
| 5   | ✅     | [a headless start parks a leftover worktree and leaves it for the owner](phase-5.md)                                                                                                                                                                                            |
|     | ↳      | On a host with no herdr, a fresh start that meets a leftover worktree on the plan's branch parks its unlanded work and leaves the worktree standing, refusing before any mint with the path, the rescue ref and the frit reap cleanup. C23 proves it end to end.                |
| 6   | ✅     | [a host with herdr installed can opt out with --no-herdr](phase-6.md)                                                                                                                                                                                                           |
|     | ↳      | --no-herdr, FRIT_HERDR=false or herdr: false in a config file runs a host with herdr installed exactly as one without it, never running the local binary; nudge and message name herdr turned off. C24 proves it.                                                               |
<?/catalog?>

## Acceptance Criteria

- [x] On a host with no herdr installed, `frit claim <id>` holds the
      plan in a worktree of its own on `plan/<id>`, and `frit release`
      from that worktree ends it.
- [x] `start --go` and `pick --go` on such a host claim and hand the
      composed prompt back, dispatching nothing.
- [x] `yield` removes a herdr-less lane's worktree. nudge and message
      refuse, naming herdr not installed.
- [x] A session whose branch the harness fixes can hold and work a
      plan in a headless lane, publish it through its own branch, and
      see the plan read as landed once that branch merges.
- [x] An installed but unreachable herdr keeps today's behavior (S60,
      S61 unchanged).
- [x] README, architecture.md and the shipped skills say what works
      without herdr.
- [x] On a headless host, a leftover worktree a fresh start meets is
      parked and left standing, and the refusal names its path, the
      rescue ref and the `frit reap` cleanup.
- [x] A host with herdr installed can opt out with `--no-herdr`,
      `FRIT_HERDR=false` or `herdr: false`, and then never runs herdr.
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
