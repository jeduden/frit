---
summary: >-
  The research behind frit — what already existed on this machine, why a
  dedicated tool rather than an mdsmith extension or an off-the-shelf
  orchestrator, and how a read-only board grows into a dispatcher without
  becoming a second herdr.
---
# Why frit exists

Work on this machine is scattered by construction: many repositories,
many branches per repository, many worktrees per branch, and agents
attached to some of them. Three tools already owned a layer of that
problem and none of them knew about the others.

- **herdr** knows which agent is doing what, but only on one host and
  only by working directory.
- **plan-lane** knows who holds which plan, and moves lanes between
  machines, but only inside the one repository it ships in.
- **mdsmith** knows how markdown files relate to each other, but only
  within one checkout.

The layer nobody occupied is an index keyed on *repo × branch × host*.
That is what frit is, and the survey in [options.md](options.md) found
no shipping tool that fills it.

These notes are dated 2026-08-15 and describe the reasoning at that
point. Where a decision has since been overturned, the note says so
inline rather than being quietly rewritten — the point of keeping them
is to record how the conclusion was reached, including the wrong turns.
Current behaviour lives in [CLAUDE.md](../../../CLAUDE.md) and
[PLAN.md](../../../PLAN.md).

## Documents in this folder

- [README.md](README.md) — this overview
- [design.md](design.md) — the measured inventory of the machine, the
  join that makes the index possible, reading plans out of git without a
  checkout, multi-host topology, the dispatch ladder from a read-only
  board to a seeded prompt, and the discovery verbs that make dispatch
  usable
- [options.md](options.md) — build, extend or adopt: why mdsmith needed
  no generalisation and still was not the right home, a survey of 14
  agent orchestrators against six requirements, and the three-layer
  model that explains why none of them competes
