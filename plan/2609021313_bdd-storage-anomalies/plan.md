---
id: 2609021313
title: The storage-anomaly scenarios run under godog
status: "🔳"
summary: >-
  The lease-protocol matrix's "Storage anomalies" section — S37..S44,
  S67..S69, S71 and S78 — is declared in features/storage.feature but
  every row is still @pending: declared, skipped, proving nothing. This
  plan writes each of the thirteen as a real Given/When/Then over the
  lease API, raw git against the bare origin the fixtures build, and
  the cmd/frit fixtures, bound in the section's own step file, so a
  regression in what the doc promises when a person or a forge touches
  origin outside the fence fails the build. It stands alone: no other
  conversion plan is a prerequisite, and none waits on it.
model: sonnet
depends-on: []
---
# The storage-anomaly scenarios run under godog

## Goal

Every row of the matrix's storage-anomaly section is a passing godog
scenario. None is tagged `@pending`. A regression in any promise the
section makes — a hand-moved ref fences the holder, a forged trailer
never passes a check, a park keeps work a remote GC would reap — fails
`go test ./...`.

## Context

**The gap.** Plan 2609012000 stood the harness up: S16 runs for real,
and [storage.feature](../../features/storage.feature) declares the
thirteen storage rows with `@pending`. `TestFeatures` skips each one.
The section is the doc's trust boundary made concrete. Raw write
access to origin is outside the fence, and the doc says what frit does
when someone uses it. Today nothing executes a single row of it.

**What already exists, and is reused.** The lease API in
[internal/claim](../../internal/claim) covers most of the section:
`Acquire`, `Takeover`, `Renew`, `Yield`, `Scavenge`, `ParkUnlanded`,
`RemoteTip`, `ReadMarker`, `RescueRefs`. Its one CAS, `casPush`,
classifies a lost push by what holds the ref now, never by stderr, and
that is the seam S37, S38 and S41 exercise. `FenceError` carries the
mover's marker, so a fence names whoever the trailer says. The
`RescueConflictError` comment in
[lease.go](../../internal/claim/lease.go) already cites S37–S39 and
S69 by number; no unit test does. `TestParkTwoTipsFromOneLaneBothLand`
in [lease_test.go](../../internal/claim/lease_test.go) is S78's shape
without its number. The fixtures in
[claim_test.go](../../cmd/frit/claim_test.go) build a bare origin
outside the fleet root; `cloneAgain` in
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) finds that origin
through `remote.origin.url`. A "person" in this section is raw git run
against that bare path: `update-ref -d`, `update-ref` to an older
tip, `gc --prune=now`, `clone --mirror` for a backup, `remote set-url`
for a migration.

**The rows, triaged.** Three shapes, and the phase order follows them.

- Raw git against origin, drivable now: S37 (a deleted ref refuses
  the holder's next CAS), S38 (a hand-force-pushed ref fences it, and
  a forged marker is what the fence reports), S39 (a ref pushed back
  to an older tip lets a stale takeover CAS win — ABA, by design),
  S41 (a rewritten remote fails every CAS safe and a fresh acquire
  wins at epoch 1), S69 (a forged trailer naming the holder is still
  fenced: the token decides, the trailer reports), S71 (a restored
  backup fences the holder, who re-reads and converges).
- Park, rescue and evidence: S40 (`gc --prune=now` on origin reaps a
  scavenged marker chain; the rescue ref keeps the work), S78 (two
  parks from one lane at two tips land under two content-addressed
  refs, and `orphans` lists both as `Rescued`), S68 (a force-pushed
  default branch breaks the ancestry half of landed evidence; the
  status glyph on origin's default branch still counts), S67 (one
  `ls-remote` per decision; a failed CAS re-reads before classifying).
- Doc-by-argument: S42 (one coordination remote, the `remote:` field
  of `.frit.yml` read by [repocfg](../../internal/repocfg); a second
  git remote is never consulted), S44 (a fork's origin is never the
  coordination point; the lease lands on the configured remote).

