---
n: 3
title: The scavenge and unwind rows S8, S9, S12 and S13 run for real
status: "✅"
result: false
---
Convert the last four process-death rows — S8, S9, S12, S13 — from
`@pending` declarations into passing scenarios, closing the section.
These are the scavenge-and-unwind rows, and like phase 1's S7 they
drive `internal/claim`'s exported API directly rather than a
`cmd/frit` verb: an unwind is `claim.Release`, a teardown is
`claim.Scavenge`, and the landed check is `claim.WorkLanded`. No
resume path, no herdr, no board.

**Assumes.** `claim.Release` (`internal/claim/lease.go`) pushes a
release-marker child by CAS and deletes nothing; `claim.Released`
reads a tip's subject back as `plan <id>: release`. `claim.Scavenge`
parks any unlanded work to a rescue ref
(`refs/frit/rescue/<id>/<holder>-<tip>`) and only then deletes the
work ref by CAS. It consults no staleness window — that gate lives up
in `cmd/frit`'s `mintOrTakeOver`, never in the claim atom — so a
landed tip is scavenged with nothing seeded. `claim.WorkLanded`
reads whether a tip's content already sits on the base, and the base
it judges against is refreshed from origin (`freshBase` fetches
`origin/main`), so a status flip on the branch is never the evidence.
Phase 1's own `pushesAWorkCommit` step already commits and pushes a
work commit on `plan/<id>`; it is reused here as-is. `claimableRepo`
builds the origin-and-clone pair, and `leaseFor` names the machine
and its lane with `Base: "origin/main"` — the four fixtures these
rows need are all already in the file.

**Value.** The section's teardown promises stop being declarations.
An unwind leaves a release marker and never a dangling delete (S8).
Pushed work is parked before any delete and never lost (S9). A lease
whose work already landed is cleaned with nothing parked (S12). A
plan marked done on its own branch is still unlanded until origin's
default branch carries it (S13). A regression in any of the four
fails the build, and process-death.feature finally runs every row.

**RED.** Drop `@pending` from S8, S9, S12 and S13 in
[process-death.feature](../../features/process-death.feature) and
write each one's Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S8:'`: strict mode reports the new steps undefined and
the subtest fails. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S8, unwind's remote delete fails. Given a held lease, when the
  handoff unwinds and releases it, then origin still carries the work
  ref and its tip is a release marker. The promise is that an unwind
  deletes nothing, so no delete can fail and lose the hold.
- S9, unwind deletes a branch with pushed work. Given a held lease
  with a pushed work commit, when the ref is scavenged, then the work
  is parked to a rescue ref that carries the tip, and only then is the
  ref deleted from origin. Losing the work by deletion is impossible.
- S12, killed after merge, before status flip. Given a held lease
  whose pushed work has since squash-landed on origin's default
  branch, when the ref is scavenged, then nothing is parked — the
  content already landed — and the ref is still deleted, with no
  window seeded.
- S13, status flipped on branch, not merged. Given a held lease with
  pushed work, and the plan marked done on the branch while origin's
  default branch is untouched, when the landed evidence is read, then
  the work reads unlanded — evidence is origin's default branch only,
  never a status flipped on the branch.

**GREEN.** Add the four scenarios' steps to
`cmd/frit/bdd_process_death_test.go`, appended beside phases 1 and 2;
no new registrar file, since this is the same section. New building
blocks: a step that releases a held lease and one that reads the ref
back as surviving-and-released (S8), a step that scavenges the ref
and one that reads a rescue ref's parked tip (S9), a step that
squash-lands the pushed work on origin's default branch (S12), and a
step that flips the plan's status on the branch alone plus one that
reads `WorkLanded` against `origin/main` (S13). "pushes a work commit
on the lane" is reused from phase 1 by S9, S12 and S13.

**Guard the edges.** A step text this file or `bdd_lease_test.go`
already defines must not be redefined: strict mode reports it
ambiguous. The world must refuse a machine the scenario never
introduced. S12's "nothing parked" step must read the actual rescue
refs on origin and assert the list is empty, not merely that the
returned struct's field is blank — a scavenge that parked under a
name the struct did not report would otherwise pass.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S'` reports S1..S13
all PASS and none SKIP — the whole process-death section runs. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean. The plan's
Acceptance Criteria are all met: this is the last phase, so its
closing commit ticks them, sets the plan `status:` to ✅ and runs
`mdsmith fix PLAN.md`.

Write the handoff to `phase-3.result.md`. Record any finding a row
exposed, and confirm the section is complete: no `@pending` left in
`features/process-death.feature`.
