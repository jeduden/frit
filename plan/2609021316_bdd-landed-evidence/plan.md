---
id: 2609021316
title: The landed-evidence scenarios run under godog
status: "🔳"
summary: >-
  The lease-protocol matrix's landed-evidence half of "Lifecycle
  anomalies" — S54, S59, S79..S85, S87 — is declared in
  features/landed-evidence.feature but every row is still @pending:
  declared, skipped, proving nothing. This plan writes each of the ten
  as a real Given/When/Then over the lease API, DefaultRef, Gather and
  reap, bound in the section's own step file, so a regression in how
  frit decides work has landed fails the build. It stands alone: no
  other conversion plan is a prerequisite, and none waits on it.
model: sonnet
depends-on: []
---
# The landed-evidence scenarios run under godog

## Goal

Every row of the matrix's landed-evidence half of "Lifecycle
anomalies" (S54, S59, S79, S80..S85, S87) is a passing godog
scenario. None is tagged `@pending`. A regression in any promise the
section makes — squash-landed content read as landed, a stale local
`main` never deciding evidence, an unreadable origin never read as
"gone" — fails `go test ./...`.

## Context

**The gap.** Plan 2609012000 stood the harness up: S16 runs for real,
and [landed-evidence.feature](../../features/landed-evidence.feature)
declares its ten rows with `@pending`. `TestFeatures` skips each one.
The rows are how scavenge, reap and the read verbs decide that work
has landed, and what they refuse to do without that evidence. Today
nothing executes a single one of them.

**What already exists, and is reused.** The landed check lives in
[internal/claim](../../internal/claim). `Scavenge`, `ParkUnlanded`,
`HasUnlanded`, `WorkLanded` and `ContentLanded` are in
[lease.go](../../internal/claim/lease.go). They sit over the
unexported `hasWork` and `landedByContent`. The latter is the
`merge-tree --write-tree` no-op check. `landedTip` and `isAncestor`
in [claim.go](../../internal/claim/claim.go) are the ancestry half.
`Scavenge` refuses on an unreadable remote. It never reads the ref as
gone. It also gates every local `update-ref -d` on a fresh
`gitwt.List` read, through `checkedOut`. `DefaultRef` in
[gitobj/git.go](../../internal/gitobj/git.go) reaches
`refs/remotes/origin/main` before any local `main`. `Gather` in
[fleet/gather.go](../../internal/fleet/gather.go) fetches `--prune`
through `fetchRemote`. It names a failed fetch through `staleFetch`.
It reports a lagging local default branch through
`laggingDefaultBranch`. The global `--fetch` flag is in
[cmd/frit/main.go](../../cmd/frit/main.go). `reap` in
[cmd/frit/reap.go](../../cmd/frit/reap.go) parks through
`parkBranch` before any `branch -D`. It decides through
[internal/reap](../../internal/reap).

The vocabulary in
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) already says
"holds the lease for plan". It also says "commits work on the lane it
never pushes". Those steps are reused as they stand. `claimableRepo`
and `cloneAgain` build the origin-and-clone pair. `claimableRepo`
never sets `origin/HEAD`, which is the S85 shape for free. The
cmd/frit fixtures already build the verb-level shapes: `landPlan`,
`landedLeaseRepo`, `doneGlyphRepo`, `landedDeletedClone`,
`strandedCheckout`, `addOrigin` and `deadHold`. `squashLandOnMain` in
[internal/claim/lease_test.go](../../internal/claim/lease_test.go)
is not importable from cmd/frit. A step reproduces its four git
commands. Unit tests already pin rows by number. `grep -rn "S80\b"
internal` finds the lag test. Each scenario mirrors the unit test's
fixture rather than inventing one.

**The rows, triaged.** Three shapes, and the phase order follows them.

- Lease-API and `DefaultRef`, drivable now: S54 (squash-landed
  content scavenged clean with no glyph), S83 (an unreadable origin is
  a fault, local ref kept), S84 (evidence reads origin's default
  branch, never a lagging local `main`), S85 (`DefaultRef` reaches the
  remote-tracking ref with `origin/HEAD` unset).
- Verb-level, over `reap` and `Gather`: S79 (a branch a worktree
  stands on keeps its local copy), S80 (a local default branch behind
  its fetched remote-tracking ref is a named problem), S81 (an
  unstaffed hold with a live holder is refused), S82 (a follow-up
  commit is parked before `branch -D`; a refused park refuses the
  teardown), S87 (`--fetch` refreshes before reading; `--no-fetch`
  names the staleness).
