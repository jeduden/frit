---
n: 2
title: start's refusal names a live agent, not a blanket un-matured takeover
status: "✅"
result: true
summary: >-
  `frit start`'s refusal now names a live agent attending a held lane —
  "already held (...); a live agent is on this lane; nudge or open it
  instead of starting" — rather than the blanket un-matured-takeover
  wording, which wrongly suggested waiting would free it. A hold this
  machine cannot prove keeps its existing, already-honest wording
  unchanged. lease-protocol.md's S76 narrative now records that both
  `open` and `start` name the ladder's own rungs.
---
## Handoff

`liveHoldRefusal` in [cmd/frit/start.go](../../cmd/frit/start.go) calls
`holdKindFor` — the same read phase 1 built for `open` — from
`startRefusal`, ahead of the generic `claimRefusal` fallback. It fires
only for `report.HoldLive`; every other kind, `HoldUnproven` included,
falls through untouched, so `claimRefusal`'s "not takeable until the
window matures" wording stays exactly as it was for a hold this machine
cannot prove — that wording was already honest, and this phase does not
touch it or `claimRefusal` itself, which stays shared with the plain
`frit claim` verb. `startRefusal` and `buildStart` now thread `coordOK`
through, the same value `open` already computes, so `holdKindFor` reads
the ambiguous-repository case identically in both places (it falls back
to `HoldUnproven`, never guessing `HoldLive`).

**What a later phase inherits.** `nudge`'s own refusal — Task 3, not
started — is the last rung: today it always refuses with a bare
`"no live lane for plan %d"`, whether nothing holds the plan or a
healthy-but-unattended lane sits there with its agent gone. It should
call `holdKindFor` the same way `open` and `start` now do, and tell a
lane with no hold at all apart from one that is held but has no live
pane to prompt — reusing this phase's and phase 1's wording family
rather than inventing a third one. `lease-protocol.md`'s S76 entry is
now fully closed on the documentation side: both rungs' role in
resolving it are recorded, so no further doc phase is expected once
`nudge` lands, beyond folding `nudge` itself into the same narrative
paragraph if it reads as a fourth thing worth naming there.

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean.
