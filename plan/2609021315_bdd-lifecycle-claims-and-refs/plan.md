---
id: 2609021315
title: The lifecycle claim-and-ref scenarios run under godog
status: "🔳"
summary: >-
  The claim-and-ref half of the lease-protocol matrix's "Lifecycle
  anomalies" section — S50, S51, S52, S53, S55, S56, S57, S58, S70 and
  S75 — is declared in features/lifecycle.feature, every row still
  @pending: declared, skipped, proving nothing. This plan writes each
  of the ten as a real Given/When/Then over the lease API, the claim
  verb and the cmd/frit fixtures, bound in the section's own step
  file, so a regression in what the doc promises for a renamed,
  deleted, reused or re-opened plan, or a ref gone or dated against an
  old base, fails the build. It stands alone: no other conversion plan
  is a prerequisite, and none waits on it.
model: sonnet
depends-on: []
---
# The lifecycle claim-and-ref scenarios run under godog

## Goal

Every row of the matrix's lifecycle claim-and-ref half (S50, S51, S52,
S53, S55, S56, S57, S58, S70, S75) is a passing godog scenario. None
is tagged `@pending`. A regression in any promise the half makes — an
id-only ref, origin as the only authority, a base fetched at claim
time — fails `go test ./...`.

## Context

**The gap.** Plan 2609012000 stood the harness up: S16 runs for real,
and [lifecycle.feature](../../features/lifecycle.feature) declares the
ten rows with `@pending`. `TestFeatures` skips each one. The twenty-row
"Lifecycle anomalies" section is split across two feature files. This
plan owns `lifecycle.feature`, the claim-and-ref half. The sibling
file, `landed-evidence.feature`, belongs to another plan and is not
touched here.

**What already exists, and is reused.** The lease API in
[internal/claim](../../internal/claim) covers most of the half:
`Acquire`, `Renew`, `Release`, `Scavenge`, `RemoteTip`, `Released`.
`claim.Branch(id)` is the id-only ref. Nothing derived from a file
name reaches it, which is the whole answer to S50 and S51. The unit
test `TestAcquireIsRenameProof` in
[lease_test.go](../../internal/claim/lease_test.go) already pins the
sibling row S27. S50 mirrors its fixture. `Acquire` dates a claim
against `opts.Base` as it stands locally. The claim verb gathers
first. The gather's fetch in
[gather.go](../../internal/fleet/gather.go) refreshes `origin/main`
before the acquire. That is S70's seam, so S70 drives the verb, not
the API alone. `DefaultRef` in
[internal/gitobj/git.go](../../internal/gitobj/git.go) reads
`refs/remotes/origin/HEAD` on every call, uncached. That is S75's
seam. The fixtures in cmd/frit tests are reused as they stand.
`claimableRepo` and `cloneAgain` build the origin-and-clone pair.
`resumableRepo` is a 🔳 plan nobody holds. `landedLeaseRepo` is a
landed ref left behind. `landedDeletedClone` is a clone whose origin
deleted the ref. `commitPlan` writes a second plan file. `gitCapture`
reads any ref. No unit test cites S50..S58, S70 or S75 by number.
Each scenario mirrors the nearest unit test's fixture rather than
inventing one.

**The rows, triaged.** Three shapes, and the phase order follows them.

- Drivable now, over the lease API and git on origin: S50 (a plan file
  renamed after the claim leaves one ref, and a second acquire loses),
  S51 (two plans sharing a slug get two refs, each named by id), S56 (a
  local work ref deleted by hand changes nothing origin decides), S70
  (the claim verb dates its lease against origin's current base, not
  the clone's stale copy), S75 (a default branch renamed on origin is
  what `DefaultRef` answers after the clone re-reads origin's HEAD).
- Scavenge, with evidence and windows: S52 (a plan deleted while
  claimed: park first, scavenge later), S53 (an old ref under a reused
  id is scavenged by evidence, then acquired fresh), S55 (a ref gone
  after merge reads as released; a 🔳 unheld plan is claimable), S57
  (a plan re-opened after done: the landed ref is scavenged, the
  acquire is fresh).
- Doc-by-argument: S58 (a release before the PR merges opens a window
  in which another claim is legitimate; that is human process).

**Convention for a row the doc resolves by argument.** S58 says
"human process, TRUST". The scenario asserts the observable: after the
release a second claim succeeds at epoch 2, origin's tip is that
claim, and its marker names the new holder. Every such row becomes an
assertion about what a verb or the remote shows, never a comment.

**Where the steps go.** The section's steps live in a new
`cmd/frit/bdd_lifecycle_test.go`, appended to the step registry from
`init` exactly as
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) is. This plan
never edits `bdd_test.go`, and touches no feature file but its own.
The sibling conversion plans — process death, host death and races,
partitions and clocks, storage, identity and cross-layer, the
landed-evidence half — each own their file the same way, so all land
in any order. Every registrar binds on the one world a scenario
threads, so a step text bound in `bdd_lease_test.go` runs on the same
state this section's steps read. A lifecycle scenario reuses the lease
texts as they stand and binds only what its rows add, in its own
file; what it tracks beyond the shared world lives in a struct reached
through `section[T]`. A text this section defines that another file
already has fails as ambiguous under godog's strict mode; the fix is
to reuse the existing text, not to bind a second.

**Out of scope.** No change to the lease protocol, to `DefaultRef`, or
to any verb. `landed-evidence.feature` and its rows are another plan's.
A scenario that cannot be made to pass without changing behaviour is
a finding, parked in the handoff with the row it concerns, not a fix
made here. Two seams look like findings already and are checked, not
assumed: no code names "plan-gone" evidence for S52, and the gather's
`fetch --prune` does not by itself move `origin/HEAD` on git 2.47.

## Tasks

1. Phase 1 (proving slice): the five drivable rows — S50, S51, S56,
   S70, S75 — written and passing in `bdd_lifecycle_test.go`, the file
   registered, the section's world fixed. Driven red by dropping
   `@pending`: strict mode fails the undefined steps.
2. Later phases, shaped by Phase 1's handoff: the scavenge rows S52,
   S53, S55, S57 over `Scavenge`, `Released`, `resumableRepo`,
   `landedLeaseRepo` and `landedDeletedClone`; then the
   doc-by-argument row S58 over `Release` and a second `Acquire`.

## Execution

| Phase | Title                                                       | Tier   | Gate                                                                                                                                                         |
| ----- | ----------------------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1     | The five drivable lifecycle claim-and-ref rows run for real | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(50\|51\|56\|70\|75)_'` passes with no SKIP; the bijection gate stays green; `go test ./...` and golangci-lint clean |

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
| 1   | 🔲     | [The five drivable lifecycle claim-and-ref rows run for real](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] No scenario in `features/lifecycle.feature` carries `@pending`;
      `go test ./cmd/frit -run TestFeatures/S` reports S50, S51, S52,
      S53, S55, S56, S57, S58, S70 and S75 as PASS, none as SKIP
- [ ] Every step is bound in `cmd/frit/bdd_lifecycle_test.go`;
      `bdd_test.go`, `bdd_lease_test.go` and
      `features/landed-evidence.feature` are untouched
- [ ] Each scenario asserts an observable — a verb's result, origin's
      refs, a marker — never a comment
- [ ] A finding a row exposes is recorded in the handoff with its row
      id, not fixed silently
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
