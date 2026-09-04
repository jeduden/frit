---
n: 1
title: The four lease-API rows of landed evidence run for real
status: "✅"
result: true
summary: >-
  S54, S83, S84 and S85 drop `@pending` and run as real scenarios in
  the new `cmd/frit/bdd_landed_evidence_test.go`: a lane's work
  squash-merged onto the default branch is scavenged clean, with no
  rescue ref, once the plan's status is never flipped; an unreadable
  origin fails the scavenge naming the read it could not complete,
  leaving the local work ref exactly where it was; a lagging local
  `main` is never what `WorkLanded` or a scavenge decides against —
  `DefaultRef`'s remote-tracking answer is; and `origin/HEAD` unset,
  the shape `claimableRepo`'s own clone already has, is exactly where
  `DefaultRef` still reaches `refs/remotes/origin/main`, never
  `refs/heads/main`. The file registers itself the way
  `bdd_lease_test.go` does, reuses "holds the lease for plan" as-is,
  and keeps its own state in a `landedEvidenceState` reached through
  `section[T]` rather than a field on `world`.
---
## Handoff

**Rows needing a step the lease world lacked.** All four did.
`bdd_lease_test.go`'s vocabulary covers a claim and an unpushed
commit; none of these rows works with unpushed content. S54 needed a
step that pushes real work for a holder (`pushesWorkOnTheLane`,
recording its tip in the section state rather than `world.local`,
since that field is reserved for the lease world's own deliberately
unpushed commit) and the squash-merge step reproducing
`squashLandOnMain`'s four git commands
(`squashMergesOntoTheDefaultBranch`). S83 needed a `Runner` that fails
only `ls-remote` (`failingLsRemote`), swapped in for exactly the one
scavenge that scenario runs. S84 and S85 both needed a second
machine's clone (`clonesTheRepository`, over `cloneAs`) and a way to
read the fresh remote-tracking ref without ever touching that clone's
own local `main` (`refreshRemoteTracking`, a plain `fetch` of `main`
alone).

**A design decision the plan asked to be recorded.** The section
extends the world through its own `section[landedEvidenceState]`
rather than a new world type — the option `bdd_lease_test.go`'s
convention already exists for, and the one the sibling
`bdd_process_death_test.go` (`deathState`) also took.

**A shape decision that made S84 and S85 straightforward.** The
squash-merge always runs from a second machine's clone (`"box-b"`,
introduced by `clonesTheRepository`), never from the holder's own.
`claimableRepo`'s repo — the holder's own clone — is then never
touched again after `pushesWorkOnTheLane`, so its local `main` stays
exactly where the plan's initial commit left it: naturally behind
once `"box-b"` squash-merges, and still carrying no
`refs/remotes/origin/HEAD` symref, since that repo was only ever
`remote add`ed and pushed to, never `git clone`d. Both S84 and S85
read their evidence off that same untouched clone, so S85 needed no
separate step to unset `origin/HEAD` — it already had that shape from
`claimableRepo`, exactly the assumption phase 1's spec named.

**No finding.** Every row passed on the assertions the spec asked
for; none needed weakening. The guard rails held as designed:
`failingLsRemote` fails only `ls-remote` (pinned by
`TestFailingLsRemoteRefusesOnlyLsRemote`), S84's local-`main`-behind
check refuses a scenario where local already matches origin's default
branch (an accidental fast-forward would fail loudly, not pass), and
every new step resolves its quoted machine through `cloneOf`, so a
scenario cannot pass by falling back to whatever the world happens to
hold (`TestLandedEvidenceStepsRefuseAMachineTheyNeverMet`).

**A stale gate string, not introduced here.** This plan's own
`plan.md` and `phase-1.md` carry the same `TestFeatures/S(54|83|84|85)_`
pattern the sibling `bdd-process-death` plan's phase-1 handoff already
flagged as non-matching (`plan/2609021310_bdd-process-death/
phase-1.result.md`): `TestFeatures` names each subtest `"<id>: <title>"`,
so the working form is `TestFeatures/S(54|83|84|85):` — verified below.
Out of scope to fix here, since it touches no file this plan owns.

**What the verb-level rows will need.** S79 (scavenge refuses a
branch a worktree stands on), S81 (an unstaffed hold with a live
holder is refused) and S82 (a follow-up commit is parked before
`branch -D`) run over `reap`, not the bare lease API — `reap`'s own
fixtures (`strandedCheckout`, `landPlan`, `addOrigin`, `deadHold`) are
what those rows will build their Givens from, the way this phase
built S54's squash step from `internal/claim/lease_test.go`'s own
fixture rather than inventing one. S80 (a local default branch behind
its own fetched remote-tracking ref is a named problem) and S87
(`--fetch` refreshes before reading; `--no-fetch` names the
staleness) run over `Gather` and the global `--fetch` flag —
`landedDeletedClone` is the fixture shape to mirror, and both rows can
reuse this phase's `refreshRemoteTracking` idea, generalized to
`Gather`'s own fetch path rather than a bare `git fetch`. S59 (status
flipped ✅ early by hand) can only assert the observable that exists
today: a dependent of a hand-flipped ✅ reads as ready. `internal/doctor`
has no early-✅ check — its checks are goal, schema, execution-row,
tier, id-sync and phase-n-sync — so that gap stays a finding recorded
with the row, not a fix made in this plan.

All tests are green: `go test ./cmd/frit -run 'TestFeatures/S(54|83|84|85):'`
reports four PASS, none SKIP; `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are clean;
`go test ./internal/scenario` (the bijection gate) stays green.
