---
n: 5
title: "A genuine two-process race and two repos sharing one id: S72, S74 run real"
status: "✅"
result: true
summary: >-
  S72 and S74 drop `@pending` and run as real scenarios, closing the
  plan. Two goroutines, each from its own clone of the same origin,
  race `claim` and `start --go` for one unclaimed plan; the
  server-side force-with-lease CAS decides exactly one winner, and the
  loser's own refusal names this host as the plan's holder, the only
  lane a race confined to one host could produce. Two repositories
  sharing one plan id claim independently with no collision, each
  worktree path and each herdr pane label carrying its own
  repository's name — the fleet's real key proven to be host,
  repository and id together, not the id alone. All eighteen rows the
  plan set out to convert now run for real, together.
---
## Handoff

**Both rows landed exactly as phase-5.md predicted, with no finding
against the product.** S72's two goroutines, released together by a
closed channel and joined by a `sync.WaitGroup`, each ran a full
verb — `claim` from the Given step's own root, `start --go` from a
second root `cloneRepoIntoRoot` cloned off the same origin — with no
`t.Chdir` in either, `--root` carrying the working directory instead
exactly as every other step in this file already does. Fifteen
repeated `-race` runs of the scenario turned up no data race and no
flake: the git-level `--force-with-lease` push is the real arbiter,
and `standUpClaimWorktree`'s own herdr call — which `claim` makes on a
plain mint, not only `start --go` — needed the reachable `startHerdr`
fake installed before the race so the winner, whichever verb it was,
could complete cleanly instead of unwinding and reading as "refused"
for an unrelated reason.

S74's two claims needed no race and no new herdr shape.
`discovery.Resolve` reports a bare numeric id two repositories in one
root both answer to as `*Ambiguous` — a real, correct refusal, and
deliberately not the one this row exercises. Selecting each claim by
its own repository-named title sidestepped that refusal without
touching it: both claims minted cleanly, and `defaultLanePath` and
`laneLabel` already fold the repository's own name into the worktree
path and the herdr `worktree create` label with no code change at
all — the row's whole point.

**No change to the lease protocol or to any verb.** Both rows exercise
exactly the mechanism the matrix already describes:
`--force-with-lease` for S72, repository-scoped naming for S74.

All tests are green: `go test ./cmd/frit -run
'TestFeatures/S(45|46|47|48|49|60|61|62|63|64|65|66|72|73|74|76|77|86):'`
reports eighteen PASS, none SKIP; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean; `go test
./internal/scenario` (the bijection gate) stays green.

**This is the plan's last phase.** Every row of the identity section
(S45..S49, S66) and the cross-layer section (S60..S65, S72..S74, S76,
S77, S86) now runs as a real godog scenario, none `@pending`. The
plan's own Acceptance Criteria are met and its `status:` is ✅.
