---
id: 2609181909
title: frit tells a blocked or finished agent from an unknown one
status: "🔲"
summary: >-
  frit reads only working, idle and unknown from herdr and folds every
  other status into unknown. herdr's own vocabulary also names blocked
  and done, and its explain output reports a visible blocker. A lane
  waiting on an approval prompt therefore reads as unknown, so message
  refuses it and the board cannot say it is waiting. Carry blocked and
  done through as their own statuses, keep unknown from ever reading
  as idle, and refuse to type into a blocked pane.
model: sonnet
depends-on: []
---
# frit tells a blocked or finished agent from an unknown one

## Goal

`frit board`, `who`, `message` and `nudge` tell a lane waiting on an
operator from one frit cannot read. A lane blocked on an approval
prompt reports as blocked, not unknown or deserted.

## Context

**What frit reads today.** Only `herdr agent list` and `herdr agent
prompt`. From each list entry it keeps the agent kind, status, cwd,
session id, pane id and title. `Pane.Presence()` in
[parse.go](../../internal/herdr/parse.go) returns working, idle or
unknown. Any other status a newer herdr reports collapses to unknown,
though the field's own comment says such a value is carried verbatim.

**What herdr reports beyond that.** `herdr agent wait --until` accepts
idle, working, blocked, done and unknown. `herdr agent explain --json`
reports `visible_blocker` beside `visible_idle` and `visible_working`.
Each list entry carries `revision` and `state_change_seq`, which frit
drops. Only idle and working have been seen in live `agent list`
output. Whether the list emits blocked and done, or only `explain`
shows a blocker, is unproven, and the design depends on it.

**Why it matters.** Issue #198's follow-up comment showed a responder
stopping at its own permission gate. That lane is waiting on an
operator. Read as unknown, it is refused by `message`, since
[`messageSend`](../../cmd/frit/dispatch.go) refuses unknown, and its
ask is withheld from the board, since `askable` in
[discovery.go](../../internal/report/discovery.go) admits only working
and idle. It can then read as deserted, and a `(dead)` reading invites
a yield. Plan
[2609181901](../2609181901_ask-message-with-a-reply-path/plan.md)
wants to say an ask is pending on a blocked lane; it needs this read.

**The rule to keep.** A false idle invites a dispatch onto an occupied
lane, so unknown must never read as idle. Widening the vocabulary must
not weaken that. There is also a new hazard. Text typed into a pane
that waits on a yes/no prompt can answer the prompt. So `message` and
`nudge` keep refusing a blocked pane, now with the reason named. This
is the one place a new status changes a refusal's wording, not just a
label.

**Why not a frit monitor.** The presence note,
[cross-host-presence](../../docs/research/cross-host-presence.md),
rejects one: herdr already monitors each agent, and frit consumes it.
This plan reads more of herdr's own answer and adds no watcher.

**What is reused.** `Pane.Presence()` stays the one place a status is
classified, and every consumer already goes through it: the board's
agent status, `who`, the dispatch reports, `nudgeSend` and
`messageSend`. Widening it once carries the change everywhere. The JSON
contract in [ux-principles](../../docs/ux-principles.md) makes the
status a field a consumer branches on, so a new value is a documented
widening, not a silent one.

**Scope.** Reading statuses, and how each consumer treats them. The
change counters are a follow-up: with `observe`'s per-host memory they
would show how long a pane has been quiet. They are named here, not
built.

**BDD coverage.** Command-level, one host: a `@C<n>` row per
[docs/development.md](../../docs/development.md)'s executable scenario
matrix. No `@S<n>` applies, since no lease protocol changes. Phase 1
takes the next free `C<n>` against origin's matrix at execution time.
Plan 2609181901 reserves C13, so expect C14.

## Tasks

1. Phase 1 settles what herdr emits for a blocked and a finished
   agent, records the answer, and carries both through `Presence()`.
   It makes `message` and `nudge` refuse a blocked pane by name, and
   shows the status on the board and `who`.
2. Later phases are specced from Phase 1's handoff. Expected: carry
   `state_change_seq` and report how long a lane has been quiet; let
   the board say a pending ask sits on a blocked lane.

## Execution

| Phase | Title                               | Tier   | Gate                                                                                                                          |
| ----- | ----------------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------- |
| 1     | Blocked and done read as themselves | sonnet | probe of a real blocked pane recorded; C14 runs the built frit: board shows blocked, message refuses by name; `go test ./...` |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row-expr: |
  [if result {
    "|  | ↳ | \(summary) |"
  }, if !result {
    "| \(n) | \(status) | [\(title)](phase-\(n).md) |"
  }][0]
footer: |

?>

| #   | Status | Phase                                             |
| --- | ------ | ------------------------------------------------- |
| 1   | 🔲     | [Blocked and done read as themselves](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [ ] What herdr emits for a blocked and a finished agent is probed on
      a real pane and recorded, not assumed.
- [ ] A blocked lane reports blocked in `board --json` and `who`, and a
      finished one reports done. Neither reads as unknown.
- [ ] An unrecognised status still reads as unknown, and unknown never
      reads as idle.
- [ ] `message` and `nudge` refuse a blocked pane and say it waits on
      an approval prompt, so no text lands in the prompt.
- [ ] The widened status set is documented as a JSON contract change.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
