---
id: 2608161808
title: The herdr join — which lane has an agent, live
status: "🔳"
summary: >-
  Join the index to herdr's live pane state so the board knows which
  lane is being worked right now, and by what. Adds the who command,
  makes stale agent-aware, and carries an honest third state for a
  pane whose agent frit cannot read. Read-only throughout.
model: sonnet
depends-on: [2608142306]
---
# The herdr join

## Goal

Turn the index from "which branch has not moved" into "which lane has
no agent and has not moved". Read herdr's live pane list, resolve each
pane back to a plan, and report presence. Read nothing back from any
agent.

## Context

The index is correct but blind to the present. It knows every plan,
every worktree and every claim, but not which of them has a human or
an agent on it right now. That last fact lives in exactly one place:
each host's herdr socket.

This is deliberately the last piece of the read-only board, and the
smallest — one socket call and one `git rev-parse` per pane — because
it is the only part that needs a live server. Everything before it
depends on nothing but git.

The join is a chain that was already reliable, and no step needs new
bookkeeping:

```text
pane (herdr agent list)
  → cwd
  → worktree root (git rev-parse --show-toplevel)
  → branch
  → plan id in plan/<id>-<slug>
  → plan file: title, status, dependencies
```

## Phase 1: read the socket

Call `herdr agent list` and parse its per-pane JSON. Each record
carries `agent`, `agent_status`, `cwd`, `foreground_cwd`, `pane_id`,
`workspace_id`, the agent session id, and the terminal title.

The parser is the risk surface and stays pure, tested against a
fixture for each record shape: an integrated agent, a bare pane with
no agent, and a pane whose `agent_status` reads `unknown`. A missing
or unreachable socket is not fatal — the board still answers from git,
with presence simply absent.

## Phase 2: resolve a pane to a lane

Turn each pane's `cwd` into a plan. Resolve the worktree root with
`git rev-parse --show-toplevel` rather than string-matching against
the worktree list, because `cwd` is the pane's shell directory and
drifts. From the root comes the branch, from the branch the plan id,
and from the id the plan already in the index.

A pane that resolves to no plan is kept, not dropped. It is a real
agent doing real work outside the convention, and the board that
hides it is lying.

## Phase 3: the who command and an honest third state

Ship `frit who`: every lane with a live agent, the agent kind, and
its status. `agent_status` reads `unknown` for a non-integrated agent,
and three workspaces on the reference machine showed exactly that. The
board reports that as its own state, never as idle. A false idle is
worse than an admitted unknown, because it invites dispatch onto an
occupied lane.

Then make `stale` agent-aware. Today it reports a branch that has not
moved. With presence it can separate two very different lanes: one
with an agent still on it, and one abandoned with no agent and no
recent commit. Only the second is stale in the sense that matters.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase             | Design | Implement | Gate that catches a wrong answer                            |
| ----------------- | ------ | --------- | ----------------------------------------------------------- |
| 1 read the socket | sonnet | sonnet    | parser unit tests over integrated, bare and unknown records |
| 2 resolve to lane | sonnet | sonnet    | fixture where cwd has drifted below the worktree root       |
| 3 who and stale   | opus   | sonnet    | test that an unknown agent is never reported as idle        |

## Non-goals

- No reading an agent back. `agent.read` exists in the socket API and
  frit must never call it. That is how a board becomes a chat client.
- No dispatch. Presence is what dispatch will read, but sending
  anything to a lane is a later plan and a deliberate escalation.
- No SSH fan-out. This reads the local socket only; the multi-host
  swap from one socket to many is its own plan.

## Tasks

1. [x] Parse `herdr agent list` per-pane JSON into typed records
2. [x] Resolve a pane to a plan through the cwd join, tolerating drift
3. [x] Ship `frit who` with an honest unknown state
4. [ ] Make `stale` distinguish an abandoned lane from a live one
5. [ ] Give `who` a `--json` form, pinned by a golden test

## Acceptance Criteria

- [x] `herdr agent list` output is parsed into typed records, covering
      an integrated agent, a bare pane, and an unknown agent
- [x] A pane's cwd resolves to a plan via `rev-parse`, not string match
- [x] A pane that resolves to no plan is reported, not dropped
- [x] `frit who` lists every live lane, its agent, and its status
- [x] An `unknown` agent is never reported as idle
- [ ] `stale` separates a lane with a live agent from an abandoned one
- [ ] A missing herdr socket leaves the git-only board working
- [ ] `frit who` has a `--json` form pinned by a golden test
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
