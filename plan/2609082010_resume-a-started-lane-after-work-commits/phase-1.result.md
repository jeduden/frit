---
n: 1
title: Resume after normal dispatch and pushed work
status: "✅"
result: true
summary: >-
  Reproduced issue #186 through a real CLI `start --go` dispatch, then
  fixed marker lookup so a resume's ownership proof walks past a work
  commit that only shares the marker's own "plan <id>: " prefix.
---
## Handoff

Reproduced issue #186 exactly: `frit start --go` dispatches a lane for
real (CLI acquisition, worktree stand-up and session bind, no
hand-built lease), two ordinary work commits titled `plan <id>:
<title>` — this project's own convention — are pushed on top of the
persisted token, and a resumed `start --go` from inside the lane
refused with the takeover-window message instead of resuming.

Root cause: `latestMarker` in
[internal/claim/lease.go](../../internal/claim/lease.go) read only the
*nearest* commit `git log -1 --grep` matched against the `plan <id>: `
prefix. A work commit sharing that prefix but carrying no real marker
kind failed `parseMarker` and was never retried — the governing claim
or beat marker further back in history was never seen, so
`OwnAdvance`/`tokenProves` read the lease as unprovable and the resume
fell through to the ordinary claim path. Reproduced independently
against this very repository's own history (plan 2609061856's leftover
branch): the same masking made `heldError` drop `Known` and `Landed`,
turning the dedicated "already landed" refusal into a generic lost-race
message.

Fix: `latestMarker` now walks every commit the grep pattern matches,
nearest first, until one actually parses as a marker, instead of
trusting the first hit. `commitMarker`'s single-commit read is
unchanged, so a release marker still shadows work pushed on top of it
exactly as before.

Coverage: `OwnAdvance`/`heldError` unit tests in
[internal/claim/lease_test.go](../../internal/claim/lease_test.go); a
control (plain `red:`/`green:` subjects) and the plan-prefixed
regression, both driven through a real CLI dispatch, plus their `--json`
and dry-run counterparts, in
[cmd/frit/start_test.go](../../cmd/frit/start_test.go); `@S94` in
[features/cross-layer.feature](../../features/cross-layer.feature), bound
in `cmd/frit/bdd_identity_and_cross_layer_test.go`.
Every new test was confirmed red before the fix and green after,
including S94 itself. Docs: `S86`/`S94` cited in
[docs/claiming.md](../../docs/claiming.md)'s Self-resume section; the
matrix row in
[docs/research/lease-protocol.md](../../docs/research/lease-protocol.md)
updated off `pending`.

Full `go test ./...`, `golangci-lint run` and `mdsmith check .` pass.
This was the plan's only phase; nothing is left open for it.
