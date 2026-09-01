---
n: 2
title: The skills treat a dispatched phase as already running
status: "✅"
result: true
summary: >-
  plan-pick and plan-phase now say what to do when a phase is already
  running: report the pane pick --go dispatched to and never re-run
  /plan-phase, and never start a second runner on a plan a phase load
  already shows held live.
---
## Handoff

Both canonical skills, under `internal/skills/assets`, carry the guard
this phase closes — the one vector no frit verb can intercept, because
a re-typed `/plan-phase` never passes through frit at all.

`plan-pick/SKILL.md` gets a new paragraph right after `pick --go`'s own
description: `prompt_dispatched: true` means the phase is already
running in the reported `pane`; report it and stop, never invoke
`/plan-phase` in the picking session — that is the exact re-run the
issue leads with. `plan-phase/SKILL.md`'s own "Honor the answers" step
gets the mirror: a plan `frit phase <id>` already shows held live
(`held: true`, `dead: false`) is already running somewhere, so report
that and never start a second runner beside it. Neither skill needed a
new frit field — `prompt_dispatched` from phase 1 and the `held`/`dead`
pair `PlanCard` already carries were both already on the wire.

Both edits stayed inside every cap in `.mdsmith.yml`'s `skill` block:
file length, section length, and the 650-token heuristic budget, with
the readability rule (MDS023) catching one over-long sentence in the
first draft — split into three shorter ones, not padded out.

`go run ./cmd/frit skills --via "go run ./cmd/frit" --force` regenerated
the two touched dogfood copies under `.claude/skills`; the other four
skills came back byte-identical. `TestDogfoodCopiesMatchCanonical`,
`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run`, and
`mdsmith check .` are all clean.

This closes plan 2609011806 and issue #126: both ways a phase could be
started twice in one lane — a fresh `pick`/`start --go` onto a
live-but-unbound lane, and a caller re-running `/plan-phase` after a
dispatch it already saw — now refuse or are told not to. The deferred
per-phase lock the plan's Context named out of scope is still not
needed: nothing here calls for a persistent lock the skill must
acquire and release.
