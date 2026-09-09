---
name: plan-start
description: >-
  Open the lane for one explicitly named plan, even when another plan
  ranks higher. Trigger on "start plan X", "open a lane for plan X",
  "run plan X now", "begin work on plan X".
---
# plan-start

`go run ./cmd/frit start` is the rung-three escalation `plan-drive` climbs to
resume an existing lane; this skill fronts the same verb for a fresh,
named start. The selector is required — this skill never infers one,
and ranking never overrides it.

## Method

**`go run ./cmd/frit start <selector> --go --json`** claims the named plan,
stands its lane up through herdr, and dispatches its next open phase.
A plain `go run ./cmd/frit start <selector>` composes the escalation without
running it — read the dry run before adding `--go`.

Branch on the JSON:

- `prompt_dispatched: true` — the phase is already running in `pane`.
  Report it and stop; never invoke `/plan-phase` yourself.
- `refused` non-empty — the reason stays on this plan. Do not fall
  back to another ready plan or retry with a different selector.

## Notes

- The named plan starts even when `go run ./cmd/frit pick` would rank another
  plan first — starting it never touches that other plan.
- Choosing unspecified work is `plan-pick`; raising or resuming an
  existing pane is `plan-drive`. This skill is only for a caller who
  already knows which plan to start.
