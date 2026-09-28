# Wayfinder compared with the plan skills

Recorded 2026-09-28. Wayfinder is read at
`github.com/mattpocock/skills` commit `c55ee46` (2026-09-18):
`skills/engineering/wayfinder/SKILL.md` and
`docs/engineering/wayfinder.md`, plus its pipeline neighbours
`grilling`, `research`, `prototype`, `to-spec`, `to-tickets`,
`implement-spec` and `handoff`. Our side is frit's skills bundle and
smalt's `forward`, `trace-forward` and `orchestrate`. This note is dated
and not kept current; the plan beside it is the live record.

## What each is for

Wayfinder DECIDES. It charts an effort too big for one session as a
map: one parent issue on the tracker, with child issues that are
decision tickets. Each ticket is a question. One is resolved per
session until nothing is left to decide. It writes no product code. A
cleared map flows to `to-spec`, then `to-tickets`, then
`implement-spec`.

Ours BUILDS. A plan is an execution contract. Phase 1 is a proving
slice closed by a human sign-off. frit adds an atomic claim and lanes
across hosts. In smalt, discovery happens by playing a crude prototype
on a `fwd/<topic>` branch, not by talking.

The two converged on four ideas without contact. Do not chart what you
cannot see yet. Prototype before you specify. Point at sources rather
than pasting them. Do one unit of work per session.

## Where Wayfinder is ahead

- **Fog is written down.** A map's "Not yet specified" section holds
  questions you can tell are coming but cannot phrase yet. They
  graduate into tickets as answers arrive. Our plans only imply it, by
  declaring Phase 1 alone.
- **Out of scope is explicit.** Its own section, with the ticket
  closed. Our plans have no such section.
- **Decisions are indexed.** "Decisions so far" is one line per closed
  ticket, linking to where the detail lives: an index, not a store.
- **Deciding without building.** Grilling is its default ticket type.
  We have no interview skill at all.
- **Research is a planned, saved unit.** An AFK subagent per research
  ticket, fired in parallel when the map is charted.
- **Human and agent work are typed.** Each ticket is HITL (a human
  answers for themselves) or AFK (the agent alone).
- **Things are named, not numbered.** Narration uses a ticket's title,
  never a bare `#42`.

## Where ours is ahead

- **The claim.** A ref pushed to origin with a lease: exactly one
  winner, fencing, a takeover window, rescue refs. Wayfinder claims by
  self-assignment, which two sessions can race.
- **Reconciliation.** `plan-sync`, `plan-tidy`, `plan-drive` and
  `frit doctor`. Wayfinder has none; tracker state is the truth.
- **Staying current.** Plans transcribe nothing the tree can
  contradict, and links are linted. Wayfinder only asks specs to omit
  file paths.
- **No pasted prototypes.** smalt's trace refuses a pasted prototype
  line mechanically. Wayfinder's `to-spec` allows inlining a
  prototype's decision-rich snippet.
- **No self-licensing.** Wayfinder's own FAQ reports an agent writing
  "this map carries execution" into the map's Notes, then obeying it.
  Our skills are vendored copies that a drift gate keeps identical to
  the bundle.
- **Budget.** smalt's orchestrator dispatches inside the usage window
  and the CI queue.

## Token cost

Sizes are bytes divided by four, a rough heuristic.

| Cost                              | Wayfinder and pipeline                                   | Ours (smalt)                    |
| --------------------------------- | -------------------------------------------------------- | ------------------------------- |
| Skill descriptions, every session | 0 for the user-invoked five; ~235 for helpers            | ~935; all 11 are model-invoked  |
| Skill text for one unit of work   | ~5.2k: wayfinder, grilling, domain-modeling, tracker doc | ~1.2k: plan-phase, plan-handoff |
| Working state for that unit       | map index plus tickets opened on demand                  | one `frit phase` bundle, ~1.75k |

Per unit of work we are about four times leaner, because frit computes
in code and the skill only names the command. Our weak spot is the
fixed cost paid in every session, plan work or not.

## What not to adopt

- The issue tracker as the store. It loses the atomic claim, offline
  lint and the schema.
- Self-assignment as the claim.
- Inlining prototype snippets into a spec.
- A Notes field that can switch a map into execution.
