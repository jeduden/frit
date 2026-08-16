---
id: 2608161810
title: The dispatch ladder — from a board to a seeded prompt
status: "🔲"
summary: >-
  Climb from a read-only board to dispatch without building a prompt
  UI, because the plan already contains the prompt. Adds open, nudge
  and start, a ladder that composes a typed slash command, hands the
  pane to herdr, and never reads a reply back. Every rung that sends
  is dry-run until --go; open sends nothing and needs no gate.
model: opus
depends-on: [2608161808, 2608161809]
---
# The dispatch ladder

## Goal

Grow the board upward into a dispatcher without duplicating herdr. The
plan already declares, per phase, the model tier and the gate. So
dispatch composes a short slash command from the plan, hands the pane
to the agent, and steps back. It never writes a prompt and never reads
a reply.

## Context

Read-only is rung zero, not the ceiling. The obvious next step — a box
you type a prompt into — is a surface herdr already owns, and
duplicating it would make frit a worse herdr.

The way out is that the plan already contains the prompt. Plans
declare, per phase, the model tier and the gate that proves the work.
Combined with the `plan-phase` skill, which loads only the front
matter and one `## Phase N` section, the whole prompt is about twenty
characters: `/plan-phase 2607191320 8`. The tier comes out of the
Execution table, so dispatch is typed — phase 8 asks for opus and gets
opus. No general-purpose orchestrator can do this, because none of
them reads the plan.

The ladder is built from herdr's protocol-17 surface. `worktree.create`
and `agent.start` stand a lane up. `agent.prompt` sends the composed
command. Herdr's own pane focus and `--remote` attach do the handoff.
`agent.wait` bounds `agent.start` until the agent has come up, which is
a wait on a status, not on a reply — so it is not the forbidden
`agent.read`. The `agent.start.args` field is the important one. The
tier from the table maps straight onto `--model haiku|sonnet|opus`.

Three rules keep this from becoming herdr, and every phase is built to
hold them:

- **The tool composes the prompt; the user never writes one.** Sent
  text is always a slash command naming a plan and a phase. When free
  prose is genuinely needed, that is the signal to drop to rung 1 and
  hand the human to the pane.
- **One-way door.** It sends, then hands over. `agent.read` exists in
  the API and frit must never call it. Reading the conversation back
  is exactly how a board grows into a chat client.
- **Dry-run by default.** Every rung above 1 prints the composition it
  would run; `--go` runs it. Read-only stays the default and the
  escalation stays auditable.

## Phase 1: open, rung 1

Ship `frit open <selector>`: focus the pane a lane is already running
in, or attach to it over SSH. It sends no text and starts no agent. It
is the read-only handoff, and the last rung the first version was
scoped to ship.

`open` reads presence from the herdr join and resolves the plan
through the discovery selector, so it is a thin composition of two
plans already landed.

## Phase 2: nudge, rung 2

Ship `frit nudge <selector>`: compose the typed slash command from the
plan's next open phase and `agent.prompt` it into an existing idle
lane. The tier is read from the Execution table, never chosen here.

Dry-run is the default. `frit nudge shader-unit` prints the exact
composition and the target pane; only `--go` sends it. A lane with no
idle agent is refused rather than interrupted.

## Phase 3: start, rung 3

Ship `frit start <selector>`: the full escalation. Claim the lane
through plan-lane, create the worktree through `worktree.create`, start
the agent at the table's tier through `agent.start`, prompt it, and
focus the pane. Every mutation is delegated — frit claims nothing and
spawns nothing it does not hand straight to herdr.

Not every dispatch is bare, and the escape hatch is the one git
already established for commit messages: a prefilled template, not an
empty box.

```sh
frit start shader-unit --phase 3 --note "skip the VRT case, it's flaky"
frit start shader-unit --phase 3 --edit   # $EDITOR, prefilled
```

A `--note` is a rider on a subject the tool still owns. `--edit` hands
the composed prompt to `$EDITOR` to amend before sending. Neither is a
UI: one is a flag, the other is your editor.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase           | Design | Implement | Gate that catches a wrong answer                               |
| --------------- | ------ | --------- | -------------------------------------------------------------- |
| 1 open, rung 1  | sonnet | sonnet    | test that open sends no text and starts no agent               |
| 2 nudge, rung 2 | opus   | sonnet    | test that dry-run is default and the tier comes from the table |
| 3 start, rung 3 | opus   | opus      | test that every mutation is delegated and agent.read is unused |

## Non-goals

- No prompt box. Sent text is always a composed slash command. Free
  prose drops to rung 1 and hands over.
- No reading a reply. `agent.read` is never called, at any rung.
- No reimplemented claims. Every mutation delegates to plan-lane or
  herdr; frit owns no second registry.
- No multi-host dispatch beyond attach. `open` may attach over SSH;
  starting a lane on another host waits for the fan-out plan.

## Tasks

1. Ship `frit open` — rung 1, focus or attach, no text sent
2. Compose the typed slash command from a plan's next open phase
3. Ship `frit nudge` — rung 2, dry-run by default, `--go` to send
4. Ship `frit start` — rung 3, claim, worktree, agent, prompt, focus
5. Add `--note` and `--edit` as prefilled-template escape hatches
6. Give every dispatch verb a `--json` dry-run form

## Acceptance Criteria

- [ ] `frit open` focuses or attaches and sends no text
- [ ] The composed prompt is always a slash command naming plan and phase
- [ ] The model tier is read from the Execution table, never chosen
- [ ] `frit nudge` and `frit start` are dry-run unless `--go` is given
- [ ] `frit start` delegates the claim to plan-lane and the spawn to herdr
- [ ] `agent.read` is never called by any code path
- [ ] `--note` rides the composed prompt; `--edit` opens `$EDITOR` prefilled
- [ ] A dry-run `--json` form prints the composition without running it
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
