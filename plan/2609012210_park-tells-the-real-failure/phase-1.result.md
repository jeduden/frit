---
n: 1
title: park classifies its failure into four outcomes
status: "✅"
result: true
summary: >-
  park now reads the remote with remoteHolderErr on a failed push and
  answers each of the four shapes honestly: the push's own error when
  the rescue ref is absent, an UnconfirmedPushError wrapping both
  faults when the confirmation read itself fails, a RescueConflictError
  naming both the sha found and the tip being parked on a genuine
  conflict, and a clean no-op when the read confirms the tip already
  landed. "moved by hand" is said only in that last, true case.
---
## Handoff

`park` in
[internal/claim/lease.go](../../internal/claim/lease.go) no longer
discards the push's error or folds every non-landed outcome into
`RescueConflictError`. It classifies a failed push exactly the way
`casPush` already classifies its own — with `remoteHolderErr`, which
keeps a read fault apart from a confirmed-absent ref:

- absent ref → the push's own error, unwrapped
- read also fails → `&UnconfirmedPushError{Err: errors.Join(pushErr,
  readErr)}`, so both faults are reachable through `errors.Is`
- a different object at the exact rescue name → `RescueConflictError`,
  now carrying `Found` and `Tip` so its message names both commits
- the read confirms the rescue ref already holds the tip → nil, the
  half-done-retry no-op unchanged

`RescueConflictError` gained the `Found` and `Tip` fields; it had
exactly one construction site (`park` itself), so no other caller
needed updating. `Yield` and `ParkUnlanded` both already return
`park`'s error as-is, and yield's document warning, start's wrapped
error, and reap's per-lane reason all render an error's own text — so
none of the three callers needed a wording change to carry the richer
messages through.

Four scripted-runner unit tests in
[internal/claim/park_test.go](../../internal/claim/park_test.go) pin
the four shapes directly against `park`, mirroring the fenced-runner
pattern already used for `casPush` in `caspush_test.go`. They went red
against the old classification (three of four; the already-parked
no-op was already true), then green after the rework.
`TestScavengeIsIdempotent` and `TestScavengeRefusesAForeignRescue`
stayed green throughout, unedited. `go test ./...`, `go tool
-modfile=tools/go.mod golangci-lint run ./...`, and `go build ./...`
are all clean.

This closes plan 2609012210 and the first ask of
[issue #129](https://github.com/jeduden/frit/issues/129): a failed
park no longer sends the operator to inspect a rescue ref that may
never have existed. The plan's second task — caller-side wording in
yield, start and reap beyond what the richer error text already
carries — was left for a later plan if the handoff there ever shows
the need; nothing in this phase's work found one.
