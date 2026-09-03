---
n: 1
title: The six raw-git rows of storage anomalies run for real
status: "✅"
result: false
---
Six storage rows need only raw git against origin and the lease API:
S37, S38, S39, S41, S69, S71. Convert them from `@pending` declarations
into passing scenarios. This fixes three things the later phases copy.
The first is the section's step file and its registration. The second
is the "a person" fixture that reaches the bare origin. The third is
the convention for a fence: assert what origin holds and what the
error names.

**Assumes.** `TestFeatures` in
[cmd/frit/bdd_test.go](../../cmd/frit/bdd_test.go) runs each tagged
scenario as its own subtest under godog's strict mode and skips a
`@pending` one. Steps bind through the `registrars` slice; a file
appends its registrar from `init`, as
[bdd_lease_test.go](../../cmd/frit/bdd_lease_test.go) does. Every
registrar binds on the one world a scenario threads, so a storage
step reads the clones the reused lease steps built; this section's
own state lives in a struct reached through `section[T]`.
`claimableRepo` builds a bare origin outside the fleet root;
`cloneAgain` reads `remote.origin.url` to find it, and a "person" step
runs `git -C <origin>` there. In
[internal/claim/lease.go](../../internal/claim/lease.go), `casPush`
reads the ref after a lost push: an absent ref is returned as a plain
refusal, a foreign tip as a `FenceError` carrying that tip's marker.
`Takeover` mints a child of the tip it is handed and CASes on exactly
that tip. `RemoteTip` reads origin's current tip. A marker is a commit
whose message is `plan <id>: <kind>` and trailer lines; `leaseMessage`
is its shape, and `ReadMarker` reads it back.

**Value.** The section stops being six declarations and becomes six
executable promises about the trust boundary: a hand-moved or deleted
ref refuses the holder, a forged trailer is reported but never passes
a check, a rewound or restored origin lets a CAS win or lose on the
tip alone, and a rewritten remote fails every CAS safe. Any of those
regressing fails the build, and the file the remaining seven rows join
already exists.

**RED.** Drop `@pending` from S37, S38, S39, S41, S69 and S71 in
[storage.feature](../../features/storage.feature) and write each one's
Given/When/Then. Run `go test ./cmd/frit -run TestFeatures/S37_`:
strict mode reports the new steps undefined and the subtest fails.
That is the red — commit it.

The scenarios, in the matrix's own terms:

- S37, work ref hand-deleted. Given "box-a" holds the lease, when a
  person deletes the work ref on origin, then "box-a"'s renewal is
  refused and origin carries no work ref. The refusal is `casPush`'s
  absent-ref branch, a plain error, not a fence; assert the refusal
  and origin's emptiness, not an error type the code never promised.
  "box-a"'s local ref is left where it was.
- S38, work ref hand-force-pushed. Given "box-a" holds the lease, when
  a person force-pushes a commit carrying a marker forged to name
  "mallory" onto the work ref, then "box-a"'s renewal is fenced naming
  "mallory" and the error suggests yield. The fence reports the forged
  trailer as written: TRUST, not verification.
- S39, work ref force-pushed backward. Given "box-a" holds the lease
  at a first tip and renews to a second, and "box-b" observed the
  first tip, when a person pushes origin's ref back to the first tip
  and "box-b" takes over from that stale observation, then the
  takeover lands and "box-a"'s renewal from the second tip is fenced
  naming "box-b". The stale CAS won because the ref matched again:
  ABA, and nothing in frit is asked to see it.
- S41, remote rewritten or migrated. Given "box-a" holds the lease,
  when origin is replaced by a fresh bare remote carrying only `main`
  and every clone is pointed at it, then "box-a"'s renewal is refused
  and "box-b" acquires at epoch 1 on the new remote.
- S69, marker body forged. Given "box-a" holds the lease, when a
  person pushes a child of the tip whose message is a beat marker
  forged to name "box-a" and its own lane, then "box-a"'s renewal from
  its recorded tip is fenced, and the fence's marker names "box-a".
  The trailer said the holder's own name and the renewal was refused
  anyway: the token is the fence, the trailer only reports.
- S71, origin restored from backup. Given "box-a" holds the lease and
  a mirror backup of origin is taken, and "box-a" renews once more,
  when origin is restored from the backup, then "box-a"'s renewal
  from its recorded tip is fenced and the fence names "box-a" itself.
  And when "box-a" re-reads origin's tip and renews from it, the
  renewal lands and a further renewal lands too: origin converged on
  the holder's single chain.

**GREEN.** Add `cmd/frit/bdd_storage_test.go`: a world for this
section holding the origin path, each machine's clone, each lease as
taken, and the error the When step produced; the step functions; and
an `init` appending the registrar. Word every step for this section —
the lease world's texts are taken, and its state is out of reach — and
record the wording in the handoff so the later phases share it. A
quoted machine name in a step is checked against its role, as the
lease world does, so a scenario cannot pass by naming the wrong box.
A "person" step runs raw git against the bare origin through
`gitCapture`, never through a verb. A forged marker is minted with
`commit-tree` from a message built to `leaseMessage`'s shape. Every
step function ships with a unit test of its own, per CLAUDE.md.

**Guard the edges.** A step text `bdd_lease_test.go` already defines
must not be redefined: strict mode reports it ambiguous. The world
must refuse a machine the scenario never introduced, and a "person"
step must refuse to run when no origin has been built. S69's forged
trailer would let `RenewToBind`'s own-hold reconcile continue from the
forged tip; that is A7 and the trust domain by design, so the scenario
drives `Renew` and the handoff notes the `RenewToBind` behaviour as a
finding. A scenario that only passes by weakening an assertion is a
finding for the handoff, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S(37|38|39|41|69|71)_'`
passes with every one of the six reported PASS and none SKIP. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Name the step wordings this
section settled on, and whether a shared per-scenario world would have
let the lease vocabulary be reused. Record any finding a row exposed.
Say what the park and evidence rows — S40, S78, S68, S67 — will need
from `Scavenge`, `ParkUnlanded`, `orphans` and the landed-evidence
reads, and restate S43's keying finding for its own phase.
