---
summary: >-
  How a frit on one host learns another host's local facts — a live
  pane, an in-flight merge, an open PR. herdr already monitors agents,
  so the question is the transport: pull herdr over ssh, or publish its
  readings to origin as a heartbeat. The lean is to look at local herdr
  first.
---
# Cross-host presence

Dated 2026-09-04. Research notes record the reasoning, wrong turns and
all; see [CLAUDE.md](../../CLAUDE.md).

## The problem

frit steers agents from their local edits, and it runs on many hosts at
once. The one place those hosts coordinate is git origin. A fact that
never reaches origin is local, and a distant frit cannot see it. Three
such facts matter:

- a live pane, and whether its agent works or idles;
- an in-flight merge the agent is finishing;
- a branch pushed to origin and open as a pull request.

None of these is a ref, so none is on origin. Read only the shared refs
and a deserted, unlanded lane looks identical to one whose work is open
as a PR and about to land.

## What went wrong, once

On 2026-09-03 a frit read the second case for the first. `frit pick`
refused plan 2609021315 as a deserted, dead hold and pointed at `frit
yield`. board showed `dead: true`. The reader yielded the hold and
prepared to rebase and land the work by hand. The branch was already
pushed and open as PR #151, its CI still running, the agent alive and
finishing a merge. The yield only parked a rescue ref, so no harm
landed, but the misread is the one this note exists to prevent.

Two facts were local the whole time: the live pane, and the open PR.
Neither was on origin, so the refusal could not know them. The wrong
move was to infer disposition from refs that cannot carry it.

## Why not a frit agent monitor

The first instinct is a per-host frit process that watches agents. That
duplicates herdr, which already owns panes, worktrees and prompts and
monitors the agent on each — including when it idles. frit's rule is to
consume, not reimplement (see
[architecture.md](../architecture.md)). So the monitor is not the gap.
herdr is the monitor. The gap is getting one host's herdr reading to
another host.

## The real choice: the transport

**Pull herdr over ssh — what exists.** frit already reads other hosts'
herdr with the `--hosts` flag, over ssh. It is pure: frit still writes
only the claim, honoring the one-mutation rule. Its costs are two. The
read is synchronous, so a host that is asleep, offline or firewalled
reads as presence unknown, not as a fact. And it carries only what herdr
sees — the pane, never the pull request.

**Publish herdr to origin as a heartbeat — the later option.** A small
resident on each host watches herdr and the local repo, then pushes a
summary to an advisory ref, say `refs/frit/presence/<host>`. Every frit
then reads presence from origin. There is no ssh fan-out. It tolerates
an offline host through a last heartbeat and a staleness age, the way
the lease already ages a hold. It makes coordination-is-origin literally
true for presence, not only for the claim. The costs: it bends the
one-mutation rule, since frit would write an advisory ref beside the
claim, and it is a daemon to run and reap.

## The lean

Look at local herdr first. While the fleet is small and mostly online,
the ssh pull is enough, and it keeps the one-mutation rule clean. Reach
for the origin heartbeat only when the two costs of the pull are felt in
practice: an offline host reading as unknown, and ssh fan-out that grows
with the fleet.

## What no transport fixes

The pull request is the agent's fact, not herdr's. herdr sees the pane,
not the PR. So neither transport surfaces "this work is open as a PR" on
its own. The one honest source is the agent, and the way to reach it is
to ask. That is what the `frit message` verb is for — see plan
2609032048. A heartbeat could carry richer status only if the agent
itself publishes it, which is the agent writing to origin, not frit
monitoring.
