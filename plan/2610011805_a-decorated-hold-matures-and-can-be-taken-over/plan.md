---
id: 2610011805
title: A decorated hold with no lease matures, and can be taken over
status: "✅"
summary: >-
  A hold made only of a legacy decorated branch, plan/<id>-<slug>, with
  no id-only lease ref beside it, never entered the staleness sampler:
  start read "seen unchanged for 0s" on every call, so a branch
  untouched for 55 days could never be taken over. release called the
  same plan "nothing holds it" while yield called it "held live by
  another lane". The sampler now watches the decorated branch's tip.
  release and yield refuse it in the same words. A matured decorated
  hold is taken over by start and claim, which retire the decorated
  branch and mint the id-only lease.
model: opus
depends-on: []
---
# A decorated hold with no lease matures, and can be taken over

## Goal

A deserted legacy decorated hold matures on the observer's own clock
the way a lease does. Once matured, `frit start` takes it over with no
hand-run git. Until then, `release` and `yield` give the same answer
about who holds it.

## Context

**What happened.** Issue #204, frit 0.14.0. The `.frit.yml` listed
both default hold patterns, `plan/{id}` and `plan/{id}-*`. Plan
`2608070719` was held by `plan/2608070719-radiant-suns-instruments`,
both locally and on origin. That branch's only commit was a legacy
claim, 55 days old. Its worktree was clean, with no agent attached.
`start` refused with "seen unchanged for 0s of the 2h0m0s takeover
window" on every call, 20 minutes apart. `observations.json` never got
a key for the plan. `release`, run inside the lane, said "nothing
holds it". `yield`, run from the main checkout, said "held live by
another lane". `reap` did not list the lane.

**Why the window never starts.** `heldBranches` in
[internal/fleet/gather.go](../../internal/fleet/gather.go) marks a plan
`Held` when any hold pattern matches, decorated or not. `leaseTips`
fills `HoldTip` only from the id-only ref `plan/<id>`. So a plan held
by a decorated branch alone is `Held` with an empty `HoldTip`.
`observeHolds` in [cmd/frit/main.go](../../cmd/frit/main.go) watches
`HoldTip` only. On an empty one it skips the plan, and on a fetching
pass it deletes the plan's key. So `StaleFor` stays zero and `Stale`
never turns true.

**Why release and yield disagree.** `release` reads `HoldTip == ""`
as "nothing holds it" before it looks at `Held`. `yield` finds no
local `plan/<id>` to park, falls into `yieldNothingLocal`, sees `Held`,
and refuses with `foreignHoldRefusal`. That says "held live by another
lane" even when run from the hold's own lane. "Live" claims more than
frit knows: an unmatured window is all the refusal can vouch for.

**Why nothing can end it.** `mintOrTakeOver` in
[cmd/frit/claim.go](../../cmd/frit/claim.go) takes over by CAS on
`HoldTip`. A decorated hold has none, so `claim.Takeover` would fail
on an empty tip even once the window matured. `reap` drops only
unstaffed holds, and it refuses a decorated one with "migrate first".
No verb performs that migration.

**Reuse first.** Searched: `observe`, `discovery.Observe` and
`StaleHold` (reused unchanged — the window is the same rule on a
different tip). `claim.Scavenge` already parks unlanded work and then
deletes a ref by CAS on its observed tip. Phase 3 lifts its body into
a branch-named form instead of writing a second delete.
`claim.Acquire` already mints the id-only lease create-only, and that
is what a decorated takeover needs once the decorated branch is gone.
`refuseForeignHold` and `tokenlessOwnLane` in
[cmd/frit/release.go](../../cmd/frit/release.go) already word an
unproven hold; phase 2 makes `yield` reach them instead of keeping its
own copy. `lanes.Migratable` names the decorated-to-id-only mapping,
but only for unstaffed lanes and only as a report, so it is not reused.

**The issue's two questions.** A hold made only of a branch name is
meant to go through the sampler. It counts as a hold, and the refusal
promises it will mature. That is phase 1. The window does not start
from the tip's commit date: the protocol never compares a marker's
timestamp across machines (S33–S36). It dates staleness on the
observer's own clock, so a skewed or forged date cannot mature a live
hold early. A 55-day-old branch therefore waits one `takeover-window`
from the first pass that sees it.

