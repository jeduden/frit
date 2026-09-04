---
n: 2
title: the deserted refusals name the pane and lead with resume
status: "✅"
result: true
summary: >-
  desertedRefusal and parkFirstRefusal now read the same liveLaneFor a
  fresh acquire's pre-flight uses, once their own gate already decided
  a hold is deserted or unparked. Attended, the refusal names the pane
  and leads with `frit open <id>`, keeping `frit yield <id>` only as a
  trailing fallback; unattended, both refusals render exactly as
  before.
---
## Handoff

Landed as scoped, reusing `liveLaneFor` and `liveLaneRefusal`'s naming
rather than a third read or a third wording variant.

**Where the read landed.** `desertedRefusal` and `parkFirstRefusal`
(`cmd/frit/start.go`) each call `liveLaneFor(c, plan, rt)` themselves,
but only after their own gate already fires — held, dead, not stale,
and (respectively) in this exact lane or carrying an unparked suffix.
An ordinary start, or one that never reaches the deserted/park-first
case, pays no extra herdr read: the call sits behind the same early
returns that already made these functions cheap for the common case.
Both functions' signatures grew to
`(c *cli, rt *runtime, ...) (string, []hostProblem, error)`, so their
own presence read's problems ride into the eventual refusal doc via
`carryLiveLaneProblems` — widened from `*report.StartDoc` to the
existing `problemAdder` interface so `claim.go`, which shares both
refusal functions and their live-pane read, carries the same problems
onto `ClaimDoc` rather than a duplicate.

**The wording.** A new `resumeRefusal(plan, lane)` renders "deserted
hold: a live herdr pane (<pane>) on lane <branch> attends it; resume
it with `frit open <id>` — run `frit yield <id>` only to set the work
aside instead" — resume named first, yield demoted to the explicit
fallback. It reuses a new `paneNaming(lane)` helper for the
"(<pane>) on lane <branch>" clause, the same naming
`liveLaneRefusal` already renders for the unrelated live-but-unbound
refusal (#126). `liveLaneRefusal` itself is untouched — its exact
wording ("already sits on lane") is pinned by the S32 cross-layer
scenario, so `paneNaming` is a shared phrase, not a shared function
body, between the two call sites.

**One correction found by the suite, not planning.** My first pass
refactored `liveLaneRefusal` to build its sentence from `paneNaming`
too, which silently reworded "already sits on lane" — S32
(`features/races.feature`) pins that exact substring and caught it
immediately on `go test ./...`. Fixed by leaving `liveLaneRefusal`'s
own string literal alone and only using `paneNaming` for the new
`resumeRefusal`; the full suite and the godog run are both green
after.

**Guard the edges, confirmed.** `TestStartNamesYieldForADesertedLaneOnThisHost`
and `TestStartRefusesAnUnparkedSuffixFromOutsideTheLane` (no live pane)
still pass unchanged, as do the unrelated live-lane tests
`TestStartGoRefusalOfALiveLaneCarriesInJSON` and
`TestLiveLaneRefusalNamesThePaneAndItsBranch`. New tests in
`cmd/frit/deserted_test.go` pin the attended case for both refusal
sites — `TestStartNamesThePaneForADesertedLaneAWorkingPaneAttends`
(the observed working-agent case), its idle-status sibling
`TestStartNamesThePaneForADesertedLaneAnIdlePaneAttends` (a cheap
guard that nothing keys off `agent_status`), and
`TestStartNamesThePaneForAnUnparkedSuffixALivePaneAttends` for
`parkFirstRefusal`'s own fixture — plus
`TestStartGoRefusalOfAnAttendedDesertedLaneCarriesInJSON` for the JSON
contract. `resumeRefusal` and `paneNaming` get their own pure-function
test, `TestResumeRefusalNamesThePaneAndLeadsWithOpen`, in
`cmd/frit/start_test.go`.

**For Phase 3.** The observable to assert under godog: run S32's own
deserted-lane setup (S77's Given — a held, session-bound lease dead,
unmatured, this exact lane) but with a herdr pane live on the branch,
then check `start`'s refusal contains `frit open <id>` ahead of `frit
yield <id>` in the string, the same ordering
`TestResumeRefusalNamesThePaneAndLeadsWithOpen` pins directly. The
existing `startRefusesAndNamesYield` step stays the unattended case's
own check, unchanged; a new step is needed for the attended one.

Verified: `go test ./...` (godog included) and `go tool
-modfile=tools/go.mod golangci-lint run` are both clean; `mdsmith
check .` passes.
