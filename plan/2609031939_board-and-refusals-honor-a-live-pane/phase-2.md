---
n: 2
title: the deserted refusals name the pane and lead with resume
status: "✅"
result: false
---
Make the deserted-hold refusals answer to the live pane: when a pane
attends the lane, name that pane and lead with resuming it, not with a
`frit yield` teardown. This is the refusal wording that pointed a
reader at a teardown while a pane was open.

**Assumes.** [`desertedRefusal`](../../cmd/frit/start.go) and
[`parkFirstRefusal`](../../cmd/frit/start.go) both fire on `plan.Held
&& plan.Dead && !plan.Stale` and each returns a "deserted hold: … run
`frit yield <id>`" string. Neither consults whether a pane attends the
lane. [`liveLaneFor`](../../cmd/frit/dispatch.go) finds that pane and
carries its `PaneID`, and [`liveLaneRefusal`](../../cmd/frit/start.go)
already turns a found pane into wording that names it. Phase 1's
handoff says how the live-pane fact is best read at these sites. An
explicit `start <id>` still reaches these refusals even after plan
2609031211 makes `pick --go`'s walk advance past them, so the wording
still matters for the operator who names the lane.

**Value.** The refusal stops sending a reader toward a teardown while
a pane is live. A lane idling between phases is described as what it is
— attended, resumable in its open pane — with `frit yield` demoted to
the fallback for when the work must actually be set aside.

**RED.** Add a unit test to
[cmd/frit/start_test.go](../../cmd/frit/start_test.go). Build the
deserted-hold fixture the existing refusal tests use — held, bound
session gone, unmatured window, an unparked suffix on its own lane —
but with herdr showing a live *working* pane on the branch (the
`liveLeaseFixture` + `withHerdr` shape). Run an explicit `start <id>`.
Assert the refusal names the pane (its `PaneID`) and does not lead with
`frit yield`. This fails today: the refusal is the fixed "deserted
hold … run `frit yield`" string, pane or no pane. Commit the red.

The refusal gates on `plan.Held && plan.Dead && !plan.Stale` and never
reads `agent_status`. A live pane is a live pane whether it works or
idles, so the working case is representative. Add an idle-pane case as
a cheap guard that the reconciliation stays status-agnostic. Then no
future change can accidentally key the refusal on `agent_status`.

**GREEN.** Fold the live-pane read into `desertedRefusal` and
`parkFirstRefusal` (or a shared helper Phase 1 introduced). When a pane
attends the lane, return wording that names the pane and leads with
resuming it — reuse `liveLaneRefusal`'s pane-naming rather than writing
a third variant — keeping `frit yield` only as the trailing fallback.
When no pane attends, return the existing deserted/park-first wording
unchanged.

**Guard the edges.** A deserted lane with **no** live pane still gets
the exact "deserted hold … run `frit yield <id>`" text it gets today —
the wording and the remedy are unchanged there, so the guarantee for a
truly gone lane does not regress. These must still pass, and the #126
live-lane refusal stays untouched:

- `TestStartGoRefusalOfALiveLaneCarriesInJSON`
- `TestLiveLaneRefusalNamesThePaneAndItsBranch`

Every changed function keeps its dedicated unit test.

**Gate.** The new attended-refusal test passes; the no-pane deserted
refusal is unchanged; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-2.result.md`. Record the final wording for
the attended case and which refusal sites now read the live pane, so
Phase 3's scenario asserts the exact observable — the pane named, the
yield demoted.
