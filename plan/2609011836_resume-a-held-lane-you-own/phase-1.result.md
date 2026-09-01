---
n: 1
title: A lane whose token this machine holds resumes from outside, not refuses
status: "✅"
result: true
summary: >-
  A held lane whose marker-recorded checkout carries a token matching
  the hold, with herdr showing no agent on it, resumes from outside the
  lane on that token; the holder string is never consulted, no token
  waits the window, and a live agent — bound or sitting in the lane —
  still vetoes.
---
## Handoff

Reworked 2026-09-01. The first landing gated the resume on the marker's
`holder:` equalling this hostname — identity-based self-recognition,
which the lease protocol rejects (A1): a cloned machine or a reused
path yields the same string with no race needed. The plan's own author
caught it. Nothing kept consults the holder string for the decision;
the tests that pinned the holder rule were reworked, not preserved.

The resume decision is pinned on the token. `start` asks one question
— is this lane already ours to pick back up — through one seam, and
two routes answer it with the same proof. The persisted token read
from the lane you stand in still answers first, unchanged. When no such
token answers, the hold's own marker says where the lane is, and the
token persisted in *that* checkout is read and proved against origin's
tip exactly as the in-lane path proves it — the proof is shared code,
not copied. The `lane:` trailer only says where to look. A checkout
with no token gets nothing and the ordinary claim path stays the
arbiter, whatever the holder string says.

The liveness half is read from outside too, and it is wider than a
session check. One pane list must show no live agent on the session
the marker binds and none sitting in the recorded checkout. The second
clause is what closes the state #122 actually reports — `dead: false`,
`agent: ""` — which is a hold whose marker names no session at all: a
fresh `claim` mints none, and only `start`'s bind adds one. No session
can be confirmed gone, but the token needs none, and an empty roster
on the lane is the whole of the liveness question. A lane an agent was
stood up in by hand, never bound, is still occupied by that agent.
Unknown stays unknown: an unreachable herdr keeps the window, since
from outside the lane nothing else vouches for it.

The scenario doc records the resolution: S76's silent dead end is
resolved when the token is on disk, S77 names `start` as the verb that
rebuilds the pane in place, and the Self-resume section carries a dated
paragraph on resolving the token from the marker's lane.

Phase 2's reattach stand-up is unchanged by the rework: a resume
resolved from the marker is still the from-outside one, and it reopens
the recorded checkout and starts the agent in that pane.

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean.
