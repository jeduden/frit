---
n: 3
title: The two CAS/TRUST rows S67 and S68 run for real
status: "✅"
result: true
summary: >-
  S67 and S68 drop `@pending` and run as real scenarios in
  `cmd/frit/bdd_storage_test.go`: a renewal that loses
  its CAS to a rival takeover is still fenced correctly while a
  person's `fetch --prune` races it on the same clone, and resolves
  the loss with exactly one `ls-remote` (S67); a person's raw
  force-push rewrites origin's default branch to a parentless commit
  carrying the same tree, so an already-merged branch is no longer an
  ancestor of it, yet `reap --go` still tears the stranded checkout
  down on its status glyph alone (S68). Six steps are new: the raced
  renewal and its read-count check, the merged-and-landed checkout
  fixture, the force-push, the ancestor check, and the `reap` read.
  `go test ./cmd/frit -run 'TestFeatures/S(67|68):'` reports both
  PASS, neither SKIP; `go test ./cmd/frit -run TestFeatures/S` shows
  S37..S44, S67..S69, S71 and S78, with S42, S43 and S44 the only
  remaining SKIP; the bijection gate, `go test ./...` and
  `golangci-lint run` stay clean.
---
## Handoff

**Eleven of thirteen storage rows now execute.** S67 and S68 join the
six phase 1 landed and the two phase 2 landed. Ten rows pass; three —
S42, S43 and S44 — remain `@pending`.

**S67's own shape.** `casPush` arbitrates on an explicit
`--force-with-lease=<ref>:<expected>` value, never a local
remote-tracking ref. A concurrent `fetch --prune` only touches local
remote-tracking copies. So the race changes nothing about what the
decision reads, or how many times it reads it. The new step,
`renewsWhileAPersonsFetchPruneRacesIt`, mints its own `gitwt.Runner`
closure rather than reusing `comesBackAndRenews` — that shared step
hard-wires `gitwt.Exec` — and runs a real `fetch -q --prune origin`
right before the renewal's own `push`, then counts every `ls-remote`
call. `TestRenewsWhileAPersonsFetchPruneRacesItCountsOneLsRemoteOnALoss`
proves both halves at once: the fence still names the correct rival,
and the count is exactly one.

**S68's own shape.** `landedEvidence` in `cmd/frit/main.go` pairs
`Merged` (`gitobj.MergedRefs`'s literal `--merged` check) with
`ByPlanID` (`index.LandedIDs`'s read of a plan's own status glyph, off
`gitobj.DefaultRef`'s `preferred` ref). `reap_test.go`'s own
`TestReapSquashMergedBranchIsReapedEvenNotAnAncestor` already proves
glyph evidence closes the "never merged" gap. This row proves the
sibling case: a branch that **was** merged, whose ancestry a later
force-push erases, while the rewritten default branch's own plan file
still reads done.
`hasACheckoutWhoseBranchLandsPlanOnOriginsMain` builds a fresh
repository — not a lease-based `claimableRepo` clone — since S68 is
about a checkout's own branch and the default branch, never the lease
work ref phases 1 and 2 touched; it reuses `strandedCheckout`,
`landPlan` and `addOrigin` from `cmd/frit/reap_test.go` and
`cmd/frit/main_test.go` unmodified. `reapCmd.Run` fetches before it
reads (`gatherFleet` with `Fetch: c.Fetch`, default true), so the
person's own force-push to the bare origin is visible to the `reap`
read the same way any other person-driven anomaly in this section is.

**A finding, not a fix: this phase's own fixture pattern for S68 is
narrower than the row's full name.** The row's matrix wording is
"default branch force-pushed," and the scenario proves the glyph half
of that — status evidence surviving an ancestry-breaking rewrite. It
does not exercise a force-push that also changes the plan's *content*
mid-rewrite (a person editing the plan file directly during the
rewrite, say) — that shape is not named anywhere in the matrix and was
out of scope by the plan's own "no change to the lease protocol or
verb" rule; the scenario asserts only what the row's own one-line
promise names.

**What S42, S43 and S44 still need.** S43's own finding from phase 1's
handoff still stands, unchanged: `observe.Key` in
`internal/observe/observe.go` keys an observation window on repository
name and plan id, never on the remote URL, so a `git remote set-url`
on origin voids no window today. The row is not drivable to green as
the doc currently reads. Whichever phase takes S43 must open by
recording this and asking which side moves — the doc's promise, or
`observe.Key`'s shape — before any scenario is written, per the plan's
own Acceptance Criteria.

S42 and S44 are the doc-by-argument rows the plan's own Context
names: one coordination remote, read from `.frit.yml`'s `remote:`
field through `internal/repocfg`, and a fork's own origin never
becoming the coordination point. Neither needs this phase's own
vocabulary — no raced renewal, no force-pushed default branch. S42
needs a "person" fixture with a *second* git remote on a clone (never
consulted), asserting a lease still lands on the configured remote
alone; `repocfg.Load` and its `Compiled()` accessor, already read by
`repoLanes` in `cmd/frit/main.go`, are the primitive to assert
against. S44 needs a fork-shaped clone — a second bare repository
whose own `origin` is the first bare repository's clone URL, not the
fleet's shared coordination remote — and an assertion that a lease
pushed from a checkout of the fork lands on the *configured* remote
(the fleet's own origin, read from `.frit.yml`), while the fork's own
origin carries no work ref. Both are new fixtures this section has not
built yet: every prior row shared one bare origin per scenario, and
these two are the first to need two remotes in play at once.
