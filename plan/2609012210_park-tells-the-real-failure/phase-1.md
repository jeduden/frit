---
n: 1
title: park classifies its failure into four outcomes
status: "✅"
result: false
---
Make `park` in
[internal/claim/lease.go](../../internal/claim/lease.go) answer each of
its four failure shapes with the truth, driven red/green by a scripted
runner.

**Assumes.** `park` pushes create-only and, on failure, classifies by
reading the remote. `remoteHolderErr` already separates a read fault
from an absent ref. `UnconfirmedPushError` already carries a wrapped
underlying error for the push-unconfirmable case. A scripted
`gitwt.Runner` closure (the pattern near the fenced-renew tests in
[internal/claim/lease_test.go](../../internal/claim/lease_test.go))
can make the push fail with a chosen error and the `ls-remote` read
answer a chosen sha, an empty list, or an error.

**RED.** Four unit tests on `park`, through `Yield` or directly. Each
uses a scripted runner whose push fails with a distinct error. That
error stands in for a hook rejection or a timeout kill.

1. Read answers *absent*: the returned error's text carries the push's
   own error and does not say "moved by hand".
2. Read *fails*: the returned error reports that the park could not be
   confirmed, carries both the push's and the read's errors via
   unwrap, and does not say "moved by hand".
3. Read answers a *different sha*: still a `RescueConflictError`, and
   its message now names both the sha found and the tip being parked.
4. Read answers *the tip*: nil error — pinned by asserting alongside
   the existing `TestScavengeIsIdempotent`, which must stay green.

**GREEN.** Rework `park`. Keep the push's error instead of discarding
it. Classify with `remoteHolderErr` instead of `remoteHolder`. Return
a typed error per shape: the push's own failure for absent; an
unconfirmed-park error wrapping both faults for a failed read, reusing
`UnconfirmedPushError`'s shape; `RescueConflictError` extended with
the found sha for a genuine conflict. yield, start and reap render the
error's text already, so no caller changes.

**Gate.** `go test ./internal/claim` red first, then green; the
existing foreign-rescue refusal and idempotent-scavenge tests
unchanged and green; then the full suite and lint.
