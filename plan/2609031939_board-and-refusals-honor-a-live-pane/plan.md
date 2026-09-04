---
id: 2609031939
title: reports and refusals stop calling an attended lane dead
status: "🔳"
summary: >-
  frit reports two facts about a held lane from herdr and lets them
  contradict each other. A `dead` field marks a lane whose bound
  session herdr confirms gone; `agent`/`agent_status` marks a live pane
  herdr found on that same lane. When a lane's original session rotated
  away but a live pane still works or idles on it, board prints `dead:
  true` beside `agent: claude, agent_status: working`, `ready` and
  `pick` carry the same `dead: true` with no agent beside it, and
  start's "deserted hold" refusal points at `frit yield`, a teardown,
  though a pane is open. A reader concludes the lane is gone and
  reaches for the teardown. This plan makes the `dead` render — board,
  and the discovery report `cardOf` behind `ready`, `pick` and `find` —
  and the deserted refusals answer to the live pane: no survey report
  calls an attended lane dead, and a refusal for a lane a pane attends
  names that pane and leads with resume, not yield. Reproduction
  scenarios pin each on the observed working-agent case, and a
  cross-layer matrix row, S89, runs the state end-to-end under godog.
model: sonnet
depends-on: [2609031211]
---
# reports and refusals stop calling an attended lane dead

## Goal

Every frit survey report and refusal treats a held lane a live pane
attends as attended, not dead. The `dead` field renders false — in
board, and in `ready`, `pick` and `find` through their shared
discovery render — while herdr shows a live pane on the lane, whether
that pane is working or idle. A start refusal for such a lane names the
pane and leads with resume, not a `frit yield` teardown. So a reader
never mistakes a lane an agent is actively working, or idling between
phases, for one that is gone.

## Context

**The contradiction, observed.** On 2026-09-03 `frit board --json`
reported plan 2609021313 with `dead: true` beside `agent: claude,
agent_status: working` — and `dead: true` in `frit ready --json` too —
while `frit pick`/`frit start` refused it with "deserted hold: its
branch carries an unparked suffix; run `frit yield 2609021313` to park
it first". The pane was open the whole time. The agent was actively
working — it went on to finish the plan and land it — and idle between
phases at other moments. Read together, `dead: true` and a live working
agent are a flat contradiction. "dead" plus a teardown-shaped remedy
win the impression: the lane looks gone.

**Why both fields are "true" at once.** They answer different
questions, both to herdr. board's
[`Dead`](../../internal/report/board.go) marks "the session the lease
marker binds is confirmed gone" — an identity fact, set by
`observeHolds` from herdr's session check. board's `Agent` and
`AgentStatus` mark "a live pane herdr found on the lane now" — a
different herdr query, the pane list. A lane whose originally-bound
session rotated away while a live pane still sits on its branch is
`Dead` and attended at once. The model is self-consistent; the render
is not, because `dead`'s plain meaning to a reader is "nobody is here",
which the live pane disproves.

**Where the render and the refusals decide.** Two render paths carry a
`dead` field straight from `p.Dead`, neither reconciled with a live
pane. board, through [board.go](../../internal/report/board.go), takes
the live pane's `agent`/`agent_status` and prints it beside a `dead`
that ignores it. The discovery report, through `cardOf` in
[discovery.go](../../internal/report/discovery.go), backs `ready`,
`pick` **and** `find` from one site, and its card does not carry the
agent at all — so that render has no live-pane fact to consult, and
closing it means plumbing the fact into `cardOf`, not just gating a
copy. Reconciling at `cardOf` closes all three discovery reports at
once, `find` included. [orphans.go](../../internal/report/orphans.go)
is left out of scope on purpose. It says "dead" twice — a
`StaleHold.Dead` field and the membership-based `Deserted` category.
Listing a bound-session-gone lane for cleanup is a defensible thing for
a teardown verb to do. Reconciling it is a separate question from the
survey reports a person reads to decide.
[`desertedRefusal`](../../cmd/frit/start.go) and
[`parkFirstRefusal`](../../cmd/frit/start.go) fire on `plan.Held &&
plan.Dead && !plan.Stale` and never ask whether a pane attends the
lane — though [`liveLaneFor`](../../cmd/frit/dispatch.go), which finds
exactly that pane and carries its `PaneID`, already runs a few checks
later in `buildStart` and is what
[`liveLaneRefusal`](../../cmd/frit/start.go) uses to name a pane. The
fact needed to tell "gone" from "working or idle" is in hand at the
refusal sites, and one herdr read from filling it at the render sites;
the messages just do not consult it.

**Two Deads, kept apart.** `discovery.Plan.Dead` is a decision input
the takeover and refusal logic reads; the lease protocol still needs
"the bound session is confirmed gone" to classify a takeover. This
plan does not change that identity fact or any takeover decision. It
changes what board *renders* and what the deserted refusals *say* when
a live pane attends the lane — the reporting layer, not the protocol.
The JSON Contract holds: every key stays present, and the plan decides,
with the contract in view, whether `dead`'s render is gated on the live
pane or a companion signal carries "attended" (board already prints
`agent`/`agent_status`, so a consumer has the raw fact either way).

