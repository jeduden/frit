---
n: 2
title: Reattach stands the agent up in the lane, not the caller's pane
status: "✅"
result: true
summary: >-
  A from-outside resume now puts the lane's own checkout back on screen
  and starts its agent in the pane that comes back, renewing unbound so
  the caller's session never lands on the lease; the self-resume from
  inside still drives the pane it stands in.
---
## Handoff

The reattach lands where the work is. A resume resolved from the hold's
marker reopens the checkout that marker records and starts the agent in
the pane herdr hands back for it. `worktree open` is the verb that does
it — herdr had it, frit had not wrapped it — and it is the right shape
for a reattach in a way `worktree create` never was: it sends no base,
because nothing is being dated against anything, and it is idempotent on
a checkout already on screen, so a reattach run from inside the lane
answers with the pane that lane is already in. `pane current` stays the
self-resume's read, and the from-inside test now pins that it reopens
nothing.

Driving it turned up a second read of the caller's pane, one the phase 1
handoff had not named. The resume was stamping the calling pane's herdr
session onto the lease before standing anything up. From inside the lane
that is right — it is the lane's own session. From outside it is a
session on the caller's terminal, and a later takeover asking herdr
about it would be told it is alive and veto forever, on a pane that has
nothing to do with the plan. A reattach now renews unbound and lets the
bind record the agent it is about to start, which is the only session
that was ever true of the lane.

Three edges are pinned. An open that fails is a stand-up failure, not a
lease problem: the renewal already stands, the lane's token proved
the lease, and releasing it because a pane would not come back would
strand the checkout's own commits. The path reopened is the lane the
marker records, never the naming convention's — reopening the
convention's path would stand an agent up beside the commits rather than
on them. And the workspace label is now one string for create and open
both, so a reattached lane reads on screen exactly like a fresh one.

What the plan's Tasks named second needed no code. `open` already sends
you to `frit start <id>`, and that advice was only ever wrong because
`start` refused; phases 1 and 2 make it true. Naming a distinct resume
rung there would also misfire, since the same refusal covers unheld
plans, where start is simply start. The plan text now records that.

One thing this handoff first named as left open is not. It read the
`dead: false`, `agent: ""` state #122 reports as an unbound hold no
session check could ever confirm, and proposed widening the proof. The
phase 1 rework, which replaced the holder-string gate with the token,
did exactly that: the token needs no session, and the liveness read now
asks the pane list whether any agent sits in the recorded checkout. The
reported state resumes.

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean.
