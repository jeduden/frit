---
name: plan-drive
description: >-
  Drive lanes from outside them: see what is outstanding and who is
  live, then raise, nudge or escalate one lane at the lowest rung that
  moves it. Trigger on "what's running", "who is on plan X", "open plan
  X", "nudge the lane", "the lane is idle", "escalate plan X".
---
# plan-drive

Survey the fleet, then act on one lane at the lowest rung that moves
it. These verbs drive lanes; they never work a plan themselves.

## Survey

- `{{frit}} board` — outstanding plans: status, who holds each, the
  agent on it. `--wip` limits it to work in progress.
- `{{frit}} who` — which lanes have a live agent, each marked idle,
  working or unknown.

## Escalation ladder

Climb only as far as the lane needs. Each rung is dry-run and sends
nothing until `--go`.

1. **`{{frit}} open <id>`** — raise the pane the lane runs in; sends no
   text. "no live lane" means there is nothing to raise, so climb a
   rung.
2. **`{{frit}} nudge <id>`** — prompt the plan's next open phase into
   its idle lane. It refuses a lane that is working, not idle, and one
   with no live agent — neither is rung two.
3. **`{{frit}} start <id>`** — compose the full escalation and, with
   `--go`, stand the lane up and run it. `--phase` picks the phase,
   `--note` folds in a rider, `--edit` opens the prompt first.

## Notes

- `open` and `nudge` need a lane that already exists; `start` is the
  rung that creates one — a plan with no live lane starts, not opens.
- Claiming and beginning an unheld plan is `plan-pick`, not a rung
  here.
