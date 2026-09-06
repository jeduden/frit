---
id: 2609061156
title: The README is a lean front page; its reference lives in docs/
status: "✅"
summary: >-
  The README was pinned against its 300-line lint cap because it
  carried both the pitch and the full reference. The command reference
  already moved to docs/commands.md, and the dry-run rationale to
  ux-principles.md. What still bloats the front page is the reference
  prose beneath: the First run walkthrough, the Configuration block,
  and the Scripting with JSON section that mostly restates
  ux-principles' own JSON contract. Move each to a focused docs/ page,
  or fold it where the fact already lives, and leave the README a
  short pointer and a Documentation map row. The front page then says
  what frit is, the problem it solves, a minimal quickstart, and where
  every detail lives — nothing a reader must scroll past to get
  started.
model: sonnet
depends-on: []
---
# The README is a lean front page; its reference lives in docs/

## Goal

The README is a front page: what frit is, the problem it solves, a
minimal quickstart, and a map to the docs. Each detailed reference —
getting started, configuration, JSON scripting — lives on a focused
`docs/` page it links, so the front page never crowds a newcomer and
never fights the line cap again.

## Context

**Why now.** The README sat at its 300-line cap (MDS022), so any
addition broke the lint. Grouping the command table under subheadings
pushed it over, which is what surfaced the real problem: the README
carries both the pitch and the reference. The command reference is
already out, in [docs/commands.md](../../docs/commands.md), and the
dry-run rationale is in
[docs/ux-principles.md](../../docs/ux-principles.md). This plan
finishes the split for the sections still on the front page.

**What is left to move.** Three sections are reference, not pitch:

- **First run** — the init-to-first-lane walkthrough. A newcomer's
  guide, not a front-page hook. Its home is a `docs/` getting-started
  page.
- **Configuration** — the `.frit.yml` keys and precedence. Reference
  a person returns to, not reads once. The precedence is also in
  [CLAUDE.md](../../CLAUDE.md) and
  [docs/ux-principles.md](../../docs/ux-principles.md); the page
  should point at those rather than restate them.
- **Scripting with JSON** — mostly restates the JSON contract already
  in
  [docs/ux-principles.md](../../docs/ux-principles.md#the-json-contract).
  It can shrink to a pointer plus the one example.

**Reuse first.** No new doc machinery. `docs/` already holds the
focused pages ([architecture](../../docs/architecture.md),
[ux-principles](../../docs/ux-principles.md),
[claiming](../../docs/claiming.md),
[commands](../../docs/commands.md)), each linked from the README's
Documentation map. This plan adds pages of the same shape and one map
row apiece. The mdsmith caps — the 300-line file cap and the skill
token budget — are the gate that a moved section actually left the
front page.

**Out of scope.** No change to what any section says, only where it
lives. The pitch — the opening, the problem, the how-frit-works
diagram — stays on the front page. No change to the command reference
already extracted.

## Tasks

1. Phase 1 (proving slice): move the First run walkthrough to a
   `docs/` getting-started page, leave the README a two-line
   quickstart plus a pointer, and add the page to the Documentation
   map. Proves the extraction shape the later sections copy.
2. Later phases: Configuration to its own page or folded to the
   existing precedence docs; Scripting with JSON shrunk to a pointer
   and one example.

## Execution

| Phase | Title                                                    | Tier   | Gate                                                                                                                                                       |
| ----- | -------------------------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | First run becomes a getting-started page                 | sonnet | the walkthrough lives on a docs page linked from the README and its map; the README stays under the 300-line cap; `mdsmith check .` clean                  |
| 2     | Configuration becomes a focused docs page                | sonnet | the `.frit.yml` reference lives on a docs page linked from the README and its map; the README stays under the 300-line cap; `mdsmith check .` clean        |
| 3     | Scripting with JSON shrinks to a pointer and one example | sonnet | the README's JSON section keeps one example and points at ux-principles.md for the rules; the README stays under the 300-line cap; `mdsmith check .` clean |

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

| #   | Status | Phase                                                                                                                                                                                                        |
| --- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 1   | ✅     | [First run becomes a getting-started page](phase-1.md)                                                                                                                                                       |
|     | ↳      | First run moved to docs/getting-started.md; the README keeps a two-line quickstart and a pointer, plus a Documentation map row.                                                                              |
| 2   | ✅     | [Configuration becomes a focused docs page](phase-2.md)                                                                                                                                                      |
|     | ↳      | Configuration moved to docs/configuration.md, pointing at CLAUDE.md and ux-principles.md for the precedence order instead of restating it; the README keeps a one-line pointer plus a Documentation map row. |
| 3   | ✅     | [Scripting with JSON shrinks to a pointer and one example](phase-3.md)                                                                                                                                       |
|     | ↳      | Scripting with JSON keeps its one-line claim and one example; the duplicated "Three rules" paragraph is gone, replaced by a pointer to ux-principles.md's JSON contract.                                     |
<?/catalog?>

## Acceptance Criteria

- [x] The First run walkthrough lives on a focused `docs/` page, linked
      from the README body and its Documentation map
- [x] The README keeps a minimal quickstart and points at that page
      for the full walkthrough
- [x] The README stays within the 300-line cap (`mdsmith check`)
- [x] Configuration and Scripting with JSON are moved or folded, each
      leaving a pointer, in later phases
- [x] `mdsmith check .` is clean and every moved link resolves
- [x] All tests pass: `go test ./...`
