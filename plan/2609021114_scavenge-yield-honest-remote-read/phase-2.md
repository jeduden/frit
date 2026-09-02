---
n: 2
title: Yield refuses rather than guess when the still-held read fails
status: "✅"
result: false
---
Make `Yield`'s still-held check in
[internal/claim/lease.go](../../internal/claim/lease.go) refuse instead
of silently treating an unreadable remote as "not held". Drive it
red/green with a scripted runner.

**Assumes.** Phase 1's `UnconfirmedDeleteError` and the
`remoteHolderErr` split it reuses set the precedent this phase follows
for a second call site. `Yield` already refuses outright with
`StillHeldError` when the read confirms this lane is still the live
holder (F4, F5) — an unreadable read must refuse too, not fall through
to `park`, since a fold-to-absent read of "" cannot be told apart from
a genuinely absent ref by `remoteHolder`'s own contract.

**RED.** One unit test on `Yield`, through a scripted runner whose
`ls-remote` for the still-held check fails.

1. The still-held read fails: `Yield` returns a typed error naming the
   plan and wrapping the read's fault, does not call `park`, and parks
   nothing to the rescue ref — an unconfirmed live-or-not holder must
   not be treated as fenced.

**GREEN.** Replace `remoteHolder(repoDir, opts.Remote, ref, run) ==
local` with `remoteHolderErr`. A read fault returns a new typed error
(e.g. `UnconfirmedYieldError`, `PlanID` and `Err` fields, `Unwrap`
returning `Err`) before `park` is ever called. A confirmed match with
`local` keeps today's `StillHeldError`. A confirmed non-match proceeds
to `park` exactly as today.

**Gate.** `go test ./internal/claim` red first, then green; every
existing `Yield` test (still-held refusal, no-local-ref no-op, the
foreign-holder no-op) stays green unchanged; then the full suite and
lint.
