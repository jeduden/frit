---
n: 1
title: The six raw-git rows of storage anomalies run for real
status: "✅"
result: true
summary: >-
  S37, S38, S39, S41, S69 and S71 drop `@pending` and run as real
  scenarios in the new `cmd/frit/bdd_storage_test.go`: a hand-deleted
  work ref refuses the holder's next renewal with a plain error, never
  a fence (S37); a hand-force-pushed ref carrying a marker forged to
  name "mallory" fences the holder and the error suggests yield (S38);
  a ref force-pushed back to a stale first tip lets a takeover CASed
  from that same stale observation land — ABA — and fences the honest
  holder's own later renewal (S39); a remote replaced by a fresh bare
  repository carrying only `main` refuses every CAS against the old
  history and a fresh acquire on the new remote lands at epoch 1
  (S41); a forged beat naming the holder itself still fences the
  renewal, because the SHA-based CAS token decides, never the
  trailer's claim (S69); and an origin restored from a mirror backup
  fences a holder whose recorded tip the backup does not carry, until
  it re-reads the restored tip and renews from there, converging for
  good (S71). `go test ./cmd/frit -run 'TestFeatures/S(37|38|39|41|69|71)_'`
  reports all six PASS, none SKIP; the bijection gate, `go test
  ./...` and `golangci-lint run` stay clean.
---
## Handoff

**Six of thirteen storage rows now execute.** `bdd_storage_test.go`
registers itself the way every section's step file does, reuses the
lease world's own vocabulary for every step whose meaning is
unchanged, and keeps the rest — the bare origin's path, a mirror
backup, each acquired-fresh lease, a per-holder tracked tip — in its
own `storageState`, reached through `section[T]`. No field was added
to `world`, and `bdd_test.go` and `bdd_lease_test.go` are untouched.

**Reused as-is from the lease world.** `"([^"]+)" holds the lease for
plan (\d+)`, `"([^"]+)" comes back and renews its lease`, `the
renewal is fenced, naming "([^"]+)"`, `the error suggests yield`,
`"([^"]+)" takes the lease over` and `origin still holds the
takeover` (S39's takeover check, reused from
`bdd_partitions_and_clocks_test.go`) all carry across sections
unmodified. `theRenewalIsAPlainWin`, also from the partitions file,
backs S71's two landed renewals. The shared world's own acquisition
tip (`w.lease.Tip`) never moves once a scenario starts renewing —
`comesBackAndRenews` sets only `w.err`, not `w.lease` — which is
exactly right for S37, S38, S39 and S69: each renews at most once
from the original claim, or (S39) a stale "from" still produces the
correct fence, since any mismatched CAS against origin's current tip
reads the same mover regardless of which of the holder's own valid
past tips it was attempted from.

**New vocabulary this phase added.** A "person" acts through
`gitCapture` against the bare origin (never a verb): `a person
deletes the work ref on origin` (S37), `a person force-pushes a
marker forged to name "([^"]+)" onto the work ref` (S38), `a person
force-pushes the work ref back to "([^"]+)"'s first tip` (S39), `a
person pushes a beat marker forged to name "([^"]+)" onto the work
ref` (S69). A forger's marker is minted by `forgeMarker`, straight
commit-tree plumbing built to `leaseMessage`'s own trailer shape,
never through `claim`'s mint path. The remote-migration and
backup/restore rows needed their own state and their own tracked
renewal, since a scenario that renews more than once needs the tip
the *previous* renewal actually landed at, not the stale original
claim: `origin is replaced by a fresh remote carrying only main`,
`every machine is pointed at the new remote`, `"([^"]+)" acquires the
lease on the new remote`, `"([^"]+)" acquired at epoch 1 on the new
remote` (S41); `a mirror backup of origin is taken`, `"([^"]+)"
renews its lease again`, `origin is restored from the backup`,
`"([^"]+)" re-reads origin's tip and renews from it` (S71). `the
renewal is refused` is shared between S37 and S41 — a plain error,
not a `FenceError`, since an absent ref is `casPush`'s
never-arbitrated branch.

**Would a shared per-scenario world have let the lease vocabulary be
reused more?** No further reuse was left on the table. Every lease-
world step this phase's six rows could plausibly want — acquire,
renew, fence, takeover, yield-suggestion — was already reusable as
soon as its meaning matched exactly; the six rows that needed their
own wording (a person's raw git, a migrated remote, a mirror backup)
are all genuinely new actions the lease world never had reason to
express. The one place a second, section-owned tip tracker
(`storedTip` in `storageState`) was necessary rather than convenient
is S71's chain of three renewals; S39 shows the same multi-renewal
shape does *not* always need one, since ABA's own mechanics make a
stale "from" produce the same correct fence.

**Findings.** Two are worth carrying forward.

- *S69 and `RenewToBind`.* The scenario drives `Renew`, not
  `RenewToBind`, deliberately. `RenewToBind`'s own-hold reconcile
  (`ownHold` in `lease.go`) treats a fenced tip as this lane's own
  stale baseline whenever the fence's marker names this same holder
  and lane — exactly the shape S69's forged marker produces on
  purpose. Reconciling through a forged tip naming the holder itself
  is A7 and the trust domain working as designed (a marker's claimed
  identity is never itself verified, only reported), not a bug — but
  it means `RenewToBind` specifically, unlike plain `Renew`, would
  not fence on this exact forgery; it would treat the forged commit
  as its own prior beat and renew past it. No test exists for that
  distinction yet; a future phase or a dedicated unit test in
  `internal/claim` may want to pin it explicitly so the reconcile's
  scope stays a documented choice rather than an unexercised corner.
- No other row exposed a gap between the matrix's promise and the
  code. Each of S37, S38, S39, S41 and S71 asserts exactly what
  `internal/claim` already does, and no assertion was weakened to
  reach green.

**What the later phases need, restated from the plan's own triage.**
Two threads, each already scoped in the plan's own Context.

- *Park and evidence — S40, S78, S68, S67.* `claim.Scavenge` and
  `claim.ParkUnlanded` already exist and are exercised by
  `internal/claim`'s own unit tests
  (`TestParkTwoTipsFromOneLaneBothLand` is S78's shape, unnumbered);
  their storage-section scenarios drive the same `gitCapture`-based
  "person" pattern this phase established for `gc --prune=now` (S40)
  and reuse `claimableRepo`/`cloneAgain` for a second park at a second
  tip (S78). S68 needs a "person" step that force-pushes the fixture
  repository's own default branch — distinct from the work ref every
  step in this phase touched — and then reads the plan's status glyph
  off `main` through whatever read verb the fleet walk uses, since
  ancestry evidence stops matching but glyph evidence does not. S67
  needs one `ls-remote` snapshot asserted per decision, and a failed
  CAS's re-read exercised explicitly — `casPush`'s own
  `pushThenConfirm` already does this; the scenario's job is to prove
  it, likely by racing a step-injected `fetch --prune` against a
  renewal the way the partition runner already races push and fetch
  failures.
- *S43's keying finding, restated.* `observe.Key` in
  `internal/observe/observe.go` keys an observation window on
  repository name and plan id, never on the remote URL. The matrix
  promises "observer state keys on remote URL; old windows void"
  (S43). Today, `git remote set-url` changes neither key component, so
  an old window survives an edited origin URL — the row is not
  drivable to green as the doc currently reads. That phase must open
  by recording this and asking which side moves (the doc's promise,
  or `observe.Key`'s shape) before any scenario is written; no
  scenario should assert a promise the code does not keep.
