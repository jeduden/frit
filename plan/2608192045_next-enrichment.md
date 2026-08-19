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

Phase 1 is complete. The phases below were appended at its sign-off.

## Phase 2: the Execution tier and gate

Parse the `## Execution` table into a per-phase tier and gate, and have
`frit next <id>` print them for the phase it points at. This is the
inline half of the linting answer: a `## Phase N` with no Execution row
— no tier, no gate — is surfaced through the report's `Problems`
channel, not rendered as a blank. frit never fails on it; it says so.

## Phase 3: the phase body and a section-derived ledger

Parse each `## Phase N` section body, so `frit next` prints the phase
an executor reads, not just its title. When a plan carries no
front-matter `phases:` ledger — as frit's own plans do not — derive the
phase list from the `## Phase N` headings so `next` still points at the
first one. Section state carries no status, so this reports the phases
without inventing one.

## Phase 4: frit doctor, the fleet-wide health report

Add a read-only `frit doctor` verb that scans every plan and lists the
semantic gaps frit now depends on: a missing `## Goal`, a phase with no
Execution row, a tier that is not a known model. This is the
fleet-wide half of the linting answer — a health report in the shape of
`orphans` and `stale`, not a mutation.

The check itself uses **mdsmith as a library**, not a hand-rolled
linter. mdsmith already validates a plan against `plan/proto.md`. It
also projects the front matter through a CUE schema, in `extract`. frit
runs that validation through the imported package and reports the
findings. Some of it lives in mdsmith's `internal/` today, the
`extract` projection among it. That is the moment to ask mdsmith to
promote the entry point, per the standing rule in
[CLAUDE.md](../CLAUDE.md), not to reimplement a checker here.

doctor validates against a repository's `plan/proto.md`, so a repo that
has none has nothing to check. Shipping that schema as an `init`
default is its own concern, tracked in plan 2608192121; doctor here
assumes it is present, as it is in frit's own repo.

The verb documents its own mechanics, reuse included. `frit doctor
--help` enumerates what it checks — a missing `## Goal`, a phase with no
Execution row, a tier that is not a known model — so a reader learns
what a finding means without opening the source. It also names where
each check comes from: the repository's own `plan/proto.md` schema, run
through mdsmith's validation and CUE projection as a library, not a
second rule set frit invented. The help is the contract for what doctor
promises to catch, and the provenance of every finding.

## Phase 5: the skills lean on the output

Trim the shipped `plan-pick` and `plan-phase` skills so they read
frit's output instead of the plan file, dropping the "read the plan
file" prose the enrichment makes redundant. Regenerate frit's own
`.claude/skills` from the bundle, and keep every skill under the kind's
line cap.

## Tasks

1. Phase 1 — proving slice: `frit show` prints the Goal from the body,
   end to end. Done.
2. Phase 2 — Execution tier and gate on `frit next`; a phase with no
   row surfaced via `Problems`.
3. Phase 3 — phase-section body on `frit next`; a section-derived
   ledger when front matter carries none.
4. Phase 4 — `frit doctor`: a fleet-wide report of plans with a
   semantic gap, checked through mdsmith as a library.
5. Phase 5 — the skills lean on the enriched output; re-dogfood.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase         | Design | Implement | Gate that catches a wrong answer                                       |
| ------------- | ------ | --------- | ---------------------------------------------------------------------- |
| 1 Goal slice  | opus   | sonnet    | test that `frit show` prints a body-only Goal, table + json            |
| 2 tier & gate | sonnet | sonnet    | test that `next` prints them, and a row-less phase is a gap            |
| 3 phase body  | sonnet | sonnet    | test that `next` prints the body, ledger derived from headings         |
| 4 frit doctor | sonnet | sonnet    | a gapped plan is listed, a clean one is not, `--help` lists the checks |
| 5 skills lean | sonnet | sonnet    | shipped skills drop the file-read prose and stay under the cap         |

## Acceptance Criteria

- [x] `frit show <id>` prints the plan's Goal, read from the body, for
      a plan that carries it in a `## Goal` section
- [x] The Goal travels in the `--json` document, every key present
- [x] A plan with no `## Goal` section renders an empty Goal, not a
      crash
- [x] The golden files are re-recorded and the diff read
- [ ] `frit next <id>` prints the target phase's tier, gate and body
- [ ] A phase with no Execution row is surfaced as a `Problems` entry,
      never a blank tier
- [ ] `frit doctor` lists every plan with a semantic gap and omits the
      clean ones
- [ ] `frit doctor --help` enumerates the checks it runs, and names
      their source: `plan/proto.md` validated through mdsmith as a
      library, not a rule set frit reimplements
- [ ] The shipped skills read frit's output instead of the plan file,
      and stay under the skill kind's line cap
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
