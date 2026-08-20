---
id: 'int & >=2601010000'
title: 'string & != ""'
status: '"🔲" | "🔳" | "✅" | "⛔"'
summary: 'string | *""'
model: '"haiku" | "sonnet" | "opus" | *""'
depends-on: '[...int] | *[]'
---
<?require
filename: "[0-9]*_*.md"
?>

# ?

<!-- Plan conventions:
  - Work test-driven: write a failing test, make it
    pass, commit.
  - Plan files must pass `mdsmith check plan/`.
  - Use Markdown links for real repo paths in prose.
    Bare backticked paths are allowed in commands,
    code blocks, and placeholders.

  Plan ids:
  - The id is the minute-precision UTC creation
    time: `date -u +%y%m%d%H%M` (2608142306 is
    2026-08-14 23:06 UTC). Use it as both the
    frontmatter `id:` and the filename prefix.
  - Taken already? Add one minute and check again.
  - frit keys a plan as host:repo:id across the whole
    fleet, so the id only has to be unique per repo.

  Status values:
  - 🔲 not started
  - 🔳 in progress
  - ✅ completed
  - ⛔ superseded (replaced by another plan)

  Phases and the Execution table:
  - Split work into `## Phase N: <title>` sections.
    One phase is one sitting: red, green, commit.
  - An `## Execution` table carries one row per
    phase, naming the model tier and the gate that
    catches a wrong answer. frit reads that table to
    dispatch a phase at the right tier, so the row is
    load-bearing, not documentation.
  - Both sections are optional for a plan small
    enough to land in one go.

-->

## ...

<?allow-empty-section?>

## Goal

One-sentence summary of what this task achieves and why
it matters.

## ...

<?allow-empty-section?>

## Tasks

1. First concrete step
2. Second concrete step
3. ...

## ...

<?allow-empty-section?>

## Acceptance Criteria

- [ ] Criterion described as observable behavior
- [ ] Another criterion
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
