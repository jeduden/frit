---
n: 1
title: A session renewal reads the work ref's current tip, guarded to self
status: "✅"
result: true
summary: claim.RenewToBind is the session-stamping renewal — a CAS lost to this lane's own hold is a stale baseline, so it renews again from the tip that won; a foreign or unreadable mover is returned as the fence it is, and an unmoved ref still costs exactly one mint.
---
## Handoff

`claim.RenewToBind` in
[internal/claim/lease.go](../../internal/claim/lease.go) is the atom
the bind needed. It is `Renew` plus one decision: when the CAS loses,
ask whether the tip that beat it is still our own hold, and if it is,
renew again from there. `Renew` itself is untouched, so the
beat-for-holder and resume paths behave exactly as before — nothing
that already worked shifted.

**The guard is free, and it had to read the governing marker.** The
phase spec proposed reading the fence's tip with `commitMarker`. That
would have answered "foreign" for the very case being fixed: the
advance that causes this bug is the lane's own ordinary work commit,
whose message carries no lease marker at all. `FenceError` already
arrives carrying the *governing* marker — `fenceError` fills it via
`fetchedMarker`, which walks back from the tip to the nearest marker
for the plan, straight past any run of work commits. So `ownHold`
reads `fenced.Known`, `fenced.Marker.Holder` and `fenced.Marker.Lane`
off the error in hand: same machine, same lane, and the reconcile
fires. No extra git call, and the unmoved path never enters the branch
at all — `advance` wins first time.

**The refusals are the safe direction.** An unread marker
(`Known` false) is returned as a fence: not provably ours is treated
as not ours. A holder that differs, or the same holder on a different
lane, is likewise a real fence and keeps its warning verbatim — the
mover is still named, and nothing is stamped over it. The reconcile
runs at most once; a second loss from the re-read tip is returned as
it comes, so a genuinely contended ref cannot spin.

**Proven, against real git rather than a scripted runner.** The phase
spec called for the fake `gitwt.Runner`; the file's renewal and fence
tests all drive real origin-and-clone fixtures, and a scripted runner
could not have produced the walk-back marker read the guard turns on,
so the tests follow the file.
`TestBindRenewReconcilesARefTheOwnHolderAdvanced` pushes the lane's own
work commit onto `plan/7` between the acquire and the renewal, then
asserts the beat is a child of that advanced tip and carries
`session: sess-1`. `TestBindRenewStillFencesAForeignTip` puts box-b's
takeover marker on the ref and asserts the `FenceError` still names
box-b with nothing stamped. `TestBindRenewFromAnUnmovedTipIsAPlainRenew`
counts `commit-tree` calls through a wrapping runner and pins the
common path at exactly one mint. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` are green.

**What Phase 2 inherits.** `bindSession` in
[cmd/frit/start.go](../../cmd/frit/start.go) swaps `claim.Renew` for
`claim.RenewToBind` and nothing else: the signature is identical, and
the token is persisted on a win the same way, so the call site changes
by one identifier. The stale `lease.Tip` stays the right argument to
pass — reconciling it is now the atom's job, not the caller's. The
warning-not-abort contract needs no change either: what still comes
back after the reconcile is a genuinely foreign fence, which is
precisely what that warning was always for.
