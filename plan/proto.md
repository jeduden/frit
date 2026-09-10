---
id: 'int & >=2601010000'
title: 'string & != ""'
status: '"🔲" | "🔳" | "✅" | "⛔"'
summary: 'string | *""'
model: '"haiku" | "sonnet" | "opus" | "fable" | *""'
depends-on: '[...int] | *[]'
phases: >-
  [...{n: int | string, title: string & != "",
  status: "🔲" | "🔳" | "✅" | "⛔"}] | *[]
---
<?require
filename: "*.md"
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

  Layout:
  - A plan is one file, `plan/<id>_<slug>.md`, or a
    folder, `plan/<id>_<slug>/plan.md`, holding
    companion files (research, fixtures, diagrams)
    beside it. Both key by the front-matter `id:`; the
    naming convention is checked by the `plan` kind's
    `path-pattern:` in `.mdsmith.yml`, not by the
    `filename:` require above.
  - A folder plan sits one directory deeper than a
    flat plan, so a Markdown link to a repo path needs
    one extra `../`.

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
  - A gate catches a wrong answer in what the phase
    ships. A phase that ships or edits a skill, or
    any claim about a verb's behavior, gates by
    running the command against the built frit and
    confirming the output matches the claim. Lint
    and the dogfood-match test pass on a false one,
    so neither is that gate.
  - Each phase states its BDD scenario coverage in
    its own spec: an `@S<n>` it adds or extends, or
    plainly that none applies. Follow the procedure
    in docs/development.md's executable scenario
    matrix; S93 (plan 2609052303 phase 1) is the
    worked example.
  - Both sections are optional for a plan small
    enough to land in one go.

  Phases as their own files (folder plans):
  - A folder plan keeps each phase in its own
    `phase-N.md` beside `plan.md`, carrying `n`,
    `title`, `status` and `result` front matter — the
    single home of that phase's status. `result` is
    `false` on the spec, and its `phase-N.result.md`
    record carries `result: true` plus a non-empty
    `summary` line once the phase closes.
  - Such a plan may add an optional `## Phases`
    catalog that regenerates from those files. The
    working directive — the one every closed-out plan
    already carries — sits below, ready to copy.
  - The section is optional: the `## ...` slot above
    admits it, so a plan without it still validates.
    See this repo's plan 2608310418 for a live one.

-->

Working `## Phases` catalog directive. Fenced below so mdsmith shows
it rather than running it here; copy only the `<?catalog ... ?>` lines
verbatim into a plan's own `## Phases` section, unfenced, so it runs:

```yaml
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
```

Glob both files, not just `phase-*.md` alone: a closed phase's
`phase-N.result.md` also matches `phase-*.md` and carries the same
`n`, so the naive single-glob form renders that phase's row twice.
The `row-expr` above branches on `result` instead, so a
`phase-N.result.md` contributes a summary row, not a second spec row.

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
