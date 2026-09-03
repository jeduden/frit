---
n: 4
title: S43 runs for real once its finding is settled
status: "✅"
result: false
---
S43 is the one storage row where the doc and the code disagreed. Its
finding is settled before any scenario is written, as the plan's
Acceptance Criteria demand. The matrix used to promise "observer state
keys on remote URL; old windows void." It does not, and it should not.
`observe.Key` in
[internal/observe/observe.go](../../internal/observe/observe.go) keys a
window on the local repository name and the plan id. It never keys on
the remote URL. So a `git remote set-url` changes neither. That is
correct, not a bug.

Liveness is the tip, not the URL. `discovery.Observe` keys the window
on the tip SHA. An equivalent remote reads the same tip, so the window
stays live. That is the OBS invariant S7, S23, S33, S34 and S62 already
state. A divergent remote reads a different tip, so the window voids on
tip change — the same self-healing every other OBS row relies on. Keying
on the URL would be strictly worse. A benign migration, S41's own
sibling, would needlessly void every window and delay every takeover.

**The finding, resolved: the doc moves.** The S43 row in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md)
now reads "a URL edit voids nothing: the CAS token on the ref is the
coordinate, not the URL; the holder renews unbroken; observation keys
on repo, id and tip (OBS)." No code changes. The plan's own "no change
to the lease protocol or to any verb" rule holds. The scenario asserts
the promise the code keeps.

**Assumes.** `claim.Renew`'s CAS is `--force-with-lease=<ref>:<expected
tip>` against the `origin` remote of the holder's own clone. A `git
clone --mirror` of origin carries every ref at the tip it holds when
the mirror is taken. S71 already builds and depends on exactly this,
through `aMirrorBackupOfOriginIsTaken`. So a mirror taken after
acquisition still carries the holder's recorded tip. Repoint the
holder's `origin` at it, and the renewal's force-with-lease matches, so
the CAS lands on the repointed remote. `renewsItsLeaseAgain` — S71's own
tracked renewal, CASed from this section's `storedTip` — is reused
unmodified. It is the renewal-across-the-edit this row needs, and it
records the landed tip on `storageState`.

**Value.** S43 pins a promise a person could otherwise assume runs the
other way. Editing origin's URL does not strand a holder, and it does
not void its observation window. Coordination follows the ref, not the
URL string. A repository migrated to a new URL keeps its live leases. A
regression that made a verb resolve holdership from the URL, or key
observation on it, would fail the build.

**RED.** Drop `@pending` from S43 in
[storage.feature](../../features/storage.feature) and write:

- S43, origin URL edited mid-lifecycle. Given "box-a" holds the lease
  for plan 43, when a person edits origin's URL to an equivalent
  mirror and "box-a" renews its lease again, then the renewal lands,
  unbroken by the URL edit, and origin's tip carries "box-a"'s renewal.

Run `go test ./cmd/frit -run 'TestFeatures/S43:'`. Strict mode reports
the new steps undefined and the subtest fails. That is the red — commit
it.

**GREEN.** Add to `cmd/frit/bdd_storage_test.go`, reusing
`holdsTheLease` and `renewsItsLeaseAgain` as they stand:

- `a person edits origin's URL to an equivalent mirror` — mirror-clones
  the current origin into a fresh temp path, the way
  `aMirrorBackupOfOriginIsTaken` does. Then it runs `remote set-url
  origin <mirror>` on the holder's own clone, and records the mirror on
  `storageState`. That is the URL edit a person runs, to a remote
  carrying the same content under a new address.
- `the renewal lands, unbroken by the URL edit` — refuses if no renewal
  ran, then asserts `w.err` is nil. The CAS crossed the edit and landed.
- `origin's tip carries "([^"]+)"'s renewal` — reads `claim.RemoteTip`
  off the holder's repointed `origin`. It checks the tip equals the one
  the tracked renewal recorded. That proves the renewal reached the new
  remote, not the old one.

Every step function ships with a unit test of its own, per CLAUDE.md.

**Guard the edges.** The mirror-edit step refuses a holder the scenario
never introduced. It is the same discipline every prior "person" step
holds to. The renewal-lands check refuses before any renewal has run.
The origin-tip check reads the holder's clone, so a holder the scenario
never met refuses rather than reading a zero value.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S43:'` passes with the
subtest PASS and no SKIP. `go test ./cmd/frit -run TestFeatures/S`
still reports S37..S44, S67..S69, S71 and S78, with S42 and S44 the
only remaining `SKIP`. The bijection gate, `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` stay clean.

Write the handoff to `phase-4.result.md`. Say that S43's finding is
settled in the doc and its scenario passes. Say what S42 and S44 — the
two doc-by-argument rows still `@pending` — need, per phase 3's handoff.
