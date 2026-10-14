---
n: 1
title: claim stands its own worktree up without herdr
status: "✅"
result: true
summary: >-
  On a host with no herdr installed, claim mints the lease and stands
  the lane up with git on the minted tip, its token kept and no
  session bound; release from that lane ends it. C15 now pins the
  success rather than the refusal.
---
# Phase 1 result

## Handoff

A host with no herdr installed can now hold a plan. `frit claim <id>`
mints the lease and checks `plan/<id>` out into the lane's usual path
with `git worktree add`. It keeps the lane's token there and binds no
session. `frit release` from inside that worktree ends the lease
unaided. A `git worktree add` that fails unwinds the lease the way a
herdr failure does.

**What herdr's create checks out.** The mint already moves the local
`plan/<id>` to the minted tip, and the test fakes for herdr's create
run `git worktree add <path> plan/<id>`. The git stand-in does the
same, so the lane stands on the claim marker, and the token, beat and
divergence checks read it as they read a herdr lane.

**Verified by.** Unit tests, written red first, for the git helper,
for the choice between herdr and git, for a headless claim end to
end, and for the unwind. C15, the command scenario run by godog
against the built binary under a `$PATH` holding git alone, is
rewritten. It now pins the claim, the lane on origin's tip, no
session in the marker, the token, and the release. With the git
fallback switched off, C15 fails. A hand run of the built frit
under a git-only `$PATH` claimed, released from the lane, and saw
`pick --go` still refuse.

**BDD decision.** No new S row: a sessionless hold is S61's shape. An
S103 was written, passed, and was dropped because the lease-protocol
matrix sits at its token budget and the row only restated S61.

**Still refusing.** `start --go` and `pick --go` keep the missing-herdr
refusal. Its wording now says the phase runs in a herdr pane, since
the worktree no longer needs herdr. The `plan-pick` skill points a
headless session at `frit claim <id>`.

**For Task 2.** The herdr-or-git choice for the worktree is the one
seam to extend. start's own stand-up creates, reattaches or reuses a
pane. Its fresh-worktree branch can take the same choice. With no
pane, the prompt is reported rather than dispatched.
