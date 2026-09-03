---
n: 2
title: The two park rows of storage anomalies run for real
status: "✅"
result: false
---
Two storage rows share one shape: a scavenge that parks unlanded work
before it deletes the ref, and `frit orphans`, the read that finds a
rescue ref later. S40 proves a remote `gc --prune=now` cannot reap
work the rescue ref still names, once the marker chain that used to
carry it is gone. S78 proves a second park from the same lane, at a
different tip, lands under its own ref rather than colliding with the
first. S68 and S67 stay out of this phase — their own shape, TRUST and
CAS rather than PARK, is phase 3.

**Assumes.** `claim.Scavenge` parks unlanded work to a content-
addressed rescue ref (`refs/frit/rescue/<id>/<holder>-<tip>`) before
it CAS-deletes the work ref. It is idempotent. A tip already parked is
a no-op; a different tip from the same holder lands beside it, never
over it (`TestParkTwoTipsFromOneLaneBothLand` in
[internal/claim/lease_test.go](../../internal/claim/lease_test.go)).
`claim.RescueRefs` lists a plan's rescue refs with one `ls-remote`.
`frit orphans --root <dir>` walks a fleet root and reports each
repository's rescue refs bucketed by plan
([cmd/frit/main.go](../../cmd/frit/main.go)'s `rescuedHeld`,
[internal/report/orphans.go](../../internal/report/orphans.go)).
[cmd/frit/bdd_process_death_test.go](../../cmd/frit/bdd_process_death_test.go)
already carries this phase's exact setup vocabulary — S9's shape is
S40's own: `"([^"]+)" pushes a work commit on the lane`, `the ref is
scavenged` and `the pushed work is parked to a rescue ref, not lost`
— proven at `@S9` in
[features/process-death.feature](../../features/process-death.feature).
Reusing them is this section's own rule from phase 1, generalized:
a step text already bound anywhere is reused, not redefined, wherever
its meaning matches exactly. `claimableRepo(t, root, name, id, title)`
builds the origin outside `root`, but the *clone* itself lives at
`root/name`, so `filepath.Dir(repo)` is the `--root` an `orphans` read
against that same clone needs — no new field to carry it.

**Value.** The park rows are the promise a scavenge's own delete keeps
even after the deleted ref is truly gone from history: a remote GC run
against origin for real cannot destroy what the rescue ref still
reaches, and two parks from the one lane never collide, both readable
back through the same verb an operator runs to find them. Regressing
either — a GC that reaps a rescue ref, a second park that clobbers the
first — fails silently today; after this phase it fails the build.

**RED.** Drop `@pending` from S40 and S78 in
[storage.feature](../../features/storage.feature) and write each
scenario:

- S40, remote GC reaps deleted markers. Given "box-a" holds the lease
  and pushes a work commit on the lane, when the ref is scavenged and
  a person runs `git gc --prune=now` on origin, then the pushed work
  is still parked to a rescue ref, not lost — the same check S9 already
  makes, now run again after a real GC to prove survival, not merely
  the park itself.
- S78, two parks from one lane, different tips. Given "box-a" holds
  the lease and pushes a work commit on the lane, when the ref is
  scavenged, then the pushed work is parked; when "box-a" acquires the
  lease again, pushes a second work commit and the ref is scavenged a
  second time, then that work is parked too, and `orphans` lists both
  tips as rescued for "box-a"'s lane.

Run `go test ./cmd/frit -run 'TestFeatures/S(40|78):'`: strict mode
reports the new steps undefined and the subtests fail. That is the
red — commit it.

**GREEN.** Add to `cmd/frit/bdd_storage_test.go`:

- `a person runs "git gc --prune=now" on origin` — `gitCapture` against
  the bare origin found through `originOf`, never through a verb.
- `"([^"]+)" acquires the lease again` — `claim.Acquire` on the same
  clone and plan, refusing a holder the scenario never introduced;
  updates `w.lease` so the steps S78 reuses from `bdd_process_death_test.go`
  (which read `w.holder`/`w.lease`, not this section's own state) see
  the second acquisition.
- `orphans lists both tips as rescued for "([^"]+)"'s lane` —
  `claim.RescueRefs` against origin for a length-2 sanity check, then
  `frit orphans --root <filepath.Dir(repo)>` decoded into a
  `report.OrphansDoc`, asserting the plan's own `Rescued` entry carries
  two refs. Both reads matter: `RescueRefs` is the primitive the row's
  promise rests on, `orphans` is the verb an operator actually runs.

Every step function ships with a unit test of its own, per CLAUDE.md.

**Guard the edges.** `a person runs "git gc --prune=now" on origin`
refuses when no origin has been built yet, the same discipline every
other "person" step in this file holds to. `"([^"]+)" acquires the
lease again` refuses a holder that never held the lease, and refuses
if the ref is not actually gone (a caller skipping the scavenge step
would otherwise silently no-op into a takeover instead of a fresh
claim). `orphans lists both tips as rescued` refuses when the rescue
count is not exactly two, on either read — the primitive or the verb
— so a regression in either layer is caught, not just one.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(40|78):'` passes
with both reported PASS and neither SKIP; the bijection gate,
`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
stay clean.

Write the handoff to `phase-2.result.md`. Say what S68 (default branch
force-pushed) and S67 (`fetch --prune` races a read) will need beyond
this phase's park vocabulary — both are TRUST/CAS rows, not PARK, and
neither `Scavenge` nor `orphans` is the seam they exercise.
