---
n: 3
title: yield tears a headless lane down, and the pane verbs say why not
status: "✅"
result: false
---
On a host with no herdr installed, yield still parks a fenced lane's
divergence, but then warns that it cannot read the current pane. The
worktree is left standing. nudge and message refuse with "herdr
unreachable", which misnames a host where herdr is not installed at
all.

**Yield.** With herdr missing, the teardown reads the lane from the
caller's own directory instead of the current pane. It checks that
directory resolves to this plan's lane, as the pane check does today.
Then it removes the worktree with `git worktree remove`, run from the
repository's primary checkout. It never removes the primary checkout
itself: that is the shared clone, and a lane held from there, which
Task 4 brings, must survive. A dirty worktree is not forced. git's
refusal is a warning, and the lane stays standing.

**The pane verbs.** nudge and message refuse with `herdr not
installed; no pane to reach`. That is certain, unlike an unreachable
socket, and they carry no herdr problem for it. An installed herdr
that does not answer keeps "herdr unreachable". open keeps its
honest presence-unknown answer. It sends nothing, and the missing
herdr is already named among its problems.

**RED.** Unit tests come first, failing:

- A fenced lane claimed on a headless host parks, and the yield run
  from inside it removes its worktree.
- The headless teardown leaves the primary checkout standing, and
  leaves a lane of another plan standing.
- nudge and message on a headless host refuse naming herdr not
  installed.

C19 pins the nudge refusal, and C20 the headless yield, which parks
the fenced lane's work and then removes its worktree.

**GREEN.** tearDownLane branches on the missing-herdr probe it already
pays for, through its current-pane call failing as not found. Both
pane verbs gain a not-installed case ahead of the unreachable one.

**BDD coverage.** C19 and C20, both command-level: a refusal's wording,
and a local teardown on one host. The lease side of yield is
unchanged, so there is no `@S<n>` row.

**Docs and skill.** The README's herdr line lists only what still
needs it. `plan-tidy` stops saying yield tears down "through herdr".

**Gate.** C19 and C20 pass. With the teardown branch removed, C20
fails. `go test ./...` and golangci-lint are clean.
