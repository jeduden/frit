---
n: 2
title: Yielding one's own live lane is refused toward release
status: "✅"
result: true
summary: C6 proves yield's live-lane refusal, pointed at release
---
## Handoff

`C6` in [command-scenarios.md](../../docs/research/command-scenarios.md)
and its tagged scenario in
[features/commands.feature](../../features/commands.feature) now drive
the real `frit yield` on a plan this very lane just claimed: a fresh
`frit claim` leaves the lease branch at origin's own tip — no
divergence, the live-holder shape `claim.Yield` reads as
`StillHeldError` rather than fenced. `cmd/frit/bdd_commands_test.go`
adds `a plan freshly claimed by this lane` (drives the real `frit
claim` CLI against a `claimableRepo` fixture, the same fake
`TestClaimStandsUpItsWorktree` uses for its worktree-create call) and
`yield refuses it, naming release as the way out` (reads the
refusal's `refused` and `release` wording off the command's own
stdout, and confirms nothing was parked). Phase 1's `it is yielded`
step is reused unchanged. Neither `yield.go` nor `lease.go` was
touched.

`main` had moved since phase 1 landed — plan 2609061024 added `C4`
and `C5`. Merging it in hit a mangled `command-scenarios.md` from the
mdsmith merge driver (garbled conflict markers in the table) and a
duplicated pair of C3's step functions at the tail of
`bdd_commands_test.go`; both resolved by hand before this phase's own
`C6` row was allocated on the merged tree.

Gate green: the `C6` scenario passes, `go test ./internal/scenario`
(bijection), `go test ./...`, and
`go tool -modfile=tools/go.mod golangci-lint run` all clean; `mdsmith
check .` clean.

**This closes the plan.** All Acceptance Criteria are met: the C3
no-op and this phase's C6 refusal both drive the real command, neither
duplicates `S93`'s cross-host case, and every gate is green. The
Goal's third clause — "the shape a refusal carries" — is proven by
`C6`'s own Then step (the refused/release/no-parked wording), not a
separate `--json` phase; no further phase is needed.
