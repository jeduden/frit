---
n: 5
title: a headless start parks a leftover worktree and leaves it for the owner
status: "✅"
result: true
summary: >-
  On a host with no herdr, a fresh start that meets a leftover
  worktree on the plan's branch parks its unlanded work and leaves the
  worktree standing, refusing before any mint with the path, the
  rescue ref and the frit reap cleanup. C23 proves it end to end.
---
# Phase 5 result

## Handoff

The owner chose to park and never remove. On a host with no herdr, a
fresh `start --go` that meets a worktree already on `plan/<id>` parks
the branch's unlanded work to its rescue ref. It leaves the worktree
standing and refuses before anything is minted. The refusal is
explicit. It names the path and the rescue ref, or says nothing
unlanded needed parking. It says no herdr here can confirm nobody
works in the worktree. And it names the cleanup: `frit reap` to
review and `frit reap --go` to remove, then start again. With herdr
installed, herdr's answer still decides. Phase 2's reading of a
missing herdr as "no pane" is gone.

**Verified by.** Unit tests, written failing first, for the headless
reconcile, the empty-park wording, a failed park, and start itself.
C23 is a command scenario run through godog against the built frit
under a git-only `$PATH`. It sets up a released lane's leftover
holding unlanded work. The start fails, naming the path, the rescue
ref and `frit reap --go`. The worktree still stands, and origin's
`plan/7` is unchanged. Then `reap --go` removes the worktree. C23
fails with the headless branch disabled.

**Like its sibling refusals,** the live pane and the unanswering herdr,
this one is an error on stderr with a non-zero exit, not a refusal in
the start document.
