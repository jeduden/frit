---
n: 1
title: The five drivable lifecycle claim-and-ref rows run for real
status: "✅"
result: true
summary: >-
  S50, S51, S56, S70 and S75 drop `@pending` and run as real
  scenarios in `cmd/frit/bdd_lifecycle_test.go`, the section's own
  step file and world state, registered from `init` exactly as
  `bdd_lease_test.go` is: a renamed plan file cannot fork the work
  ref, two plans sharing a title mint two id-only refs, a local ref
  deleted by hand is restored by the next renewal and decides nothing
  in between, `frit claim` dates its lease against origin's current
  main rather than a clone's stale copy, and `DefaultRef` reads
  origin's renamed default branch fresh on every call, proven across
  two renames. All five reuse the lease world's own
  `"([^"]+)" holds the lease for plan (\d+)` and
  `"([^"]+)" comes back and renews its lease`; nothing else in
  `bdd_lease_test.go` fit, so the other twelve steps are new. `go
  test ./...` and golangci-lint stay clean, and the whole
  `TestFeatures` suite — every section landed so far — still runs
  with no ambiguous step.
---
## Handoff

**All five rows landed exactly as phase-1.md predicted, no
assertion weakened.** S50 and S56 reuse
`holdsTheLease`; S56 also reuses `comesBackAndRenews` for its own
renewal. Everything else — the rename-and-push, the second acquire
and its loss, the two-plan slug collision, the local branch's
deletion and restoration, the claim verb's own base trailer, and the
whole default-branch-rename fixture — needed a step the lease world
never carried, so `bdd_lifecycle_test.go` binds twelve new ones. None
collided with a text any other section already defines; `go test
./cmd/frit -run TestFeatures` still runs every section's scenarios on
one shared registry with no ambiguity.

S51's own fixture reuses `claimableRepo` for the first plan and
`commitPlan` for the second, sharing one title so the two files share
a slug; `acquiresTheLeaseForPlan` was written to serve both S50's
fresh-second-machine shape and S51's same-machine-second-plan shape,
picking up an existing clone when the scenario already gave the
holder one rather than always minting a new one.

**Finding, confirmed and left unfixed as scoped: `DefaultRef`'s own
freshness is never exercised by frit's actual fetch path.** S75
proves `gitobj.DefaultRef` re-reads `refs/remotes/origin/HEAD`
uncached, but only after the scenario itself runs
`git remote set-head origin -a`. Nothing in the product calls that:
`fleet.fetchRemote` (`internal/fleet/gather.go`) runs exactly
`git fetch --prune --quiet <remote>`, which — confirmed by hand
against the installed git — never moves
`refs/remotes/origin/HEAD` on its own. A repository whose origin
renames its default branch therefore keeps reading the old name
through every ordinary `frit` verb until something outside frit's own
flow (a person, a fresh clone) runs `remote set-head` for it. This is
the seam plan.md's own Context section named as worth checking, not
assuming; it is confirmed still open and left alone, since no verb
change is in this phase's scope.

**Finding, confirmed and still true: no code names "plan-gone"
evidence for S52.** A repository-wide search for `plan-gone`, in any
casing, matches nothing under `internal/` or `cmd/`. S52 — a plan
deleted while claimed — has no existing signal for "the plan file
itself is gone" to drive a scavenge decision on; whatever S52's own
phase builds will be new vocabulary, not a rename of something already
there.

**What the scavenge rows need, read off the API as it stands.**
`claim.Scavenge` and `claim.Released` are the two entry points; S53
(id reused) and S57 (re-opened after done) both scavenge a ref whose
tip already reads `Released`, then acquire fresh on top of it —
mechanically close to `TestClaimReacquiresAReleasedLease`'s own
shape, driven through the `claim` verb rather than the API alone so
the CLI's own scavenge-then-acquire sequencing is what is proven. S55
(merge + branch auto-delete) wants a ref merged into main with no
plan-file evidence of its own id anymore — `resumableRepo` already
builds the "no hold branch, 🔳 on main" half of that shape and is the
nearest fixture to extend, not invent from. `landedLeaseRepo` and
`landedDeletedClone` (both in `claim_test.go`) are named in plan.md as
already fitting S55's and S57's own before-state and are unused by any
existing scenario today; a next phase reaching for them is reuse, not
new fixture cost. S52 alone has no such fixture to reach for and will
need one, per the finding above.

All tests are green: `go test ./cmd/frit -run
'TestFeatures/S(50|51|56|70|75):'` reports five PASS, none SKIP;
`go test ./cmd/frit -run TestFeatures` (every landed section) stays
green; `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean; `mdsmith check .` is clean.
