---
n: 1
title: the survey keys live lanes by repository
status: "✅"
result: false
---
Make the survey's live-lane map answer the question `liveLaneFor`
answers. Is a pane live on this plan's branch, in this plan's own
repository? Then the survey cannot show a lane attended that the verb
would refuse. This is the proving slice. It fixes the join every later
phase reads through, and its fixture is the shape Phase 2 copies.

**Assumes.** Plan 2609032048 has landed: `presenceFor` answers the
`attended` callback with the pane's status, `askOf` gates the ask on
it, and `liveLaneFor` checks `fleet.RepoName(lane.Root, …) == p.Repo`
inline.

**RED.** Two repositories under one root, `atlas` and `orrery`, each
carrying plan 7 held on `plan/7` with a bound session herdr confirms
gone. herdr shows one live working pane, in `orrery`'s lane. Add unit
tests, leading with the observed case:

- `board --json`: `orrery`'s row names the agent, clears `dead` and
  carries `ask`; `atlas`'s row reads `dead: true`, `agent: ""`,
  `ask: ""`.
- `ready --json` (and through it pick and find, which share
  `cardsOf`): the same split on the two cards.
- a linked worktree: `orrery`'s live lane stands in a worktree whose
  directory name is not the repository's, and still resolves to
  `orrery` — the case `fleet.RepoName` exists for and `Lane.Repo`
  gets wrong.

Each fails today: `liveByBranch` keys `plan/7` once, and whichever
lane the walk reaches last wins for both repositories. Commit the red.

**GREEN.** Lift the repository resolution out of
[`liveLaneFor`](../../cmd/frit/dispatch.go) into one helper the two
joins share — a lane's repository through the host's own git — and key
[`liveByBranch`](../../cmd/frit/main.go)'s map by repository and
branch. `laneFor` takes the plan's repository alongside its `Holds`.
`liveLaneFor` calls the same helper and keeps its own loop; its tests
pass without edits. Every changed function keeps its dedicated unit
test.

**Guard the edges.** A single-repository fleet reads exactly as before.
A lane whose root git cannot be asked falls back to the root's
basename, as `fleet.RepoName` does today, and matches a plan whose
repository is named that way. `who` is untouched.

**Gate.** The new tests pass; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Name the helper and where it
lives, and the map's key shape. Name the fixture Phase 2 should reuse
to stand two hosts up with one unread.
