---
n: 5
title: The two doc-by-argument rows S42 and S44 run for real
status: "✅"
result: true
summary: >-
  S42 and S44 drop `@pending` and run as real scenarios in
  `cmd/frit/bdd_storage_test.go`, the last two storage rows: a second
  git remote added to the holder's clone never splits coordination — a
  renewal still lands on the remote `.frit.yml` declares, read through
  `repocfg`, and the second carries no work ref (S42); a lease acquired
  from a fork checkout whose own origin is not the configured
  coordination remote lands on that configured remote, and the fork's
  origin carries no work ref for the plan (S44). Both are the first
  rows the section builds with two remotes in play; seven steps are
  new, each with its own unit test. `go test ./cmd/frit -run
  'TestFeatures/S(42|44):'` passes, no SKIP; `go test ./cmd/frit -run
  TestFeatures/S` reports all thirteen storage rows — S37..S44,
  S67..S69, S71 and S78 — PASS with no remaining SKIP; the bijection
  gate, `go test ./...` and `golangci-lint run` stay clean.
---
## Handoff

**Every storage row now runs. The plan's Goal is met.** S42 and S44
join the eleven earlier phases landed, so all thirteen rows of the
matrix's storage-anomaly section — S37..S44, S67..S69, S71 and S78 —
are passing godog scenarios. No scenario in
[storage.feature](../../features/storage.feature) carries `@pending`.

**S42 and S44's own shape.** Both are the section's first rows with two
remotes in play at once; every prior row shared one bare origin per
scenario. Neither needed a code change — each is a doc-by-argument row
the plan's Context named, drivable to green by asserting the observable
that documents it. The coordination remote is the one `.frit.yml`
declares, read through `repocfg.Load`; the scenarios assert against
`repocfg.Load(repo).Remote`, so a regression that let a second remote
or a fork's origin become the arbitration key would fail the build.

- S42 reuses `holdsTheLease` and `renewsItsLeaseAgain` unmodified.
  Three steps are new. `aPersonAddsASecondGitRemoteToClone` wires a
  fresh bare remote named `backup`, carrying main and no work ref, onto
  the holder's clone. `theRenewalLandsOnTheConfiguredRemoteAlone` reads
  the configured remote through `repocfg` and checks the tracked
  renewal's tip sits on it. `theSecondRemoteCarriesNoWorkRef` proves
  the mirror was never consulted.
- S44 builds a fork-shaped fixture in
  `hasACheckoutOfAForkWhoseOriginIsNotTheConfiguredRemote`: a shared
  coordination remote seeded through `claimableRepo`, a `--bare` fork
  taken of it, and the holder's checkout cloned from the fork — so the
  checkout's `origin` is the fork — with the coordination remote wired
  on as `fleet` and declared `remote: fleet` in the checkout's
  `.frit.yml`. `acquiresTheLeaseFromACheckoutOfTheFork` acquires with
  `Remote` and `Base` taken from `repocfg.Load`, so the lease lands on
  the configured remote, not the fork's origin;
  `theLeaseLandsOnTheConfiguredCoordinationRemote` and
  `theForksOwnOriginCarriesNoWorkRefForThePlan` assert both ends.

**Nothing is parked.** No finding surfaced in this phase: S43's finding,
settled in phase 4, was the section's only one, and it moved the doc,
never the code. The plan's "no change to the lease protocol or to any
verb" rule held across all five phases.
