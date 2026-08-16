---
id: 2608161809
title: Discovery — what can I start, and what blocks it
status: "✅"
summary: >-
  The discovery verbs that make dispatch usable: ready, pick, next,
  show and find, over the dependency DAG, the holds and the
  status ledger. Plans are named by selector, not by a ten-digit id.
  Read-only throughout.
model: opus
depends-on: [2608142306]
---
# Discovery

## Goal

Answer the questions a person actually has before dispatching
anything. What can I start now? What next? Where is that plan about X?
Which phase is next? What blocks this? Each needs data only a
cross-repo, cross-branch index holds, which is where the index earns
its keep.

## Context

Dispatch assumes a plan id and a phase number, and in practice you
have neither. Discovery is therefore not a convenience layer bolted on
top — it is the layer that makes the dispatch verbs usable at all, and
it stays entirely read-only.

The data is already indexed. Plan front matter carries `depends-on`,
and on the reference machine 105 of one repository's 152 plans declare
dependencies, so there is a real DAG to walk rather than a flat list.
Status is the four-value vocabulary already in use: 🔲 not started,
🔳 in progress, ✅ done, ⛔ superseded. Holds are read from refs, as
the orphan report already does.

## Phase 1: selectors, not ids

Every command taking a plan accepts three forms, resolved in order,
because typing a ten-digit timestamp is its own friction:

- **An exact id** — `2608062210`. Unambiguous, scriptable, what
  `--json` emits.
- **A slug fragment** — `shader-unit`. Resolved against titles and
  branch names. Ambiguity prints the candidates and exits non-zero
  rather than guessing.
- **Nothing at all** — inferred from the current directory. Standing
  in a worktree, the plan is the one that worktree is working. That is
  the cwd join run backwards.

This phase ships the resolver alone, tested against each form and
against the ambiguous case, before any verb consumes it.

## Phase 2: ready, the flagship

Ship `frit ready`: every plan whose dependencies are all ✅, that
nobody holds, on any branch, in any repo. That single question cannot
be answered today without reading 152 files by hand, and it is the
reason the index exists.

The DAG walk is the risk surface. A dependency edge points at a plan
id, which resolves through the same `host:repo:id` key the index is
built on. A plan whose upstreams are not all done is withheld; a plan
already held is withheld; what remains is startable now.

## Phase 3: next, show and pick

Three verbs read the same graph from different angles.

- `frit next` — the first phase of a plan not yet at ✅, defaulting to
  the plan the current worktree is on. The rule the `plan-phase` skill
  already follows.
- `frit show <id>` — the upstream DAG walk for one plan, so "what
  blocks this" has a direct answer. Blockers show by default — the
  unfinished upstreams — because a done dependency blocks nothing;
  `--all` (aliased `--deps`) keeps every edge, done ones included.
- `frit pick -n 5` — ranked startable candidates nobody holds, for
  when `ready` returns more than a person wants to read.

## Phase 4: find

Ship `frit find "raymarch"`: search titles and summaries across every
branch and repo. It reads the same blobs the index already streamed
out of git, so it needs no checkout and no new walk. It is the verb
for when you remember the topic but not the id.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase              | Design | Implement | Gate that catches a wrong answer                           |
| ------------------ | ------ | --------- | ---------------------------------------------------------- |
| 1 selectors        | opus   | sonnet    | resolver tests over id, slug, cwd, and the ambiguous case  |
| 2 ready            | opus   | sonnet    | fixture DAG where one unmet edge must withhold a plan      |
| 3 next, show, pick | sonnet | sonnet    | test that next skips ✅ phases and stops at the first open |
| 4 find             | haiku  | sonnet    | search fixture matching on title and on summary            |

## Non-goals

- No dispatch. Discovery resolves what could be started and reads what
  blocks it. Sending anything to a lane is the next plan.
- No writes. Nothing here claims, creates a worktree, or edits a plan.
- No new git walk. `find` and `ready` read the blobs the index
  already collected, never a fresh checkout.

## Tasks

1. Build the plan selector: id, slug fragment, cwd inference
2. Ship `frit ready` over the dependency DAG, holds and status
3. Ship `frit next`, `frit show` and `frit pick`
4. Ship `frit find` searching titles and summaries across refs
5. Give every new command a `--json` form, pinned by golden tests

## Acceptance Criteria

- [x] A plan resolves from an exact id, a slug fragment, or the cwd
- [x] An ambiguous selector prints candidates and exits non-zero
- [x] `frit ready` lists plans with all dependencies ✅ that nobody holds
- [x] A plan with one unmet dependency is withheld from `ready`
- [x] `frit next` returns the first phase of a plan not at ✅
- [x] `frit show <id>` walks the upstream DAG: blockers, `--all` for every edge
- [x] `frit pick -n N` ranks startable candidates nobody holds
- [x] `frit find` matches titles and summaries across every ref
- [x] Every new command has a `--json` form pinned by a golden test
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
