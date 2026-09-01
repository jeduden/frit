---
n: 1
title: open names the honest next step per held-lane kind
status: "🔲"
result: false
---
Break the loop at its entry rung. `open` is the first rung a stuck
operator hits, and it always names `frit start` — even for a lane
`start` will refuse. Teach `open`'s next-step projection the hold's kind
so it names a resume, a wait, or a live holder instead. The `start` and
`nudge` refusal wording is a later phase, shaped by this one's handoff.

**Assumes.** `openNextAction(focused, presenceUnknown, id)` in
[internal/report/dispatch.go](../../internal/report/dispatch.go) is a
pure projection. It returns `frit start <id>` for a laneless,
known-presence plan, unit-tested directly today
(`TestOpenNextActionNamesStartOnlyWhenNoLaneIsLiveAndKnown`). The `open`
command in [cmd/frit/dispatch.go](../../cmd/frit/dispatch.go) already
reads the plan and runs `liveLaneFor`. Plan 2609011836's
`laneTokenResumeTip` / `tokenProves` read the token the marker's
recorded lane persists — the proof this machine holds the lane — and
`laneUnattended` reads whether an agent is live on it. Those are the
reads `open` reuses. `frit start <id>` now resumes a lane whose token
this machine holds with no live agent, so naming it a resume is honest.

**Value.** The stuck operator's first rung stops lying. `open` names the
step that will actually work: a resume for a lane whose token this
machine holds unattended, a wait or take-over for a lane it cannot
prove, and "a live agent is on it" otherwise — never a `frit start` that
refuses. A plan nothing holds is unchanged: `frit start <id>` still
creates its lane.

**RED.** In
[internal/report/dispatch_test.go](../../internal/report/dispatch_test.go),
extending the pure-projection tests already there, and in
[cmd/frit/dispatch_test.go](../../cmd/frit/dispatch_test.go) for the read
that feeds the projection.

- `TestOpenNextActionResumesALaneThisMachineHolds`: kind = the token at
  the marker's recorded lane proves the hold, no live agent. Assert the
  projection names the resume (`frit start <id>`, framed as resume in the
  message), not a bare start recommendation that would refuse.
- `TestOpenNextActionNamesTheWaitForALaneItCannotProve`: kind = no token
  on disk here, no live agent. Assert it does not name `frit start`; it
  names the wait or take-over.
- `TestOpenNextActionNamesTheLiveAgent`: kind = a live agent present.
  Assert it names that a live agent is on it, not a start.
- `TestOpenNextActionStillStartsAnUnheldLanelessPlan`: kind = unheld, no
  lane. Assert it still returns `frit start <id>` — the common path is
  untouched.

**GREEN.** In [internal/report/dispatch.go](../../internal/report/dispatch.go),
give `openNextAction` (and `OpenDoc`'s refresh) the hold kind as an
input, and branch the next action on it. In
[cmd/frit/dispatch.go](../../cmd/frit/dispatch.go), read that kind where
`open` already gathers the plan and its live lane — resolvability via the
same token check `laneTokenResumeTip` runs, liveness off herdr — and pass
it in. Keep the projection pure and its single writer, so the field can
never lag the facts.

**Guard the edges.** Presence unknown keeps withholding a next action, as
`open` already does — an unread herdr must not be named as "no agent".
The kind is read only when the plan is held; an unheld laneless plan
takes the unchanged `frit start` branch. A hold whose token cannot be
resolved falls back to the safe generic wording rather than claiming it
is resumable here.

**Gate.** `frit open` names a resume for a lane whose token this machine
holds unattended, a wait for a lane it cannot prove, and the live agent
otherwise, and never a `frit start` that would refuse; a truly laneless
unheld plan still names `frit start <id>`. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
