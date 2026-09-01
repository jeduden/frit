---
n: 2
title: bindSession stamps the session instead of self-fencing
status: "✅"
result: false
---
Wire the bind to the reconcile Phase 1 built. A fresh dispatch whose
lane advanced the shared work ref before the bind stamps the session
onto the lease instead of warning that this machine fenced itself. A
genuine foreign move still warns.

This file was reconstructed at execution time: the plan's Execution
row and Task 2 named the phase, but no `phase-2.md` was written when
the plan was created. Its spec is those two, plus the handoff Phase 1
left in [phase-1.result.md](phase-1.result.md).

**Assumes.** `claim.RenewToBind` from Phase 1: same signature as
`claim.Renew`, reconciling a CAS lost to this lane's own hold and
returning a foreign fence unchanged. `bindSession` in
[cmd/frit/start.go](../../cmd/frit/start.go) already passes the mint
tip and already treats a failure as a warning, not an abort.

**Value.** The reported failure is closed where a user meets it: `frit
start --go` and `frit pick --go` bind the herdr session onto the lease
even though the agent has begun committing on the shared ref, so a
later takeover can consult herdr instead of falling back to the
staleness window. Nobody is told to run `yield` against a healthy lane.

**RED.** In [cmd/frit/start_test.go](../../cmd/frit/start_test.go),
against the fake herdr the start tests already use. Reading the agent
back is the last call before the bind, so a fake that drives the
repository there lands its commit in exactly the real window.

- `TestStartBindsTheSessionOntoARefTheLaneAlreadyAdvanced`: the lane
  pushes an ordinary work commit — no marker of its own — onto
  `refs/heads/plan/7` while start reads the agent back. Assert no
  `fenced` problem is emitted, the ref's tip is a beat carrying
  `session: sess-1`, and that beat is a child of the lane's commit
  rather than of the mint tip.
- `TestStartWarnsWhenAForeignMoveFencesTheBind`: another machine
  seizes the ref in the same window with a takeover marker under
  `box-b`. Assert the warning names `box-b`, the exit code is still
  zero and the lane is still reported running, and nothing was stamped
  over the mover.

**GREEN.** In [cmd/frit/start.go](../../cmd/frit/start.go): `bindSession`
calls `claim.RenewToBind` where it called `claim.Renew`. The stale mint
tip stays the argument — reconciling it is the atom's job now — and the
warning-not-abort contract is untouched, since what still comes back is
a genuinely foreign fence.

**Guard the edges.** No other caller of `claim.Renew` moves: the
beat-for-holder and resume paths keep the plain renewal, whose fence is
the real thing. `frit pick --go` reaches the same `startExecute`, so it
is fixed by the same line rather than by a second edit.

**Gate.** A fresh dispatch whose ref advanced before the bind emits no
`fenced` problem and the lease carries the session; a foreign move still
warns. `go test ./...` and `go tool -modfile=tools/go.mod golangci-lint
run` are green.

Write the handoff to phase-2.result.md
