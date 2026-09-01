---
id: 2609011611
title: Binding a lane's session renews from the work ref's current tip
status: "🔳"
summary: >-
  A fresh pick --go or start --go mints the lease, stands the lane up,
  then binds the herdr session onto the lease so a later takeover can
  ask herdr whether the holder is alive. The bind renews from the tip
  the mint returned — but the lease work ref and the lane's own working
  branch are one ref, so the lane advances it the moment the agent
  starts, before the bind's CAS runs. The renewal then loses its
  force-with-lease baseline, and casPush reads the ref back to a marker
  whose holder is this same machine and reports it as a foreign fence,
  telling the user to run yield — which would tear a healthy running
  lane down. The session is never stamped on the lease, so the herdr
  liveness veto is silently forgone. Fix it where the baseline is
  chosen: the bind renews from the ref's current tip, guarded to our own
  hold, so a ref the lane advanced is stamped rather than self-fenced,
  and a genuine foreign move still warns. Closes the bind self-fence.
model: opus
depends-on: []
---
# Binding a lane's session renews from the work ref's current tip

## Goal

A fresh `frit pick --go` or `frit start --go` binds the herdr session
onto the lease even though the lane has already advanced the shared work
ref: the bind renews from the ref's current tip, guarded to our own
hold, rather than from the stale mint tip that self-fences. The session
trailer lands on the lease, so a later takeover can consult herdr; a
genuine foreign move still warns. This closes the bind self-fence.

## Context

**The confirmed failure.** `startExecute` in
[cmd/frit/start.go](../../cmd/frit/start.go) mints the lease
(`startAcquire`), stands the lane up (`standUpLane`), then binds the
agent's herdr session (`bindSession`), which calls `claim.Renew` to CAS
a beat marker carrying the session onto the work ref. On a fresh
dispatch that renewal came back `fenced: the work ref … was moved by
je-framework; run yield`. The lease marker on the remote carries
`session: -` and no beat rides above it, so the bind's push never
landed. The mover the fence names is this same machine, and `run yield`
against a healthy, working lane would tear it down.

**Why the baseline goes stale.** `bindSession` renews from
`lease.Tip` — the tip `startAcquire` returned before the lane stood up.
But the lease work ref is `refs/heads/plan/<id>`, the very branch the
lane's worktree checks out and commits on: the lease's markers and the
agent's work share one ref. The bind needs the herdr session id, which
only exists once `standUpLane` has dispatched the pane, so the bind
cannot run before the lane starts — and by the time it does, the agent
has begun advancing the shared ref. `claim.Renew`'s `casPush` in
[internal/claim/lease.go](../../internal/claim/lease.go) then CASes with
`--force-with-lease` against the pre-stand-up `lease.Tip`, loses,
re-reads the ref, and — finding a marker whose holder is ours — returns
`FenceError` naming this machine. A CAS lost to our own hold is not a
foreign fence; the baseline was simply stale.

**Reuse first, and where the fix goes.** The remedy is to choose the
bind's baseline from the ref as it is now, not as it was at mint.
`remoteHolder` and `ReadMarker` already read the work ref's current tip
and its marker, and `claim.Renew` already renews from any given tip, so
the bind renews from the current tip once it has confirmed that tip
still carries our own hold — same holder, and the lane's own path on the
marker's `lane:` trailer, the identity check `orphans`/`reap` already
trust. A current tip under a foreign holder is a real fence and keeps
its warning. A separate long-lived work ref for the lease is rejected
deliberately: the shared ref is the model the whole protocol is built
on, and splitting it is a far larger change than teaching one renewal to
read the tip it is renewing.

**Out of scope.** The `bindSession` warning-not-abort contract stays: a
bind that still fails after reading the current tip — a genuine foreign
takeover in the window — remains a warning, never an abort, since the
lane is up and the lease is valid. The board/`--json` rendering and the
takeover/veto path are untouched.

## Tasks

1. Phase 1 (proving slice): a claim-atom reconcile — renewing the lease
   to stamp a session reads the work ref's current tip and renews from
   it when that tip still carries our own hold, so a ref advanced since
   the mint is stamped rather than self-fenced; a foreign tip still
   fences. Driven red at the lease unit level with the fake runner.
2. Phase 2: `bindSession` uses that reconcile, so a fresh `pick
   --go`/`start --go` whose lane advanced the shared ref before the bind
   stamps the session and emits no `fenced` problem — proven at the cmd
   level; a genuine foreign move still warns.

## Execution

| Phase | Title                                                               | Tier   | Gate                                                                                                                                                               |
| ----- | ------------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | A session renewal reads the work ref's current tip, guarded to self | opus   | with the remote ahead of the mint tip under our own holder, the renewal reconciles and stamps the session; a foreign tip still fences; `go test ./...` green       |
| 2     | bindSession stamps the session instead of self-fencing              | sonnet | a fresh dispatch whose ref advanced before the bind emits no `fenced` problem and the lease carries the session; a foreign move still warns; `go test ./...` green |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "!phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row: "| {n} | {status} | [{title}](phase-{n}.md) |"
footer: |

?>

| #   | Status | Phase                                                                             |
| --- | ------ | --------------------------------------------------------------------------------- |
| 1   | ✅     | [A session renewal reads the work ref's current tip, guarded to self](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] A fresh `frit pick --go` / `frit start --go` binds the herdr
      session onto the lease even when the lane has advanced the shared
      work ref before the bind — no `fenced` problem is emitted
- [ ] The lease's work ref carries the bound session trailer after a
      dispatch, so a later takeover can consult herdr
- [ ] A renewal that loses to a genuinely foreign holder still fences
      and the bind still warns rather than aborts
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
