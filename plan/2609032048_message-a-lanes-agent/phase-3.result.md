---
n: 3
title: S91 runs the ask-the-agent state under godog
status: "✅"
result: true
summary: >-
  Cross-layer row S91 — bound session gone, pane still working, work
  unclassifiable — is documented in the matrix and runs for real under
  godog: board and ready carry the ask, start's deserted refusal leads
  with `frit message` ahead of `frit yield`, and `message --go` with the
  refusal's own text reaches the live pane. The plan is complete.
---
## Handoff

**The id moved from S90 to S91.** Plan 2609031951 landed its own S90
between this plan's writing and this phase, so the row, the scenario
tag, the plan's summary, context, task list, Execution row and
acceptance criteria all say S91 now; 2609031951's S90 row and scenario
are untouched, and the scenario-to-matrix bijection is exact.

**The row.** S91 sits after S90 in the cross-layer table of
`docs/research/lease-protocol.md`, mechanism: board and ready carry
`ask` naming `frit message`; start's deserted refusal leads with it,
`frit yield` trailing; `message --go` reaches the pane (RESUME, YIELD).

**The scenario.** `@S91` in `features/cross-layer.feature` reuses S89's
fixture and Given whole — this machine holds plan 7 bound to a session
with its token persisted, a takeover bound to a session at a new epoch
lands on it, and herdr shows a live pane (`wLive:p1`, `claude`,
`working`) on the lane — then pins the route end to end:

- `frit board --json` — plan 7's row is not dead and its `ask` equals
  `frit message 7 "what is your status?"`.
- `frit ready --json` — plan 7's card carries the same ask.
- `start --go` from the lane refuses, naming `frit message` before
  `frit yield`.
- the lane runs that exact ask under `--go`, and the fake herdr records
  `agent prompt wLive:p1 "what is your status?"` with message reporting
  it sent — no busy refusal, since the pane is working.

**What changed in the step file.** The one shared change is that S89's
own Given, `herdr shows a live pane on the lane`, now arms the
recording fake (`recordingHerdr`) and keeps its recorder on the section
state, so S91's last step can prove the send; S89 reads nothing back
from the recording and passes unchanged. S91 adds five steps in its own
registrar, `registerAskTheAgentIdentityAndCrossLayer`, each with the
dedicated precondition-refusal and exact-shape unit tests S89's steps
have. The message text the scenario sends is the `askText` literal,
pinned equal to `report.AskCommand`'s own question by
`TestAskTextIsTheQuestionAskCommandCarries`, so the send proves the
refusal's verbatim remedy and not a paraphrase.

**Verified.** `go test ./cmd/frit -run 'TestFeatures/S91:'` reports
S91 PASS, not SKIP; `go test ./internal/scenario` is green; `go test
./...`, `go tool -modfile=tools/go.mod golangci-lint run` and `mdsmith
check .` are all clean.

**Left open, deliberately.** The plain board's printed ask line is
pinned by its unit test only; S91 reads `--json`, per the JSON Contract
rule that an agent branches on a field, not a rendered line. frit still
has no pull-request awareness: the route ends at the agent.
