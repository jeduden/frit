---
n: 1
title: reap <id> retires one landed leftover worktree, not the fleet
status: "🔲"
result: false
---
Give `reap` the selector its sibling verbs already have, proven on the
case that motivates it: a just-merged plan whose lease is released and
whose leftover worktree is still checked out. `frit reap <id>` retires
that one lane; the fleet's other landed leftovers are left standing. The
unstaffed-hold drop and the rescue-ref prune are later phases, shaped by
this one's handoff.

**Assumes.** `reapCmd` in [cmd/frit/reap.go](../../cmd/frit/reap.go)
carries only `Go bool`; its `Run` walks each repository and feeds the
stranded lanes to `reapStranded`, which parks and removes under `--go`
through `reap.Decide`. `resolveSelector` in
[cmd/frit/main.go](../../cmd/frit/main.go) turns a plan id or slug into
one plan — the resolver `yield` and `start` call — and refuses an unknown
or ambiguous one. `claim.Branch(id)` is a plan's hold branch; a lane's
`Holds` carry it. reap's tests already build worktrees on plan branches
with a herdr fake for the live-pane guard.

**Value.** The landed leftover a merged lane leaves can be retired on
its own, through the frit verb rather than raw git, without the
fleet-wide sweep that would tear down every other landed worktree. The
teardown, its guards, and the dry-run/`--go` gate are exactly reap's
own — only the set is narrowed.

**RED.** In [cmd/frit/reap_test.go](../../cmd/frit/reap_test.go), against
the worktree-and-herdr fixtures already there.

- `TestReapSelectorRetiresOnlyTheNamedLane`: two landed, stranded
  worktrees on different plan branches, neither with a live pane. Run
  `reap <id> --go` for one. Assert its worktree is removed and the other
  is left standing — the fleet-wide reap would have taken both.
- `TestReapSelectorDryRunNamesOnlyThatLane`: no `--go`; assert the report
  lists the selected plan's leftover and not the other's.
- `TestReapSelectorRefusesALivePaneOnTheLane`: the selected lane has a
  live herdr pane. Assert it is refused with the same live-pane reason
  the fleet-wide reap uses, and nothing is removed.
- `TestReapNoSelectorStillSweepsTheFleet`: no argument; assert both
  stranded worktrees are still reaped — the default is unchanged.
- `TestReapUnknownSelectorRefuses`: a selector matching no plan; assert
  the same refusal `resolveSelector` gives `yield`.

**GREEN.** In [cmd/frit/reap.go](../../cmd/frit/reap.go). Add
`Selector string` to `reapCmd`, an optional positional matching
`yieldCmd`'s. In `Run`, when it is set, `resolveSelector` it to one plan
and keep only the stranded lanes whose repository is the plan's and whose
`Holds` carry `claim.Branch(plan.ID)`, before the existing
`reapStranded` call; when it is empty, pass the lanes unfiltered as
today. The filter is a `lanes.Lane` predicate — repo plus hold branch,
the identity `orphans`/`reap` already trust — not a pane or path match.

**Guard the edges.** The filter narrows only; `reap.Decide`, the park,
the live-pane refusal, and the `--go` gate are untouched, so a targeted
reap can only tear down what the fleet-wide reap would have. A selector
that resolves but matches no stranded lane reaps nothing and says so,
rather than falling through to the whole fleet. An unknown or ambiguous
selector refuses before any teardown, the way `yield` does.

**Gate.** `frit reap <id>` dry-run names only that lane's worktree;
`frit reap <id> --go` removes it and leaves the other landed worktree
standing; a live pane on it refuses; a bare `frit reap` still sweeps the
fleet. `go test ./...` and `go tool -modfile=tools/go.mod golangci-lint
run` are green.
