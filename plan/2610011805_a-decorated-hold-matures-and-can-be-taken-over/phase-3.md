---
n: 3
title: A matured decorated hold is taken over
status: "✅"
result: false
---
RED, each failing on today's code:

1. A lease-level decorated takeover over a pushed `plan/7-shader-unit`
   deletes it from origin and its unused local copy, then acquires
   `plan/7` at epoch 1. A claim-only chain parks nothing. A chain
   carrying unlanded work parks it to the rescue ref first.
2. A decorated branch that moved past the observed tip refuses as a
   lost race naming the new tip, and parks, deletes and mints nothing.
   An unreadable origin is a fault. A local branch a worktree stands
   on, or one carrying commits past the observed tip, is kept.
3. `mintOrTakeOver` on a matured decorated-only hold takes it over
   rather than failing on "no lease marker". A moved branch resets the
   window. A successful claim or start names the retired branch, and
   its rescue when one was parked.

GREEN: a decorated takeover in the lease package. It reads every
decorated branch on origin first, or locally for one never pushed,
and refuses if any moved. Then it retires each — park, then delete by
CAS on the observed tip — and acquires the id-only lease, which stays
the arbiter against any other claimant. `mintOrTakeOver` routes a
matured hold with no lease tip there. Claim and start report the
retired branch in the existing scavenged and rescue fields.

BDD coverage: a takeover is the lease protocol's (OBS, TAKE, PARK, A2)
and races another host's push, so it needs `@S` rows. S100: a
deserted decorated hold is claimed, its branch gone from origin, a
fresh `plan/<id>` minted. S101: a holder that pushed after its window
matured is not seized — the window restarts, and nothing is deleted.

Gate: the tests above, S100 and S101 pass. Against the built frit, in
a scratch fleet copying the issue's branch with its window seeded
matured, `board --json` reads it stale, a dry-run `start` composes
instead of refusing, and `claim` deletes the decorated branch from
origin and mints `plan/<id>`. `go test ./...` and golangci-lint pass.