**A seam the doc and the code disagree on.** S43 promises observer
state keyed on the remote URL, so an edited URL voids old windows.
[observe.Key](../../internal/observe/observe.go) keys on repository
name and plan id. A `remote set-url` changes neither, so today the old
window survives. The row is not drivable to green as written. Its
phase opens by recording that finding and asking which side moves; no
scenario asserts a promise the code does not keep.

**Convention for a row the doc resolves by argument.** S44 says
"unsupported, documented". The scenario asserts the observable: the
lease pushed from a fork's clone lands on the configured remote, and
the fork's own origin carries no work ref. Every such row becomes an
assertion about what a verb or the remote shows, never a comment.

**Where the steps go.** The section's steps live in a new
`cmd/frit/bdd_storage_test.go`, appended to the step registry from
`init` exactly as `bdd_lease_test.go` is. This plan never edits
`bdd_test.go`, and touches no feature file but its own. Every
registrar binds on the one world a scenario threads, so a storage
step reads the clones and lease the reused lease steps set up. What
this section tracks beyond that — a backup mirror, a forged tip —
lives in its own struct, reached through `section[T]`, never as a
field added to `world`. A step text `bdd_lease_test.go` already
defines is reused, not redefined: redefining it fails as ambiguous
under godog's strict mode. The sibling conversion plans — process
death, host death and races,
partitions and clocks, identity and cross-layer, the two lifecycle
halves — each own their file the same way, so all seven land in any
order.

**Out of scope.** No change to the lease protocol or to any verb. A
scenario that cannot be made to pass without changing behaviour is a
finding, parked in the handoff with the row it concerns, not a fix
made here.

## Tasks

1. Phase 1 (proving slice): the six raw-git rows — S37, S38, S39,
   S41, S69, S71 — written and passing in `bdd_storage_test.go`, the
   file registered, the "a person" fixture over the bare origin set.
   Driven red by dropping `@pending`: strict mode fails the undefined
   steps.
2. Later phases, shaped by Phase 1's handoff: the park and evidence
   rows S40, S78, S68, S67 over `Scavenge`, `ParkUnlanded`, `orphans`
   and the landed-evidence reads; then S43 once its finding is
   settled; then the doc-by-argument rows S42 and S44 over
   `repocfg` and a fork-shaped clone.

## Execution

| Phase | Title                                                  | Tier   | Gate                                                                                |
| ----- | ------------------------------------------------------ | ------ | ----------------------------------------------------------------------------------- |
| 1     | The six raw-git rows of storage anomalies run for real | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(37\|38\|39\|41\|69\|71)_'` passes, no SKIP |
| 2     | The two park rows of storage anomalies run for real    | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(40\|78):'` passes, no SKIP                 |
| 3     | The two CAS/TRUST rows S67 and S68 run for real        | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(67\|68):'` passes, no SKIP                 |
| 4     | S43 runs for real once its finding is settled          | sonnet | `go test ./cmd/frit -run 'TestFeatures/S43:'` passes, no SKIP                       |
| 5     | The two doc-by-argument rows S42 and S44 run for real  | sonnet | `go test ./cmd/frit -run 'TestFeatures/S(42\|44):'` passes, no SKIP                 |

