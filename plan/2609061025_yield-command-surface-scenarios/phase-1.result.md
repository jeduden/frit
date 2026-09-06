---
n: 1
title: Yield on a plan nobody holds is a clean no-op
status: "✅"
result: true
summary: C2 proves yield's clean no-op on a plan nobody holds
---
## Handoff

`C2` in [command-scenarios.md](../../docs/research/command-scenarios.md)
and its tagged scenario in
[features/commands.feature](../../features/commands.feature) now drive
the real `frit yield` on a plan nobody holds: it reuses C1's
`a plan nobody has ever held` fixture, adds `it is yielded` and
`yield parks nothing and refuses nothing` steps in
[cmd/frit/bdd_commands_test.go](../../cmd/frit/bdd_commands_test.go),
and reads the outcome off the command's own stdout — no `refused:`
line, no `parked:` line — never an internal call. `yield.go` was not
touched.

Gate green: the `C2` scenario passes, `go test ./internal/scenario`
(bijection), `go test ./...`, and
`go tool -modfile=tools/go.mod golangci-lint run` all clean; `mdsmith
check .` clean.

**Inherits.** The command world's `world.planID`, `commandState`
section, and the `runCLI`/`section[T]` plumbing now carry a `yield`
step alongside `release`'s. The next phase — yielding one's own live
lane, refused toward `release` — reuses this same fixture shape but
needs a *held* lane instead of C1/C2's unheld one; it is not yet
covered by any `S<n>`, so it earns its own `C<n>` row rather than
reaching for `S93` (which covers the cross-host foreign-hold case,
not this single-host one). No later phase exists yet as a file — plan
Tasks names it but `phase-2.md` has not been written.
