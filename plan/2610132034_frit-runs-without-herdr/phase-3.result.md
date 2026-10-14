---
n: 3
title: yield tears a headless lane down, and the pane verbs say why not
status: "✅"
result: true
summary: >-
  On a host with no herdr installed, yield removes a fenced lane's
  worktree with git from the primary checkout, never the checkout
  itself; nudge and message refuse naming herdr not installed. C19
  and C20 cover it.
---
# Phase 3 result

## Handoff

On a host with no herdr installed, yield now removes the lane as well
as parking its work. It reads the lane from the calling directory,
checks that it is this plan's own lane, and removes it with `git
worktree remove` from the primary checkout. The primary checkout
itself is never removed: a yield run from there warns and leaves it.
A dirty worktree is not forced, and git's refusal becomes a warning.

nudge and message refuse with `herdr not installed; no pane to
reach`, and carry no herdr problem. An installed herdr that does not
answer still reads as "herdr unreachable". open is unchanged, as
planned: it sends nothing, and names the missing herdr among its
problems.

**Verified by.** Unit tests, red first, for git's worktree removal,
the headless teardown end to end from a fenced lane, the two guards
(the primary checkout, another plan's lane), and both verbs'
refusals. Two command scenarios run through godog against the built
binary under a git-only `$PATH`. C19 has message refuse on the
certain absence. C20 claims, fences the lane with a takeover from a
second clone, and has yield park the work and remove the lane. With
the teardown branch disabled, C20 fails.

**Noticed, not changed.** After `release`, yield from the same lane
refuses as "still held by this lane". That happens with herdr too,
and is outside this plan.

**For Task 4.** The primary-checkout guard is in place for the lane
held from the caller's own checkout. Recognition, landing, renewal
and the marker trailer remain the open decisions carried over from
plan 2609302340.
