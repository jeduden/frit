---
n: 1
title: The observer watches a decorated hold
status: "✅"
result: true
summary: >-
  A plan held by a decorated branch alone now gets an observation key
  on the first pass that sees it. Its span grows across passes, and
  it reads stale once the takeover window matures, like a lease does.
---
# Phase 1 result

## Handoff

The sampler now watches a hold made only of a legacy decorated branch.
The fleet gather carries each live decorated hold's tip beside the
lease tip, origin's copy preferred. With no lease ref, the plan's
watched tip is its decorated tips joined in branch order. So a move on
any of those branches restarts the window, the same as a lease
renewal. A plan with a lease ref is watched on it alone, as before.
The takeover count and the dead-session read still need a lease
marker, so a decorated-only hold matures on the bare window and is
never read dead.

**Verified by.** Unit tests on the gather (a decorated-only hold
carries its tip; origin's copy wins; a released decorated branch
carries none), on the watched tip, and on `observeHolds` (a first pass
writes the key; a seeded three-hour window reads stale). One cmd test
drives the issue's shape end to end: a legacy claim pushed to origin
with no `plan/<id>`, and `gatherFleet` records its window. Each failed
first on the missing key or the zero span. Against the built frit in
a scratch fleet copying the issue's branch, two `start` runs three
seconds apart read `seen unchanged for 0s` then `3s`, and
`observations.json` carries the key.

**The gap this opens, closed by phase 3.** A matured decorated hold
now reads `Stale`, so `discovery.Ready` offers it for takeover. But
`mintOrTakeOver` still CASes on the empty lease tip, so `start --go`
or `claim` on it fails on "no lease marker" rather than taking over.
The old gather test's comment warned of exactly this. Phase 3 must
land in the same change.

**For phase 2.** Nothing here touches `release` or `yield`. They still
disagree on this hold.
