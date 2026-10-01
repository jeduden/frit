---
n: 2
title: release and yield agree on a decorated hold
status: "✅"
result: false
---
RED, each failing on today's code:

1. `release`, run inside a lane held only by `plan/7-shader-unit`,
   refuses rather than reporting "nothing holds it". `yield`, run from
   outside the lane, carries the same `refused` and `next_action`. Both
   name the decorated branch, and neither says "live".
2. The refusal for an unmatured hold held by another lane says "held
   by another lane" without "live".

GREEN: `release` reads "nothing holds it" only for a plan that is not
held. A held plan with no lease ref reaches the unproven-hold path.
That path becomes the one decision both verbs share, ordered: a
matured or dead hold points at `frit claim`. A decorated hold with no
lease ref is named as such, with the wait-or-take-over `next_action`.
A tokenless own lane keeps its S49 wording. Anything else is another
lane's hold. `yield` reaches it when it has nothing local to park.

BDD coverage: one host, no lease race, a refusal's wording across two
verbs — a command scenario, not a lease-protocol one. C17 drives
`release --json` from the lane and `yield --json` from outside it, and
requires the two refusals to match.

Gate: the cmd test and C17 pass; the cmd test failed first with the
issue's own "nothing holds it" against "held live by another lane".
`go test ./...` and golangci-lint pass.
