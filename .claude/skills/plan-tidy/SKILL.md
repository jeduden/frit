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

- `go run ./cmd/frit orphans --json` — claims and checkouts that no longer add
  up: claimed but unstaffed, prepared but unstarted, held past its
  takeover window, or gone.
- `go run ./cmd/frit stale --days N --json` — worktrees whose tip has not moved.

## Match the verb

- **This lane's own lease**, done or abandoned → `go run ./cmd/frit release`.
  It ends the lease from this lane's own token; a plan nobody holds is
  a no-op, not a refusal.
- **A fenced lane** — the lease moved under it, a stranger's claim now
  covers this worktree → `go run ./cmd/frit yield`. It parks the local
  divergence to a rescue ref, then tears the worktree down through
  herdr, so nothing local is lost.
- **Many orphans at once** → `go run ./cmd/frit reap` tears down everything
  `orphans` reports; dry-run unless `--go`, and it parks each branch's
  unlanded work to a rescue ref before deleting.
- **Work already landed** → neither verb; `orphans` flags it as landed,
  nothing to act on.
- **A deserted row**, acted on from outside its lane — herdr confirms
  the session gone, none can resume it → `go run ./cmd/frit yield <id>` parks
  any unparked suffix, then `go run ./cmd/frit claim <id>` (or `start <id>`)
  takes it over at the next epoch. Both refuse until yield has parked
  it. First check `go run ./cmd/frit board --json`: a row whose `ask` is
  non-empty still has an agent on it — run that command and wait for
  its answer before yielding.

## Notes

- Never hand-run git for teardown: `git worktree remove` and `git
  branch -D` throw away the divergence a rescue ref keeps, and neither
  touches the claim ref — the hold outlives the worktree.
- `go run ./cmd/frit board --json` shows who holds what, for the wider picture.
- A foreign hold is `yield`'s job; `release` only ends this lane's own
  lease.
