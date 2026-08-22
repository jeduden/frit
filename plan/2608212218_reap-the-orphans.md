---
id: 2608212218
title: frit reaps the orphans it reports
status: "✅"
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
    status: "✅"
  - n: 2
    title: reap the remaining orphan kinds
    status: "✅"
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

The verb does not invent teardown. `claim.Scavenge` in
[lease.go](../internal/claim/lease.go) already drops a work ref on
landed evidence, parking any unlanded commits first; Phase 2 reuses it
for the claimed-no-checkout kind. Phase 1's kind — a stranded
checkout — has no such ref left to CAS against by definition, so it
tears down with plain git porcelain instead: `git worktree remove`
then `git branch -D`, in that order, since git refuses to delete a
branch any worktree still has checked out. `yield`'s herdr-based
teardown in [yield.go](../cmd/frit/yield.go) does not fit here either:
it tears down the *calling pane's own* live workspace, and a stranded
lane has no live pane to key that on.

The delete gate is frit's own landed check, not a raw ancestor test.
It is the same two facts `repoLanes` already joins the claims
against: `gitobj.MergedRefs`'s ancestry, and `index.LandedIDs`'s
default-branch plan status — the signal that closes the squash-merge
gap ancestry cannot see. [internal/reap](../internal/reap) re-checks a
stranded lane's own branch against that evidence per worktree, rather
than trust the lane's stranded classification alone. A branch whose
ref was simply dropped by hand — no merge, no landed status — is
refused, not reaped, even though `lanes.Find` already calls the lane
stranded. Reap follows the house rule for a mutating verb: a dry-run
by default, acting only on `--go`, exactly like `nudge` and `start`.

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

GREEN: a `reap` command that reuses `lanes.Find` for the set.
[internal/reap](../internal/reap)'s `Decide` re-checks each stranded
worktree's own branch against the caller's landed evidence and gates
the teardown; `git worktree remove` then `git branch -D` do the
teardown itself, since no live pane or work ref is guaranteed to key a
herdr or `claim.Scavenge` teardown on. Its report is a document in
[report](../internal/report), rendered as a table and as `--json`
with every key present.

Gate: the four RED cases pass; `--json` carries the reaped and refused
sets as `[]` when empty; `go test ./...` and `mdsmith check .` are
clean.

## Phase 2: reap the remaining orphan kinds

Reap extends over the other kinds `orphans` reports: an unstaffed
hold, a prunable worktree, a never-started one.

An unstaffed lane's canonical id-only ref (`claim.Branch`) is dropped
through `claim.Scavenge`. Any unlanded work is parked to a rescue ref
first. Only that ref is Scavenge's to CAS against — it is hardcoded to
`plan/<id>`. A lane held only on a decorated legacy branch is refused
instead, with a migrate-first reason, rather than silently doing
nothing useful. A hold Scavenge cannot drop — fenced by another
machine since it was observed, or already gone — is refused with the
reason it read. Neither case is a hard command failure.

The drop is gated on abandonment evidence, not on the missing
checkout. "Claimed, no local checkout" proves nothing by itself. The
checkout may be another machine's, or the claim seconds old with its
stand-up still pending. `lanes.Build` already filters landed refs, so
an unstaffed hold is by construction a live-looking lease. What earns
the drop is the lease protocol's own evidence: a matured staleness
window, or a bound session herdr confirms dead. That is the same gate
`discovery.Ready` and takeover honor. Reap gathers the fleet beside
its repo walk, exactly as `orphans` does, to read that evidence. A
live lease is refused, with a pointer at `release` and `claim`.

The stranded delete honors the same park-before-delete rule.
Ordinary-merge evidence is tied to the tip — an ancestor of the base
loses nothing to `branch -D`. The squash-merge glyph is not: a
follow-up commit the squash never carried would be destroyed. So the
branch tip's unlanded work is parked through `claim.ParkUnlanded`
before the delete — the park half of a scavenge, exported for exactly
this. A park that cannot happen refuses the whole teardown. The dry
run previews the rescue ref, so `--go` never moves work the report did
not name. As groundwork, `claim.Scavenge` also stopped reading a
failed `ls-remote` as an absent ref. An unreadable remote is now a
surfaced fault, not a silent no-op that cleans the local ref.

A prunable or never-started worktree is torn down the same primitive
way a stranded checkout is: `git worktree remove`. Its branch is left
alone. Unlike a landed lane's, a prunable or empty checkout's branch
may still be live work under another name.

RED surfaced a real hazard, not the four cases first assumed.
`lanes.Find`'s stranded pass and its empty/prunable pass are
independent. A worktree whose branch ref vanished without landing (S79
— see
[2608220940](2608220940_scavenge-spares-a-checked-out-branch.md))
reports a zero-commit HEAD indistinguishable from one that never
started, so the same worktree surfaces in both sets. Excluding it from
the empty pass by path was the wrong fix. It made every genuinely
never-started worktree unreapable too, since an unborn branch has no
live ref either and so is *always* also classified stranded. GREEN
instead lets git itself be the arbiter. `worktree remove` refuses a
directory still holding real content it cannot reconcile against a
resolvable commit. That refusal is carried back as a `RefusedWorktree`
rather than a command failure, so one ambiguous worktree never stops
the rest of a repository from being reaped.

Gate: a stale or dead unstaffed hold is dropped on `--go` with its
unlanded work parked first; a live, decorated or fenced one is
refused. A squash-landed branch's tip is parked before its delete. A
prunable and a never-started worktree are each reaped on `--go`. The
S79 shape is refused rather than destroyed. `--json` carries every new
kind's list as `[]` when empty. `go test ./...`, `go vet`,
`golangci-lint run` and `mdsmith check .` are clean.

## Execution

Tier is per phase, set by the most demanding ingredient. The design
is settled here, so both phases implement from written assertions.

| Phase             | Design | Implement | Gate that catches a wrong answer                                 |
| ----------------- | ------ | --------- | ---------------------------------------------------------------- |
| 1 reap a checkout | opus   | sonnet    | landed lane reaped on --go; unlanded refused; dry-run by default |
| 2 remaining kinds | opus   | sonnet    | each kind reaped or refused; unlanded work parked first          |

## Acceptance Criteria

- [x] `frit reap` removes a landed checkout and deletes its branch
- [x] It is a dry-run by default; it acts only on `--go`
- [x] A branch frit does not read as landed is refused, not deleted
- [x] A squash-merged branch is read as landed and reaped, its tip
      parked first
- [x] A claimed-no-checkout hold that is stale or dead is dropped,
      unlanded work parked; a live lease is refused
- [x] `--json` carries the reaped and refused sets, always present
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
