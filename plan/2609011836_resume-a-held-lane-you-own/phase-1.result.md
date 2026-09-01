---
n: 1
title: A host-owned lane with no live agent resumes, not refuses
status: "✅"
result: true
summary: >-
  A held lane whose hold this host owns, herdr-confirmed with no live
  agent, now enters start's resume path from the hold's own marker
  rather than the takeover refusal; a foreign holder, a live agent and
  unreadable liveness all keep the window.
---
## Handoff

The resume decision is pinned. `start` now asks two proofs of the same
question — is this lane already ours to pick back up — through one seam,
`startResume`. The persisted token still answers first, unchanged. When
no token answers, the hold's own marker does: holder equal to this
host's, a lane recorded, and herdr positively confirming no agent on the
bound session. Anything else returns nothing and the ordinary claim path
stays the arbiter.

The red that drove it was not the refusal the issue reports. With a
bound session herdr can see is gone, the fleet already reads the hold as
dead, which makes it ready — so `start` was *seizing this host's own
lane*, minting a takeover at epoch 2 and recording the naming
convention's path as the lane. That is the behaviour the first test
overturns: the same fixture now beats from the hold's tip under the same
epoch, naming the lane the hold already records.

Worth knowing for whatever comes next: the exact state issue #122
reports — `dead: false`, `agent: ""` — cannot be reached through this
guard, and no test can construct it. `deadSession` and `SessionDead`
read the same marker and the same pane list, so "the hold is not dead"
and "the session is confirmed dead" are the same question answered
twice; they cannot disagree within one run. A hold reads `dead: false`
with no agent on it only when its marker names no session at all, and an
unbound session can never be positively confirmed gone. So the deadlock
as reported is not closed by the resume decision alone. Either the
liveness proof has to widen past a bound session — a lane's own checkout
and token are evidence of ownership that no session id carries — or the
refusal itself has to learn to distinguish "held by me, unattended" from
"held by another", which is the issue's own third suggestion.

Three edges came out of driving it, all now pinned by tests. A foreign
holder is untouched in both shapes: an unbound one still waits the
window, and one whose session is confirmed dead is still seized through
the takeover transition rather than resumed. Unknown liveness is not
death — an unreachable herdr keeps the refusal. And a hold recording no
lane is not resumed at all: `leaseMessage` writes `-` where a lane-less
lease has no path, and renewing on that would stamp `-` into the very
trailer `orphans` and `reap` read to tell a real checkout from a foreign
one.

What phase 2 inherits, and the one thing to fix before this is usable
from outside the lane: the resume path's stand-up still drives
`pane current`. Inside the lane that is right — it is the pane you are
standing in. From outside it is the *caller's* pane, so an agent would
be started in the wrong directory. The decision now reaches that
stand-up from outside for the first time, which is exactly the
`worktree.create` collision the plan's second task names. Until it is
shaped, treat the from-outside resume as reaching the right lease, not
the right pane.

The plan stays 🔳, not ✅, though every Acceptance Criterion listed is
met: the criteria all speak to the resume *decision*, and Tasks item 2 —
the reattach stand-up, and `open` naming the rung rather than a `start`
that refuses — has no phase file yet.

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean. `plan-drive`'s rung 3 names the resume, in
the canonical asset and the dogfood copy both.
