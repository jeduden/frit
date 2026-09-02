---
id: 2609021310
title: The process-death scenarios run under godog
status: "🔳"
summary: >-
  The lease-protocol matrix's "Process death, at every lifecycle step"
  section, S1..S13, is declared in features/process-death.feature but
  every row is still @pending: declared, skipped, proving nothing. This
  plan writes each of the thirteen as a real Given/When/Then over the
  lease API and the cmd/frit fixtures, bound in the section's own step
  file, so a regression in what the doc promises for an agent killed at
  any step fails the build. It stands alone: no other conversion plan
  is a prerequisite, and none waits on it.
model: sonnet
depends-on: []
---
# The process-death scenarios run under godog

## Goal

Every row of the matrix's process-death section (S1..S13) is a passing
godog scenario. None is tagged `@pending`. A regression in any promise
the section makes — a clean retry, a takeover that inherits pushed
work, scavenge acting on landed evidence — fails `go test ./...`.

## Context

**The gap.** Plan 2609012000 stood the harness up: S16 runs for real,
and [process-death.feature](../../features/process-death.feature)
declares S1..S13 with `@pending`. `TestFeatures` skips each one. The
thirteen rows are the section the doc opens with — an agent killed at
every step from before the local ref write to after the merge — and
today nothing executes a single one of them.

**What already exists, and is reused.** The lease API in
[internal/claim](../../internal/claim) covers most of the section:
`Acquire`, `Takeover`, `Release`, `Scavenge`, `RemoteTip`. The
vocabulary in [bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go)
already says "holds the lease", "commits work it never pushes", "takes
the lease over"; those steps are reused as they stand. `claimableRepo`
and `cloneAgain` build the origin-and-clone pair. `startHerdr` and
`withHerdr` fake the daemon for the one row that mentions a session.
Unit tests already pin several rows by number — `grep -rn "S3\b"
cmd/frit` finds the resume test — and each scenario mirrors the unit
test's fixture rather than inventing one.

**The rows, triaged.** Three shapes, and the phase order follows them.

- Lease-API only, drivable now: S1 and S2 (a claim killed before or
  after the local write leaves origin untouched; a retry or another
  claimant wins at epoch 1), S10 and S11 (a takeover is a child of the
  pushed tip, so pushed work is inherited and local-only work is the
  loss), S7 (a tip change or a vanished ref resets an observation).
- Verb-level, with the observation window and the resume path: S3 and
  S4 (the same lane reclaims instantly; another host waits the window
  and takes over), S5 (board shows a held lane with no session), S6
  (an idle agent never renews; the veto cannot fire because there is
  no bound session).
- Scavenge and unwind: S8 (an unwind whose remote delete fails leaves
  a release marker, never a dangling hold), S9 (unwind deletes only a
  landed ref), S12 (scavenge acts on landed evidence with no window),
  S13 (a status flipped on the branch is not evidence; only origin's
  default branch is).

**Convention for a row the doc resolves by argument.** S1 says
"nothing shared happened". The scenario asserts the observable: origin
has no work ref, and the retry acquires at epoch 1. Every such row
becomes an assertion about what a verb or the remote shows, never a
comment.

**Where the steps go.** The section's steps live in a new
`cmd/frit/bdd_process_death_test.go`, appended to the step registry
from `init` exactly as `bdd_lease_test.go` is. This plan never edits
`bdd_test.go`, and touches no feature file but its own. The sibling
conversion plans — host death and races, partitions and clocks,
storage, identity and cross-layer, the two lifecycle halves — each
own their file the same way, so all seven land in any order. A step
text this section defines that another already has fails as ambiguous
under godog's strict mode; the fix is to reuse the existing step.

**Out of scope.** No change to the lease protocol or to any verb. A
scenario that cannot be made to pass without changing behaviour is a
finding, parked in the handoff with the row it concerns, not a fix
made here.

## Tasks

1. Phase 1 (proving slice): the five lease-API rows — S1, S2, S7, S10,
   S11 — written and passing in `bdd_process_death_test.go`, the file
   registered, the pattern for a doc-by-argument row set by S1. Driven
   red by dropping `@pending`: strict mode fails the undefined steps.
2. Later phases, shaped by Phase 1's handoff: the verb-level rows S3,
   S4, S5, S6 over the resume path and the herdr fake; then the
   scavenge and unwind rows S8, S9, S12, S13 over `Scavenge` and the
   landed-evidence fixtures.

## Execution

| Phase | Title                                                 | Tier   | Gate                                                                                                                                                      |
| ----- | ----------------------------------------------------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | The five lease-API rows of process death run for real | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(1\|2\|7\|10\|11)_'` passes with no SKIP; the bijection gate stays green; `go test ./...` and golangci-lint clean |

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

| #   | Status | Phase                                                               |
| --- | ------ | ------------------------------------------------------------------- |
| 1   | 🔲     | [The five lease-API rows of process death run for real](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] No scenario in `features/process-death.feature` carries
      `@pending`; `go test ./cmd/frit -run TestFeatures/S` reports
      S1..S13 as PASS, none as SKIP
- [ ] Every step is bound in `cmd/frit/bdd_process_death_test.go` or
      reused from `bdd_lease_test.go`; `bdd_test.go` is untouched
- [ ] Each scenario asserts an observable — a verb's result, origin's
      refs, a marker — never a comment
- [ ] A finding a row exposes is recorded in the handoff with its row
      id, not fixed silently
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
