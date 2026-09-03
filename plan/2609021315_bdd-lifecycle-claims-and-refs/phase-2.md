---
n: 2
title: The scavenge-by-release-evidence rows run for real
status: "✅"
result: false
---
Convert S53 (plan id reused) and S57 (plan re-opened after done) from
`@pending` into passing scenarios. S55 (merge + branch auto-delete)
joins them, since it shares S53's and S57's own fixture shape and Then
step. S52 (plan deleted while claimed) and S58 (released before the
PR merges) are not this phase's. S52 needs its own fixture, named
below. S58 is doc-by-argument, per plan.md's own task split.

**Assumes.** `claim.Released(repoDir, tip, planID, run)` reads a work
ref's tip subject and reports whether it is a release marker. Today
that is used only by `release`'s own no-op check, never to drive a
scavenge.

`claim.Scavenge(repoDir, opts, from, run)` does not itself read the
tip's marker kind. It CASes on `from` against the remote's current
tip, parks unlanded work, then deletes — the same mechanics whether
`from` is a release marker, a landed tip, or anything else.

Nothing in `cmd/frit/claim.go` or `release.go` calls `Scavenge` on a
merely released ref today. Only a landed one — by ancestry, in
`mintClaim`, or by glyph, in `releaseUnheld` — is scavenged
automatically. This phase drives `claim.Scavenge` directly against a
released tip, the mechanism the matrix's own outcome column names.
It does not add the evidence-detection wiring itself; that wiring is
out of this plan's scope, named in plan.md's Context and confirmed by
phase 1's handoff.

`resumableRepo(t, root, name, id, title)` in
[claim_test.go](../../cmd/frit/claim_test.go) already builds a plan
whose work ref was never even minted — the fixture phase 1's handoff
named as the nearest shape for S55. It needs no scavenge at all, since
there is nothing on the ref to scavenge.

**Value.** Two more rows executable, and a fixture pattern — build a
released lease, replace or flip the plan's own file, scavenge the
released ref by evidence, then reclaim — that both rows share and
that S52's own phase can measure itself against instead of inventing
its shape from nothing. A regression that let a scavenged ref's next
acquire silently continue the old epoch, instead of starting fresh at
1, fails the build.

**RED.** Drop `@pending` from S53, S55 and S57 in
[lifecycle.feature](../../features/lifecycle.feature) and write each
one's Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S(53|55|57)_'`: strict mode reports the new steps
undefined and the subtests fail. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S53, plan id reused. Given plan 7 is done and its lease is released,
  when a different plan's file replaces it under the same id 7, and
  the released ref is scavenged by evidence, then origin carries no
  `plan/7` ref, and a fresh `frit claim 7` succeeds at epoch 1 — never
  epoch 2, the sign the whole ref, not merely its tip, was gone before
  the new claim minted. Build the released lease the way
  `TestClaimReacquiresAReleasedLease` does — `claim.Acquire` then
  `claim.Release` — over `claimableRepo`; replace the plan file with
  `commitPlan` under a new title.
- S55, merge + branch auto-delete. Given a plan merged into main whose
  own lease branch is already gone — `resumableRepo`, unchanged, since
  a GitHub-style auto-delete after merge leaves exactly its shape: 🔳
  on main, no `plan/7` ref anywhere — when `frit claim 7` runs, then it
  succeeds at epoch 1, the same Then S53 and S57 share: nothing live
  to lose to, nothing to scavenge, an ordinary fresh acquire.
- S57, plan re-opened after done. Given plan 7 is done and its lease
  is released, when the same plan file is marked ✅ and then re-opened
  to 🔲, and the released ref is scavenged by evidence, then origin
  carries no `plan/7` ref, and a fresh `frit claim 7` succeeds at
  epoch 1. The fixture is S53's own, with the file edited in place
  rather than replaced — the two rows differ in what changed about the
  plan, never in the ref mechanics, which is exactly what the matrix
  says: both read "scavenge old ref by evidence; fresh acquire".

**GREEN.** Extend `cmd/frit/bdd_lifecycle_test.go`: a shared Given
builds a released lease and records its tip in `lifecycleState`. A
When for each row makes its own edit to the plan file. A shared step
drives `claim.Scavenge` directly against the recorded release tip —
the evidence a future verb would still need to decide *when* to call
this, which this phase does not add. A shared Then runs `frit claim`
(reusing S70's own `fritClaimsPlan`) and checks the marker's epoch,
reusing `ReadMarker` the way S70's base check already does.
S55 reuses that same Then directly over `resumableRepo`, with no
scavenge step at all. Every step function ships with a unit test of
its own, per CLAUDE.md.

**Guard the edges.** The Then step must read the epoch off the marker
`ReadMarker` returns, not merely check `code == 0` — a claim that
silently reacquired at epoch 2 on a ref the scavenge failed to delete
would still print "claimed plan 7" and must still fail this check.
`claim.Scavenge`'s own CAS must be exercised for real: if scavenging a
released (not landed) tip needed a code change to succeed, that is a
finding for the handoff, with the row it blocks named, not a fix made
here. A scenario that only passes by weakening an assertion is a
finding, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(53|55|57)_'` passes
with all three reported PASS and none SKIP. `go test ./cmd/frit -run
TestFeatures` (every section landed so far) stays green. `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
clean.

Write the handoff to `phase-2.result.md`. Say what S52's own phase
needs: whether `claim.Scavenge`, driven directly as this phase drives
it, is enough to prove "PARK first" over a plan-gone Given, or whether
a finding blocks it. Say what S58 needs from `Release` and a second
`Acquire`, and whether this phase's shared fixture pattern reaches
that far too.
