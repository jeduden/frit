---
id: 2608192045
title: Enrich next and show so a skill leans on frit's output
status: "🔳"
summary: >-
  frit next and show read only front matter, so a plan's Goal, phase
  bodies, tiers and gates stay invisible and a skill must tell the
  agent to open the file. Parse those from the plan body and render
  them, so the output carries what the agent needs and the skill can
  shed its prose.
model: sonnet
depends-on: [2608192020]
---
# Enrich next and show so a skill leans on frit's output

## Goal

Make `frit show` and `frit next` print a plan's Goal, and the target
phase's body, tier and gate, read from the plan body. An agent then
reads frit's output instead of opening the file. The shipped skills
shed their "read the plan file" prose, and the tokens it costs.

## Context

[planmeta.Parse](../internal/planmeta/plan.go) reads a plan's front
matter and discards the body: it already holds the parsed
`markdown.Parse` document, whose body AST it never walks. So
[discovery.Plan](../internal/discovery/discovery.go) and
[report.NextDoc](../internal/report/discovery.go) carry only front
matter — a `PhaseCard` is number, title and status, with no body,
tier, gate or Goal.

That is why `frit next` on frit's own plans reports `(no phase
ledger)`. Those plans track phases as `## Phase N` sections and an
`## Execution` table, not as a front-matter `phases:` list. frit reads
neither. The plan-phase and plan-pick skills work around this by
telling the agent to open the file — the exact prose this plan
removes.

### Reuse

The body is already parsed. `markdown.Parse(source)` (the mdsmith
public parser frit is required to share, not hand-roll a second one)
returns the front matter split off and the body as a goldmark AST.
This plan walks that AST for headings and the Execution table rather
than adding a parser. The carry path also exists: `Phases` already
travels front matter → [index](../internal/index/index.go) →
[fleet](../internal/fleet/gather.go) → `discovery.Plan` → report, so
the new fields ride the same rails.

## Non-goals

- No new mutation. frit reads more of a file it already reads; it
  still never edits a plan.
- Not a per-phase status overhaul. Which phase is "next" still comes
  from the front-matter `phases:` ledger when present; deriving status
  from section state is out of scope.
- No second parser. If the AST cannot answer a shape, the answer is to
  ask mdsmith to expose it, not to hand-roll one.

## Phase 1: the Goal, end to end

The proving slice: carry a plan's `## Goal` from its body all the way
to `frit show`. Extend `planmeta.Parse` to walk the body AST for the
`## Goal` section and return its prose on `Plan`. Carry it through
index, fleet and `discovery.Plan`, and render it under `frit show` and
in the `--json` document. This establishes the whole seam — body
parse, carry, render, golden — that phases 2+ copy. It ends in
sign-off on that seam and the output shape.

## Tasks

1. Phase 1 — proving slice: `frit show` prints the Goal parsed from
   the body, end to end, then sign-off.
2. (determined after Phase 1 sign-off)

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase        | Design | Implement | Gate that catches a wrong answer                            |
| ------------ | ------ | --------- | ----------------------------------------------------------- |
| 1 Goal slice | opus   | sonnet    | test that `frit show` prints a body-only Goal, table + json |

## Acceptance Criteria

- [x] `frit show <id>` prints the plan's Goal, read from the body, for
      a plan that carries it in a `## Goal` section
- [x] The Goal travels in the `--json` document, every key present
- [x] A plan with no `## Goal` section renders an empty Goal, not a
      crash
- [x] The golden files are re-recorded and the diff read
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
