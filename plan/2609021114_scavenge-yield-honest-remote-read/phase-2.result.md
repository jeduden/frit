---
n: 2
title: Yield refuses rather than guess when the still-held read fails
status: "✅"
result: true
summary: Yield's still-held check switches from remoteHolder to remoteHolderErr; a read fault now returns a typed UnconfirmedYieldError and refuses before park is ever called, instead of falling through to park a lease that may still be held live.
---
## Handoff

`Yield`'s still-held check in
[internal/claim/lease.go](../../internal/claim/lease.go) called
`remoteHolder(repoDir, opts.Remote, ref, run) == local` to decide
whether this lane still holds the live lease. An unreadable remote
read as `""`, so `"" == local` was false and Yield fell through to
`park` a lease that might in fact still be live. The site now calls
`remoteHolderErr`: a read fault returns a new `UnconfirmedYieldError`
(`PlanID`, `Err` — the read's own failure, reachable through
`Unwrap`, the same shape as `casPush`'s `UnconfirmedPushError`) and
`park` is never reached. A confirmed match with `local` keeps today's
`StillHeldError`; a confirmed non-match proceeds to `park` exactly as
today.

**Proven.** `TestYieldReportsAnUnconfirmedYieldWhenTheStillHeldReadFails`
in [internal/claim/lease_test.go](../../internal/claim/lease_test.go)
scripts the still-held `ls-remote` to fail, against a real repository
via `originAndClone`/`Acquire`. Unlike phase 1's Scavenge test, no
call-counting was needed: the still-held check is `Yield`'s only
remote read before `park`, so any `ls-remote` failure exercises it
directly. All four pre-existing `Yield` tests (current-holder refusal,
both no-local-ref no-ops, local-divergence parking) stay green
unchanged. `go test ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean.

**This plan is done.** Both Acceptance Criteria pairs are met; `plan.md`
flips to ✅.

**A CLI-layer gap `/code-review high` found and closed after this
phase's own commit.** `cmd/frit/yield.go`'s `Run` only special-cased
`StillHeldError`; every other `claim.Yield` error, including the new
`UnconfirmedYieldError`, fell into the generic `doc.Warn(fmt.Sprintf(
"park: %v", err))` branch — mislabeling a pre-park refusal as a park
failure and, worse, printing `"yielded plan %d"` ahead of the warning,
claiming a success that never happened (nothing was parked or torn
down). Fixed to `doc.Refuse`, the same as `StillHeldError`, driven by
a new `TestYieldReportsAnUnconfirmedYieldAsARefusalNotAWarning` in
[cmd/frit/yield_test.go](../../cmd/frit/yield_test.go) — it breaks the
origin remote's URL after fencing the lane and runs `yield --no-fetch`
so `claim.Yield`'s live still-held read fails for real, while fleet
discovery stays on local refs.

**Left for elsewhere.** Phase 1's `code-review`-found third call site —
`park`'s own rescue-push confirmation folding a failed read to
absent — resolved itself when this lane merged `origin/main`: PR #136
landed plan 2609012210's actual fix, and `park` now calls
`remoteHolderErr` too (`internal/claim/lease.go:744`). Nothing further
to do there. The considered-and-not-taken consolidation of `park`,
`casPush`, Scavenge's delete and Yield's still-held check onto one
shared push-then-classify helper remains plan 2609021115's to design,
now that all four real call sites (`UnconfirmedPushError` ×2,
`UnconfirmedDeleteError`, `UnconfirmedYieldError`) exist to draw its
shape from.
