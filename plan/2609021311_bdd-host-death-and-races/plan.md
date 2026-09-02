---
id: 2609021311
title: The host-death and race scenarios run under godog
status: "🔳"
summary: >-
  The lease-protocol matrix's "Host death, suspension, zombies" section
  (S14..S19) and its "Races" section (S26..S32) are declared in
  features/host-death.feature and features/races.feature, but every
  row except S16 is still @pending: declared, skipped, proving nothing.
  This plan writes each of the twelve as a real Given/When/Then over
  the lease API and the cmd/frit fixtures, bound in the two sections'
  shared step file, so a regression in what the doc promises for a
  dead host, a zombie or a race fails the build. It stands alone: no
  other conversion plan is a prerequisite, and none waits on it.
model: sonnet
depends-on: []
---
# The host-death and race scenarios run under godog

## Goal

Every row of the matrix's host-death section (S14..S19) and races
section (S26..S32) is a passing godog scenario. None is tagged
`@pending`. A regression in any promise the two sections make — one
CAS winner, a fenced zombie, a takeover that waits for the window —
fails `go test ./...`.

## Context

**The gap.** Plan 2609012000 stood the harness up: S16 runs for real.
[host-death.feature](../../features/host-death.feature) declares S14,
S15, S17, S18 and S19 with `@pending`, and
[races.feature](../../features/races.feature) declares S26..S32 the
same way. `TestFeatures` skips each one. These are the rows the doc
leans on hardest — a host that never comes back, a zombie re-running
its claim, N claimants on one ref — and today nothing executes a
single one of them.

**What already exists, and is reused.** The lease API in
[internal/claim](../../internal/claim) covers most of both sections:
`Acquire`, `Takeover`, `Renew`, `Release`, `Yield`, `RemoteTip`,
`ReadMarker`. S16 is real, and its vocabulary in
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) already says
"holds the lease", "commits work on the lane it never pushes", "takes
the lease over", "push of that work is rejected" and "yield parks";
those steps are reused as they stand. `claimableRepo` and
`cloneAgain` build the origin-and-clone pair. For the verb-level
rows, `withHerdr` and `herdrReturning` in
[who_test.go](../../cmd/frit/who_test.go) and `startHerdr` in
[start_test.go](../../cmd/frit/start_test.go) fake the daemon. The
observation window needs no clock seam: `resetWindow` in
[claim.go](../../cmd/frit/claim.go) and `observeHolds` in
[main.go](../../cmd/frit/main.go) both take an explicit `now`. One
unit test already pins a row by number — `TestAcquireIsRenameProof`
in [lease_test.go](../../internal/claim/lease_test.go) is S27 — and
each scenario mirrors the unit test's fixture rather than inventing
one.

**The rows, triaged.** Three shapes, and the phase order follows them.

- Lease-API only, drivable now: S26 (two more claimants lose to one
  winner and each names it), S27 (a rename between claimants reaches
  no ref, so there is one ref and one winner), S28 (a human deletes
  the ref; the loser's retry claims it fresh at epoch 1), S30 (a
  zombie's raw push against a new claimant's takeover is refused as
  non-fast-forward), S17 (a holder suspended through a takeover, a
  release and a re-claim is fenced by the re-claim and parked by
  yield), S19 (a renewal against a completed plan's absent ref fails
  its CAS; the raw push that then lands is TRUST).
- Verb-level, with the window, the resume path or the herdr fake:
  S14 (the lane resumes on its own token after a push it never saw
  confirmed; a broken local ref stays local), S15 (a matured window
  takes over with no human step), S18 (a resume is refused while a
  live session owns the lane), S31 (orphans reports a sleeping host;
  a claim before the window matures refuses; a waking session vetoes
  the takeover), S32 (two lanes on one host: the refusal names the
  winning lane).
- Needs an injected race: S29 (the holder releases between the
  loser's failed CAS and its reconciliation read). The `gitwt.Runner`
  type is a function, so a wrapper around `gitwt.Exec` can run the
  release on the loser's first read. That wrapper is test scaffolding
  in the step file, not a seam added to the verbs.

**Convention for a row the doc resolves by argument.** S19 says a raw
push "is TRUST" and S28 says a hand-deleted ref "is TRUST". The
scenario asserts the observable: origin accepts the raw push, the
retry acquires at epoch 1, the fenced holder's next verb refuses.
Every such row becomes an assertion about what a verb or the remote
shows, never a comment.

**Where the steps go.** Both sections' steps live in one new
`cmd/frit/bdd_host_death_and_races_test.go`, appended to the step
registry from `init` exactly as `bdd_lease_test.go` is. One file
rather than two, because the zombie and race rows share a vocabulary
— a takeover, a fenced push, a loser reading the marker — and one
world threads it. This plan never edits `bdd_test.go`, and touches no
feature file but its two. The sibling conversion plans — process
death, partitions and clocks, storage, identity and cross-layer, the
two lifecycle halves — each own their file the same way, so all seven
land in any order. A step text this file defines that another already
has fails as ambiguous under godog's strict mode; the fix is to reuse
the existing step.

**Out of scope.** No change to the lease protocol or to any verb. A
scenario that cannot be made to pass without changing behaviour is a
finding, parked in the handoff with the row it concerns, not a fix
made here.

## Tasks

1. Phase 1 (proving slice): the six lease-API rows — S17, S19, S26,
   S27, S28, S30 — written and passing in
   `bdd_host_death_and_races_test.go`, the file registered, the
   pattern for a doc-by-argument row set by S19. Driven red by
   dropping `@pending`: strict mode fails the undefined steps.
2. Later phases, shaped by Phase 1's handoff: the verb-level rows
   S14, S15, S18, S31 and S32 over the resume path, the explicit-time
   window and the herdr fake; then S29 over a runner wrapper that
   injects the release into the loser's read.

## Execution

| Phase | Title                                                       | Tier   | Gate                                                                                                                                                             |
| ----- | ----------------------------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | The six lease-API rows of host death and races run for real | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(17\|19\|26\|27\|28\|30)_'` passes with no SKIP; the bijection gate stays green; `go test ./...` and golangci-lint clean |

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

| #   | Status | Phase                                                                     |
| --- | ------ | ------------------------------------------------------------------------- |
| 1   | 🔲     | [The six lease-API rows of host death and races run for real](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] No scenario in `features/host-death.feature` or
      `features/races.feature` carries `@pending`; `go test ./cmd/frit
      -run TestFeatures/S` reports S14..S19 and S26..S32 as PASS, none
      as SKIP
- [ ] Every step is bound in
      `cmd/frit/bdd_host_death_and_races_test.go` or reused from
      `bdd_lease_test.go`; `bdd_test.go` is untouched
- [ ] Each scenario asserts an observable — a verb's result, origin's
      refs, a marker — never a comment
- [ ] A finding a row exposes is recorded in the handoff with its row
      id, not fixed silently
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
