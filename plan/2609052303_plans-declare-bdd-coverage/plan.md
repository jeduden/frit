---
id: 2609052303
title: Plans declare and include their BDD scenario coverage
status: "🔳"
summary: >-
  A behavior that touches the lease protocol can land with unit tests
  alone and no BDD scenario, because nothing in the planning path
  surfaces the executable scenario matrix and the bijection gate is
  silent about a behavior that never got a matrix row. Plan 2609052054
  made frit yield honest on a foreign hold with only cmd-level tests;
  the cross-host behavior it added — a distant host reading a hold it
  never fetched — is exactly what the matrix exists to catalog, yet no
  S-row names it. First give that behavior its scenario, proving the
  path end to end. Then make plan authoring name its BDD coverage, so
  the next protocol-touching plan cannot skip it. Then widen the BDD
  tier so a command behaviour that is not a lease-protocol scenario has
  a home of its own, rather than being forced into the protocol catalog
  or left to unit tests alone.
model: sonnet
depends-on: []
---
# Plans declare and include their BDD scenario coverage

## Goal

A behavior worth a BDD scenario gets one, and a plan says so. A lane's
cross-host behavior lands with its `@S<n>` scenario, not unit tests
alone. Plan authoring makes the BDD decision explicit. And a command
behavior that is not a lease-protocol scenario has a tier of its own,
rather than no home at all.

## Context

**The gap, from a real miss.** Plan 2609052054 made `frit yield`
honest on a foreign, unfenced hold — a distant host, running yield
against a plan another lane holds and this host never fetched, now
refuses honestly and names the takeover, where before it reported a
false success. That behavior sits on the "steering is local,
coordination is origin" boundary the lease-protocol matrix exists to
catalog. Yet it landed with `cmd/frit` unit and integration tests
alone: no matrix row, no `@S<n>` scenario. Nothing failed, because the
bijection gate only fires when the matrix and features disagree — it
is silent about a behavior that never got a row on either side.

**Why the planning path let it through.** Neither
[plan/proto.md](../proto.md) nor the `plan-new` skill mentions the
matrix, features, or `docs/development.md`. The executable-scenario
procedure lives only in
[docs/development.md](../../docs/development.md) under "The executable
scenario matrix". So when a plan is authored, "does this touch the
lease protocol, so does it need a scenario?" is never asked.

**Reuse first.** The scenario machinery already exists and is not
rebuilt here. `internal/scenario` holds `features/` in bijection with
the matrix in
[lease-protocol.md](../../docs/research/lease-protocol.md). The gate is
`TestMatrixAndFeaturesAreInBijection`. Scenarios run from `cmd/frit`'s
`TestFeatures`; a `@pending` tag is declared but unwritten, and skipped.
The fenced-lane yield already has scenarios (S16, S20) whose steps —
`holdsTheLease`, `yieldParks`, the shared `world` — live in
[the host-death steps](../../cmd/frit/bdd_host_death_and_races_test.go)
and [the lease steps](../../cmd/frit/bdd_lease_test.go); Phase 1 reuses
that vocabulary rather than a new world. The skills dogfood
workflow — a canonical asset under `internal/skills/assets`, the
regenerated `.claude/skills` copy, and `TestDogfoodCopiesMatchCanonical`
— is the one Phase 2 edits the `plan-new` skill through.

**What Phase 1 proves.** The next free id is S93 (matrix and features
both reach S92). Phase 1 adds it: a matrix row, a tagged scenario for
the yield-honesty behavior already implemented, and its steps. It
demonstrates the Goal end to end and fixes the scenario shape the
instructions in Phase 2 point at.

**Out of scope.** No change to the yield behavior itself — Phase 1
writes a scenario over code that already passes its unit tests. Phase 3
widens where scenarios may live; it does not retro-fit scenarios onto
every existing command.

## Tasks

1. Phase 1 (proving slice): the yield-honesty behavior gets scenario
   S93 — a row in the matrix, a tagged `@S93` scenario in
   `host-death.feature`, and its steps reusing the lease world — and it
   runs green. The bijection gate stays satisfied.
2. Phase 2: plan authoring names its BDD coverage. A `CLAUDE.md` rule,
   a `plan/proto.md` cue, and a `plan-new` skill step (canonical asset
   plus regenerated dogfood copy) require every plan to decide, and
   every phase to state, its scenario coverage — pointing at S93 as the
   worked example and at `docs/development.md` for the procedure.
3. Phase 3: a command behavior that is not a lease-protocol scenario
   gets a BDD home — a second matrix/features pair, or `internal/scenario`
   widened past the hardcoded `lease-protocol.md` path — so refusal
   wording, `--json` shape and `next_action` can carry scenarios
   without being forced into the protocol catalog.

## Execution

| Phase | Title                                         | Tier   | Gate                                                                     |
| ----- | --------------------------------------------- | ------ | ------------------------------------------------------------------------ |
| 1     | The yield-honesty behavior gets scenario S93  | sonnet | `TestFeatures/^S93:` green, driving the built dispatch; bijection green  |
| 2     | Plan authoring names its BDD coverage         | sonnet | the rule shows in built `frit skills`; dogfood-match and `mdsmith` green |
| 3     | A command behavior gets a BDD home of its own | opus   | a command scenario's gate fails pre-fix; the lease bijection stays green |

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

| #   | Status | Phase                                                                      |
| --- | ------ | -------------------------------------------------------------------------- |
| 1   | ✅     | [The yield-honesty behavior gets scenario S93](phase-1.md)                 |
|     | ↳      | S93 lands — matrix row, tagged scenario, dedicated steps — all gates green |
<?/catalog?>

## Acceptance Criteria

- [x] The yield-honesty behavior of plan 2609052054 has a matrix row
      and a tagged `@S93` scenario that runs green from `TestFeatures`
- [x] The S93 scenario drives the real `frit` command and asserts the
      refusal, the named way out, the untouched lease and nothing parked
- [ ] `CLAUDE.md`, `plan/proto.md` and the `plan-new` skill require a
      plan to decide its BDD coverage and each phase to state it
- [ ] The `plan-new` instruction is in the built `frit skills` output,
      with `TestDogfoodCopiesMatchCanonical` green
- [ ] A command behavior that is not a lease-protocol scenario can
      carry a BDD scenario without a lease-protocol matrix row
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
