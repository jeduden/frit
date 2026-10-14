---
n: 5
title: a headless start parks a leftover worktree and leaves it for the owner
status: "✅"
result: false
---
A fresh `start --go` that finds a worktree already on `plan/<id>` asks
herdr whether a pane sits in it. If none does, start parks the branch's
unlanded work and removes the worktree. Phase 2 read a missing herdr
as "no pane", so a headless host removed the worktree too. But on a
headless host a lane is run by hand, with no pane, so "no pane" proves
nothing there.

**The owner's decision.** Park, never remove, and say so explicitly.
On a host with no herdr, start parks the leftover's unlanded work to
its rescue ref and leaves the worktree standing. It refuses before
anything is minted, naming four things:

- the worktree's path;
- the rescue ref the work was parked to, or that nothing unlanded
  needed parking;
- that no herdr here can confirm nobody works in the worktree;
- the cleanup once the owner has confirmed it is unused: `frit reap`
  to review, `frit reap --go` to remove, then start again.

With herdr installed, nothing changes: herdr's answer still decides.

**RED.** Unit tests, written failing first. A headless reconcile
parks and keeps the worktree. Its error names the path, the rescue
ref and `frit reap`. An empty park says nothing needed parking.

C23 runs the built frit under a git-only `$PATH`. A released lane's
leftover worktree holds unlanded work. `start 7 --go` fails, naming
the path, the rescue ref and `frit reap`. The worktree still stands,
and origin's `plan/7` is unchanged. `reap --go` then removes the
worktree, and the plan starts.

**GREEN.** The headless flag the execute step already carries reaches
the leftover reconcile. Headless, it parks and returns the explicit
refusal. The live-pane check no longer reads a missing herdr as "no
pane". That reading is gone, and so is the unit test that pinned it.

**BDD coverage.** C23 is command-level: one host, a refusal's wording,
and a local cleanup path. The lease is untouched because the refusal
runs before the mint, so there is no lease-protocol row.

**Gate.** C23 passes and fails with the headless branch removed. `go
test ./...`, the coverage gate and golangci-lint are clean. The
refusal's cleanup line is proved by C23's own `reap --go`.
