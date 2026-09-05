---
n: 3
title: The board, orphans, show and next surface the same state
status: "✅"
result: false
---
Surface a token-less held lane before a refusal does. `board` and
`orphans` scan the whole fleet. `show`, `next` and `phase` run from
inside a lane. Each gets the same `next_action` wording phase 2 gave
`release` and `start`. A healthy lane shows nothing extra.

**Assumes.** `holdKindFor` (cmd/frit/dispatch.go) already tells
`HoldUnproven` from every other kind. But it only reads one held plan,
from outside it. It cannot answer "is this checkout on this host" for
a whole fleet without a herdr call per plan. That is a trap code
review already caught once, in phase 2's `start.go`.

`tokenlessOwnLane` (cmd/frit/claim.go) is the cheaper, herdr-free
proof. It is pure git, and it takes any path, not just
`os.Getwd()`. `resumableFromAnyLane` in main.go already loops a
repository's worktrees this same way, for `desertedHeld`.

`unprovenNextAction(id)` (internal/report/dispatch.go) is the one
wording every verb reuses. `discovery.Plan.Held` is true for a
claim-only lane, and `Dead` is false: `deadSession` reads false for a
marker whose `session:` trailer was never bound. So this shape never
collides with `Deserted`.

**Value.** `board`, `orphans`, `show`, `next` and `phase` name the way
out. A person or a skill meets it before `release`'s or `start`'s
refusal ever does. `plan-phase` stops on it, rather than starting
doomed work. `plan-drive` and `plan-tidy` point at the same field
their skills already read others from (`ask`, `next_action`).

**RED.** In internal/report/board_test.go:

- `TestBoardDocMarkUnprovenNamesTheWayOut`. `AddPlan`, then
  `MarkUnproven(repo, id)`. That sets the row's `NextAction` to
  `unprovenNextAction(id)`. A repo/id that does not match is a no-op.

In internal/report/orphans_test.go:

- `TestOrphansAddUnprovenNamesTheWayOut`. `AddUnproven(repo, plans)`
  appends one `Unproven` row per plan. Each carries
  `unprovenNextAction(id)`. `Any()` reports true.

In internal/report/discovery_test.go and phase_test.go:

- `TestNextMarkUnprovenNamesTheWayOut`,
  `TestShowMarkUnprovenNamesTheWayOut`,
  `TestPhaseMarkUnprovenNamesTheWayOut`. Each doc's `MarkUnproven(id)`
  sets `NextAction` from the same helper.

In cmd/frit/board_test.go:

- `TestBoardNamesTheWayOutForATokenlessOwnLane`. Build the phase-2
  fixture: `claim.Acquire` plus `git worktree add`, no `Renew`. Run it
  through `frit board --json`. The row's `next_action` is non-empty,
  and the table prints it.

In cmd/frit/orphans_test.go:

- `TestOrphansNamesTheWayOutForATokenlessOwnLane`. Same fixture.
  `unproven` carries the plan, and the table prints it.

In cmd/frit/next_test.go, show_test.go and phase_test.go (or their
existing files):

- `TestNextNamesTheWayOutFromATokenlessLane`,
  `TestShowNamesTheWayOutFromATokenlessLane`,
  `TestPhaseNamesTheWayOutFromATokenlessLane`. Run each from inside
  the fixture's lane. `next_action` is non-empty. A healthy lane —
  its token proves — leaves it empty, in all five verbs. One test per
  verb, reusing an existing green fixture for the healthy case.

**GREEN.** internal/report gains `BoardPlan.NextAction` and
`BoardDoc.MarkUnproven(repo string, id int64)`, scanning `d.Plans` for
the match. `id` alone is not enough.

Two repositories can share a plan id (S74).

It also gains `OrphanRepo.Unproven []Unproven` (`PlanID`, `Branch`,
`NextAction`), `Any()` extended, and `OrphansDoc.AddUnproven(name
string, plans []discovery.Plan)`, mirroring `AddDeserted`.

Last, `NextDoc.NextAction`, `ShowDoc.NextAction` and
`PhaseDoc.NextAction`, each with its own `MarkUnproven(id int64)`.

cmd/frit gains `unprovenHeld` in main.go, `desertedHeld`'s own shape.
It filters `p.Held`, then tests every worktree in `repo.Worktrees`
with `tokenlessOwnLane`. No herdr call there, so an ambiguous or
coordinate-less repository costs nothing extra. `boardUnproven` adds
a `map[string][]gitwt.Worktree` cache keyed by repo, so `boardCmd.Run`
calls `gitwt.List` once per repository, never once per held plan.
`laneOverride` returns its resolved `root` alongside plan and source,
for `nextCmd.Run` and `showCmd.Run` to pass to `claim.ReadToken`.
`phaseCmd.Run` already holds `root`; the same one-line check applies
there.

`printBoard` gains a trailer line, `boardAsks`'s own shape, for a row
whose `NextAction` is non-empty. `printOrphans` gains an `Unproven`
row carrying its `NextAction`. `printNext`, `printShow` and
`printPhase` each gain a `printUnproven` call, after their existing
rescue-refs print.

**Guard the edges.** `MarkUnproven` matches on `(repo, id)`. It never
matches on `id` alone (S74). `unprovenHeld` and `boardUnproven` read
no herdr socket at all — `tokenlessOwnLane` is pure git. So an
unreachable herdr costs this new signal nothing, unlike `holdKindFor`.
A plan whose token proves, or that is not `Held`, or whose repository
the gather could not place, stays untouched in every one of the five
docs.

**Skills.** `internal/skills/assets/plan-drive/SKILL.md` gets one
clause, on the `board --json` bullet, naming `next_action`.
`plan-tidy/SKILL.md` gets one clause, on the `orphans --json` bullet,
naming `unproven` and `next_action`. `plan-phase/SKILL.md` gets one
sentence in "Honor the answers": a non-empty `next_action`, from
`phase` or `show`, means stop and report — the same as an
already-held live lane.

Regenerate the dogfood copies: `go run ./cmd/frit skills --via "go
run ./cmd/frit" --force --root .`. `mdsmith check .` and `go test
./internal/skills/... -run TestDogfoodCopiesMatchCanonical` both stay
green, inside each skill's token budget.

**Gate.** Run `board`, `orphans`, `show`, `next` and `phase` against
the phase-2 fixture. Each prints the new wording, and carries
`next_action`, in both table and `--json`. A healthy lane shows
nothing extra, in any of the five. `go test ./internal/report -update`
re-records `board.json`, `orphans.json`, `next.json`, `next-lane.json`,
`show.json`, `show-lane.json` and `phase.json`; the diff is read
before committing. Every shipped skill's own claim is confirmed
against the built frit. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
