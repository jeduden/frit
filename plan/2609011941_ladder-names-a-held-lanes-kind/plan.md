---
id: 2609011941
title: The escalation ladder names a held lane's kind, not a refusal loop
status: "🔲"
summary: >-
  The v0.11.0 retest of #122 shows the ladder round-trips on a held lane
  you own with no live agent: open prints "start it with frit start",
  start refuses "already held … not takeable until the window matures",
  and nudge says "no live lane". Three rungs, no exit. The reporting half
  (#121) is fixed by #124 — the board reads the state correctly now — but
  every rung still names the same next step regardless of the hold's
  kind. open's next action is always frit start; start's refusal always
  reads as an un-matured takeover; neither tells apart a lane whose token
  this machine holds unattended, a lane it cannot prove, and a lane a
  live agent is on. This plan makes the ladder honest: open, start and
  nudge each name
  the true kind of a held lane and the real next step, so no rung
  recommends a rung that will refuse. It is the guidance half, distinct
  from the resume mechanism in plan 2609011836, and it depends on that
  mechanism — the honest "resume it with frit start" next step is only
  true once start actually resumes a lane whose token it holds. Addresses
  #122.
model: sonnet
depends-on: [2609011836]
---
# The escalation ladder names a held lane's kind, not a refusal loop

## Goal

`frit open`, `frit start`, and `frit nudge` each name the true kind of a
held lane and the real next step. So a held lane whose token this
machine holds is never sent around the [issue #122][issue] refusal loop.
No rung points at a rung that refuses.

[issue]: https://github.com/jeduden/frit/issues/122

## Context

**The loop, retested on v0.11.0.** For a lane whose token this machine
holds with no live agent — worktree present, tree clean, branch pushed,
`held: true`,
`dead: false`, `agent: ""` — the ladder round-trips. `open` prints
"start it with frit start". `start` refuses "already held … not takeable
until the window matures". `nudge` says "no live lane". No rung names a
way back onto the lane.

**This is the guidance half, not the display half.** #124 fixed the
reporting (#121): the board now prints a header, names the `hold` and
`agent` columns, and renders a held-but-unattended lane as `idle` rather
than `-`. Reading the board now tells you the lane is held and
unattended. But every rung of the *ladder* still names one next step
regardless of the hold's kind, so the guidance itself is the loop.

**Where each rung goes wrong.** `openNextAction` in
[internal/report/dispatch.go](../../internal/report/dispatch.go) returns
`frit start <id>` for any laneless plan — it never reads who holds it, so
it recommends `start` even when `start` will refuse. `notMaturedReason`
in [cmd/frit/main.go](../../cmd/frit/main.go) and `claimRefusal` in
[cmd/frit/claim.go](../../cmd/frit/claim.go) phrase every held plan as an
un-matured takeover, never telling apart a hold this machine can prove —
its token on disk — from one it cannot. `nudge` in
[cmd/frit/dispatch.go](../../cmd/frit/dispatch.go)
refuses with a bare "no live lane", the same words whether nothing holds
the plan or a healthy lane sits there with its agent gone.

**Reuse first, and where the fix goes.** The facts that tell the kinds
apart already exist, and plan 2609011836 already reads them: the token
persisted in the lane the marker records proves whether this machine
holds the lane (`laneTokenResumeTip` / `tokenProves`), and
`laneUnattended` reads whether an agent is live on it. The holder string
is for reporting, never the test. The fix threads that same kind into
each rung's message. `open` names a resume for a lane whose token this
machine holds unattended, a wait-or-take-over for a lane it cannot
prove, and "a live agent is on it" otherwise, rather than a blanket
`frit start`. `start`'s refusal says which kind it is. `nudge` tells
"held, no agent attached" apart from "no lane at all".

**Why it depends on 2609011836.** That plan makes `frit start` actually
resume a lane whose token this machine holds, with no live agent. Until
it lands, `open`
naming `frit start` as the resume would still point at a refusal. So the
guidance rides on the mechanism: this plan is `depends-on: [2609011836]`
and names the resume only once the resume works.

**The scenario doc follows.** The lease-protocol research doc,
[lease-protocol.md](../../docs/research/lease-protocol.md), records this
deadlock as S76 — "pane gone before the window matures … a silent dead
end", mitigated only by `orphans` naming the deserted hold. Once the
ladder names the resume, that mitigation is no longer the whole story.
This plan updates S76 to record that `open` and `start` point at the
resume, so the dead end is named at the rung a stuck operator actually
stands on.

**Out of scope.** The resume mechanism itself — the guard and the
reattach stand-up — is plan
[2609011836](../2609011836_resume-a-held-lane-you-own/plan.md), which
owns the Self-resume and S76/S77 edits for the mechanism. The board
table rendering is #121, fixed by #124. This plan changes only what the
ladder *says*, not what it does beyond naming.

## Tasks

1. Phase 1 (proving slice): `open` names the honest next step per
   held-lane kind. `openNextAction` learns the hold's kind — a lane whose
   token this machine holds unattended, a lane it cannot prove, or a live
   agent present — and stops recommending `frit start` for a lane `start`
   would refuse, naming the resume, the wait, or the live agent instead.
   Driven red as the pure projection the report layer already unit-tests,
   then wired where `open` reads the hold.
2. Later phases, shaped by Phase 1's handoff: `start`'s refusal names
   which held kind it is rather than a blanket un-matured takeover, and
   `nudge` tells "held, no agent attached" apart from "no lane at all".

## Execution

| Phase | Title                                              | Tier   | Gate                                                                                                                                                 |
| ----- | -------------------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | open names the honest next step per held-lane kind | sonnet | `open` names a resume, a wait, or a live holder by held-lane kind, never a refusing `start`; an unheld plan says `frit start`; `go test ./...` green |

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

| #   | Status | Phase                                                            |
| --- | ------ | ---------------------------------------------------------------- |
| 1   | 🔲     | [open names the honest next step per held-lane kind](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] `frit open` on a lane whose token this machine holds, unattended,
      names a resume, not a blanket `frit start` that would refuse
- [ ] `frit open` on a lane this machine cannot prove (no token) names
      the wait or the take-over, not `frit start`
- [ ] `frit open` on a lane with a live agent names that a live agent is
      on it
- [ ] A plan that nothing holds, with no live lane, still names
      `frit start <id>` unchanged
- [ ] `lease-protocol.md`'s S76 records that `open` and `start` name the
      resume, not only that `orphans` names the deserted hold
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
