---
id: 2609012100
title: reap takes a selector, so one landed lane retires without its neighbours
status: "🔲"
summary: >-
  frit reap tears down every leftover orphans reports across the whole
  fleet, or nothing — it has no selector. yield and release already take
  a plan id or slug; reap is the odd verb out. So retiring one landed
  lane forces a choice between raw git, against the plan-tidy rule, and a
  fleet-wide sweep that also tears down every other landed worktree and
  prunes every landed rescue ref. This plan gives reap the selector its
  sibling verbs already have: frit reap <id> narrows the same teardown to
  the named plan's lane — its landed or stranded leftover worktree, its
  unstaffed hold, its landed rescue ref — with every guard unchanged: a
  live pane still refuses, a divergent suffix is still parked before
  delete, and only landed or abandoned lanes go. Dry-run stays the
  default and --go still acts. A bare frit reap keeps its whole-fleet
  behavior exactly. The selector only narrows the set fed to the
  machinery reap already runs; it adds no new teardown path. Phase 1
  proves it on the landed leftover worktree, the case a just-merged lane
  leaves behind.
model: sonnet
depends-on: []
---
# reap takes a selector, so one landed lane retires without its neighbours

## Goal

`frit reap <selector>` tears down just the named plan's lane — its
landed or stranded leftover worktree, its unstaffed hold, its landed
rescue ref. The guards and the dry-run/`--go` gate are reap's own. So a
single landed lane retires without touching its neighbours, and a bare
`frit reap` keeps its whole-fleet behavior.

## Context

**The gap.** `reapCmd` in [cmd/frit/reap.go](../../cmd/frit/reap.go)
carries only `--go`. `Run` walks every repository and tears down every
kind of leftover `orphans` reports — stranded worktrees, unstaffed
holds, landed rescue refs — or, without `--go`, prints them all. There
is no way to name one lane. `yield` and `start` already take a
`Selector` (plan id or slug) resolved by `resolveSelector`, and `release`
targets one lease; `reap` is the sibling that never got one. So retiring
one landed lane — the leftover a just-merged plan leaves, worktree still
checked out — forces either raw git against the plan-tidy rule, or a
fleet-wide `reap --go` that also tears down every other landed worktree
and prunes every landed rescue ref.

**What reap already does, and stays doing.** `reapStranded` classifies
each stranded worktree through `reap.Decide` and, under `--go`, parks any
divergent suffix to a rescue ref before removing the worktree — a live
herdr pane on it refuses, and a park that cannot happen refuses that
teardown whole. `reapUnstaffed` drops an unstaffed hold only on
abandonment evidence. `reapPruned` prunes landed rescue refs and foreign
checkouts. Every one of those guards is load-bearing and unchanged here.

**Reuse first, and where the fix goes.** The selector narrows the set,
never the teardown. `resolveSelector` in
[cmd/frit/main.go](../../cmd/frit/main.go) — the resolver `yield` and
`start` already call — turns the argument into one plan; `reap`'s `Run`
then filters the stranded lanes (and, in later phases, the unstaffed
holds and rescue refs) to that plan's own lane before handing them to the
same `reapStranded`/`reap.Decide` machinery. A lane is matched by its
repository and its hold branch, `claim.Branch(id)`, the identity check
`orphans` and `reap` already trust — never a pane or path string. With no
selector, the unfiltered set is passed exactly as today.

**Out of scope.** What reap *decides* is untouched: the live-pane
refusal, park-first, and the landed/abandoned evidence stay verbatim, so
a targeted reap can never tear down something the fleet-wide reap would
have spared. The whole-fleet default is preserved. `orphans`' own report
is unchanged — it still surveys the fleet; only `reap` learns to act on
one lane.

## Tasks

1. Phase 1 (proving slice): `frit reap <id>` scoped to one plan's landed
   or stranded leftover worktree. Resolve the selector, filter the
   stranded lanes to that plan's lane, and tear it down under `--go`
   through the unchanged `reapStranded`/`reap.Decide` path; a live pane
   still refuses; a bare `frit reap` still sweeps the fleet. Driven red
   at the cmd level against the worktree-and-herdr fixtures reap's tests
   already use.
2. Later phases, shaped by Phase 1's handoff: extend the selector filter
   to `reapUnstaffed` (the hold drop) and `reapPruned` (landed rescue
   refs), so `reap <id>` retires all of one plan's leftovers; and the
   refusal wording when a selector names a plan with nothing to reap.

## Execution

| Phase | Title                                                         | Tier   | Gate                                                                                                                                                                                  |
| ----- | ------------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | reap <id> retires one landed leftover worktree, not the fleet | sonnet | `frit reap <id> --go` removes only that lane's worktree, leaving other landed worktrees standing; a live pane refuses; bare `frit reap` still sweeps the fleet; `go test ./...` green |

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

| #   | Status | Phase                                                                       |
| --- | ------ | --------------------------------------------------------------------------- |
| 1   | 🔲     | [reap <id> retires one landed leftover worktree, not the fleet](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] `frit reap <id>` dry-run names only the selected plan's leftover,
      not the whole fleet's
- [ ] `frit reap <id> --go` removes that lane's leftover worktree and
      leaves every other landed worktree standing
- [ ] A live herdr pane on the selected lane still refuses the teardown
- [ ] A bare `frit reap` (no selector) still reports and, with `--go`,
      tears down the whole fleet's leftovers unchanged
- [ ] An unknown or ambiguous selector is refused the way `yield`'s is
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