**What is reused.** `liveLaneFor` is the one live-pane read both the
render path and the refusals fold in; no second herdr query is added.
The refusal that already reads right for a live pane —
`liveLaneRefusal`, naming the pane — is the wording the deserted
refusals move toward. The godog row reuses the herdr-fake vocabulary
and the cross-layer step file plan 2609021314 landed and plan
2609031211 extends; this plan depends on 2609031211, so it builds on
that plan's walk-advance fix and its S88, and takes the next free id,
S89, in the same "Cross-layer" section.

**Why depends-on 2609031211.** That plan makes `pick --go`'s walk
advance past these refusals rather than stall on them. It edits the
same start.go region and the same cross-layer matrix and feature
files. Landing it first keeps this plan's edits clean and its S-ids
from colliding. It leaves this plan its true remainder: the `dead`
render everywhere, and the deserted-refusal wording, which still
surfaces for an explicit `start <id>` even after the walk advances.

**Out of scope.** No change to the lease protocol, to how a takeover
is decided, or to `discovery.Plan.Dead` as a decision input. `pick`'s
walk-advance behavior is 2609031211's, not this plan's.

## Tasks

1. Phase 1 (proving slice): the `dead` render — board, and `cardOf`
   in discovery.go behind `ready`, `pick` and `find` — stops reporting
   `dead: true` for a held lane whose bound session is gone but a live
   pane attends it, whether that pane is working or idle. Reproduce the
   contradiction on the observed working-agent case first, then
   reconcile board's render and `cardOf` with the live pane — plumbing
   the fact into the discovery card, which does not yet carry it —
   deciding the field shape against the JSON Contract.
2. Phase 2: the deserted refusals name the live pane and lead with
   resume, not yield, when a pane attends the lane. Reproduce the
   teardown-first wording for an explicit `start <id>` first, then fold
   `liveLaneFor` into `desertedRefusal` and `parkFirstRefusal`.
3. Phase 3: document the state as cross-layer matrix row S89 and run it
   end-to-end under godog — board reports the attended lane as attended,
   and the refusal names the pane — so the contradiction cannot return.

## Execution

| Phase | Title                                                    | Tier   | Gate                                                                                                                                                           |
| ----- | -------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | no survey report calls an attended lane dead             | sonnet | New unit tests: board and the discovery render (ready/pick/find) do not mark a held lane, bound session gone, live working pane, as dead; suite and lint clean |
| 2     | the deserted refusals name the pane and lead with resume | sonnet | New unit test: an explicit `start <id>` on a lane a live pane attends names the pane and does not lead with `frit yield`; suite and lint clean                 |
| 3     | S89 runs the attended-lane state end-to-end under godog  | sonnet | `TestFeatures/S89:` passes with no SKIP; `go test ./internal/scenario` bijection stays green; `go test ./...` and lint clean; `mdsmith check .` passes         |

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

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                                                                        |
| --- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [no report calls an attended lane dead](phase-1.md)                                                                                                                                                                                                                                                                                                                                          |
|     | ↳      | board and the discovery card behind ready, pick and find no longer render `dead: true` for a held lane whose bound session herdr confirms gone but whose branch a live pane still attends, working or idle. The rendered field is gated on the live pane rather than copied straight from the identity fact, applied once at each render's shared site so ready, pick and find cannot drift. |
| 2   | 🔲     | [the deserted refusals name the pane and lead with resume](phase-2.md)                                                                                                                                                                                                                                                                                                                       |
| 3   | 🔲     | [S89 runs the attended-lane state end-to-end under godog](phase-3.md)                                                                                                                                                                                                                                                                                                                        |
<?/catalog?>

## Acceptance Criteria

- [ ] `frit board --json`, `frit ready --json`, `frit pick --json` and
      `frit find --json` over a held lane whose bound session herdr
      confirms gone but whose branch a live pane attends do not report
      that lane as `dead: true`, whether the pane is working or idle;
      board's `agent`/`agent_status` still shows the live pane
- [ ] The JSON Contract holds: every documented key is present, and the
      chosen render for the attended-lane case is deliberate, not a
      dropped or nulled field; where the discovery card did not carry
      the live-pane fact, it is plumbed into `cardOf` rather than
      guessed
- [ ] `frit orphans` is deliberately unchanged: its `Deserted` category
      still lists a bound-session-gone lane, a separate cleanup concern
- [ ] An explicit `start <id>` refusal for a lane a live pane attends
      names that pane and leads with resuming it, not with `frit yield`
- [ ] A lane with no live pane still reads and refuses exactly as before
      — the deserted wording and the yield remedy are unchanged there
- [ ] Cross-layer matrix row S89 is documented in
      [docs/research/lease-protocol.md](../../docs/research/lease-protocol.md),
      and a `@S89` scenario in
      [features/cross-layer.feature](../../features/cross-layer.feature)
      reproduces the state and runs for real, not `@pending`
- [ ] `go test ./cmd/frit -run 'TestFeatures/S89:'` reports S89 PASS,
      not SKIP; `go test ./internal/scenario` stays green
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