Every gate also keeps the bijection gate green and `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` clean.

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

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| --- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | ✅     | [The six raw-git rows of storage anomalies run for real](phase-1.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
|     | ↳      | S37, S38, S39, S41, S69 and S71 drop `@pending` and run as real scenarios in the new `cmd/frit/bdd_storage_test.go`: a hand-deleted work ref refuses the holder's next renewal with a plain error, never a fence (S37); a hand-force-pushed ref carrying a marker forged to name "mallory" fences the holder and the error suggests yield (S38); a ref force-pushed back to a stale first tip lets a takeover CASed from that same stale observation land — ABA — and fences the honest holder's own later renewal (S39); a remote replaced by a fresh bare repository carrying only `main` refuses every CAS against the old history and a fresh acquire on the new remote lands at epoch 1 (S41); a forged beat naming the holder itself still fences the renewal, because the SHA-based CAS token decides, never the trailer's claim (S69); and an origin restored from a mirror backup fences a holder whose recorded tip the backup does not carry, until it re-reads the restored tip and renews from there, converging for good (S71). `go test ./cmd/frit -run 'TestFeatures/S(37 |
| 2   | ✅     | [The two park rows of storage anomalies run for real](phase-2.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
|     | ↳      | S40 and S78 drop `@pending` and run as real scenarios in `cmd/frit/bdd_storage_test.go`: a remote `git gc --prune=now`, run for real against the bare origin after a scavenge has already deleted the work ref, cannot reap the work the rescue ref still names (S40); and a lane scavenged twice at two different tips lands both under their own content-addressed rescue refs, never colliding, and `frit orphans` reports both back for the same plan (S78). Both reuse S9's own setup vocabulary from `cmd/frit/bdd_process_death_test.go` — `pushes a work commit on the lane`, `the ref is scavenged`, `the pushed work is parked to a rescue ref, not lost` — unmodified. Three steps are new: `a person runs "git gc --prune=now" on origin`, `"([^"]+)" acquires the lease again` and `orphans lists both tips as rescued for "([^"]+)"'s lane`. `go test ./cmd/frit -run 'TestFeatures/S(40                                                                                                                                                                                    |
| 3   | ✅     | [The two CAS/TRUST rows S67 and S68 run for real](phase-3.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
|     | ↳      | S67 and S68 drop `@pending` and run as real scenarios in `cmd/frit/bdd_storage_test.go`: a renewal that loses its CAS to a rival takeover is still fenced correctly while a person's `fetch --prune` races it on the same clone, and resolves the loss with exactly one `ls-remote` (S67); a person's raw force-push rewrites origin's default branch to a parentless commit carrying the same tree, so an already-merged branch is no longer an ancestor of it, yet `reap --go` still tears the stranded checkout down on its status glyph alone (S68). Six steps are new: the raced renewal and its read-count check, the merged-and-landed checkout fixture, the force-push, the ancestor check, and the `reap` read. `go test ./cmd/frit -run 'TestFeatures/S(67                                                                                                                                                                                                                                                                                                                      |
| 4   | ✅     | [S43 runs for real once its finding is settled](phase-4.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
|     | ↳      | S43 drops `@pending` and runs as a real scenario in `cmd/frit/bdd_storage_test.go`: a person edits origin's URL to an equivalent mirror, "box-a" renews across the edit, and the renewal lands on the repointed origin — coordination is the CAS token on the ref, not the URL. Its finding is settled in the doc, not the code: the S43 matrix row in `docs/research/lease-protocol.md` is corrected to what `observe.Key` and `discovery.Observe` actually keep, since a `git remote set-url` voids no window and keying on the URL would be strictly worse. Three steps are new. `go test ./cmd/frit -run 'TestFeatures/S43:'` passes, no SKIP; `go test ./cmd/frit -run TestFeatures/S` shows S42 and S44 the only remaining SKIP; the bijection gate, `go test ./...` and `golangci-lint run` stay clean.                                                                                                                                                                                                                                                                            |
| 5   | 🔲     | [The two doc-by-argument rows S42 and S44 run for real](phase-5.md)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
<?/catalog?>

## Acceptance Criteria

- [ ] No scenario in `features/storage.feature` carries `@pending`;
      `go test ./cmd/frit -run TestFeatures/S` reports S37..S44,
      S67..S69, S71 and S78 as PASS, none as SKIP
- [ ] Every step is bound in `cmd/frit/bdd_storage_test.go` or reused
      from `bdd_lease_test.go`; `bdd_test.go` is untouched
- [ ] Each scenario asserts an observable — a verb's result, origin's
      refs, a marker, a rescue ref — never a comment
- [ ] A finding a row exposes is recorded in the handoff with its row
      id, not fixed silently; S43's keying finding is recorded before
      its scenario is written
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
