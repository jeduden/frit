---
id: 2608212223
title: A thin skill fronts every frit verb an agent uses
status: "🔲"
summary: >-
  The shipped suite fronts finding, running, authoring and syncing
  plans. The teardown, cleanup and health verbs — yield, release,
  reap, orphans, stale, doctor — have no skill, so an agent reaches
  for git worktree remove and branch -D instead of a frit verb. Add
  the thin skills that front them, and record the rule that every
  agent-facing verb ships with one, so no verb ever leaves an agent to
  act ad hoc.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: a teardown skill fronts the cleanup verbs
    status: "🔲"
  - n: 2
    title: cover the rest and record the rule
    status: "🔲"
---
# A thin skill fronts every frit verb an agent uses

## Goal

Every frit verb an agent uses is fronted by a thin skill, so an agent
reaches for a frit verb, never for ad-hoc git. The rule that a new
verb ships with its skill is recorded, so the coverage does not drift.

## Context

Four skills ship today. `plan-pick` fronts ready, pick, claim, start,
board and who; `plan-phase` fronts next and nudge; `plan-sync` fronts
the status reconcile; `plan-new` authors a plan. The mutating and
health verbs are the gap: yield, release, reap, orphans, stale and
doctor have no skill. An agent with no skill for teardown hand-runs
`git worktree remove` and `git branch -D` — the surgery frit exists to
replace. This session did exactly that.

Skills need no new machinery. The canonical text lives in
[internal/skills/assets](../internal/skills), one directory per skill.
`//go:embed all:assets` bundles any new directory, and `frit skills`
mirrors it into `.claude/skills`. The
[skill kind](../.mdsmith.yml) lints both copies: readability on, an
80-line cap, names unique across the assets. A skill is a procedure
read under time pressure, so thin is the point, not a limit to fight.

Phase 1 fronts the teardown and cleanup verbs, the acute gap. Phase 2
covers doctor, records the standing rule in
[CLAUDE.md](../CLAUDE.md), and folds `reap` into the teardown skill
once plan 2608212218 lands it.

## Tasks

1. Ship a teardown skill fronting yield, release, orphans and stale.
2. (determined after Phase 1)

## Phase 1: a teardown skill fronts the cleanup verbs

A `plan-tidy` skill fronts the shipped teardown and cleanup verbs. It
tells an agent to read `frit orphans` and `frit stale`, then act with
`frit yield` or `frit release` — and never with hand-run git. It is
thin: the method is which verb answers which mess, and the one rule
that git surgery is the wrong tool.

RED, the skill gate rather than a Go test:

- `mdsmith check .` fails on the new skill until it is readable and
  under the 80-line cap, so the asset is written to pass the skill
  kind.
- `internal/skills` enumerates the bundled skills; its test grows to
  expect `plan-tidy` among them, and fails until the directory exists.

GREEN: a new `plan-tidy/SKILL.md` under
[internal/skills/assets](../internal/skills). It carries the `name`
and `description` frontmatter, the verb-to-mess method, and the
no-ad-hoc-git rule. `frit skills` regenerates the dogfooded copy, so
the two never drift.

Gate: `mdsmith check .` is clean on both copies; `go test ./...`
passes with `plan-tidy` bundled; `frit skills` is idempotent on a
second run.

## Phase 2: cover the rest and record the rule

The remaining agent-facing verb without a skill is doctor. It is
fronted, either by its own thin skill or folded into `plan-new` and
`plan-sync` where plan health already lives — decided after Phase 1
fixes the skill shape. `reap` is added to `plan-tidy` once plan
2608212218 ships the verb. The standing rule is written into
[CLAUDE.md](../CLAUDE.md): a new agent-facing verb ships with the thin
skill that fronts it, in the same change.

RED and the exact placement are settled after Phase 1. The gate is
that every agent-facing verb maps to a skill, the mapping is checked,
and CLAUDE.md records the rule so a future verb cannot ship skill-less.

## Execution

Tier is per phase, set by the most demanding ingredient. The skills
are short procedures over shipped verbs, so both phases are cheap once
the shape is fixed.

| Phase               | Design | Implement | Gate that catches a wrong answer                                 |
| ------------------- | ------ | --------- | ---------------------------------------------------------------- |
| 1 teardown skill    | opus   | sonnet    | plan-tidy passes the skill kind; bundled; frit skills idempotent |
| 2 rest and the rule | sonnet | sonnet    | every agent-facing verb maps to a skill; CLAUDE.md records it    |

## Acceptance Criteria

- [ ] A `plan-tidy` skill fronts yield, release, orphans and stale
- [ ] It says: read with orphans/stale, act with frit, never raw git
- [ ] The skill passes the mdsmith skill kind, both copies
- [ ] `frit skills` regenerates the dogfooded copy, idempotently
- [ ] doctor is fronted by a skill
- [ ] CLAUDE.md records that a new verb ships with its skill
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
