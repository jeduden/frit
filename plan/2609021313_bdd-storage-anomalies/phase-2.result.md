---
n: 2
title: The two park rows of storage anomalies run for real
status: "✅"
result: true
summary: >-
  S40 and S78 drop `@pending` and run as real scenarios in
  `cmd/frit/bdd_storage_test.go`: a remote `git gc --prune=now`, run
  for real against the bare origin after a scavenge has already
  deleted the work ref, cannot reap the work the rescue ref still
  names (S40); and a lane scavenged twice at two different tips lands
  both under their own content-addressed rescue refs, never colliding,
  and `frit orphans` reports both back for the same plan (S78). Both
  reuse S9's own setup vocabulary from
  `cmd/frit/bdd_process_death_test.go` — `pushes a work commit on the
  lane`, `the ref is scavenged`, `the pushed work is parked to a
  rescue ref, not lost` — unmodified. Three steps are new: `a person
  runs "git gc --prune=now" on origin`, `"([^"]+)" acquires the lease
  again` and `orphans lists both tips as rescued for "([^"]+)"'s
  lane`. `go test ./cmd/frit -run 'TestFeatures/S(40|78):'` reports
  both PASS, neither SKIP; the bijection gate, `go test ./...` and
  `golangci-lint run` stay clean.
---
## Handoff

**Eight of thirteen storage rows now execute.** S40 and S78 join the
six phase 1 landed, reusing this section's own conventions — a step
text already bound anywhere is reused, not redefined — generalized one
step further than phase 1's own handoff asked: this phase's setup
vocabulary came from `bdd_process_death_test.go`, a sibling section's
file, not `bdd_lease_test.go`. `pushesAWorkCommit`, `theRefIsScavenged`
and `thePushedWorkIsParkedToARescueRef` — S9's own steps, proven at
`@S9` in `features/process-death.feature` — needed no change to carry
S40's and half of S78's own weight.

**New vocabulary this phase added, in `cmd/frit/bdd_storage_test.go`.**
`aPersonRunsGitGCPruneNowOnOrigin` runs `git gc --prune=now --quiet`
against the bare origin through `originOf`, never a verb — S40's own
anomaly, run after a scavenge has already deleted the work ref, so the
only surviving reference to the parked chain is the rescue ref itself.
`acquiresTheLeaseAgain` is S78's second cycle: a fresh `claim.Acquire`
on the very plan a scavenge just deleted the ref for, refusing unless
`claim.RemoteTip` reads empty first — a caller that skipped the
scavenge step would otherwise silently land a takeover instead of the
epoch-1 claim the row's shape depends on. `orphansListsBothTipsAsRescued`
checks the promise from both ends: `claim.RescueRefs` for a length-2
sanity read of the primitive, then `frit orphans --root
<filepath.Dir(repo)>` decoded into a `report.OrphansDoc`, asserting the
plan's own `Rescued` entry carries two refs — the verb an operator
actually runs, not just the API it stands on. `filepath.Dir(repo)` was
enough to find the `--root` `orphans` needed; `claimableRepo` already
places a clone at `<root>/<name>`, so no new state was needed to carry
the root forward.

**A finding, not a fix: phase 1's own gate command never actually ran
its six scenarios by name.** `go test -run` matches subtest names
literal-substring, and `TestFeatures` names each subtest `<ID>:
<Name>` — `t.Run(sc.ID+": "+sc.Name, ...)` in `bdd_test.go` — which Go
then space-to-underscore mangles into `S37:_work_ref_hand-deleted`.
The colon survives that mangling; `phase-1.md`, `phase-1.result.md`
and `plan.md`'s own Execution table all wrote the gate as
`'TestFeatures/S(37|38|39|41|69|71)_'` — a trailing underscore with no
colon before it, which matches no subtest at all.
`go test ./cmd/frit -run 'TestFeatures/S(37|38|39|41|69|71)_'` prints
"no tests to run" and exits 0: a silently vacuous gate, not a failing
one, so it never caught the mismatch. The six scenarios themselves are
fine — `go test ./cmd/frit -run 'TestFeatures/S(37|38|39|41|69|71):'`
(colon, not underscore) reports all six PASS today, and the broader
`go test ./cmd/frit -run TestFeatures/S` this plan's own Acceptance
Criteria already prescribes never had the bug, since it matches on
`S` alone. This phase's own gate and `plan.md`'s Execution table both
carry the colon form. Phase 1's already-closed files are left as they
are — the fact is recorded here, not fixed by hand-editing a landed
phase — but a future pass fixing every closed phase's gate wording
across this plan's siblings would be in scope for whoever notices
next.

**What phase 3 needs.** S68 (default branch force-pushed) and S67
(`fetch --prune` races a read) are TRUST/CAS rows, not PARK — neither
`Scavenge` nor `orphans` is the seam. S68 needs a "person" step that
force-pushes the fixture repository's own `main`, distinct from every
step in this file that touches the plan's own work ref, and then reads
the plan's status glyph off `main` through whatever read verb the
fleet walk uses — proving ancestry evidence stops matching while glyph
evidence still counts. S67 needs one `ls-remote` snapshot asserted per
decision and a failed CAS's re-read exercised explicitly; `casPush`'s
own `pushThenConfirm` already does this, so the scenario's job is to
prove it, likely racing a step-injected `fetch --prune` against a
renewal the way `bdd_partitions_and_clocks_test.go`'s `partitionRunner`
already races push and fetch failures — a `gitwt.Runner` wrapper armed
per-holder through `w.pc()`-style state, reused or paralleled in this
section's own `storageState`.

**S43's keying finding still stands, unchanged from phase 1's own
handoff.** `observe.Key` in `internal/observe/observe.go` keys an
observation window on repository name and plan id, never on the remote
URL, so a `git remote set-url` on origin voids nothing today — the row
is not drivable to green as the doc currently reads. Whichever phase
takes S43 must open by recording this and asking which side moves
before any scenario is written.
