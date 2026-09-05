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

**Assumes.** Plan 2609032048 has landed. `presenceFor` answers
`cardsOf`'s `presence` callback with the pane's status; `agentFor`
hands `board` the same status beside the agent. `askOf` gates the ask
on it, and `liveLaneFor` checks `fleet.RepoName(lane.Root, …) == p.Repo`
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
  directory name is not the repository's. Its red is `atlas`'s row,
  not `orrery`'s: today's survey ignores the repository entirely, so
  `orrery`'s row already reads attended regardless of the directory
  name. What the bullet pins is the green: a key built from
  `Lane.Repo`, the lane directory's own name, would leave `orrery`'s
  row dead too, while `fleet.RepoName` keeps it attended — the case
  that function exists for.

Each fails today: `liveByBranch` keys `plan/7` once, and whichever
lane the walk reaches last wins for both repositories. Commit the red.

**GREEN.** Lift the repository resolution out of
[`liveLaneFor`](../../cmd/frit/dispatch.go) into one helper the two
joins share — a lane's repository through the host's own git — and key
[`liveByBranch`](../../cmd/frit/main.go)'s map by repository and
branch. `laneFor` already receives the plan, so its signature does not
move; its lookup keys by `p.Repo` and each hold branch. `liveLaneFor`
calls the same helper and keeps its own loop; its tests pass without
edits. Every changed function keeps its dedicated unit test. Resolve a
lane's repository once per host and root, never per lane: two panes
sharing one worktree — two terminals in the same lane — share the
answer, since `fleet.RepoName` runs `git worktree list`, an unbounded
ssh round trip for a remote pane.

**Guard the edges.** A single-repository fleet reads exactly as before.
A lane whose root git cannot be asked falls back to the root's
basename, as `fleet.RepoName` does today, and matches a plan whose
repository is named that way — never a bare-branch fallback when the
repository key misses; that fallback is the bug this phase fixes.
`who` is untouched.

**Gate.** The new tests pass; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean.

Write the handoff to `phase-1.result.md`. Name the helper and where it
lives, and the map's key shape. Name the fixture Phase 2 should reuse
to stand two hosts up with one unread.
