---
n: 4
title: The takeover answers its code review
status: "✅"
result: true
summary: >-
  A live agent on the decorated branch now vetoes the takeover (S102).
  A refused delete is a fault, a vanished branch is skipped, every
  retired branch is reported with what was kept, and the configured
  remote's copy is the one watched.
---
# Phase 4 result

## Handoff

The decorated takeover now checks herdr before it touches anything. A
live agent in a local worktree on the decorated branch refuses the
claim as a live agent session, and the branch stands. The decorated
takeover and scavenge share one CAS delete. A delete the server
refuses while the branch has not moved is a fault naming the push's
own error, and no longer restarts the window. A branch that vanished
before the takeover is skipped.

Claim and start now carry a `retired` list in `--json`, one entry per
branch: its rescue, whether origin's copy was deleted, and whether
this host's copy still stands. The table prints each, saying "kept"
for a branch never pushed. The lease and decorated tip readers now
rank the configured remote's copy first, so another remote's stale
copy can no longer decide what the observer watches.

**Verified by.** S102 claims the plan with the veto disabled and
refuses with it. Unit tests cover the server-refused delete, a branch
moving mid-delete, a vanished branch, the kept-local flags, the
remote ranking and the printer. The JSON golden files were
re-recorded; the only change is `"retired": []`.
