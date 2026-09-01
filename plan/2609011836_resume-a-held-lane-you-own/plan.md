---
id: 2609011836
title: Resume a held lane you own without waiting the takeover window
status: "🔳"
summary: >-
  A lane whose pane was closed but is otherwise healthy — worktree
  present, tree clean, branch pushed, held: true, no live agent — cannot
  be resumed through any rung. open (rung 1) names start; start (rung 3)
  refuses "already held … not takeable until the window matures"; nudge
  (rung 2) refuses a lane with no live agent. The window never matures:
  the staleness sample restarts, so it reads "seen unchanged for 0s". The
  only escape is to drive the multiplexer by hand, throwing away the
  composed escalation start exists to give. start's refusal is about
  takeover — wresting a plan from another holder — and a hold whose
  holder is this host with no live agent is not a takeover: nothing is
  taken. The one bypass, the self-resume, fires only from inside the
  lane's worktree with a matching persisted token. Teach the resume path
  a second entry: a held plan whose remote hold's holder is this host,
  which herdr confirms carries no live agent, resumes rather than
  refuses. Reattach re-acquires on the deterministic branch and stands an
  agent back up, preserving the worktree's commits. A foreign holder
  still waits the window; a live agent still vetoes. Closes #122.
model: opus
depends-on: []
---
# Resume a held lane you own without waiting the takeover window

## Goal

A held plan whose hold this host owns, with no live agent on it, resumes
through `frit start`. The plan re-acquires on its deterministic branch
and an agent is stood back up. It is no longer refused as an un-matured
takeover. This breaks the [issue #122][issue] deadlock, where `open`
sends you to `start` and `start` refuses on the fact `open` reported.

[issue]: https://github.com/jeduden/frit/issues/122

## Context

**The confirmed deadlock.** A lane whose pane was closed, but which is
otherwise healthy — worktree present, tree clean, branch pushed,
`held: true`, `stale: false`, `dead: false`, `agent: ""` — cannot be
resumed through any rung. `open` (rung 1) prints "no live lane … start
it with frit start". `start` (rung 3) answers "already held … not
takeable until the window matures". `nudge` (rung 2) refuses a lane with
no live agent. So `open` names `start` as the remedy and `start` refuses
on the fact `open` reported.

**Why the refusal is the wrong concept.** `claimRefusal` in
[cmd/frit/claim.go](../../cmd/frit/claim.go) refuses any held plan
outside the ready set with "already held … not matured", and a held lane
is never ready. That refusal guards *takeover* — wresting a plan from
another holder — and is right to be careful. But a hold whose holder is
this host, with no live agent, is not a takeover: nothing is being taken
from anyone. Resuming your own unattached lane and seizing someone
else's are different acts; only the second should wait the window. The
window also cannot mature — the staleness sample restarts, so the
refusal reports "seen unchanged for 0s" — so waiting is not even a real
option.

**What the one bypass covers, and the gap it leaves.** `start` already
models a resume: `startResumeTip` in
[cmd/frit/start.go](../../cmd/frit/start.go) resolves the lane's own
lease from a persisted token ahead of the "already held" refusal, and
`startAcquire` re-acquires it through `claim.Resume`. But `resumeToken`
is cwd-derived — it fires only from *inside* the lane's worktree with a
token still on disk. A closed pane leaves you outside the lane, or the
token gone, and the bypass never triggers. So the healthy held lane you
own falls through to the takeover refusal.

**Reuse first, and where the fix goes.** The remedy is a second entry
into the resume path, not a new transition. `SessionLive` / `SessionDead`
in [internal/herdr/session.go](../../internal/herdr/session.go) already
answer "is a live agent on this lease" — the takeover veto's own read.
`remoteHolder` and `ReadMarker` in
[internal/claim/lease.go](../../internal/claim/lease.go) read the hold's
`Holder` off its marker, and `claim.Resume` / `RenewToBind` re-acquire
and re-bind. The fix teaches `start` that a held plan whose remote hold's
`Holder` is this host, and which herdr confirms has no live agent,
resumes — re-acquiring on the deterministic branch and standing an agent
back up — rather than refusing. Reattach preserves the worktree's
commits: it puts an agent back, it does not park or reset.

**Out of scope.** Seizing another host's lane is unchanged: a foreign
holder still waits the takeover window, and a live agent still vetoes.
The orphaned-worktree case (#118, fixed by #120) where nothing is
healthy is a different verb. Standing an agent back onto an existing
worktree from *outside* the lane — the herdr mechanic that avoids a
`worktree.create` collision — is the reattach stand-up a later phase
shapes once Phase 1 pins the resume decision under test.

## Tasks

1. Phase 1 (proving slice): the resume decision. A held plan whose remote
   hold's holder is this host, herdr-confirmed with no live agent, enters
   `start`'s resume path instead of the "already held … not matured"
   refusal; a foreign holder still refuses, and a live agent still
   vetoes. Driven red at the cmd level against the lease fixtures and the
   herdr fake the dispatch tests already use.
2. Later phases, shaped by Phase 1's handoff: the reattach stand-up
   (put an agent back on the existing worktree from outside the lane
   without a `worktree.create` collision), and `open` naming the resume
   rung rather than a `start` that will refuse.

## Execution

| Phase | Title                                                           | Tier | Gate                                                                                                                                                                         |
| ----- | --------------------------------------------------------------- | ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | A host-owned lane with no live agent resumes, not refuses       | opus | a this-host hold with no live agent makes `frit start <id>` resume rather than refuse it held; a foreign holder refuses; a live agent vetoes; `go test ./...` green          |
| 2     | Reattach stands the agent up in the lane, not the caller's pane | opus | a from-outside resume opens the checkout the hold records and starts the agent in that pane, never `pane current`; the self-resume still uses its own; `go test ./...` green |

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

| #   | Status | Phase                                                                         |
| --- | ------ | ----------------------------------------------------------------------------- |
| 1   | ✅     | [A host-owned lane with no live agent resumes, not refuses](phase-1.md)       |
| 2   | 🔲     | [Reattach stands the agent up in the lane, not the caller's pane](phase-2.md) |
<?/catalog?>

## Acceptance Criteria

- [x] A held plan whose remote hold's holder is this host, with no live
      agent, resumes through `frit start` rather than being refused as an
      un-matured takeover
- [x] The resume re-acquires on the deterministic branch and preserves
      the worktree's commits — it does not park or reset
- [ ] The reattach stands the agent up in the checkout the hold records,
      never in the pane the caller happens to be standing in
- [x] A plan held by another host still waits the takeover window
- [x] A plan whose hold carries a live agent is still vetoed, never
      resumed over
- [x] Liveness that cannot be confirmed dead does not resume — the guard
      fails safe toward the window
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
