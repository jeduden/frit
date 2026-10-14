---
n: 2
title: start and pick hand the prompt back without herdr
status: "✅"
result: true
summary: >-
  On a host with no herdr installed, start --go and pick --go claim,
  stand the lane up with git and hand the prompt back as handoff
  handed-back; a start from inside that lane resumes on its token and
  hands back again. C16 is rewritten to the hand-back and C18 added.
---
# Phase 2 result

## Handoff

`start --go` and `pick --go` no longer refuse a host with no herdr.
They claim the plan and stand the lane up with git, the way claim
does since phase 1. Then they report a fourth handoff value,
`handed-back`: `started` true, `prompt_dispatched` false, no `pane`
and no `next_action`. The caller runs `prompt` in `lane`, and the
table says so in one line. No key was added. A start run from inside
that lane resumes on its token and hands back again. A reattach from
outside reuses the existing checkout.

A leftover worktree on the claim branch is reconciled as before. The
check for a live pane on it now reads a missing herdr as no pane.
The missing-herdr refusal and its wording are gone: nothing gives
them any more. A missing herdr is not carried as a problem on a
hand-back, since it is the host's mode rather than a fault. A dry run
still carries it, as before.

**Verified by.** Unit tests for the handed-back mark, the start
gate, the headless stand-up and resume, the live-pane check, and the
pick walk. The command scenarios run through godog against the built
binary under a git-only `$PATH`: C16 is rewritten, pick hands back
the top candidate and leaves the other alone. C18 is new: start hands
back, then resumes from its lane and hands back again. With the
headless switch forced off, both fail. A hand run of the built frit
showed the table line and the resumed `--json`.

**Skills.** `plan-pick` and `plan-start` teach the `handed-back`
branch: run `prompt` in `lane` yourself. The plan-pick line about the
missing-herdr refusal is dropped.

**For Task 3.** yield still tears a lane down through herdr's pane,
and nudge, message and open still need a pane. A lane with no pane is
now common, so yield's teardown should fall back to `git worktree
remove`. The pane verbs should name herdr not installed rather than
unreachable.

**Review follow-up.** A code review found that a headless start never
read the configured remote hosts. The presence read stopped at the
missing local herdr. A lane live on another host therefore went
unseen, and the start handed back a second lane beside it. Before
this phase, the refusal made that gap moot. Now the hosts are still
read past a missing local herdr, and only the live-lane lookup uses
their panes. Every other reader keeps stopping at the error, and
open never tries to focus through the missing herdr. A unit test
reproduced the takeover first. C22, a command scenario, pins the
refusal against the built frit, because it is one host's decision
made before any mint. No lease-protocol row was needed.

**Second review follow-up.** The dry run on a headless host still
promised a pane, and the hand-back table still printed an agent line
as if one were running. The start document now carries a `headless`
key, set in a preview as in a run. The dry run says `--go` hands the
prompt back, and the agent line reads as yours to run. A missing
herdr is not carried as a problem in a dry run either. This reverses
the earlier choice to keep the dry run as it was, since the
plan-start skill tells a reader to trust the dry run before `--go`.

**Third review follow-up.** open, nudge and message read the configured
hosts past a missing herdr and then threw that read away, since they
refuse or report presence unknown there anyway. The live-lane lookup
now takes its presence read as an argument. Only start's three gates
read past the missing herdr. A refusal on a headless host no longer
lists the missing herdr as a problem, matching the hand-back. The
`plan-pick` example now shows `--json`, since the agent branches on
`handoff`. BDD coverage: C22 now also checks that its refusal carries
no herdr problem, and fails without the fix. The skipped host read is
a cost, not a behavior, so a unit test pins it and no scenario does.
