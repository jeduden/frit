---
n: 1
title: A lane whose token this machine holds resumes from outside, not refuses
status: "✅"
result: false
---
Pin the resume decision under test, the load-bearing change #122 turns
on. A held plan whose marker-recorded lane carries a token matching the
hold, herdr confirming no agent on that lane, enters `start`'s resume
path instead of the takeover refusal. This is the cmd-level slice; the
from-outside reattach stand-up is phase 2.

**Reopened 2026-09-01.** As first written this phase gated the resume on
the marker's `holder:` equalling this hostname. The lease protocol
rejects that outright: identity strings admit cloned machines and
reused paths as one holder with no race needed (A1); the token is the
identity, and `holder:` / `lane:` are for reporting. The mechanism below
replaces it. The tests it drove are reworked, not kept.

**Assumes.** `startResumeTip` / `resumeToken` in
[cmd/frit/start.go](../../cmd/frit/start.go) and
[cmd/frit/claim.go](../../cmd/frit/claim.go) resolve a resume ahead of
the "already held" refusal, but only from a cwd-derived token: `ownToken`
reads the lane's own git dir, so it fires only from inside the lane.
`claimRefusal` returns "already held … not matured" for a held plan
outside the ready set. `ReadMarker` in
[internal/claim/lease.go](../../internal/claim/lease.go) reads the
hold's `lane:` trailer; `ReadToken` in
[internal/claim/token.go](../../internal/claim/token.go) reads the token
a checkout persisted; `OwnAdvance` accepts origin's tip as the lane's
own advance beyond it. `herdr.List` answers which panes carry an agent
and where each sits. The start tests already script a herdr fake and
origin-and-clone lease fixtures.

**Value.** The deadlock breaks: your own unattached lane is no longer
refused on a window that cannot mature, and the proof is the one the
protocol already trusts. `start` re-acquires it on the deterministic
branch through the resume transition it already owns. A lane this
machine cannot prove is untouched — only the act that was never a
takeover stops being treated as one.

**RED.** In [cmd/frit/start_test.go](../../cmd/frit/start_test.go),
against fixtures whose lane persists its token.

- `TestStartResumesFromOutsideOnTheTokenTheMarkerLocates`: a held hold
  recording a lane whose token matches it, bound session confirmed
  gone. Run `start` from outside. Assert no "already held" refusal, the
  resume beat CASed from the hold's tip under the same epoch, naming
  the recorded lane.
- `TestStartResumesAnUnboundHoldOnItsToken`: the #122 state exactly —
  no session on the marker, empty herdr roster. The token needs no
  session; it resumes.
- `TestStartDoesNotResumeALaneWhoseTokenIsGone`: `holder:` equals this
  hostname, token removed. The refusal stands — a string proves nothing
  (A1).
- `TestStartResumesWhateverTheHolderStringSays`: `holder:` names
  another machine, token on disk (S48). It resumes, never seized.
- `TestStartStillRefusesAForeignHoldWithNoToken` and
  `TestStartTakesOverAForeignHoldWithNoToken`: no token, unbound or
  dead — the window and the takeover transition are untouched.
- `TestStartStillVetoesALaneWithALiveAgent`,
  `TestStartDoesNotResumeOverAnAgentSittingInTheLane`,
  `TestStartWithUnconfirmedLivenessDoesNotResume`: a live bound session,
  an unbound agent in the checkout, and an unreachable herdr all keep
  the refusal.

**GREEN.** In [cmd/frit/start.go](../../cmd/frit/start.go), replace the
holder-gated resolve with one that reads the marker's `lane:`, reads the
token persisted in that checkout, proves it against origin's tip the
way `ownToken` does — shared, not copied — and then asks herdr, in one
pane list, whether the bound session is live or any agent sits in the
lane. Only a herdr that answered, and showed neither, resumes. Leave
`startResumeTip`'s in-lane path as the first source. Record the
resolved dead end in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md):
S76, S77 and the Self-resume section, dated.

**Guard the edges.** Nothing kept gates the decision on `holder:`. No
token, no resume — from outside, unknown liveness fails safe toward the
window. A hold recording no lane is not resumed: `-` is not a path.
`pick --go` ranks ready plans, and a held lane is not ready, so this is
reached only by an explicit `start <id>`.

**Gate.** With a token-bearing lane and no agent on it, `frit start
<id>` from outside resumes and prints no "already held … not matured";
no token refuses whatever the holder string; a live agent, bound or in
the lane, vetoes; an unreachable herdr refuses. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
