---
id: 2609061025
title: frit yield's command-surface outcomes are proven by command scenarios
status: "🔳"
summary: >-
  The BDD scenarios drive frit yield's fence-and-park lease mechanics
  across hosts, and plan 2609052303 added S93 for the cross-host
  yield-honesty behavior. But the frit yield command surface a
  developer meets on one host is still unproven by any scenario:
  yielding a plan nobody holds is a clean no-op, yielding one's own
  live lane is refused toward release, and the table and --json a
  refusal carries. These are single-host command outcomes, not lease
  races, so their home is command-scenarios.md's C<n> catalog. This
  plan gives them their scenarios, reusing the yield command handlers
  performYield, yieldError and yieldNothingLocal that unit tests reach
  today but no scenario does.
model: sonnet
depends-on: []
---
# frit yield's command-surface outcomes are proven by command scenarios

## Goal

`frit yield`'s single-host command outcomes are proven by command
scenarios: the clean no-op on a plan nobody holds, the refusal of
one's own live lane toward `release`, and the shape a refusal carries.
A developer reading `yield` sees each outcome driven by the real
command.

## Context

**The gap.** A BDD-only coverage run drives `yield`'s park mechanics
through the cross-host world, but leaves the command surface dark —
`performYield`, `yieldError`, `yieldNothingLocal`, `tearDownLane` in
[cmd/frit/yield.go](../../cmd/frit/yield.go). Plan 2609052303 added
`S93` for the cross-host yield-honesty behavior — a distant host
refusing a hold it never fetched. The single-host outcomes a developer
meets are still unproven.

**The home.** These outcomes are one host computing a refusal or a
no-op, with no lease race. That is exactly what
[command-scenarios.md](../../docs/research/command-scenarios.md), the
`C<n>` catalog plan 2609052303 landed, exists for. `C1` there is
already `release` on a plan nobody held — a no-op, not a refusal. The
`yield` no-op is its sibling. This plan writes `C<n>` rows, not `S<n>`
ones, so the lease matrix stays cross-host.

**Reuse first.** The command feature file and its steps stand in
[features/commands.feature](../../features/commands.feature) and
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go).
The yield handlers are reused unchanged: no yield behavior is added,
only scenarios over code that already passes its unit tests. `S93`
already covers the foreign-hold refusal, so this plan does not repeat
it — it covers the outcomes `S93` does not.

**C-id allocation.** Merge `main`, read the catalog, allocate the next
free `C<n>`. Never a hardcoded id across lanes.

**Out of scope.** The cross-host yield-honesty refusal, which is
`S93`. No change to yield's behavior.

## Tasks

1. Phase 1 (proving slice): a `C<n>` scenario for the clean no-op — a
   `frit yield` on a plan nobody holds parks nothing and refuses
   nothing. Establishes the yield command-surface step vocabulary.
2. Later phases: yielding one's own live lane is refused toward
   `release`; the table and `--json` a yield refusal carries.

## Execution

| Phase | Title                                         | Tier   | Gate                                                                                                                                      |
| ----- | --------------------------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | Yield on a plan nobody holds is a clean no-op | sonnet | the new `C<n>` scenario runs against the built frit, yield parks nothing and refuses nothing; bijection gate green; `go test ./...` green |

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

| #   | Status | Phase                                                       |
| --- | ------ | ----------------------------------------------------------- |
| 1   | ✅     | [Yield on a plan nobody holds is a clean no-op](phase-1.md) |
|     | ↳      | C3 proves yield's clean no-op on a plan nobody holds        |
<?/catalog?>

## Acceptance Criteria

- [x] A `C<n>` scenario drives the real `frit yield` on a plan nobody
      holds and shows it parking nothing and refusing nothing
- [ ] A `C<n>` scenario shows `frit yield` on one's own live lane
      refused and pointed at `release`
- [ ] The scenarios do not duplicate `S93`'s cross-host foreign-hold
      refusal
- [ ] The bijection gate `go test ./internal/scenario` is green
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
