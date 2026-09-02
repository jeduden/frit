---
n: 1
title: reap <id> retires one landed leftover worktree, not the fleet
status: "✅"
result: true
summary: >-
  `frit reap <selector>` now narrows the stranded pass to one plan's own
  landed or stranded leftover worktree — the same `Selector` argument
  `yield` and `start` already carry, resolved the same way. `--go` tears
  down only that lane, leaving every other landed worktree standing; the
  dry-run preview names only the selected lane too. The unstaffed-hold
  drop and the rescue-ref prune still sweep the whole fleet regardless of
  a selector — narrowing those is later work. A bare `frit reap` is
  unchanged. An unknown or ambiguous selector refuses before any
  teardown, the same way `yield`'s does.
---
## Handoff

`reapCmd` ([cmd/frit/reap.go](../../cmd/frit/reap.go)) now carries an
optional `Selector` positional. `reapSelectorPlan` resolves it through
the shared `resolveSelector` (same as `yield`, `guardForeign` false — a
read-then-act report, not a lane hand-off) into the one plan it names,
or reports unscoped for an empty selector. `Run` then filters
`found.Stranded` through `strandedForPlan` before handing it to the
unchanged `reapStranded`/`reap.Decide` machinery: a lane survives the
filter only when its repository matches the resolved plan's and its
`PlanID` matches — the same identity `claim.Branch(id)` encodes in a
lane's hold branch, checked directly on the field rather than on
`Holds`, since a genuinely stranded lane's `Holds` is empty by
construction (`lanes.Build` drops a merged, landed, or released ref
before it ever becomes a `Hold`). A selector naming a different
repository, or a plan with no stranded lane here, narrows to nothing
rather than falling through to the whole fleet's stranded set.

`reapUnstaffed` and `reapPruned` are called unfiltered regardless of a
selector — verified by `TestReapSelectorRefusesALivePaneOnTheLane`,
which passes a selector at a live-held plan and gets the exact
unstaffed-hold refusal (`"unchanged for"`) a bare `frit reap` would give,
proving the selector does not accidentally loosen or bypass that guard.
`TestReapNoSelectorStillSweepsTheFleet` pins the empty-selector default
unchanged, and `TestReapUnknownSelectorRefuses` pins the same
`ErrNotFound` ("no plan matches") refusal `yield` gives, before any
teardown runs.

**What the next phase inherits.** Extend the same `plan`/`scoped` values
`reapSelectorPlan` already returns into `reapUnstaffed` (the hold drop)
and `reapPruned` (the landed rescue-ref prune), narrowing each to the
selected plan's own hold branch the same way `strandedForPlan` narrows
the checkout — `reapUnstaffed`'s existing `holds(lane, canonical)` check
is the natural seam. A selector that resolves but touches nothing in any
of the three passes should still report cleanly (nothing to reap for
that plan) rather than read as a failure.

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean.
