---
n: 3
title: The two CAS/TRUST rows S67 and S68 run for real
status: "🔳"
result: false
---
Two storage rows are neither raw-CAS-on-the-work-ref (phase 1) nor
park-and-rescue (phase 2). S67 proves a decision never reads origin
more than once. A concurrent `fetch --prune` could otherwise tempt a
stale local read, and a lost CAS re-reads exactly once to classify.
S68 proves a force-pushed default branch breaks ancestry evidence, but
the plan's own status glyph on origin's rewritten default branch still
counts. `reap` already closes the squash-merge gap for a branch that
was never merged; this row closes it for one that was, and then had
its ancestry erased.

**Assumes.** `casPush` in
[internal/claim/lease.go](../../internal/claim/lease.go) pushes with
an explicit `--force-with-lease=<ref>:<expected>`, never the local
remote-tracking ref. A concurrent `fetch --prune` only touches local
remote-tracking copies, so it cannot change what the push arbitrates
against. On a lost push, `remoteHolderErr` issues exactly one
`ls-remote` to classify the loss; `Renew` calls no other remote read.
Neither `comesBackAndRenews` (`bdd_lease_test.go`) nor any storage
step hard-wires `gitwt.Exec` in a way this phase can reuse — each of
them calls `claim.Renew` directly with a fixed runner. So S67's own
step mints its own `gitwt.Runner`, following the pattern
`bdd_partitions_and_clocks_test.go`'s `partitionRunner` set: a closure
over `gitwt.Exec`. It counts `ls-remote` calls. On the renewal's first
`push`, it first runs a real `fetch --prune` against the same clone,
to simulate the race actually landing mid-decision.

For S68, `landedEvidence` in
[cmd/frit/main.go](../../cmd/frit/main.go) pairs two reads. One is
`Merged`. It is `gitobj.MergedRefs`'s own `--merged` check. The other
is `ByPlanID`. It is `index.LandedIDs`'s read of a plan's own status
glyph. Both read off `preferred`. `gitobj.DefaultRef` picks
`preferred`. It prefers a remote-tracking `refs/remotes/origin/main`
over a local `refs/heads/main`, once that ref exists.

`frit reap`'s own `--go` teardown already proves the glyph half of
this. It closes a gap ancestry cannot see: the squash-merge case. See
`TestReapSquashMergedBranchIsReapedEvenNotAnAncestor` in
[cmd/frit/reap_test.go](../../cmd/frit/reap_test.go). A branch never
merged into main is still reaped, once the plan reads done there.

S68 is the sibling case the matrix names. Its branch **was** merged.
A later force-push then erases that ancestry. The rewritten default
branch's own plan file still reads done.

`reapCmd.Run` fetches before it reads: `gatherFleet` with `Fetch:
c.Fetch`, default true. So a person's raw force-push to the bare
origin is visible to the next `reap`. That is the same way any other
person-driven anomaly in this section is. `strandedCheckout`,
`landPlan` and `addOrigin`
([cmd/frit/reap_test.go](../../cmd/frit/reap_test.go),
[cmd/frit/main_test.go](../../cmd/frit/main_test.go)) already build
the "checkout landed on main" fixture this row starts from.

This phase's own step reuses those three helpers. It does not
re-derive a lease-based fixture. S68 is about a checkout's own branch
and the default branch. It never touches the lease work ref phases 1
and 2 touched.

**Value.** S67 pins a promise the section's own trust boundary depends
on silently today. A decision is made from one live snapshot, never a
stale local view a concurrent `fetch --prune` could poison. A lost
arbitration is classified by exactly one further read. S68 pins the
other half of `reap`'s own landed check the existing unit tests do not
cover — not "never merged," but "merged, then the ancestry itself
erased by a force-push." A regression that made glyph evidence depend
on ancestry surviving would fail the build, instead of quietly
stranding a landed lane forever.

**RED.** Drop `@pending` from S67 and S68 in
[storage.feature](../../features/storage.feature) and write:

