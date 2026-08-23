---
id: 2608231006
title: A lane owns its lease after its own raw commits advance the branch
status: "✅"
summary: >-
  A lane's worktree sits on the same plan/<id> branch its lease markers
  live on, so every raw TDD commit advances origin's tip past the
  persisted lease token. ownToken's exact-equality check then reads the
  lane's own lease as foreign, and release/renew/resume refuse it.
  Recognize origin's tip as this lane's own when it descends from the
  token under the same epoch, and re-anchor to it — while still fencing
  a genuine foreign takeover.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: release recognizes a lane whose own commits advanced the tip
    status: "✅"
  - n: 2
    title: renew and resume re-anchor too, and a foreign takeover still fences
    status: "✅"
---
# A lane owns its lease after its own raw commits

## Goal

A lane that advanced its `plan/<id>` branch through ordinary
`git commit`/`git push` — the prescribed red/green workflow — is still
recognized as the lease's own holder. `release` (and `renew`/`resume`)
re-anchor to origin's current tip and succeed, rather than refusing the
lane's own lease as "held live by another lane". A genuine foreign
takeover is still fenced.

## Context

The tension is structural. A lane's worktree is stood up on branch
`plan/<id>` (`standUpClaimWorktree`,
[cmd/frit/claim.go](../cmd/frit/claim.go)) — the very ref the lease
markers live on. frit only advances the persisted token on its own
transitions (`persistToken`,
[internal/claim/token.go](../internal/claim/token.go)); a raw
`git commit` does not. So after any TDD commit, origin's `plan/<id>`
tip is ahead of the token.

`ownToken` ([cmd/frit/claim.go](../cmd/frit/claim.go)) then fails:

```go
tip = claim.RemoteTip(coord.Path, coord.Remote, plan.ID, rt.git)
if tip == "" || tip != token {   // exact equality
    return "", "", false
}
```

`releaseHeld` ([cmd/frit/release.go](../cmd/frit/release.go)) reads that
`false` as `foreignHoldRefusal(plan)` — "held live by another lane" —
even though the mover was the lane itself. This was hit for real on plan
2608221754: the lane had to hand-resync its token file to origin's tip
before `release` would go through. Because the prescribed workflow *is*
raw TDD commits, every lane that reaches for `release` after committing
hits it.

Reuse searched:

- **`ownToken`** ([cmd/frit/claim.go](../cmd/frit/claim.go)) — the one
  identity check `release`, `renew`, and `resume` all funnel through, so
  fixing it here fixes every caller. `inOwnLane` already separates "my
  lane" from "my token cannot resume it"; this plan makes the token
  check tolerate the lane's own advance.
- **`claim.RemoteTip`** — already reads origin's current tip fresh, so
  the re-anchor target needs no new read.
- **`commitMarker`** ([internal/claim/lease.go](../internal/claim/lease.go))
  — already decodes a marker's kind and epoch from its trailers. Reused
  to tell "origin advanced under my epoch by my own work" from "a
  takeover minted a new epoch": a takeover is a child of the observed
  tip too, so an ancestor test alone is not enough — the epoch and
  holder at origin's tip must still be this lease's.
- **`git merge-base --is-ancestor`** — the ancestry half of the test:
  the persisted token must still be reachable from origin's tip, or the
  chain was replaced, not extended.

The guard is not "trust any descendant". A foreign takeover mints its
marker as a child of the observed stale tip, so it can descend from the
old token; the epoch/holder check is what keeps the fence honest.

## Tasks

1. `ownToken` recognizes origin's tip as this lane's own when the token
   is an ancestor and the epoch/holder at the tip is unchanged, and
   re-anchors `release` to that tip.
2. `renew` and `resume` inherit the re-anchor through the shared
   `ownToken`/`resumeToken` path with no further code change; add the
   regression tests, pin the foreign-takeover fence, and record S86.

## Phase 1: release recognizes a lane whose own commits advanced the tip

**RED.** In [cmd/frit/release_test.go](../cmd/frit/release_test.go) (or
the claim test that exercises `ownToken`), add a test that: stands a
lane up and claims `plan/<id>`, then adds an ordinary work commit on
`plan/<id>` and pushes it — no `frit` transition between — so origin's
tip is a descendant of the persisted token under the same epoch. Run
`release` from that lane. Assert it succeeds (a release marker lands, no
"held live by another lane" refusal), and the token file need not be
hand-edited. It fails today: `ownToken`'s `tip != token` returns false
and `releaseHeld` refuses.

**GREEN.** In `ownToken`, when `tip != token`, do not immediately fail.
If the token is an ancestor of `tip` (`merge-base --is-ancestor`) and
the marker state governing `tip` is still this lease's epoch and holder
(`commitMarker` shows no intervening takeover/foreign release), treat
the lane as the owner and return the current `tip` as the anchor. Any
other case — token not an ancestor, or epoch/holder changed — returns
false as before, so a foreign move still refuses.

**Gate.** `go test ./cmd/frit -run 'TestRelease|TestClaim'` green
including the new own-advance test and the existing foreign-refusal
tests; `go build ./...`; `go vet ./...`.

## Phase 2: renew and resume re-anchor too, and a foreign takeover still fences

**RED.** Add tests that `renew` and `resume` (the self-resume path in
`claim`/`start`) likewise succeed for a lane whose own commits advanced
the tip, and a test that a genuine foreign takeover — a takeover marker
minted at a new epoch from the observed tip — is still refused by
`ownToken` as foreign, not mistaken for the lane's own advance. They
pin that the Phase 1 relaxation did not open the fence.

**GREEN.** Confirm `renew`/`resume` inherit the re-anchor through the
shared `ownToken`/`resumeToken` path, adjusting the CAS `from` to the
re-anchored tip where those transitions advance the marker. Assert the
epoch/holder guard rejects the new-epoch takeover.

Also add this case to the scenario matrix in
[docs/research/lease-protocol.md](../docs/research/lease-protocol.md).
Give it the next id S86. Place it beside its sibling S77 (own-host,
commits ahead of origin):

```text
| S86 | a live lane's own raw commits advance the branch past its persisted token | ownToken re-anchors: origin's tip descending from the token under the same epoch and holder is the lane's own advance, so release/renew/resume succeed without a hand-edited token; a new-epoch takeover minted from the observed tip still fences (RESUME, FENCE) |
```

**Gate.** `go test ./cmd/frit -run 'TestRelease|TestClaim|TestStart'`
and `go test ./internal/claim` green; a foreign takeover still fences;
`go build ./...`, `go vet ./...`, `go test ./...` all green.

## Execution

| Phase | Work                                                       | Tier   |
| ----- | ---------------------------------------------------------- | ------ |
| 1     | Proving slice: release recognizes a lane's own tip advance | sonnet |
| 2     | renew/resume re-anchor; the takeover fence stays closed    | sonnet |

## Acceptance Criteria

- [x] A lane that raw-committed on `plan/<id>` can `release` without
      hand-editing its token file.
- [x] `renew` and `resume` likewise tolerate the lane's own advance.
- [x] A genuine foreign takeover (new epoch from the observed tip) is
      still refused as foreign — the fence stays closed.
- [x] The re-anchor uses origin's current tip, read fresh, not the
      stale local view.
- [x] The scenario is recorded as S86 in the lease-protocol matrix
      ([docs/research/lease-protocol.md](../docs/research/lease-protocol.md)),
      naming the resolution and the fence that stays closed.
- [x] `go build ./...`, `go vet ./...`, `go test ./...`, and
      `mdsmith check .` all pass.
