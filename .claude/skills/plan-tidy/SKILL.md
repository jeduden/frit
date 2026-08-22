---
name: plan-tidy
description: >-
  Front the teardown and cleanup verbs: read frit orphans and frit
  stale to find the mess, then fix it with frit yield or frit
  release, never hand-run git. Trigger on "clean up worktrees", "tidy
  the lanes", "yield this plan", "release my lease", "what's
  orphaned", "what's stale".
---
# plan-tidy

A worktree or a claim left behind is not a `git worktree remove` and
`git branch -D` problem; it is a frit problem. Read the mess with
`orphans` or `stale`, then close it with the one verb that matches,
never with raw git.

## Method

1. **Read the mess.** `frit orphans` reports claims and checkouts
   that no longer add up: claimed but unstaffed, prepared but
   unstarted, held past its takeover window, or gone. `frit stale
   --days N` reports worktrees whose branch tip has not moved.
   `--json` parses either.
2. **Match the verb to the mess:**

  - This lane's own lease, done or abandoned → `frit release`. It
     ends the lease with a release marker pushed from this lane's own
     token; a plan nobody holds is a no-op, not a refusal.
  - A fenced lane — the lease moved under it, a stranger's claim now
     covers this worktree → `frit yield`. It parks the local
     divergence to a rescue ref, then tears the worktree down through
     herdr, so nothing local is lost to the teardown.
  - A hold whose work already landed → neither verb; `frit orphans`
     already reports it as scavenged, not something to act on.

3. **Never hand-run git for teardown.** `git worktree remove` and
   `git branch -D` throw away exactly the divergence `yield`'s rescue
   ref exists to keep, and neither one touches the claim ref at all —
   the hold outlives the worktree it named.

## Notes

- `frit board` shows who holds what alongside `orphans`, for the
  wider picture before acting on one lane.
- Acting on a foreign hold is `yield`'s job, not `release`'s: release
  only ever ends this lane's own lease.
