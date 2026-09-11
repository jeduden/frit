---
n: 1
title: advance relays a locally fast-forwarded lease branch, or refuses
status: "✅"
result: true
summary: >-
  Renew, Resume, Release and the session bind now build on a lane's
  local fast-forward of the tip they are handed, refuse a diverged
  local branch by name, and name every local ref move in its reflog;
  S96 covers it.
---
# Phase 1 result

## Handoff

Issue #189 is closed at the lease level. A lane whose own `plan/<id>`
branch carries an unpushed fast-forward beyond the tip its renewal is
handed keeps that work. The next beat, release or bind marker is built
on the local tip, so the push carries the local commit to origin. The
compare-and-swap still expects the handed tip, so a foreign push in
the same window fences exactly as before. The local advance stays put
for yield to park.

A local branch that has genuinely diverged from the handed tip is
refused before anything is minted. The refusal names the branch, both
tips, and the way out: merge the lease tip into the branch, never
rebase it. It is a new sibling error rather than the fresh-acquire
one, whose wording tells the caller to push or rename before claiming.

Every move frit makes to the local work ref now passes the ref's prior
value and a reason naming the transition, such as `frit: plan 7:
beat`. Claim, takeover, beat and release are therefore all
`git reflog`-recoverable.

**Deviation from the spec.** The spec named three cases. A fourth is
handled: a local ref *behind* the handed tip is the ordinary stale
view. Nothing on it is missing from the tip, so the marker is minted
on the handed tip, as before, rather than refused. Refusing it would
have fenced the session bind's own reconcile on any host whose local
copy lags origin. The phase spec now says so.

**Tests.** The red fixtures assert the fixed behavior and failed on
the old code: relay for Renew, Release and the bind; the divergence
refusal; the reflog reason. The foreign-fence, behind and no-local-ref
fixtures passed before and after, pinning what must not change.

**BDD.** S96 ("lane branch fast-forwarded locally past its renewal's
tip") is the next free number against origin's matrix, S95 being the
last. No open lane reserved anything higher. It runs in
`lifecycle.feature` without `@pending`, reusing the lease world's
holder, unpushed-commit and renew steps. The lease-protocol note was
at its token budget, so the "work ref" term lost its redundant
"in the new design" clause to make room for the row.

**Found in review: a commit during the push.** The first cut read the
local branch afresh just before moving it. So a commit the lane made
while the renewal's push was in flight was reset past — #189 again, in
a smaller window, and the session bind at `start` runs exactly then.
The move now goes only from the value read before the marker was
minted ("" meaning the branch must not exist). A branch that moved in
between is left standing, and the next renewal refuses it as
diverged. S97 in the Races section covers this. Its row pushed the
lease-protocol note over budget again, so the wordy "takeover path"
sentence and the S87 renumbering note were condensed, every fact kept.

**Found in review: `frit claim` hid the refusal.** `frit start`
surfaced the divergence as its plain error text. `frit claim` did not:
its resume path treated any resume error as doubt and fell through to
the ordinary path, which refused the plan as deserted or held and lost
the merge hint. A divergence is not doubt, since the token already
proved the lease is this lane's. So the claim now carries it in the
report as the refusal, and `--json` reads it too. C11 in the command
catalog covers it: a command's wording, not a lease race. `frit start`
had the same gap in another shape: its resume handed the divergence
back as a raw error, so `start --go` failed and `pick --go` stopped
its whole walk. It is now a refusal like a lost race, which `pick
--go` skips for the next candidate. C12 covers it.

**Found in review: a beat for another holder.** A vetoed takeover
renews the live holder's lease on its behalf, from this clone. The
relay assumed every renewal runs from the holder's own lane, and this
path breaks that. A reviewer's unpushed fixup on this clone's
`plan/<id>` was pushed into another host's lease, under that host's
name. The on-behalf beat now mints on the observed tip only when the
local branch stands at or behind it. A branch beyond or beside it is
refused, since from here it cannot be told whether it is the holder's
own lane on this host or someone else's work, so it is neither pushed
nor reset past. The veto refuses the takeover all the same; the live
holder renews itself. S98 in the Races section covers it.

**Found in review: a bare repository.** A bare repository leaves
`core.logAllRefUpdates` off, so git logged none of these branch moves
there. The move now asks for its reflog explicitly.
