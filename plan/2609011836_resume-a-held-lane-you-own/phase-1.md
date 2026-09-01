---
n: 1
title: A host-owned lane with no live agent resumes, not refuses
status: "✅"
result: false
---
Pin the resume decision under test, the load-bearing change #122 turns
on. A held plan whose remote hold's holder is this host, herdr-confirmed
with no live agent, enters `start`'s resume path instead of the takeover
refusal. This is the cmd-level slice; the from-outside reattach stand-up
is a later phase, shaped by this one's handoff.

**Assumes.** `startResumeTip` / `resumeToken` in
[cmd/frit/start.go](../../cmd/frit/start.go) resolve a resume ahead of
the "already held" refusal, but only from a cwd-derived token.
`claimRefusal` in [cmd/frit/claim.go](../../cmd/frit/claim.go) returns
"already held … not matured" for a held plan outside the ready set.
`remoteHolder` and `ReadMarker` in
[internal/claim/lease.go](../../internal/claim/lease.go) read the hold's
`Holder` and tip; `claim.Resume` re-acquires from a given tip.
`SessionLive` / `SessionDead` in
[internal/herdr/session.go](../../internal/herdr/session.go) answer the
liveness of the lease's bound session, failing safe to unknown. The
start tests already script a herdr fake and origin-and-clone lease
fixtures.

**Value.** The deadlock breaks: your own unattached lane is no longer
refused on a window that cannot mature. `start` re-acquires it on the
deterministic branch through the resume transition it already owns,
rather than sending you back to `open`. A foreign holder and a live
agent are untouched — only the act that was never a takeover stops being
treated as one.

**RED.** In [cmd/frit/start_test.go](../../cmd/frit/start_test.go),
against the lease fixtures and herdr fake the file already uses.

- `TestStartResumesAHostOwnedLaneWithNoLiveAgent`: script a held hold
  whose marker `Holder` is this host, and a herdr fake reporting no live
  agent on its bound session. Run `start` with no cwd token. Assert the
  doc is not refused with "already held" / "not matured": it enters the
  resume path — `MarkResumed` is set, `claim.Resume` runs from the hold's
  tip — and no takeover window is named.
- `TestStartStillRefusesALaneHeldByAnotherHost`: the marker `Holder` is a
  different host. Assert the "already held … not matured" refusal stands
  and no resume runs — a foreign hold is still a takeover.
- `TestStartStillVetoesAHostOwnedLaneWithALiveAgent`: this host's hold,
  but the herdr fake reports a live agent on the session. Assert `start`
  does not resume — a live lane is vetoed, never reattached over
  (the harm this guard exists to prevent).
- `TestStartWithUnconfirmedLivenessDoesNotResume`: this host's hold, but
  herdr liveness cannot be read. Assert the refusal stands — the guard
  fails safe toward the window rather than resume over a possibly live
  agent.

**GREEN.** In [cmd/frit/start.go](../../cmd/frit/start.go). Give
`buildStart` a resume resolution beside `startResumeTip`: when the plan
is held, its remote hold's `Holder` equals `hostname()`, and
`SessionDead` positively confirms no live agent on the bound session,
resolve the resume tip from the remote hold rather than the cwd token,
so the existing `resumeTip != ""` path re-acquires through `claim.Resume`
and skips the takeover refusal. Leave `startResumeTip`'s token path as
the first, cheaper source; the host-owned read is the fallback when no
token matches.

**Guard the edges.** Only `Holder == hostname()` resumes; a foreign or
unreadable holder keeps the window. Only a *positive* `SessionDead`
resumes; `SessionLive` and unknown both keep the refusal — resuming over
a possibly live agent is the one harm to avoid, so unknown fails safe.
The resume re-acquires only; it does not park or reset, so the
worktree's own commits are preserved. `pick --go` is unaffected: it
ranks ready plans, and a held lane is not ready, so this path is reached
only by an explicit `start <id>`.

**Gate.** With a lease fixture whose hold is this host and a herdr fake
showing no live agent, `frit start <id>` enters the resume path and does
not print "already held … not matured"; a foreign holder still refuses;
a live agent still vetoes; unconfirmed liveness still refuses.
`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are green.
