---
n: 4
title: Released before the PR merges runs for real
status: "🔳"
result: false
---
Convert S58 (released before the PR merges) from `@pending` into a
passing scenario — the last row of the half, per plan.md's own task
split.

**Assumes.** S58 is the doc-by-argument row. The matrix calls its
outcome "human process, TRUST", not a mechanism a test can drive
directly. Per plan.md's own Context, that convention becomes an
assertion about what a verb or the remote shows, never a comment.
The observable it names: after the release, a second claim succeeds
at epoch 2, origin's tip is that claim, and its marker names the new
holder. `TestClaimReacquiresAReleasedLease` in
[claim_test.go](../../cmd/frit/claim_test.go) already proves the
epoch half of this. A released lease's next `frit claim` reads epoch
2, not epoch 1, since nothing was scavenged, only released. But it
reads the epoch off commit body text, not `ReadMarker`, and drives
the CLI as a single anonymous identity rather than a second, named
machine. Phase 2's own handoff already named the fixture this phase
needs: `planIsDoneAndItsLeaseIsReleased`, S53's and S57's own Given,
builds exactly the released lease S58's Given wants. Unlike S53/S55/S57, S58
never scavenges — the released ref is reacquired in place, which is
the whole point: nothing here is a stale ref to clean up, only a
plan a different, legitimate claimant may pick up before the PR
closes.

**Value.** The last row of the claim-and-ref half of the matrix. After
this phase, every row is either a passing scenario or, for S58 alone,
a passing scenario over an explicit doc-by-argument convention — no
row is left declared and unproven. A regression that let a released
lease's next acquire land at the wrong epoch, or mint a marker naming
the wrong holder, fails the build.

**RED.** Drop `@pending` from S58 in
[lifecycle.feature](../../features/lifecycle.feature) and write its
Given/When/Then:

```gherkin
@S58
Scenario: released before the PR merges
  Given plan 7 is done and its lease is released
  When "box-b" acquires the lease for plan 7
  Then "box-b"'s claim succeeds at epoch 2
  And origin's tip is "box-b"'s claim
```

Run `go test ./cmd/frit -run 'TestFeatures/S58'`: strict mode reports
the two new Then steps undefined and the subtest fails. That is the
red — commit it, with this phase file itself at `status: "🔳"`.

**GREEN.** Extend `cmd/frit/bdd_lifecycle_test.go`:

- `planIsDoneAndItsLeaseIsReleased` gains one line: it registers its
  own repo under `w.clones[w.holder]`, so a second holder's acquire
  can clone from it via the shared world's own `cloneAs`/`cloneOf` —
  the same mechanism S50's and S51's own acquires already use. Nothing
  else about the Given changes; S53, S55 and S57 read none of this.
- `acquiresTheLeaseForPlan`, the shared When S50 and S51 already
  drive, gains one line: on success it also records the resulting
  `claim.Lease` on `w.lease`, the field `bdd_lease_test.go`'s own
  steps already use the same way. It was previously discarded; no
  existing scenario reads `w.lease` after calling this step, so
  nothing already landed changes behaviour.
- A new Then, bound to `"([^"]+)"'s claim succeeds at epoch (\d+)`,
  checks `w.err` is nil, `w.lease.Epoch` matches, and reads the
  marker back with `claim.ReadMarker` from the named holder's own
  clone — checking both `Kind == "claim"` and `Holder == <name>`, the
  marker naming the new holder that plan.md's own convention
  paragraph calls for.
- A new Then, bound to `origin's tip is "([^"]+)"'s claim`, is a thin
  wrapper over `originTipIs` — the helper `bdd_lease_test.go` and the
  host-death-and-races section already share — called with
  `w.lease.Tip`.

Every new or changed step function ships with a dedicated unit test of
its own, per CLAUDE.md.

**Guard the edges.** The epoch check must read `w.lease.Epoch`, the
value `claim.Acquire` itself returned, not merely `w.err == nil` — a
claim that silently landed at the wrong epoch must still fail this
check. The holder check must read the marker's own `Holder` field, not
merely that some marker exists — a claim that reused the old holder's
identity by mistake must still fail. A scenario that only passes by
weakening either assertion is a finding, not a green.

**Gate.** `go test ./cmd/frit -run 'TestFeatures/S58'` passes, PASS,
not SKIP. `go test ./cmd/frit -run TestFeatures` reports every row of
this half — S50, S51, S52, S53, S55, S56, S57, S58, S70, S75 — as
PASS, none SKIP. `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are clean.

This is the plan's last phase. Its closing commit ticks every met
Acceptance Criterion, flips `plan.md`'s own `status:` to `✅`, and runs
`mdsmith fix PLAN.md`.
