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

**Follow-ups, not chased.** `frit start` surfaces the divergence
refusal as its plain error text, which names the branch, both tips
and the merge. `frit claim` does not: its resume path treats any
resume error as doubt and falls through to an ordinary acquire, which
refuses as "already held on this host", so the merge hint is lost.
