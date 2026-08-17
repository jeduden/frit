---
id: 2608172211
title: One fleet walk — carry the repo coordinates
status: "🔲"
summary: >-
  Stop claim and start re-walking the fleet. The gather already
  discovers every repository, loads its config, and resolves its base
  ref; carry those coordinates out of the gather so a mutating verb
  reads them instead of walking the whole fleet a second time and
  re-running the same git and config reads per invocation.
model: sonnet
depends-on: [2608161810, 2608171835]
---
# One fleet walk

## Goal

Read the fleet once. `frit claim` and `frit start` need a repository's
path to run git in, its remote, and the base a lease is dated against —
all of which the gather already computed and then threw away. Carry
them out of the gather so a claim reads them off what it already
gathered, not off a second walk of the whole fleet.

## Context

`gatherFleet` walks every repository under the root through
`discover.Repos`, and for each one loads its `.frit.yml` and resolves
its default ref. That is where a plan's holds and its dependency edges
come from. But the flattened `discovery.Plan` it hands back carries only
the repository's name, not its path, its remote, or its base.

So a mutating verb has to find them again. `repoBaseFor` in
`cmd/frit/claim.go` calls `discover.Repos` a second
time to turn the name back into a path, then loads the config and
resolves the base once more. On a fleet of many repositories that is a
whole second filesystem walk, plus a repeated config read and a git
subprocess, for an answer the gather already had.

The gather is the right place to know this, because it is already there.
`discover.Repos` yields the path; the config read yields the remote and
any base override; and the default-ref resolution the gather runs for
its merged-ref filter is the same cascade a lease dates against. The fix
is to keep those three per repository and hand them back, not to compute
them twice.

Coordinates are per repository, not per plan, so they belong beside the
plans in the gather's result rather than copied onto every plan a
repository holds. `discovery.Plan` stays what it is — the view the
discovery verbs read — and the lease coordinates travel next to it.

## Phase 1: the gather carries the coordinates

Add a per-repository coordinate to `internal/fleet`'s result. It carries
the repository's path, its remote, and the base a lease is dated
against. The base is the config's when set, otherwise the default-ref
cascade the gather already runs. Populate it in the same per-repository
loop that builds the plans, so no new walk or subprocess is added.

## Phase 2: claim and start read them

Have `frit claim` and `frit start` read the coordinates off the gather's
result rather than resolving them again. Delete the second
`discover.Repos` walk and the repeated config and default-ref reads, so
each mutating verb walks the fleet exactly once.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase              | Design | Implement | Gate that catches a wrong answer                               |
| ------------------ | ------ | --------- | -------------------------------------------------------------- |
| 1 carry the coords | sonnet | sonnet    | test the gather returns each repo's path, remote and base      |
| 2 read the coords  | sonnet | sonnet    | test claim and start mint from the carried path, not a re-walk |

## Non-goals

- No change to what a claim does. This is the same lease, minted from
  the same base and remote; only where those inputs come from moves.
- No new field on `discovery.Plan`. The coordinates are per repository,
  so they travel beside the plans, not copied onto each one.
- No caching across invocations. The saving is one walk per run, not a
  daemon holding the index between runs.

## Tasks

1. Carry each repository's path, remote and base out of the gather
2. Populate the coordinates in the gather's existing per-repo loop
3. Read the coordinates in `frit claim`, dropping its second walk
4. Read the coordinates in `frit start`, dropping its second walk
5. Delete `repoBaseFor` and `repoPathFor` once nothing re-derives

## Acceptance Criteria

- [ ] The gather returns each repository's path, remote and base
- [ ] The base is the config's when set, else the default-ref cascade
- [ ] `frit claim` mints from the carried coordinates, not a second walk
- [ ] `frit start` reads the carried coordinates too
- [ ] `discover.Repos` is walked once per invocation, not twice
- [ ] `discovery.Plan` gains no repository-path field
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
