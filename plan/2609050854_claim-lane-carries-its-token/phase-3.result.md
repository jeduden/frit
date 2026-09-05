---
n: 3
title: The board, orphans, show and next surface the same state
status: "✅"
result: true
summary: >-
  `board`, `orphans`, `show`, `next` and `phase` now surface a
  token-less held lane, on this host, before a refusal ever does —
  the same `next_action` wording phase 2 gave `release` and `start`,
  through the shared `unprovenNextAction` helper. `BoardDoc.MarkUnproven`,
  `OrphansDoc.AddUnproven` (a new `Unproven` cell, distinct from
  `Deserted` and `StaleHolds`), and `NextDoc`/`ShowDoc`/`PhaseDoc`'s
  own `MarkUnproven` each carry it. `board` and `orphans` find it with
  `tokenlessOwnLane` walked over a repository's local worktrees — pure
  git, no herdr call, cached per repository for `board` so
  `gitwt.List` runs once, not once per held plan. `show` and `next`
  read it through `laneOverride`'s now-returned lane root; `phase`
  already had it. Both table and `--json` carry the field in all
  five verbs. Three shipped skills (`plan-drive`, `plan-tidy`,
  `plan-phase`) branch on it; the dogfood copies regenerated to
  match.
---
## Handoff

**Done.** internal/report gained `BoardPlan.NextAction` and
`BoardDoc.MarkUnproven(repo, id)`, matched on the pair since two
repositories can share a plan id (S74). `OrphanRepo.Unproven
[]Unproven` (`PlanID`, `Branch`, `NextAction`), `Any()` extended, and
`OrphansDoc.AddUnproven(name, plans)` mirroring `AddDeserted`.
`NextDoc.NextAction`, `ShowDoc.NextAction` and `PhaseDoc.NextAction`,
each with its own `MarkUnproven(id)`. All fed from the one
`unprovenNextAction(id)` helper phase 2 already introduced.

cmd/frit gained `unprovenHeld` (main.go), `desertedHeld`'s own shape
but reading no herdr socket at all: it filters `p.Held`, then tests
every worktree in `repo.Worktrees` with `tokenlessOwnLane`.
`orphansCmd.Run` calls it beside `desertedHeld` and `staleHeld`.
`boardUnproven` does the same for `board`, caching each repository's
`gitwt.List` result so a fleet with many held plans in one repository
pays for the list once. `laneOverride` now returns its resolved lane
root alongside the plan and source, so `nextCmd.Run` and
`showCmd.Run` can pass it to `claim.ReadToken` through the new
`markUnprovenFromLane` helper; `phaseCmd.Run` already held its own
root and calls the same helper directly. `printBoard` gained a
trailer line (`boardUnprovenLines`, `boardAsks`'s own shape);
`printOrphans` gained an `Unproven` row printing its `NextAction`;
`printNext`, `printShow` and `printPhase` each gained a shared
`printUnproven` call after their existing rescue-refs print.

Three shipped skills now branch on the field:
`internal/skills/assets/plan-drive/SKILL.md`'s `board --json` bullet
names `next_action`; `plan-tidy/SKILL.md`'s `orphans --json` bullet
names `unproven` and `next_action`; `plan-phase/SKILL.md`'s "Honor
the answers" step adds a token-less lane to the same stop-and-report
list as an already-live hold. `go run ./cmd/frit skills --via "go run
./cmd/frit" --force --root .` regenerated the three changed dogfood
copies; `TestDogfoodCopiesMatchCanonical` and `mdsmith check .` both
pass.

**Proven.** internal/report:
`TestBoardMarkUnprovenNamesTheWayOut`,
`TestOrphansAddUnprovenNamesTheWayOut`,
`TestOrphansAddUnprovenIsANoOpForAnUnknownRepo`,
`TestNextMarkUnprovenNamesTheWayOut`,
`TestShowMarkUnprovenNamesTheWayOut`,
`TestPhaseMarkUnprovenNamesTheWayOut`, plus the extended
`TestOrphanRepoAnyReportsWhateverWasFound` and
`TestOrphansKeepsCleanRepositories` cases.

cmd/frit, in the new cmd/frit/unproven_test.go:
`TestUnprovenHeldListsAClaimOnlyLaneWithNoToken`,
`TestUnprovenHeldExcludesALaneWhoseTokenProves`,
`TestUnprovenHeldExcludesAnUnheldPlan`,
`TestUnprovenHeldExcludesAPlanWithNoLocalCheckout`,
`TestOrphansNamesTheWayOutForATokenlessOwnLane`,
`TestBoardUnprovenReportsAClaimOnlyLaneWithNoToken`,
`TestBoardNamesTheWayOutForATokenlessOwnLane`,
`TestNextNamesTheWayOutFromATokenlessLane`,
`TestShowNamesTheWayOutFromATokenlessLane`,
`TestPhaseNamesTheWayOutFromATokenlessLane`, and
`TestNextLeavesTheWayOutEmptyForAHealthyLane` — the healthy-lane
negative case, pinning the field is "unprovable", not merely "held".
Every new positive test was confirmed red before its implementation
landed. Every pre-existing board/orphans/next/show/phase test still
passes unchanged.

**Verified against the built frit.** `go build ./...`, `go vet
./...`, `gofmt -l .`, `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are all clean. `go test
./internal/report -update` re-recorded `board.json`, `orphans.json`,
`next.json`, `next-lane.json`, `show.json`, `show-lane.json` and
`phase.json` — each gained exactly the one new field or list, empty
on every fixture that does not exercise this shape; the diff was read
before committing.

**A design choice, not the fork's own.** The background research fork
this phase started from suggested changing `BoardDoc.AddPlan`'s own
signature to take a fifth `unproven bool` parameter. That would have
touched every one of the roughly fifteen direct `AddPlan` call sites
across `internal/report/golden_test.go`, `internal/report/board_test.go`
and `cmd/frit/board_test.go` for no functional gain. `BoardDoc.MarkUnproven
(repo, id)`, called once after `AddPlan` only where the fleet-side
check found the shape, was implemented instead — zero pre-existing
call sites touched, matching `ReleaseDoc.RefuseUnproven`'s own
already-established idiom of a dedicated setter beside the
constructor.

**This plan is done.** Every Acceptance Criterion is met.

You may clear this session now.