**Out of scope.** A local checkout still standing on a decorated
branch after a takeover keeps its local branch: a branch a worktree
stands on is never deleted (S79). Tearing that worktree down is
herdr's, as for any lane; the report says this host's copy was kept.
`reap`'s unstaffed pass still refuses a decorated hold. Teaching it
the phase 3 retire is a follow-up once that primitive exists.

## Tasks

1. Phase 1 (proving slice): the observer watches a decorated hold.
   The fleet gather carries each live decorated hold's tip.
   `observeHolds` watches it when no id-only lease ref exists, so the
   plan gets an observation key, its span grows across passes, and it
   matures after `takeover-window`.
2. Phase 2: `release` and `yield` agree on a decorated hold. `release`
   no longer reads a held plan with no lease ref as "nothing holds it".
   Both refuse through the one shared wording. The wording drops
   "live", which frit cannot vouch for.
3. Phase 3: a matured decorated hold is taken over. `claim` and `start`
   park the decorated branch's unlanded work, delete the branch on
   origin by CAS on the tip the window matured on, then acquire the
   id-only lease. A decorated branch that moved since refuses as a
   lost race and resets the window.
4. Phase 4: the PR's code-review findings. A live agent on the
   decorated branch vetoes the takeover. A delete the server refuses is
   a fault, not a race. A branch already gone is skipped. Every retired
   branch is reported, with what was deleted and what this host kept.
   The configured remote's copy is the one watched. Scavenge and the
   takeover share one CAS delete.

## Execution

| Phase | Title                                       | Tier   | Gate                                                                                                                                                            |
| ----- | ------------------------------------------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | The observer watches a decorated hold       | opus   | a cmd test gathers a fleet held only by `plan/7-slug` and finds its key; a seeded 3h window reads stale; against the built frit, `start` reports a growing span |
| 2     | release and yield agree on a decorated hold | sonnet | cmd tests: release in the lane and yield outside it both refuse as held, neither says "nothing holds it" nor "live"; `go test ./...` green                      |
| 3     | A matured decorated hold is taken over      | opus   | new `@S100` and `@S101`: a matured decorated hold is claimed, its branch deleted on origin, `plan/<id>` minted; a moved branch is not seized; tests green       |
| 4     | The takeover answers its code review        | opus   | new `@S102`: a live decorated lane vetoes the claim; tests green                                                                                                |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                                                                      |
| --- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [The observer watches a decorated hold](phase-1.md)                                                                                                                                                                                                        |
|     | ↳      | A plan held by a decorated branch alone now gets an observation key on the first pass that sees it. Its span grows across passes, and it reads stale once the takeover window matures, like a lease does.                                                  |
| 2   | ✅     | [release and yield agree on a decorated hold](phase-2.md)                                                                                                                                                                                                  |
|     | ↳      | release and yield now refuse a hold made of a decorated branch alone in the same words, naming the branch and the takeover that ends it. No refusal calls an unmatured hold "live" any more. C17 covers it.                                                |
| 3   | ✅     | [A matured decorated hold is taken over](phase-3.md)                                                                                                                                                                                                       |
|     | ↳      | claim and start take a matured decorated-only hold over. They park its unlanded work, delete the branch on origin by CAS on the tip the window matured on, and mint plan/<id> fresh. A holder that pushed meanwhile is not seized. S100 and S101 cover it. |
| 4   | ✅     | [The takeover answers its code review](phase-4.md)                                                                                                                                                                                                         |
|     | ↳      | A live agent on the decorated branch now vetoes the takeover (S102). A refused delete is a fault, a vanished branch is skipped, every retired branch is reported with what was kept, and the configured remote's copy is the one watched.                  |
<?/catalog?>

## Acceptance Criteria

- [x] A plan held only by a decorated branch gets an observation key
      on the first pass that sees it, and its span grows across passes
- [x] Once the window matures, the plan reads stale on `board` and
      `start` no longer refuses it as not matured
- [x] `release` and `yield` both refuse a decorated hold as held;
      neither says "nothing holds it" or "held live"
- [x] `start` and `claim` take a matured decorated hold over: the
      decorated branch is gone from origin, `plan/<id>` holds a claim
      marker, and unlanded work on the decorated branch is parked first
- [x] A decorated branch that moved after its window matured is not
      deleted; the takeover refuses as a lost race
- [x] A live agent herdr shows on the decorated branch vetoes the
      takeover, and nothing is parked, deleted or minted
- [x] Every branch a takeover retired is reported in `--json`'s
      `retired` list, with its rescue and what was kept on this host
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
