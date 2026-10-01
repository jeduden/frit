---
n: 1
title: The observer watches a decorated hold
status: "🔳"
result: false
---
RED, each failing on today's code:

1. A fleet gathered over a repository whose plan 7 is held only by
   `plan/7-slug`, a legacy claim pushed to origin with no `plan/7`
   beside it, carries that branch and its tip on the plan. Today the
   plan is `Held` with nothing to watch.
2. `observeHolds` over that plan writes an observation key watching
   the decorated tip. Seeded three hours back, the next pass reads it
   `Stale` with a span of at least three hours. Today no key is
   written, and a fetching pass deletes a seeded one.
3. A plan with a lease ref still watches the lease tip, even with a
   decorated hold beside it (the migration shape). A plan with neither
   still drops its key on a fetching pass.

GREEN: the gather records each live decorated hold's tip on the plan,
preferring the remote-tracking copy as `leaseTips` does. The plan
answers one watched tip: its lease tip if it has one, else its
decorated holds' tips joined in branch order, so a move on any of them
restarts the window. `observeHolds` watches that tip. The takeover
count and the dead-session read stay on the lease tip, since a legacy
claim carries no epoch chain or session to read.

BDD coverage: the observation rule is the lease protocol's (OBS), but
a window maturing has no cross-host race of its own. The behavior it
unblocks — a matured decorated hold taken over — is the scenario.
Phase 3 adds it as `@S100`, which seeds the window this phase makes
possible. This phase is pinned by cmd-level tests on the gather and
`observeHolds`.

Gate: the tests above fail on the old code and pass after. Against the
built frit, in a scratch fleet holding a plan on `plan/<id>-slug`
only, two `start` runs a few seconds apart report a span above zero,
and `observations.json` carries the key. `go test ./...` and
golangci-lint pass.
