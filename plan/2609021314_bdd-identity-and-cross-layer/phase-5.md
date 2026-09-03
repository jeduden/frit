---
n: 5
title: "A genuine two-process race and two repos sharing one id: S72, S74 run real"
status: "🔳"
result: false
---
Convert the plan's last two rows into passing scenarios: S72 and S74.
Phase 4's handoff named exactly what both need and neither prior phase
built — `cloneRepoIntoRoot` in `bdd_process_death_test.go`, already
reusable, and a genuine two-goroutine contention no row has needed
until now. This phase closes the plan.

**Assumes.** Everything phase-1.md through phase-4.md assumed.

- This file's own thirty-four-plus-six steps and
  `identityAndCrossLayerState`.
- `planIsUnclaimed` gives the single-repo fixture both rows start
  from.
- `startHerdr` gives the reachable herdr fake whichever verb wins
  still needs behind its own worktree stand-up.

Beyond the step file:

- `cloneRepoIntoRoot`
  ([bdd_process_death_test.go](../../cmd/frit/bdd_process_death_test.go))
  clones a repo's origin into a second, independent `--root` — the
  fixture that lets two verbs contend on the same origin without
  contending on the same local clone.
- `claim.Acquire`'s push is one server-side CAS,
  `--force-with-lease` against a ref that must not yet exist
  ([lease.go](../../internal/claim/lease.go)); `lostRaceRefusal` and
  `claim.ErrLostRace` ([claim.go](../../cmd/frit/claim.go)) are what a
  losing push reports back through both `claim` and `start --go`
  (`cmd/frit/start.go:145`).
- `standUpClaimWorktree` ([claim.go](../../cmd/frit/claim.go)) means
  `claim` itself — not only `start --go` — calls herdr once it mints,
  so whichever verb wins S72's race still needs a reachable fake
  behind it, or a failed stand-up's own unwind would read as "refused"
  too and the row could not tell the two apart.
- `defaultLanePath` and `laneLabel`
  ([claim.go](../../cmd/frit/claim.go),
  [start.go](../../cmd/frit/start.go)) both fold the repository's own
  name into the worktree path and the pane label herdr is asked to
  open — S74's whole point, and already true of the code with no
  change needed.
- `discovery.Resolve` ([resolve.go](../../internal/discovery/resolve.go))
  reports a bare numeric id shared by two repositories in one root as
  `*Ambiguous` — correct, and a different row from S74's own. S74's
  own two claims select by a slug naming their own repository, never
  the bare id, so this ambiguity is never the one it hits.

**Value.** The matrix's last two cross-layer rows gain their
executable promise. A claim and a start escalation racing the same
plan on one host is no longer a story about timing: the server-side
CAS decides exactly one winner, and the loser reads its own refusal
back off the marker the winner actually wrote, naming this host as the
only lane the race could have produced. Two repositories that happen
to share a plan's numeric id never collide as lanes: each worktree and
each pane herdr is asked to open carries its own repository's name,
proving the fleet's real key is host, repository and id together, not
the id alone.

**RED.** Drop `@pending` from S72 and S74 in
[cross-layer.feature](../../features/cross-layer.feature). Write each
one's Given/When/Then. Run `go test ./cmd/frit -run
'TestFeatures/S(72|74):'`: strict mode reports the new steps undefined
and both subtests fail. That is the red — commit it.

The scenarios, in the matrix's own terms:

- S72, claim and start race on one host. Given plan 7 is unclaimed,
  when `claim` and `start --go` race to mint it — two goroutines, each
  from its own clone of the same origin, released together and waited
  on — then exactly one wins, and the loser's own refusal names this
  host as the plan's holder: the only lane a race confined to one host
  could have produced. Neither goroutine ever calls `t.Chdir`; each
  passes `--root` instead, exactly as every other step in this file
  already does, since two goroutines sharing one process-wide
  directory would race each other for a reason that has nothing to do
  with the row.
- S74, same plan id in two repos. Given plan 7 is unclaimed in two
  repositories under one root, each named in its own plan's title so a
  selector can pick one without the bare id's fleet-wide ambiguity,
  when this machine claims plan 7 in each by that name, then both are
  claimed — no collision — and each one's worktree path and each
  herdr `worktree create`'s own `--label` names its own repository:
  the fleet's real key is host, repository and id together, and
  `defaultLanePath`/`laneLabel` already carry the repository with no
  change to either.

**GREEN.** Extend `cmd/frit/bdd_identity_and_cross_layer_test.go`.

- Add a new registrar, `registerRaceAndMultiRepoIdentityAndCrossLayer`.
- Add five new steps below, each shipping its own unit test per
  CLAUDE.md.

- `claimAndStartBothRaceToMintPlan` (When, S72). Installs the
  reachable herdr fake. Clones a second root off the Given step's own
  repository. Runs `claim` and `start --go` from two goroutines,
  released together by a closed channel and joined by a
  `sync.WaitGroup`. Each writes its own result to a new `raceResult`
  pair, since this row needs both results read back at once, not the
  section's single `out`/`errOut`/`code`.
- `oneWinsAndTheLosersRefusalNamesTheWinningLane` (Then, S72).
- `planIsUnclaimedInTwoRepos` (Given, S74). A sibling of
  `planIsUnclaimed`. Builds two repositories under one root. Each
  plan is titled with its own repository's name.
- `thisMachineClaimsPlanInTwoRepos` (When, S74). Claims by each
  repository's own name, never the bare id.
- `bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo`
  (Then, S74). Reads back both claims' own worktree lines, and the
  herdr recorder's own labels.

`identityAndCrossLayerState` gains `repoA`/`repoB` (S74's own pair of
repositories) and `raceA`/`raceB` (both rows' own pair of captured
results, typed `raceResult`).

**Guard the edges.** Reuse `planIsUnclaimed` and `startHerdr` as-is,
with no change to either — strict mode reports a redefinition as
ambiguous, and `startHerdr`'s own recorder is what S74's pane-label
check reads.

S72's two goroutines must never call `t.Chdir`: both would race the
same process-wide current directory, the one piece of process state
`--root` was already built to avoid touching. Get this wrong and the
row proves nothing about a real race — it proves a bug in the
fixture.

S74's two claims must select by each repository's own name, never the
bare plan id. `discovery.Resolve` reports a numeric id two
repositories in one root both answer to as `*Ambiguous` — a real and
correct refusal, but a different row from this one. Selecting by the
plan's own title-derived slug sidesteps it without weakening anything.

**Gate.** `go test ./cmd/frit -run
'TestFeatures/S(45|46|47|48|49|60|61|62|63|64|65|66|72|73|74|76|77|86):'`
passes with every one of the eighteen reported PASS and none SKIP. `go
test ./internal/scenario` stays green. `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-5.result.md`. This is the plan's last
phase: once S72 and S74 are green, tick the plan's own Acceptance
Criteria, flip its `status:` to ✅, and run `mdsmith fix PLAN.md`.
