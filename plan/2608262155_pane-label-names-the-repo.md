---
id: 2608262155
title: The herdr pane label names the plan's repo, not the id alone
status: "✅"
summary: >-
  start labels the pane it stands a lane up in "plan <id>", so a fleet
  driving lanes across several repositories shows a column of panes that
  read only "plan 2608262155" with no repository to tell them apart. A
  plan id is a minute timestamp, unique only per repo, so two repos can
  label two different lanes identically. Carry the plan's repository into
  the pane label — the one herdr shows in its pane list — so a person
  reading the panes can see which repo each lane belongs to. The label is
  display only; frit still binds a live agent to its plan through the
  lease's herdr session, never the label text.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: The pane label carries the repo
    status: "✅"
---
# The herdr pane label names the plan's repo, not the id alone

## Goal

A pane `start` stands a lane up in is labelled with its plan's repository
as well as its id. A fleet of lanes across several repositories then
reads its herdr pane list with each pane naming its own repo. Two repos'
lanes no longer collide on the same `plan <id>` label.

## Context

**The site.** [standUpLane](../cmd/frit/start.go) hands the checkout to
herdr with `Label: fmt.Sprintf("plan %d", plan.ID)`. That string is the
[WorktreeSpec.Label](../internal/herdr/dispatch.go) herdr prints in its
pane list. It names the plan by id alone.

**Why the id alone is thin.** A plan id is the minute-precision creation
time, and frit keys a plan as `host:repo:id` across the fleet — the id is
unique only per repository, by design ([proto](proto.md)). A person
driving lanes across several repos then sees a column of panes reading
`plan 2608262155` with nothing to say which repo each belongs to, and two
repos can label two different lanes identically.

**Display only, not a key.** frit does not match a live agent to its plan
through this label, so widening it changes nothing frit reasons over.
`bindSession` in [start.go](../cmd/frit/start.go) writes the started
agent's herdr session into the lease trailer, and `who`/`board` bind
through that session, never the pane label or the agent name. The label
is text for a human reading herdr's pane list; the repo belongs in it for
the same reason the id does.

**Reuse first.** No new lookup is needed.
[discovery.Plan](../internal/discovery/discovery.go) already carries
`Repo`, the repository name, resolved by the same fleet
walk `start` reads its base and remote from. The change folds `plan.Repo`
into the label `standUpLane` already composes.

**Scope.** Only the label string composed in `standUpLane` changes.
`WorktreeSpec.Label` is already a free-form field herdr passes through as
`--label`; the agent name and every ref frit mints are untouched.

## Tasks

1. `standUpLane` composes the herdr pane label from the plan's repository
   and id, so a stood-up lane's pane names its repo.

## Phase 1: The pane label carries the repo

The label `start` hands herdr for the pane it opens must name the plan's
repository alongside its id. This is the whole change: one composed
string, pinned by the fake herdr the start tests already drive.

RED, at the [cmd/frit](../cmd/frit) level, extending the start-path test
that already runs `start --go` against the `startHerdr` fake. Assert the
`worktree create` call the fake records carries a `--label` argument that
contains the plan's repository name, not only `plan <id>`. The existing
fake records every herdr call's args, so the assertion reads the recorded
`--label` value; no new fixture is needed.

GREEN: in [standUpLane](../cmd/frit/start.go), compose the label from
`plan.Repo` and `plan.ID` — `fmt.Sprintf("%s plan %d", plan.Repo, plan.ID)`
— so the `WorktreeSpec.Label` handed to `herdr.WorktreeCreate` names the
repo. Nothing else in the handoff changes.

Gate: the RED assertion passes — a `start --go` run labels the pane with
the repo and the id; `go test ./...`, `go vet ./...`,
`go tool -modfile=tools/go.mod golangci-lint run` and `mdsmith check .`
stay clean.

## Execution

One phase: the pane label carries the repo. A cmd-level test drives the
change through the fake herdr the start path already uses, so the label
string is pinned where it is composed.

| Phase                     | Design | Implement | Gate that catches a wrong answer                              |
| ------------------------- | ------ | --------- | ------------------------------------------------------------- |
| 1 pane label carries repo | opus   | sonnet    | start --go's worktree-create `--label` contains the repo name |

## Acceptance Criteria

- [x] `start --go` labels the pane it opens with the plan's repository
      and id, not the id alone
- [x] The label is composed once, in `standUpLane`, from `plan.Repo` and
      `plan.ID`
- [x] No ref frit mints and no agent-binding path changes; the label
      stays display-only
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
