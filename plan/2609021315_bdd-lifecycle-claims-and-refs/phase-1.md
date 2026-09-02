---
n: 1
title: The five drivable lifecycle claim-and-ref rows run for real
status: "🔲"
result: false
---
Convert the five drivable lifecycle rows — S50, S51, S56, S70, S75 —
from `@pending` declarations into passing scenarios. The lease API,
the claim verb and git on origin drive all five today. This fixes
three things the later phases copy. The section's step file and its
registration. The world it threads. The rule that every step a
scenario uses is bound in this file.

**Assumes.** `TestFeatures` in
[cmd/frit/bdd_test.go](../../cmd/frit/bdd_test.go) runs each tagged
scenario as its own subtest under godog's strict mode and skips a
`@pending` one. Steps bind through the `registrars` slice; a file
appends its registrar from `init`, as
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) does. Every
registrar binds on the one `world` a scenario threads, built over
`claimableRepo`, `cloneAgain`, `leaseFor` and the `claim` API; a
section's own state sits beside it, reached through `section[T]`.
`claim.Branch(id)` is `plan/<id>`,
id only. `claim.Acquire` dates the claim against `opts.Base` as the
clone holds it; the claim verb gathers first, and the gather's fetch
in [gather.go](../../internal/fleet/gather.go) refreshes the
remote-tracking base before the acquire. `gitobj.DefaultRef` in
[git.go](../../internal/gitobj/git.go) reads
`refs/remotes/origin/HEAD` on every call. `casPush` syncs the local
copy of the work ref on every win, so a renewal restores a local ref
deleted by hand.

**Value.** The half stops being five declarations and becomes five
executable promises: a rename or a slug collision cannot fork the
ref, a hand-deleted local ref changes nothing origin decides, a claim
is dated against origin's current base, and evidence follows origin's
renamed default branch. Any of those regressing fails the build, and
the file the remaining five rows join already exists.

**RED.** Drop `@pending` from S50, S51, S56, S70 and S75 in
[lifecycle.feature](../../features/lifecycle.feature) and write each
one's Given/When/Then. Run `go test ./cmd/frit -run
TestFeatures/S50_`: strict mode reports the new steps undefined and
the subtest fails. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S50, plan file renamed after claim. Given "box-a" holds the lease
  for plan 7, when the plan file is renamed on main and pushed, and
  "box-b" acquires, then "box-b" loses to the live lease and origin
  carries exactly one `refs/heads/plan/*` ref, `plan/7`, with no slug
  in its name. Mirror `TestAcquireIsRenameProof`.
- S51, slug collision across plans. Given plans 7 and 8 share a title,
  so their files share a slug, when "box-a" acquires both, then origin
  holds `plan/7` and `plan/8`, two refs, neither naming the slug. Use
  `commitPlan` with the same title for the second plan.
- S56, local branch deleted by hand. Given "box-a" holds the lease,
  when "box-a" deletes its local `plan/7` with `git branch -D`, then
  "box-b"'s acquire still loses to origin's lease, and "box-a"'s
  renewal from its token succeeds and restores the local ref at the
  beat. Origin's tip decides both; the local copy decides nothing.
- S70, claim dated against an old base. Given a claimable plan whose
  origin's main moved after the clone last fetched — push a commit
  from a second clone — when `frit claim 7` runs, then the claim
  marker's `base:` trailer names origin's current main, not the
  clone's stale `origin/main`. Drive the verb through `run`, with the
  herdr fake installed as `TestClaimReacquiresAReleasedLease` does.
- S75, default branch renamed. Given a clone of an origin whose
  default branch is `main`, when origin renames it to `trunk` and
  points its HEAD there (`branch -m` and `symbolic-ref HEAD` on the
  bare origin), and the clone re-reads origin's HEAD (`git remote
  set-head origin -a`), then `DefaultRef` answers
  `refs/remotes/origin/trunk`. Then rename again; a second read
  answers the new name, never a cached one.

**GREEN.** Add `cmd/frit/bdd_lifecycle_test.go`: this section's step
functions, and an `init` appending the registrar. Reuse every step
text `bdd_lease_test.go` already defines; bind here only what the
five rows add, as methods on `world` that keep this section's own
state in a struct reached through `section[T]`. A quoted machine name
in a step is
checked against its role, as the lease world does, so a scenario
cannot pass by naming the wrong box. Every step function ships with a
unit test of its own, per CLAUDE.md.

**Guard the edges.** A step text `bdd_lease_test.go` already defines
must not be redefined: strict mode reports it ambiguous. The world
must refuse a machine the scenario never introduced. S75's second
rename is what proves "refreshed per read"; a scenario that reads
once proves less than the row claims. If the gather's own fetch does
not move `origin/HEAD` after the rename, that is a finding for the
handoff, with S75 named, not a reason to assert less. A scenario that
only passes by weakening an assertion is a finding, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(50|51|56|70|75)_'`
passes with every one of the five reported PASS and none SKIP. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Name the rows that needed a
step the lease world lacked. Record any finding a row exposed. Say
what the scavenge rows, S52, S53, S55 and S57, will need from
`Scavenge`, `Released` and the landed fixtures, and note that no code
names "plan-gone" evidence for S52 today, if that is still so.
