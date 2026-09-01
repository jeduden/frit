---
id: 2609011836
title: Resume a held lane you own without waiting the takeover window
status: "✅"
summary: >-
  A lane whose pane was closed but is otherwise healthy — worktree
  present, tree clean, branch pushed, held: true, no live agent — cannot
  be resumed through any rung. open (rung 1) names start; start (rung 3)
  refuses "already held … not takeable until the window matures"; nudge
  (rung 2) refuses a lane with no live agent. The window never matures:
  the staleness sample restarts, so it reads "seen unchanged for 0s". The
  only escape is to drive the multiplexer by hand, throwing away the
  composed escalation start exists to give. start's refusal is about
  takeover — wresting a plan from another holder — and resuming a lane
  whose lease this machine can prove is not a takeover: nothing is
  taken. The proof already exists: the token the lane persisted in its
  own git dir, which the self-resume checks — but only from inside the
  lane, since it is cwd-derived. Teach start to find that token from
  outside: the hold's marker records the lane's path, the token there
  is the proof, and herdr confirming no agent on the lane is the
  liveness check. Reattach re-acquires on the deterministic branch and
  stands an agent back up, preserving the worktree's commits. A lane
  with no token waits the window whatever its holder string says; a
  live agent still vetoes. Closes #122.
model: opus
depends-on: []
---
# Resume a held lane you own without waiting the takeover window

## Goal

A held plan whose lane still carries its lease token, with no live agent
on it, resumes through `frit start` from outside the lane. The plan
re-acquires on its deterministic branch and an agent is stood back up.
It is no longer refused as an un-matured takeover. This breaks the
[issue #122][issue] deadlock, where `open` sends you to `start` and
`start` refuses on the fact `open` reported.

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
another holder — and is right to be careful. But a lease this machine
can prove is its own, with no live agent on it, is not a takeover:
nothing is being taken from anyone. Resuming your own unattached lane
and seizing someone else's are different acts; only the second should
wait the window. The window also cannot mature — the staleness sample
restarts, so the refusal reports "seen unchanged for 0s" — so waiting is
not even a real option.

**What proves a lane is yours.** Not the marker's `holder:` string. The
[lease protocol](../../docs/research/lease-protocol.md) rejects
identity-based self-recognition outright: a cloned machine or a reused
path produces the same host string with no race needed (A1), so the
`holder:` and `lane:` trailers are for reporting, never for passing a
check. The token is the identity — the tip the lane persisted in its
own git dir when it last won a transition. The self-resume section
already states the rule this plan needs: a lane whose token matches
origin's tip, when herdr confirms no live session owns it, resumes with
no window; a lane that lost its token waits like any other claimant.

**What the one bypass covers, and the gap it leaves.** `start` already
models a resume: `startResumeTip` in
[cmd/frit/start.go](../../cmd/frit/start.go) resolves the lane's own
lease from a persisted token ahead of the "already held" refusal, and
`startAcquire` re-acquires it through `claim.Resume`. But `resumeToken`
is cwd-derived — it fires only from *inside* the lane's worktree with a
token still on disk. A closed pane leaves you outside the lane, or the
token gone, and the bypass never triggers. So the healthy held lane you
own falls through to the takeover refusal.

**Reuse first, and where the fix goes.** The remedy is a second way to
*find* the token, not a second proof. `ReadMarker` in
[internal/claim/lease.go](../../internal/claim/lease.go) reads the
hold's `lane:` trailer — the checkout's recorded path, which `orphans`
and `reap` already trust to locate the real worktree. `ReadToken` in
[internal/claim/token.go](../../internal/claim/token.go) reads the
token that checkout persisted, and `OwnAdvance` accepts a tip that is
the lane's own advance beyond it. `SessionLive` in
[internal/herdr/session.go](../../internal/herdr/session.go) is the
veto, and the pane list says whether any agent sits in the checkout at
all. The fix teaches `start` to resolve the lane from the marker, prove
it by the token found there, confirm through herdr that no agent is on
it, and resume through `claim.Resume` — re-acquiring on the
deterministic branch and standing an agent back up. The `lane:` trailer
only says where to look; the token is what passes the check. Reattach
preserves the worktree's commits: it puts an agent back, it does not
park or reset.

**Out of scope.** Seizing a lane this machine cannot prove is
unchanged: a hold whose lane carries no token here still waits the
takeover window, whatever its holder string says, and a live agent still
vetoes.
The orphaned-worktree case (#118, fixed by #120) where nothing is
healthy is a different verb. Standing an agent back onto an existing
worktree from *outside* the lane — the herdr mechanic that avoids a
`worktree.create` collision — is the reattach stand-up a later phase
shapes once Phase 1 pins the resume decision under test.

## Tasks

1. Phase 1 (proving slice): the resume decision. A held plan whose
   marker-recorded lane carries a token matching the hold, herdr
   confirming no agent on that lane, enters `start`'s resume path
   instead of the "already held … not matured" refusal; a lane with no
   token still refuses whatever its holder string says, and a live
   agent still vetoes. Driven red at the cmd level against the lease
   fixtures and the herdr fake the dispatch tests already use. The
   scenario doc records the resolved dead end (S76, S77, Self-resume).
2. Phase 2: the reattach stand-up. An agent goes back onto the existing
   worktree from outside the lane, through herdr's `worktree open`
   rather than the `worktree.create` that path would collide with, and
   the resume renews unbound so the caller's own session never lands on
   the lease. `open` naming a separate resume rung turned out to need no
   code: it already sends you to `frit start <id>`, and phases 1 and 2
   are what make that advice true.

## Execution

| Phase | Title                                                                   | Tier | Gate                                                                                                                                                                         |
| ----- | ----------------------------------------------------------------------- | ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | A lane whose token this machine holds resumes from outside, not refuses | opus | a recorded lane carrying its token, no agent on it, makes `frit start <id>` resume, not refuse; no token refuses whatever the holder; a live agent vetoes; tests green       |
| 2     | Reattach stands the agent up in the lane, not the caller's pane         | opus | a from-outside resume opens the checkout the hold records and starts the agent in that pane, never `pane current`; the self-resume still uses its own; `go test ./...` green |

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

| #   | Status | Phase                                                                                 |
| --- | ------ | ------------------------------------------------------------------------------------- |
| 1   | ✅     | [A lane whose token this machine holds resumes from outside, not refuses](phase-1.md) |
| 2   | ✅     | [Reattach stands the agent up in the lane, not the caller's pane](phase-2.md)         |
<?/catalog?>

## Acceptance Criteria

- [x] A held plan whose marker-recorded lane carries a token matching
      the hold, with no agent on the lane, resumes through `frit start`
      from outside the lane rather than being refused as an un-matured
      takeover — the token is the proof, never the holder string
- [x] The resume re-acquires on the deterministic branch and preserves
      the worktree's commits — it does not park or reset
- [x] The reattach stands the agent up in the checkout the hold records,
      never in the pane the caller happens to be standing in
- [x] A hold whose lane carries no token on this machine still waits
      the takeover window, whatever its holder string says
- [x] A plan with a live agent — on its bound session, or sitting in
      its lane unbound — is still vetoed, never resumed over
- [x] Liveness herdr cannot report does not resume from outside — the
      guard fails safe toward the window
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
