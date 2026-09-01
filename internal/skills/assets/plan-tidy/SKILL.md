---
name: plan-tidy
description: >-
  Front the teardown and cleanup verbs: read frit orphans and frit
  stale to find the mess, then fix it with frit yield, frit release or
  frit reap, never hand-run git. Trigger on "clean up worktrees", "tidy
  the lanes", "yield this plan", "release my lease", "what's orphaned",
  "what's stale".
---
# plan-tidy

A worktree or claim left behind is a frit problem, not a `git worktree
remove` and `git branch -D` one. Read the mess, then close it with the
verb that matches — never raw git.

## Read the mess

- `{{frit}} orphans --json` — claims and checkouts that no longer add
  up: claimed but unstaffed, prepared but unstarted, held past its
  takeover window, or gone.
- `{{frit}} stale --days N --json` — worktrees whose tip has not moved.

## Match the verb

- **This lane's own lease**, done or abandoned → `{{frit}} release`.
  It ends the lease from this lane's own token; a plan nobody holds is
  a no-op, not a refusal.
- **A fenced lane** — the lease moved under it, a stranger's claim now
  covers this worktree → `{{frit}} yield`. It parks the local
  divergence to a rescue ref, then tears the worktree down through
  herdr, so nothing local is lost.
- **Many orphans at once** → `{{frit}} reap` tears down everything
  `orphans` reports; dry-run unless `--go`, and it parks each branch's
  unlanded work to a rescue ref before deleting.
- **Work already landed** → neither verb; `orphans` flags it as landed,
  nothing to act on.
- **A deserted row**, acted on from outside its lane — herdr confirms
  the session gone, none can resume it → `{{frit}} yield <id>` parks
  any unparked suffix, then `{{frit}} claim <id>` (or `start <id>`)
  takes it over at the next epoch. Both refuse until yield has parked
  it.

## Notes

- Never hand-run git for teardown: `git worktree remove` and `git
  branch -D` throw away the divergence a rescue ref keeps, and neither
  touches the claim ref — the hold outlives the worktree.
- `{{frit}} board --json` shows who holds what, for the wider picture.
- A foreign hold is `yield`'s job; `release` only ends this lane's own
  lease.
