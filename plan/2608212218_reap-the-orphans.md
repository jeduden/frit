---
id: 2608212218
title: frit reaps the orphans it reports
status: "🔲"
summary: >-
  frit enumerates orphans — a landed checkout still standing, a
  claimed lane with no worktree, a prunable stub — but it cannot act
  on them, so cleanup falls to hand-run git worktree remove and
  branch -D, the surgery frit exists to avoid. A reap verb sweeps the
  kinds orphans already reports: it removes a landed checkout, deletes
  a landed branch, and drops a claimed-no-checkout hold. It is a
  dry-run by default and acts only on --go, gating every delete on
  frit's own landed evidence and parking any unlanded work first.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: reap a landed checkout
    status: "🔲"
  - n: 2
    title: reap the remaining orphan kinds
    status: "🔲"
---
# frit reaps the orphans it reports

## Goal

`frit reap` cleans up the orphans `frit orphans` already enumerates.
It removes a landed checkout, deletes a landed branch, and drops a
claimed-no-checkout hold. A finished lane is torn down by frit, never
by hand-run git surgery.

## Context

`orphans` reports orphan kinds but acts on none of them, so cleanup
today is manual `git worktree remove` and `git branch -D`. The kinds
live in [orphans.go](../internal/report/orphans.go) and are found by
[lanes.Find](../internal/lanes/lanes.go): stranded (landed, still
checked out), unstaffed (claimed, no checkout), empty and prunable.

The verb does not invent teardown. `yield` in
[yield.go](../cmd/frit/yield.go) already parks a lane's divergence to
a rescue ref and tears its worktree down through herdr. `claim.Scavenge`
in [lease.go](../internal/claim/lease.go) already drops a work ref on
landed evidence, parking any unlanded commits first. Reap composes
these over the orphan set rather than adding a new teardown.

The delete gate is frit's own landed check, not a raw ancestor test.
`landed` and `landedTip` in [claim.go](../internal/claim/claim.go)
read origin's default branch and count squash-merged work as landed,
where `git merge-base --is-ancestor` would not. That is the authority
reap deletes on. Reap follows the house rule for a mutating verb: a
dry-run by default, acting only on `--go`, exactly like `nudge` and
`start`.

## Tasks

1. Reap a stranded lane: remove the landed checkout, delete the
   landed branch, dry-run unless `--go`.
2. (determined after Phase 1)

## Phase 1: reap a landed checkout

`frit reap` takes a stranded lane — a checkout whose branch has
landed, still standing — and tears it down: remove the worktree,
delete the landed branch. It is a dry-run by default and acts only on
`--go`. It never deletes a branch frit's landed check does not
confirm landed.

RED, against the fixture idiom the lanes tests use:

- A stranded lane, `--go`: the worktree is removed and the branch
  deleted; the report names both.
- The same lane without `--go`: nothing is removed; the report says
  what it would do.
- A branch frit does not read as landed: refused, not deleted, even
  with `--go`.
- A squash-merged branch that is not an ancestor of the base: read as
  landed and reaped, because the landed check is the authority.

GREEN: a `reap` command that reuses `lanes.Find` for the set, the
landed check in [claim](../internal/claim) for the gate, and the
worktree teardown `yield` already uses. Its report is a document in
[report](../internal/report), rendered as a table and as `--json`
with every key present.

Gate: the four RED cases pass; `--json` carries the reaped and refused
sets as `[]` when empty; `go test ./...` and `mdsmith check .` are
clean.

## Phase 2: reap the remaining orphan kinds

Reap extends over the other kinds `orphans` reports. A claimed lane
with no checkout has its hold dropped through `claim.Scavenge`,
parking any unlanded commits to a rescue ref first. A prunable stub is
pruned. An empty worktree is removed. Each still honors the dry-run
default and the landed-or-park rule, so no unlanded work is ever lost.

RED cases and the exact per-kind behavior are settled after Phase 1
fixes the report and teardown shape. The gate is that every orphan
kind `orphans` names can be reaped or is explicitly refused with a
reason, and unlanded work is always parked before any delete.

## Execution

Tier is per phase, set by the most demanding ingredient. The design
is settled here, so both phases implement from written assertions.

| Phase             | Design | Implement | Gate that catches a wrong answer                                 |
| ----------------- | ------ | --------- | ---------------------------------------------------------------- |
| 1 reap a checkout | opus   | sonnet    | landed lane reaped on --go; unlanded refused; dry-run by default |
| 2 remaining kinds | opus   | sonnet    | each kind reaped or refused; unlanded work parked first          |

## Acceptance Criteria

- [ ] `frit reap` removes a landed checkout and deletes its branch
- [ ] It is a dry-run by default; it acts only on `--go`
- [ ] A branch frit does not read as landed is refused, not deleted
- [ ] A squash-merged branch is read as landed and reaped
- [ ] A claimed-no-checkout hold is dropped, unlanded work parked
- [ ] `--json` carries the reaped and refused sets, always present
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
