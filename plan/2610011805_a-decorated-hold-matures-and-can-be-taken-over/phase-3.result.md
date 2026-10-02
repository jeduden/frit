---
n: 3
title: A matured decorated hold is taken over
status: "✅"
result: true
summary: >-
  claim and start take a matured decorated-only hold over. They park
  its unlanded work, delete the branch on origin by CAS on the tip the
  window matured on, and mint plan/<id> fresh. A holder that pushed
  meanwhile is not seized. S100 and S101 cover it.
---
# Phase 3 result

## Handoff

A deserted decorated hold no longer needs hand-run git. Once its
window matures, `frit claim` or `frit start --go` takes it over. Each
decorated branch is read on origin first, and the takeover refuses as
a lost race if any moved. Then each branch's unlanded work is parked
to the rescue ref and the branch is deleted on origin by CAS on the
tip the window matured on. Only then is `plan/<id>` acquired, fresh at
epoch 1, since a legacy claim has no epoch chain to extend. The
acquire still arbitrates against any other claimant. The report names
the retired branch, and the rescue when work was parked.

A local copy is dropped only if it still sits at the observed tip and
no worktree stands on it. The issue's own deserted checkout therefore
keeps its local branch. Removing that worktree is herdr's, as for any
lane.

**Verified by.** Lease-level tests for the retire-then-acquire, the
park, the moved branch, the unreadable origin, a local-only branch, a
checked-out branch and a local copy ahead of origin. cmd tests for
the routing, the window reset and the report. S100 and S101 drive
`frit claim` through godog. Against the built frit, in a scratch
fleet copying the issue's branch with the window seeded matured,
`board --json` read `stale: true` at 10800s. A dry-run `start`
composed. `claim`, given a stub herdr, deleted the decorated branch
from origin and minted `plan/2608070719`. The stub then failed the
worktree stand-up, so claim unwound with a release marker as designed.

**Docs.** `claiming.md`'s Legacy holds section now covers the whole
path. It sits at its 300-line cap, so the section was rewritten
rather than extended. The lease-protocol matrix sits at its token
budget, so the S88–S90 and S93 rows were tightened to make room for
S100 and S101.

**Left out.** `reap`'s unstaffed pass still refuses a decorated hold
with "migrate first". Teaching it this retire is a follow-up. So is
naming a takeover's retired branches when it retires more than one:
the report carries one branch, preferring the one that parked work.
