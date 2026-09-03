---
n: 4
title: S43 runs for real once its finding is settled
status: "✅"
result: true
summary: >-
  S43 drops `@pending` and runs as a real scenario in
  `cmd/frit/bdd_storage_test.go`: a person edits origin's URL to an
  equivalent mirror, "box-a" renews across the edit, and the renewal
  lands on the repointed origin — coordination is the CAS token on the
  ref, not the URL. Its finding is settled in the doc, not the code:
  the S43 matrix row in `docs/research/lease-protocol.md` is corrected
  to what `observe.Key` and `discovery.Observe` actually keep, since a
  `git remote set-url` voids no window and keying on the URL would be
  strictly worse. Three steps are new. `go test ./cmd/frit -run
  'TestFeatures/S43:'` passes, no SKIP; `go test ./cmd/frit -run
  TestFeatures/S` shows S42 and S44 the only remaining SKIP; the
  bijection gate, `go test ./...` and `golangci-lint run` stay clean.
---
## Handoff

**Eleven of thirteen storage rows now execute.** S43 joins the six
phase 1 landed, the two phase 2 landed, and the two phase 3 landed. Two
rows remain `@pending`: S42 and S44.

**S43's finding, settled.** The matrix and the code disagreed, and the
doc moved. `observe.Key` in `internal/observe/observe.go` keys a
staleness window on the local repository name and the plan id, never on
the remote URL. Liveness is the tip: `discovery.Observe` keys the
window on the tip SHA. So a `git remote set-url` changes neither the
key nor the tip, and an equivalent remote keeps the window live — the
OBS invariant S7, S23, S33, S34 and S62 already state. A divergent
remote reads a different tip and voids the window on tip change. Keying
on the URL, as the old row promised, would be strictly worse: it would
needlessly void a benign migration's windows for no safety gain. The
S43 row now reads "a URL edit voids nothing: the CAS token on the ref
is the coordinate, not the URL; the holder renews unbroken; observation
keys on repo, id and tip (OBS)." No code changed, so the plan's "no
change to the lease protocol or to any verb" rule holds.

**S43's own shape.** The scenario reuses `holdsTheLease` and
`renewsItsLeaseAgain` unmodified. Three steps are new.
`aPersonEditsOriginsURLToAnEquivalentMirror` mirror-clones origin the
way S71's `aMirrorBackupOfOriginIsTaken` does, then repoints the
holder's `origin` at the mirror — a URL edit to a remote carrying the
same content. Because the mirror is taken after acquisition, it carries
the holder's recorded tip, so `claim.Renew`'s force-with-lease matches
and the CAS lands on the repointed remote.
`theRenewalLandsUnbrokenByTheURLEdit` asserts the renewal landed;
`originsTipCarriesRenewal` reads `claim.RemoteTip` off the edited
remote and checks the tip is the renewal's, proving it reached the new
remote, not the old one.

**What S42 and S44 still need (phase 5).** Both are the doc-by-argument
rows, and neither is a finding — each is drivable to green with no code
change. They are the first rows this section builds with two remotes in
play at once; every prior row shared one bare origin per scenario. S42
needs a "second git remote never consulted" fixture: a clone with a
second git remote added, asserting a renewal still lands on the
configured remote alone and the second carries no work ref. The
primitive to assert against is `repocfg.Load` and its `Compiled()`
accessor, already read by `repoLanes` in `cmd/frit/main.go`. S44 needs
a fork-shaped clone: a second bare repository whose own `origin` is not
the fleet's shared coordination remote, asserting a lease pushed from a
checkout of the fork lands on the configured remote while the fork's
own origin carries no work ref. Phase 5 is the plan's last: with S42
and S44 landed, no storage row carries `@pending`, and the Goal is met.
