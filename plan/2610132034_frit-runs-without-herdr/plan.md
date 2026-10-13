---
id: 2610132034
title: frit runs a lane on a host without herdr
status: "🔲"
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
phase is marked ⛔ in the same commit. Its five open decisions move
with it unchanged, and are settled when Task 4 is written as a phase.

## Tasks

1. Phase 1: claim on a host without herdr stands the lane's worktree
   up with git, persists its token, and releases from it.
2. `start --go` and `pick --go` without herdr: claim, stand up with
   git, and report the composed prompt and the worktree as handed
   back, with `prompt_dispatched` false. A resume reattaches with no
   `worktree open`. `open` prints the lane's path rather than
   focusing a pane. C16 is rewritten to the claim.
3. Teardown and the pane verbs: yield removes a herdr-less lane with
   `git worktree remove`. start's leftover cleanup treats a missing
   herdr as proof that no pane sits on the leftover. nudge and
   message refuse, naming herdr not installed rather than
   unreachable, and hand back the text to deliver by hand.
4. The lane held from the caller's own checkout (`claim --here`),
   for a session whose branch the harness fixes. Settle plan
   2609302340 phase 2's five decisions first: recognition by token,
   landing by glyph, renewal by re-claim, a marker trailer no verb
   tears down, and the flag surface.
5. Decide whether a host with herdr installed may opt out, through
   frit's four-place settings chain, and build it if so.

Each task ships its own skill edits and README lines in the same
change as the verb. That is CLAUDE.md's Shipping Skills rule. The
skills that touch herdr are `plan-pick`, `plan-start`, `plan-drive`,
`plan-tidy` and `plan-phase`.

## Execution

| Phase | Title                                          | Tier | Gate                                                                                                                                                                       |
| ----- | ---------------------------------------------- | ---- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | claim stands its own worktree up without herdr | opus | C15 rewritten and a new S row run the built frit under a git-only `$PATH`: worktree on `plan/<id>`, token kept, release ends it; S60, S61 unchanged; `go test ./...` green |

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

| #   | Status | Phase                                                        |
| --- | ------ | ------------------------------------------------------------ |
| 1   | 🔲     | [claim stands its own worktree up without herdr](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] On a host with no herdr installed, `frit claim <id>` holds the
      plan in a worktree of its own on `plan/<id>`, and `frit release`
      from that worktree ends it.
- [ ] `start --go` and `pick --go` on such a host claim and hand the
      composed prompt back, dispatching nothing.
- [ ] `yield` removes a herdr-less lane's worktree. nudge and message
      refuse, naming herdr not installed.
- [ ] A session with no herdr can hold a plan from its own checkout,
      per the decisions Task 4's phase records.
- [ ] An installed but unreachable herdr keeps today's behavior (S60,
      S61 unchanged).
- [ ] README, architecture.md and the shipped skills say what works
      without herdr.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
