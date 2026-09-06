---
id: 2609061129
title: The lane-facing skills tell an agent its plan id is inferred in-lane
status: "✅"
summary: >-
  frit infers a plan from the cwd whenever a verb runs in that plan's
  own lane: the worktree's branch is matched back to the id through
  the holds patterns, so a bare frit phase, show, next or yield needs
  no selector. The behavior is documented in --help and in
  docs/ux-principles.md and docs/claiming.md, but not where an in-lane
  agent looks. Every lane-facing skill — plan-phase, plan-handoff,
  plan-tidy, plan-drive — shows an explicit <id> on every command and
  never mentions inference, so an agent standing in its lane does not
  know it can omit the id. Add one terse line to those skills, through
  the canonical asset and its regenerated dogfood copy, so the surface
  the agent reads carries the fact the reference docs already hold.
model: sonnet
depends-on: []
---
# The lane-facing skills tell an agent its plan id is inferred in-lane

## Goal

An agent following a lane-facing skill learns, from the skill itself,
that when it runs a verb in the plan's own lane the id is inferred
from the branch and can be omitted. The knowledge reaches the surface
the agent actually reads, not only `--help` and the reference docs.

## Context

**The gap, from a real miss.** An agent working in a lane did not know
`frit phase` resolves its own plan with no selector. It does:
`resolveSelector` in [cmd/frit/main.go](../../cmd/frit/main.go) infers
the plan from the cwd when the selector is empty, matching the
worktree's branch back to the id through the repo's holds patterns
(`fleet.CurrentLane` in
[internal/fleet/current.go](../../internal/fleet/current.go)). Eleven
verbs carry an optional selector whose `--help` says "empty infers
from the cwd".

**The docs are not stale — they are elsewhere.** The behavior is
correct and documented: [docs/ux-principles.md](../../docs/ux-principles.md)
("the empty form is inferred … needs no argument"),
[docs/claiming.md](../../docs/claiming.md) ("frit infers the plan from
the branch"), and the design research. None of that reaches an in-lane
agent mid-task. The one surface it loads is the skill.

**Where the fix goes.** Every lane-facing skill shows an explicit
`<id>` on every command and never mentions inference:
[plan-phase](../../.claude/skills/plan-phase/SKILL.md) (`phase <id>`,
`show <id>`), [plan-handoff](../../.claude/skills/plan-handoff/SKILL.md)
(`phase <id>`), [plan-tidy](../../.claude/skills/plan-tidy/SKILL.md)
(`yield <id>`), [plan-drive](../../.claude/skills/plan-drive/SKILL.md)
(`nudge <id>`). plan-phase's own Inputs says "Plan id, or enough of
the title to resolve it", never the no-arg in-lane form.

**Reuse first.** No new machinery. The canonical text lives in
[internal/skills/assets](../../internal/skills/assets); `frit skills`
regenerates the dogfood copy, guarded by
`TestDogfoodCopiesMatchCanonical`. A shipped command reads `{{frit}}`,
filled by `--via`. This plan edits the canonical asset and regenerates,
it does not hand-edit the copy.

**Cost of the words.** The `skill` kind caps each skill at 650
heuristic tokens (MDS028). The note must be terse — one clause where it
already fits, not a new section in each skill.

**Out of scope.** No change to the inference behavior. No change to
`--help` or the reference docs, which are already correct.

## Tasks

1. Phase 1 (proving slice): plan-phase's canonical asset gains one
   terse line — in the plan's own lane the selector is inferred from
   the branch, so the id can be omitted. Regenerate its dogfood copy.
   Prove `{{frit}} phase` with no selector resolves from inside a real
   lane, and the skill still passes its token budget.
2. Later phase: the same terse note in plan-handoff, plan-tidy and
   plan-drive, each within its own token budget.

## Execution

| Phase | Title                                  | Tier   | Gate                                                                                                                                                                |
| ----- | -------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | plan-phase names the in-lane inference | sonnet | from inside a real lane the built `frit phase` with no selector resolves that plan; the regenerated dogfood copy matches; `mdsmith check` and `go test ./...` green |

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

| #   | Status | Phase                                                                                                                   |
| --- | ------ | ----------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [plan-phase names the in-lane inference](phase-1.md)                                                                    |
|     | ↳      | plan-phase's Inputs now names the in-lane inference; dogfood copy regenerated; gate confirmed against the built binary. |
<?/catalog?>

## Acceptance Criteria

- [x] plan-phase's canonical asset states that, in the plan's own
      lane, the selector is inferred and the id can be omitted
- [x] The claim is confirmed against the built frit: `frit phase` with
      no selector, run from inside a lane, resolves that lane's plan
- [x] The dogfood copy is regenerated, not hand-edited, and
      `TestDogfoodCopiesMatchCanonical` is green
- [x] Every touched skill stays within its 650-token budget
      (`mdsmith check`)
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
