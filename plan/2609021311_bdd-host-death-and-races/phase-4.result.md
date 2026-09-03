---
n: 4
title: S32, two same-host start sessions racing, runs for real
status: "✅"
result: true
summary: >-
  S32 drops `@pending` and passes as a real godog scenario driven
  through `frit start --go` twice, over a stateful herdr fake that
  turns a first call's own "worktree create" RPC into a real linked
  worktree and reflects it back as a live, session-less pane. The
  second call's own `startLiveLaneRefusal` reads that pane and refuses,
  naming the lane the first call stood up.
---
## Handoff

**Two corrections to the sketch.** Both are against the phase spec's
own fake sketch, neither against its analysis.

- The spec's fake parsed a `--cwd` argument off "worktree create" as
  the destination path. Tracing `internal/herdr/dispatch.go`'s
  `worktreePane` showed `--cwd` actually carries the *caller's own*
  invocation directory (`repoPath`) — every worktree RPC's own working
  directory, unrelated to where the checkout goes. The real
  destination rides `--path`. The fake now reads that flag by name
  instead of a fixed argument index, immune to the RPC growing more
  flags between it and `--cwd`.
- The spec's fake stood the second lane up with an independent `git
  clone`. That put the wrong path in `RepoName`'s own comparison:
  `RepoName` reads worktree-list entry zero, which for a genuinely
  *linked* worktree is the main checkout's own path, but for an
  independent clone is the clone's own — a directory named
  `<repo>-<slug>`, never matching `p.Repo`. The fake now runs `git
  worktree add` off the scenario's own "atlas" checkout instead,
  mirroring what herdr actually does in production and, as a
  consequence, matching what `liveLaneFor` compares against.

**One point the spec did not anticipate.** Running the scenario
surfaced it: the first attempt to reach `startLiveLaneRefusal` instead
hit `liveHoldRefusal` —
`startRefusal`'s own earlier, reattach-only check, which reads the
*marker's* recorded lane and a *session-bound* liveness read
(`holdKindFor`), refusing with "already held (...); a live agent is on
this lane" before `startLiveLaneRefusal` is ever reached. The fake's
first cut left `agent list` naming a bound session
(`agent_session: {"value": "sess-1"}`), which `standUpLane`'s own
`bindSession` then genuinely stamped onto the marker — making the hold
session-bound and live exactly the way `liveHoldRefusal` is designed
to catch. Dropping the bound session from the fake's `agent list`
response left the marker's own session "-" (never bound, since
`bindSession` returns immediately on an empty session id) — invisible
to the session-based veto, precisely issue #126's own gap — while the
pane itself stays visible to `startLiveLaneRefusal`'s presence-only
read. That is the shape S32 documents: a session-less lease herdr
still shows as live.

**Both maturing and phase order were as specified.** The maturing step
between the two calls (`theHoldsTakeoverWindowHasMatured`, reused
unchanged, seeded off `w.lease.Tip` read back via `claim.RemoteTip`
after the first CLI-driven call) is load-bearing exactly as the spec
said: without it, `claimRefusal`'s own gate refuses the second call
first, with the ordinary "already held" wording, before either
live-lane check runs.

This closes the plan. Every row of S14..S19 and S26..S32 runs for
real; none carries `@pending`. `go test ./...`, `go test
./internal/scenario` (the bijection gate), `go tool
-modfile=tools/go.mod golangci-lint run` and `mdsmith check .` are all
clean.
