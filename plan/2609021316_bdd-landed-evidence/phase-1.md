---
n: 1
title: The four lease-API rows of landed evidence run for real
status: "🔲"
result: false
---
Convert the four rows the lease API and `DefaultRef` alone can drive.
They are S54, S83, S84 and S85. Each goes from a `@pending`
declaration to a passing scenario. This fixes three things the later
phases copy. First, the section's step file and its registration.
Second, the squash-land step every squash row reuses. Third, the
wrapping `Runner` that refuses a remote read.

**Assumes.** `TestFeatures` in
[cmd/frit/bdd_test.go](../../cmd/frit/bdd_test.go) runs each tagged
scenario as its own subtest under godog's strict mode and skips a
`@pending` one. Steps bind through the `registrars` slice; a file
appends its registrar from `init`, as
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) does. That file
already defines "holds the lease for plan" and "commits work on the
lane it never pushes", over `claimableRepo`, `cloneAgain` and the
`claim` API. `claim.Scavenge` takes a `gitwt.Runner` and the observed
tip; it returns the fault of an unreadable remote and parks nothing
whose content is already on the base. `claim.WorkLanded` and
`claim.ContentLanded` answer the content question against a base
given as-is. `gitobj.DefaultRef` takes the same `Runner`.
`claimableRepo` adds origin by `remote add` and a push, so its clone
has no `origin/HEAD` — the S85 shape without a step to unset it.

**Value.** The section stops being four declarations and becomes four
executable promises: squash-landed content is landed without a glyph,
an unreadable origin is never "gone", a lagging local `main` never
decides, and `DefaultRef` finds origin's branch with `origin/HEAD`
unset. Any of those regressing fails the build, and the file the
remaining six rows join already exists.

**RED.** Drop `@pending` from S54, S83, S84 and S85 in
[landed-evidence.feature](../../features/landed-evidence.feature) and
write each one's Given/When/Then. Run `go test ./cmd/frit -run
TestFeatures/S54_`: strict mode reports the new steps undefined and
the subtest fails. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S54, squash-merge, status never ✅. Given "box-a" holds the lease
  and pushes real work on the lane, when that work is squash-merged
  onto origin's default branch — a fresh commit with the same content,
  no ancestry to the lane — and the plan's status is never flipped,
  then a scavenge by "box-b" at the observed tip deletes the work ref
  from origin and parks nothing. The squash step reproduces the four
  git commands of `squashLandOnMain` in
  [internal/claim/lease_test.go](../../internal/claim/lease_test.go),
  which cmd/frit cannot import.
- S83, origin unreadable while scavenge classifies the ref. Given the
  lease is held, when origin becomes unreadable — a `Runner` wrapping
  `gitwt.Exec` that fails every `ls-remote` — and a scavenge runs,
  then it returns an error naming the read, and the local work ref
  still resolves at its tip. "Gone" is only ever a remote's answer.
- S84, local default branch normally lags origin. Given the work is
  squash-landed on origin's default branch while the clone's local
  `main` stays behind — never merged, never pulled — then `WorkLanded`
  against `DefaultRef`'s answer reads the work as landed, and the
  scavenge parks nothing. The scenario also asserts that local `main`
  is behind, so the row cannot pass on an accidental fast-forward.
- S85, `origin/HEAD` unset. Given the clone has no
  `refs/remotes/origin/HEAD` symbolic ref and a local `main` that
  lags, when `DefaultRef` is asked, then it answers
  `refs/remotes/origin/main`, never `refs/heads/main`, and the
  squash-landed ✅ committed there is what the plan reads.
  [defaultref_test.go](../../internal/gitobj/defaultref_test.go)
  pins the same order over a scripted `Runner`; the scenario pins it
  over a real clone.

**GREEN.** Add `cmd/frit/bdd_landed_evidence_test.go`: a world for
this section (or the lease world extended through its own file —
decide, and record which), the step functions, and an `init`
appending the registrar. Reuse every step `bdd_lease_test.go` already
defines; define only what the four rows add — the pushed-work step,
the squash-land step, the never-flipped status, the unreadable
origin, the scavenge, and the assertions on origin's refs, the rescue
namespace and `DefaultRef`. A quoted machine name in a step is
checked against its role, as the lease world does, so a scenario
cannot pass by naming the wrong box. Every step function ships with a
unit test of its own, per CLAUDE.md.

**Guard the edges.** A step text `bdd_lease_test.go` already defines
must not be redefined: strict mode reports it ambiguous. The world
must refuse a machine the scenario never introduced. The failing
`Runner` must refuse only `ls-remote`, so a scenario cannot pass
because every git call failed. S84 must assert local `main` is
genuinely behind before it asserts the evidence. A scenario that only
passes by weakening an assertion is a finding for the handoff, not a
green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(54|83|84|85)_'`
passes with every one of the four reported PASS and none SKIP. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Name the rows that needed a
step the lease world lacked. Record any finding a row exposed. Say
what the verb-level rows, S79, S81 and S82, will need from `reap` and
its fixtures (`strandedCheckout`, `landPlan`, `addOrigin`,
`deadHold`), what S80 and S87 need from `Gather` and `--fetch`
(`landedDeletedClone`), and what S59 can assert given `doctor` has no
early-✅ check.
