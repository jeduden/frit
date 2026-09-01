---
n: 1
title: A session renewal reads the work ref's current tip, guarded to self
status: "🔳"
result: false
---
Give the lease atom the reconcile the bind needs. A renewal that stamps
a session reads the work ref's current tip. It renews from that tip when
the tip still carries our own hold. It no longer fences on a baseline
the lane has already moved past. This is the pure
[internal/claim/lease.go](../../internal/claim/lease.go) slice; wiring
`bindSession` to it is Phase 2.

**Assumes.** `remoteHolder(repoDir, remote, ref, run)` reads the work
ref's current remote tip. `ReadMarker`/`commitMarker` read a tip's lease
marker, carrying `Holder` and `Lane`. `advance` already mints a beat
child of a given tip and CASes from exactly that tip. A tip whose marker
carries our own `Holder` and this lane's own `Lane` is our hold, not a
foreign one — the identity check `orphans` and `reap` already trust.

**Value.** A session renewal no longer self-fences on a ref its own lane
advanced between the mint and the bind. It renews from where the ref
actually is, so the beat is a child of the current tip and its CAS holds
— the session is stamped. A tip that moved under a foreign holder is
still a real fence, unchanged.

**RED.** In [internal/claim/lease_test.go](../../internal/claim/lease_test.go),
against the fake `gitwt.Runner` the lease tests already use to script a
remote tip and its marker.

- `TestBindRenewReconcilesARefTheOwnHolderAdvanced`: script the remote
  work ref ahead of the passed-in `from` — a later commit whose reachable
  marker still carries this run's own `Holder` and `Lane`. Call the new
  reconcile with the stale `from`. Assert it does not return
  `FenceError`: it reads the current tip, mints a beat child of it, CASes
  from it, and returns a `Lease` whose `Tip` is that beat. Assert the
  beat carries the `Session` trailer passed in.
- `TestBindRenewStillFencesAForeignTip`: the remote ahead of `from` under
  a different `Holder`. Assert the reconcile returns `FenceError` naming
  that foreign holder — a real fence keeps its warning, and nothing is
  stamped.
- `TestBindRenewFromAnUnmovedTipIsAPlainRenew`: the remote exactly at
  `from`. Assert the reconcile renews once from `from` with no extra
  read-and-retry, returning the beat — the common path costs nothing new.

**GREEN.** In [internal/claim/lease.go](../../internal/claim/lease.go).

- Add `RenewToBind(repoDir string, opts LeaseOptions, from string, run
  gitwt.Runner) (Lease, error)`: try `advance(repoDir, opts, markerBeat,
  from, run)`; on a `FenceError` whose current tip carries our own
  `Holder` and this lane's `Lane` (read via the marker on the fence's
  `Tip`), renew again from that current tip and return it; on a foreign
  fence, return the `FenceError` unchanged.
- Read the guard off the fence's own `Tip` and marker rather than a
  fresh remote round-trip — `FenceError` already carries the tip it lost
  to, and `commitMarker` reads that tip's `Holder`/`Lane`.
- Keep `Renew` as it is: `RenewToBind` is the session-stamping variant,
  and the plain renewal the beat-for-holder and resume paths use is
  unchanged, so no existing caller shifts behavior.

**Guard the edges.** A single self-loss reconciles once; a second loss
from the re-read tip is returned as-is rather than looped, so a live
contended ref cannot spin. A fence with an unreadable marker on its tip
is treated as foreign — the safe direction, since it is not provably our
own. An unmoved ref never enters the reconcile branch: `advance`
succeeds first time.

**Gate.** With the remote work ref ahead of the mint tip under our own
holder, the session renewal reconciles and stamps the session; a foreign
tip still fences; an unmoved tip renews once. `go test ./...` and `go
tool -modfile=tools/go.mod golangci-lint run` are green.
