---
n: 1
title: advance relays a locally fast-forwarded lease branch, or refuses
status: "✅"
result: false
---
Prove issue #189 at the lease-unit level. Use the existing fake runner
in
[internal/claim/lease_test.go](../../internal/claim/lease_test.go).
Then fix `advance` where it mints the next marker.

RED: build a fixture the way `OwnAdvance`'s own tests do — an existing
claim marker at some tip `T0`, then a beat at `T1` your caller renews
from (`from = T1`). Before calling `Renew`, add one ordinary commit
onto the local `plan/<id>` ref on top of `T1` — a fast-forward, the
shape an unpushed `git merge --ff-only origin/main` leaves on a lane's
own branch. Call `Renew(repoDir, opts, T1, run)`. Assert today's
result: the local `plan/<id>` ref now points at the new marker, and
that marker's ancestry does not include the local-ahead commit — it
was minted as a child of `T1` and `syncLocalRef` reset straight past
it. Add a second fixture where the local ref instead diverges from
`T1` (an unrelated commit, not its descendant) and assert `Renew`
today resets past that too, with the same blind `update-ref`.

GREEN: in `advance` (lease.go:1022), before minting, read the local
`plan/<id>` ref. Use `rev-parse --verify --quiet`, the same call
`refuseDivergingLocalBranch` already makes. Three cases:

- No local ref, the local ref equals `from`, or it is behind `from`
  (an ancestor of it — the ordinary stale view, nothing on it missing
  from `from`): mint on `from`, exactly as today. The behind case was
  added during execution; refusing it would fence the session bind's
  own reconcile on a host whose local copy lags origin.
- The local ref is a fast-forward of `from` (`isAncestor(from,
  localTip)`): mint the marker as a child of `localTip` instead of
  `from`. Pass this new tip as `casPush`'s `marker` argument; pass
  `from` unchanged as `casPush`'s `expected` argument — the CAS still
  arbitrates against the remote's real prior value, so a genuine
  foreign race still loses or fences exactly as before. Only what the
  new marker is built on top of changes.
- The local ref exists, is not `from`, and is neither a fast-forward
  nor an ancestor of it (a real divergence): refuse before minting
  anything, naming the branch and both tips. Done as the sibling
  `LeaseDivergesError`, since `LocalDivergesError`'s wording tells a
  fresh claimant to push or rename; this one says to merge `from` in.

Thread this through `Renew`, `RenewToBind`'s two `advance` calls, and
`Release` — `advance`'s only callers. All three then inherit it
uniformly. Leave `Takeover` and `pushClaimMarker`'s existing
fresh-acquire guard untouched, per the plan's Scope note.

Also fix `syncLocalRef` (lease.go:1213). Call `update-ref <ref> <new>
<old>` with the ref's value just before the move. Fall back to the
two-argument form only when no prior value could be read (e.g. the ref
did not exist). Add `-m` naming the transition kind, mirroring
`leaseMessage`'s own kind constants. Then every move it makes —
relayed, ordinary, or the refusal path's untouched ref — is `git
reflog`-visible.

BDD coverage: pick the `@S<n>` reservation at execution time, against
origin's current
[lease-protocol.md](../../docs/research/lease-protocol.md) matrix. Do
not reuse a number only sketched in this plan's Context. Add the
scenario to [lifecycle.feature](../../features/lifecycle.feature).
Reuse the section's world and step registrar
(`cmd/frit/bdd_lifecycle_test.go`). Follow
[docs/development.md](../../docs/development.md)'s procedure.

Gate: the RED fixtures fail against today's code and pass after the
fix — the local-ahead commit's SHA is an ancestor of the ref's new tip,
and the diverging fixture refuses by name instead of resetting. A
third fixture proves a genuine foreign push during the same window
still fences, unchanged. A fourth asserts `git reflog show plan/<id>`
carries an entry for the renewal's move. `go test ./...` and
`go tool -modfile=tools/go.mod golangci-lint run` pass; the new `@S<n>`
runs without `@pending`.