- S59, the row the doc hands to `doctor`. `internal/doctor` has no
  early-✅ check today; its checks are goal, schema, execution-row,
  tier, id-sync and phase-n-sync. The scenario asserts the observable
  that exists: a dependent of a hand-flipped ✅ reads as ready. The
  missing check is a finding for the handoff, not a fix made here.

**Convention for a row the doc resolves by argument.** S84 says a
local `main` is "never authoritative". The scenario asserts the
observable: with local `main` behind, evidence still reads the work
as landed and the scavenge parks nothing. Every such row becomes an
assertion about what a verb or the remote shows, never a comment.

**Where the steps go.** The section's steps live in a new
`cmd/frit/bdd_landed_evidence_test.go`, appended to the step registry
from `init` exactly as `bdd_lease_test.go` is. This plan never edits
`bdd_test.go`, and touches no feature file but its own. The sibling
plan owning `features/lifecycle.feature` — the claim-and-ref half of
the same matrix section — and the other conversion plans each own
their file the same way, so all land in any order. A step text this
section defines that another already has fails as ambiguous under
godog's strict mode; the fix is to reuse the existing step.

**Out of scope.** No change to the lease protocol, to `doctor`, or to
any verb. A scenario that cannot be made to pass without changing
behaviour is a finding, parked in the handoff with the row it
concerns, not a fix made here.

## Tasks

1. Phase 1 (proving slice): the four lease-API rows — S54, S83, S84,
   S85 — written and passing in `bdd_landed_evidence_test.go`, the
   file registered, the squash-land step and the failing-remote
   `Runner` set for later rows. Driven red by dropping `@pending`:
   strict mode fails the undefined steps.
2. Later phases, shaped by Phase 1's handoff: the verb-level rows S79,
   S81, S82 over `reap` and its fixtures; S80 and S87 over `Gather`
   and the `--fetch` flag; then S59 over readiness, with the `doctor`
   gap recorded.

## Execution

| Phase | Title                                                    | Tier   | Gate                                                                                                                                                     |
| ----- | -------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | The four lease-API rows of landed evidence run for real  | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(54\|83\|84\|85)_'` passes with no SKIP; the bijection gate stays green; `go test ./...` and golangci-lint clean |
| 2     | The verb-level reap rows of landed evidence run for real | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(79\|81\|82)_'` passes with no SKIP; `go test ./...` and golangci-lint clean                                     |

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

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| --- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [The four lease-API rows of landed evidence run for real](phase-1.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
|     | ↳      | S54, S83, S84 and S85 drop `@pending` and run as real scenarios in the new `cmd/frit/bdd_landed_evidence_test.go`: a lane's work squash-merged onto the default branch is scavenged clean, with no rescue ref, once the plan's status is never flipped; an unreadable origin fails the scavenge naming the read it could not complete, leaving the local work ref exactly where it was; a lagging local `main` is never what `WorkLanded` or a scavenge decides against — `DefaultRef`'s remote-tracking answer is; and `origin/HEAD` unset, the shape `claimableRepo`'s own clone already has, is exactly where `DefaultRef` still reaches `refs/remotes/origin/main`, never `refs/heads/main`. The file registers itself the way `bdd_lease_test.go` does, reuses "holds the lease for plan" as-is, and keeps its own state in a `landedEvidenceState` reached through `section[T]` rather than a field on `world`. |
| 2   | 🔳     | [The verb-level reap rows of landed evidence run for real](phase-2.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
<?/catalog?>

## Acceptance Criteria

- [ ] No scenario in `features/landed-evidence.feature` carries
      `@pending`; `go test ./cmd/frit -run TestFeatures/S` reports
      S54, S59, S79..S85 and S87 as PASS, none as SKIP
- [ ] Every step is bound in `cmd/frit/bdd_landed_evidence_test.go`
      or reused from `bdd_lease_test.go`; `bdd_test.go` and
      `features/lifecycle.feature` are untouched
- [ ] Each scenario asserts an observable — a verb's result, origin's
      refs, a rescue ref, a reported problem — never a comment
- [ ] A finding a row exposes is recorded in the handoff with its row
      id, not fixed silently; S59's missing `doctor` check is one
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
