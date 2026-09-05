---
n: 1
title: Yield refuses a foreign, unfenced hold and names it
status: "✅"
result: false
---
Make `frit yield` honest about a lane it cannot end. Run against a
plan another lane holds, from a pane that is not that lane's own
worktree, with nothing local to park, yield refuses the way `release`
does. It carries the way out in `next_action`, instead of printing
`yielded plan <id>` for work it never performed.

**Assumes.** `Yield` in
[internal/claim/lease.go](../../internal/claim/lease.go) returns a
clean no-op `Scavenged{}` when the local work-ref tip is empty, before
it reads the remote holder. `yieldCmd.Run` in
[cmd/frit/yield.go](../../cmd/frit/yield.go) reads that no-op as
success: `doc.Parked("")`, `tearDownLane`, then `printYield` writes
`yielded plan <id>`. The gather has already filled `plan.HoldTip`,
`plan.Held`, `plan.Stale` and `plan.Dead` for the plan. Those are the
same facts `releaseCmd` dispatches on. `foreignHoldRefusal` in
[cmd/frit/release.go](../../cmd/frit/release.go) words a hold this
lane cannot end. `unprovenNextAction` in
[internal/report/dispatch.go](../../internal/report/dispatch.go) is
the shared way-out sentence. `YieldDoc`, in the same file, has
`Refuse` and `Warning` but no `next_action` field. `localRef` in
[cmd/frit/yield.go](../../cmd/frit/yield.go) reads this lane's own
work-ref tip, empty when the ref was never fetched or minted here.

**Value.** The one verb the "deserted row" recipe reaches for stops
reporting a success it did not perform. A person or a skill that runs
yield on a lane it cannot end is told so. They are told to wait the
window or take it over — the same answer release already gives. The
fenced lane yield exists for keeps parking and tearing down unchanged.

**RED.** In [cmd/frit/yield_test.go](../../cmd/frit/yield_test.go),
against a fixture whose remote lease is held by another lane and whose
local work ref is absent:

- `TestYieldRefusesAForeignHoldWithNothingToPark`: `frit yield <id>`
  reports no `parked` and no `torn down`. The JSON document carries a
  non-empty `refused` naming the live holder and a non-empty
  `next_action`. The remote lease tip is unchanged.
- `TestYieldRefusesAMaturedForeignHoldNamingTheTakeover`: with the
  hold's window matured, or its session confirmed gone, the refusal
  names `frit claim` as the takeover, matching release's wording.
- `TestYieldOnAnUnheldPlanIsStillACleanNoOp`: a plan whose `HoldTip`
  is empty, and which nobody holds, still succeeds. It parks nothing
  and refuses nothing — the honest no-op, distinct from the foreign
  refusal.

Confirm the fenced path still works. A lane with a local divergent ref
still parks to its rescue ref. It still tears its worktree down as
today. An existing test covers this, or add
`TestYieldStillParksAndTearsDownAFencedLane`.

**GREEN.** Give `yieldCmd.Run` a dispatch on the gather's plan facts,
mirroring `releaseCmd`. When `plan.HoldTip == ""`, nobody holds it:
the clean no-op. When `localRef` returns a divergent tip, the lane is
fenced, and `claim.Yield` parks and tears down as today. Otherwise the
hold is foreign with nothing local to park, so refuse. Word the
refusal from `foreignHoldRefusal`, shared or lifted so both verbs read
it. Add a `RefuseUnproven`-style setter to `YieldDoc`. It fills both
`Refused` and a new `next_action` field, from `unprovenNextAction`.
`printYield` prints the field under a refusal, and `renderYield`'s
JSON already serialises the doc.

**Guard the edges.** `claim.Yield`'s own empty-local no-op stays. The
command no longer calls it in the foreign case, so its contract is
unchanged and its existing tests hold. A foreign hold whose remote
read fails must still refuse, not fall through to a false success:
lean on the gather's facts, which already carry the hold, rather than
a second unbounded remote read in the command. The pane warning about
a worktree left standing is not printed under a refusal. Nothing was
torn down because nothing was owned, and the refusal already says so.

**Gate.** Against the built frit in a scratch fleet with a bare
remote: a plan claimed by one lane, then `frit yield <id>` from an
unrelated pane, prints the refusal and a non-empty `next_action`. It
writes no rescue ref and leaves the remote lease tip unchanged.
`frit yield <id> --json` carries `refused` and `next_action`. A plan
nobody holds still yields clean. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.
