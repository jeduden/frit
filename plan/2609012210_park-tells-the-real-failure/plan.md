---
id: 2609012210
title: A failed park names its real failure, not "moved by hand"
status: "✅"
summary: >-
  When the rescue-ref push fails, park reports the same "moved by hand"
  refusal no matter what actually happened — a rejected or slow pre-push
  hook, a timeout, an unreadable remote — sending the operator to
  inspect a ref that may not even exist (#129). The push's own error is
  discarded, and the confirmation read swallows its fault, folding
  "could not read" into "different object". This plan makes park
  classify its failure honestly: an already-parked rescue proceeds, a
  confirmed different object keeps today's refusal (now naming both
  commits), a push that failed leaving nothing surfaces the push's own
  error, and a failed confirmation read surfaces both faults. The
  richer errors reach yield, start and reap through the reporting they
  already have.
model: sonnet
depends-on: []
---
# A failed park names its real failure, not "moved by hand"

## Goal

A park that cannot land its rescue ref tells the operator what
actually happened: the push's own rejection, a timeout, or an
unreadable remote. The "moved by hand" refusal is reserved for the one
case it names — a confirmed different object at the rescue ref.

## Context

**The gap.** [Issue #129](https://github.com/jeduden/frit/issues/129):
`frit yield` refused a park with "rescue ref … was moved by hand" when
the ref had never existed on the remote at all — the real cause was a
slow, failing client-side pre-push hook rejecting the push. The refusal
sent the operator to fetch and inspect a ref that was not there, and
gave no hint that the push itself had failed or why.

**The cause.** `park` in
[internal/claim/lease.go](../../internal/claim/lease.go) pushes
create-only and, when the push fails, discards the push's error
entirely. It then reads the remote to classify the failure — but with
`remoteHolder`, whose contract folds a failed read into "absent". Every
outcome except "the ref already holds the tip" therefore collapses into
`RescueConflictError`, whose wording assumes a hand-moved ref. Four
distinct situations share one message, and three of the four are
misdescribed.

**Reuse first.** The classification read exists already.
`remoteHolderErr` keeps the read fault apart from an absent ref. It
was built for exactly the caller "for whom gone and unreadable must
not fold together". `UnconfirmedPushError` already models a push that
failed with a confirmation read that failed too, for the lease's own
CAS push. It carries an `Err` field and unwrap; park's unconfirmed
case reuses that shape rather than minting a parallel one. The
already-parked no-op returns nil today and is pinned by
`TestScavengeIsIdempotent`. It stays untouched and green. That answers
the issue's second ask for the case the content-addressed name is
designed around. The fake-runner pattern for driving `park` through
scripted push and read outcomes exists in
[internal/claim/lease_test.go](../../internal/claim/lease_test.go).

**Callers need no new plumbing.** The refusal reaches the operator
through three sites — yield's document warning, start's wrapped error,
reap's per-lane reason — all of which render the error's own text. A
richer typed error changes what they say without changing how they say
it.

**Considered and not taken.** Treating a rescue ref that exists with a
*different* object as "already parked" is rejected: the ref name is
content-addressed by the tip, so a different object under that exact
name is precisely the forged/hand-moved case the refusal exists for.
Instead the refusal now names both commits — the one found and the one
being parked — so a mismatch is self-diagnosing. Retrying the push
inside park is also rejected: a hook that takes minutes would stall
every teardown verb; surfacing the truth and letting the operator act
beats hiding a retry loop.

## Tasks

1. Phase 1 (proving slice): drive `park` through its four failure
   shapes with a scripted runner and make each answer honest — push
   error surfaced when the ref is absent, both faults surfaced when the
   read fails too, the conflict refusal naming both commits, the
   already-parked no-op pinned green.
2. Later, if the handoff shows the need: caller-side wording (yield,
   start, reap) beyond what the richer error text already provides.

## Execution

| Phase | Title                                          | Tier   | Gate                                                                                                             |
| ----- | ---------------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------- |
| 1     | park classifies its failure into four outcomes | sonnet | new red tests in internal/claim pass green; `TestScavengeIsIdempotent` and the foreign-rescue refusal stay green |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| --- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1   | ✅     | [park classifies its failure into four outcomes](phase-1.md)                                                                                                                                                                                                                                                                                                                                                                                                                   |
|     | ↳      | park now reads the remote with remoteHolderErr on a failed push and answers each of the four shapes honestly: the push's own error when the rescue ref is absent, an UnconfirmedPushError wrapping both faults when the confirmation read itself fails, a RescueConflictError naming both the sha found and the tip being parked on a genuine conflict, and a clean no-op when the read confirms the tip already landed. "moved by hand" is said only in that last, true case. |
<?/catalog?>

## Acceptance Criteria

- [x] A park whose push fails while the rescue ref is absent on the
      remote reports the push's own failure, not "moved by hand"
- [x] A park whose push fails and whose confirmation read also fails
      reports both faults and does not claim the ref was moved
- [x] A park finding a different object at the rescue ref still
      refuses, and the refusal names the commit found and the commit
      being parked
- [x] A park finding the tip already at the rescue ref proceeds as
      today: no error, teardown continues
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