- S67, `fetch --prune` races a read. Given "box-a" holds the lease for
  plan 67 and "box-b" takes the lease over, when "box-a" renews its
  lease while a person's `fetch --prune` races it, then the renewal is
  fenced, naming "box-b", and the renewal read origin exactly once to
  classify the loss.
- S68, default branch force-pushed. Given "box-a" has a checkout whose
  branch is merged into origin's default branch, with plan 68 marked
  done there, when a person force-pushes origin's default branch to a
  fresh commit that carries the same content without the merge, then
  "box-a"'s branch is no longer an ancestor of origin's default
  branch, and `reap` still reaps "box-a"'s checkout, landed by its
  status glyph alone.

Run `go test ./cmd/frit -run 'TestFeatures/S(67|68):'`: strict mode
reports the new steps undefined and both subtests fail. That is the
red — commit it.

**GREEN.** Add to `cmd/frit/bdd_storage_test.go`:

- `"([^"]+)" renews its lease while a person's "fetch --prune" races
  it` — refuses a holder that never held the lease; mints a
  `gitwt.Runner` closure that, on the renewal's first `push` call,
  first runs `git fetch -q --prune origin` for real against the same
  clone, then counts every `ls-remote` call the renewal itself makes
  before delegating to `gitwt.Exec`; calls `claim.Renew` through it
  and records the count on `storageState`.
- `the renewal read origin exactly once to classify the loss` —
  refuses if no racing renewal ran yet; asserts the recorded count is
  exactly 1.
- `"([^"]+)" has a checkout whose branch is merged into origin's
  default branch, with plan (\d+) marked done there` — builds a fresh
  repo and origin (`initRepo`, `addOrigin`), a worktree lane on
  `claim.Branch(planID)` via `strandedCheckout`, merges it into main
  with `git merge --no-ff`, flips the plan's status to done with
  `landPlan`, and pushes main to origin; records the root, the repo,
  the lane and the branch's own tip on `storageState` and sets
  `w.holder`/`w.clones[holder]` so later steps can find it the way
  every other storage step does.
- `a person force-pushes origin's default branch to a fresh commit
  that carries the same content without the merge` — reads main's
  current tree, mints an orphan commit over it with `commit-tree`
  (no `-p`), and force-pushes it to origin's `main`; records the fresh
  SHA.
- `"([^"]+)"'s branch is no longer an ancestor of origin's default
  branch` — `git merge-base --is-ancestor <branch tip> <fresh SHA>`;
  a nil error (still an ancestor) fails the step, since the anomaly's
  own point is that it is not.
- `reap still reaps "([^"]+)"'s checkout, landed by its status glyph
  alone` — runs `reap --go --root <root>` through `emit` into a
  `report.ReapDoc`, asserts exactly one `Reaped` entry, and confirms
  the lane's directory is gone from disk (`os.Stat` returns
  `os.ErrNotExist`) — the observable a person actually sees, not just
  the document's own claim.

Every step function ships with a unit test of its own, per CLAUDE.md.

**Guard the edges.** The racing-renewal step refuses a holder the
scenario never introduced. The checkout-and-merge step for S68 must
leave the lease world's own `w.holder`/`w.clones` reusable by the
ancestor check and the reap step without re-deriving the repo path;
`w.cloneOf` stays the one place that reads it. `reap still reaps`
refuses if no fresh SHA was recorded — the force-push step comes
first — the same "comes first" discipline every prior "person" step in
this file holds to.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(67|68):'` passes
with both reported PASS and neither SKIP; `go test ./cmd/frit -run
TestFeatures/S` still reports S37..S44, S67..S69, S71 and S78, with
S42, S43 and S44 the only remaining `SKIP`; the bijection gate, `go
test ./...` and `go tool -modfile=tools/go.mod golangci-lint run` stay
clean.

Write the handoff to `phase-3.result.md`. Say what S43's own finding
(recorded in phase 1's handoff, `observe.Key` never keys on the remote
URL) still blocks, and what S42 and S44 — the doc-by-argument rows over
`repocfg` and a fork-shaped clone — need that this phase's own
vocabulary does not already carry.
