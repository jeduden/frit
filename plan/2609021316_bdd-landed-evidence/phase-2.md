---
n: 2
title: The verb-level reap rows of landed evidence run for real
status: "✅"
result: false
---
Convert S79, S81 and S82 from `@pending` into passing scenarios. All
three are decided by `reap`, not the bare lease API: S79 is the
`checkedOut` guard `claim.Scavenge` already carries, S81 and S82 are
`reapUnstaffed` and `reapStranded`'s own refusal and park-then-delete
paths. Phase 1's four rows proved the file and its registration; this
phase proves the section reaches into `reap` itself.

**Assumes.** `reap`'s own fixtures already build every shape these
three rows need, the way `internal/claim/lease_test.go`'s
`squashLandOnMain` built S54's:

- `claimableRepo(t, root, name, id, title)` in
  [claim_test.go](../../cmd/frit/claim_test.go) returns a repo one
  level under `root`, with a bare origin outside `root` already
  pushed to — the same shape `filepath.Dir(repo)` turns into a `reap
  --root` fleet, which `bdd_host_death_and_races_test.go`'s own
  `thisHostClaimsPlan` already does for `claim`.
- `strandedCheckout`, `addOrigin`, `landPlan` and `deadHold` in
  [reap_test.go](../../cmd/frit/reap_test.go) build a landed,
  stranded worktree; a pushed origin; a plan's status flipped without
  a real merge; and a lease bound to a session herdr will report
  gone.
- `TestScavengeSparesABranchCheckedOutInALinkedWorktree` in
  [lease_test.go](../../internal/claim/lease_test.go) is S79's own
  proof at the lease-API level: a linked worktree standing on the
  plan's branch survives a scavenge that still deletes origin's copy
  and parks the tip. `checkedOut` in
  [lease.go](../../internal/claim/lease.go) is the guard; `Scavenge`
  consults it before every local `update-ref -d`.
- `TestReapRefusesAFreshUnstaffedHold` in reap_test.go is the sibling
  shape, over a hold with no bound session at all: neither stale nor
  confirmed dead, it refuses naming "held by a live lease" and the
  not-matured span. `holdRefusal` in
  [reap.go](../../cmd/frit/reap.go) gives that same wording once
  `Dead` reads false, whatever made it false. S81's own Given binds a
  session and answers for it directly, so the row exercises the
  positive-liveness path on its own terms rather than borrowing the
  no-session one.
- `TestReapSquashMergedBranchIsReapedEvenNotAnAncestor` and
  `TestReapRefusesTheTeardownWhenTheParkIsRefused` are S82's own
  shape: a squash-landed plan's stranded branch carries a commit the
  squash never captured, `parkBranch` moves it to a rescue ref ahead
  of `branch -D`, and a rescue address already occupied refuses the
  whole teardown rather than losing the commit.

**Value.** `reap` itself is now proven, not just the lease API it
calls: a branch a worktree stands on survives even a scavenge that
deletes origin's copy, a live holder's hold is never dropped out from
under it, and a follow-up commit a squash-merge never carried is
rescued before its branch goes. A regression in any of the three
fails the build.

**RED.** Drop `@pending` from S79, S81 and S82 in
[landed-evidence.feature](../../features/landed-evidence.feature) and
write each one's Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S(79|81|82):'`: strict mode reports the new steps
undefined and the subtests fail. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S79, scavenge deletes a branch a worktree still stands on. Given
  "box-a" holds the lease for plan 79 and pushes work on the lane,
  and a worktree stands on "box-a"'s own branch (a second, linked
  worktree of the same clone `claimableRepo` already built — never
  torn down independently, since nothing here reaps it), when
  "box-a" scavenges at the observed tip, then the work is parked to
  a rescue ref (unlanded content, exactly as the checkedOut linked
  worktree makes the tip un-fast-forwardable to delete outright),
  origin's work ref for the plan is gone, and "box-a"'s local work
  ref still resolves at its tip — the guard's own promise, reused
  verbatim from S54's Then steps rather than a third copy.
- S81, unstaffed hold, holder alive on another machine. Given
  "box-a" holds the lease for plan 81 bound to a session, and a
  herdr fake confirms that exact session alive and working — without
  it, the same bound-but-unanswered session reads as confirmed gone
  and the hold is dropped instead — when a fleet-wide `reap --go`
  runs over "box-a"'s own root, then the hold is refused naming
  "held by a live lease", and the hold still resolves on origin
  exactly where it was.
- S82, reaped squash-landed branch carries a follow-up commit. Given
  a repository with a stranded, landed checkout on plan 82's branch —
  `strandedCheckout` plus `landPlan`'s own ✅ glyph plus `addOrigin` —
  when a fleet-wide `reap --go` runs, then the branch is reaped, and
  the checkout's own commit — never carried by the ✅ glyph the plan
  file alone supplies — is parked to the plan's rescue ref before the
  branch goes, so it is moved, not destroyed.

**GREEN.** Extend `cmd/frit/bdd_landed_evidence_test.go`.

S79 adds two new steps. The Given: "a worktree stands on \"holder\"'s
branch". The Then: "the work is parked to a rescue ref". Every other
step S79 uses is phase 1's own, reused verbatim. That list is
pushesWorkOnTheLane, scavengesAtTheObservedTip,
originsWorkRefForThePlanIsGone and localWorkRefStillResolvesAtItsTip.

S81 and S82 are CLI-level and share a root. A Given builds the fleet
root and repo — wrapping `claimableRepo`,
`strandedCheckout`/`landPlan`/`addOrigin`, or an Acquire bound to a
session plus a herdr fake — and records `root` in
`landedEvidenceState`. A shared When ("a fleet-wide `reap --go` runs")
drives `run` the way `bdd_host_death_and_races_test.go`'s own CLI
steps already do, keeping stdout, stderr and the exit code in that
same state. Their Then steps read those buffers and, for S82,
`ls-remote` on origin's rescue namespace. Every step function ships
with a unit test of its own, per CLAUDE.md.

**Guard the edges.** S79's Then steps must be the exact ones S54
already proved, not new ones with the same shape. Reusing them is what
catches a scavenge that silently changed what "gone" or "still
resolves" means between the two rows.

S81's fixture must show a herdr answer that names the bound session.
An empty herdr response is not enough — proven, not assumed: run the
same Given through an empty herdr fake instead and the hold reads as
abandoned and gets dropped, not refused. A session bound at Acquire
that no herdr answer can find is read as confirmed gone, the same
evidence `deadHold`'s own mismatched session already relies on. Only
a herdr answer that actually names the bound session alive turns that
same hold into S81's own refusal.

S82's Then must check the rescue ref actually carries the stranded
checkout's own tip, not merely that some rescue ref exists — a rescue
parked from the wrong tip would still pass a looser check. A scenario
that only passes by weakening an assertion is a finding, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(79|81|82):'` passes
with all three reported PASS and none SKIP. `go test ./cmd/frit -run
TestFeatures` (every section landed so far) stays green. `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
clean.

Write the handoff to `phase-2.result.md`. Say what S80 and S87 need
from `Gather`, the `--fetch` flag and
`landedDeletedClone`, and what a CLI-level `reap --go` Given/When
pattern this phase establishes those two rows can or cannot reuse
(they run over `board`/`ready`, not `reap`, so likely not the same
When). Say what S59 needs from `discovery.Ready` and confirm
`internal/doctor` still has no early-✅ check.
