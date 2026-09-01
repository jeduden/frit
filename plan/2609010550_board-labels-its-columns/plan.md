---
id: 2609010550
title: The board table labels its columns, so held work never reads as free
status: "🔳"
summary: >-
  frit board renders its table with no header row, and two adjacent
  columns — the hold and the live agent — both collapse to a bare dash
  in the common held-but-idle case, so a held plan with unmerged work
  but no live session reads as unheld. The miss is silent and points
  the dangerous way: held work looks free, and a reader concludes a
  claim lapsed and that pick would hand the work away. frit ready omits
  the plan and board --json carries held true — only the table misleads.
  Fix it in the renderer, where the ambiguity lives: open the table on a
  header row that names every column, say idle for a held lane with no
  live agent so the two columns can never share a glyph, and print a
  one-line legend for the stale and dead hold markers when they appear.
  Closes issue 121.
model: sonnet
depends-on: []
---
# The board table labels its columns, so held work never reads as free

## Goal

`frit board`'s table opens on a header row that names every column, a
held plan with no live agent reads as `idle` rather than the bare `-` an
unheld plan shows, and the `(stale …)`/`(dead)` hold markers carry a
one-line legend when they appear — so a held plan with unmerged work is
never read as free. `ready` and `--json` were already unambiguous; this
closes the table-only gap. This is issue 121.

## Context

**Where the ambiguity lives.** The board table renders through
`printBoard` in [cmd/frit/main.go](../../cmd/frit/main.go): a
`tabwriter` over rows built cell by cell by `boardCell`, with no header
row. `heldCell` renders the hold column — the lane slug, or `-`, with a
`(stale …)` or `(dead)` suffix — and `agentLabel` renders the agent
column: `?` when herdr is unreachable, `-` when no agent is live, or the
agent and its status. In the common case a held-but-idle plan renders
its hold slug beside a bare `-` for the agent, and an unheld plan
renders `-` beside `-`. Nothing on screen says which column is which,
and the eye lands on the `-`. The failure is the dangerous direction:
held work looks free.

**Why the fix rides in the renderer, and nowhere else.** `frit ready`
already omits a held plan, and `board --json` already carries `held:
true`, `holds`, `stale` and `dead` — every consumer but the table reads
the state correctly. So this is table rendering, not a data or discovery
bug; the model in [internal/report/board.go](../../internal/report/board.go)
and the JSON contract are untouched.

**Reuse first.** The header rides the existing path: `boardCols` already
names the columns in order, so the header is those names as a first row
that flows through the same `fitBoard` — column widths and trimming
already account for every row, the header included, with no new width
machinery. The idle case is a third branch in `agentLabel`, keyed on the
`Held` flag `BoardPlan` already carries — no new data reaches the
renderer. The legend reads the same `p.Stale`/`p.Dead` already on the
rows, printed once beneath the table only when a marker is actually
present.

Phase 1 found one seam that did need new machinery: `tabwriter` sizes a
column by rune count, not the terminal columns a rune paints, so the
one-rune, two-column-wide status glyph shares a column with the new
six-rune `status` header word and lands one column over budget.
`printBoard` now aligns by hand with `textw.Width` instead of
`tabwriter`; see [phase-1.result.md](phase-1.result.md) for the detail.
`tabwriter` is untouched everywhere else.

**What the header row disturbs.** A few board tests in
[cmd/frit/board_test.go](../../cmd/frit/board_test.go) treat the whole
rendered buffer as a single line — `strings.TrimRight(buf, "\n")` then a
width assertion — which held only while the table was one data row. With
a header above it that measure spans two lines. Phase 1 updates those
tests to iterate the output line by line, asserting each fits, and to
locate the data row for its content checks. That line-by-line shape is
the test approach the legend phase copies. The board JSON golden pins
the document, not the table, so it is unaffected; the CLI board tests
assert with `Contains` over the whole buffer, so a header leaves them
green.

**Out of scope.** `ready`, `repos` and the other tables carry no two
adjacent columns that read as each other, so they keep their headerless
form; a fleet-wide header convention is a separate call. The `--json`
document and the `--columns` selection grammar are unchanged.

## Tasks

1. Phase 1 (proving slice): `printBoard` opens the table on a header row
   naming each selected column, and `agentLabel` says `idle` for a held
   lane with no live agent — so the hold and agent columns are labelled
   and can never both collapse to the same glyph. The board width tests
   move to a line-by-line measure.
2. Phase 2: when the hold column is shown and a row is stale or dead,
   `printBoard` prints a one-line legend beneath the table explaining
   each marker present — a clean board prints none.

## Execution

| Phase | Title                                                               | Tier   | Gate                                                                                                                                                              |
| ----- | ------------------------------------------------------------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1     | The table opens on a header row, and a held-idle lane reads as idle | sonnet | the table opens on a header row naming each column; a held-idle lane shows `idle`, not `-`; every line still fits; `go test ./...` green                          |
| 2     | The stale and dead hold markers carry a one-line legend             | sonnet | a board with a stale or dead hold prints a legend line explaining each marker present; a clean board prints none; `board --json` unchanged; `go test ./...` green |

## Phases

<?catalog
glob:
  - "phase-*.md"
  - "!phase-*.result.md"
sort: numeric:n
header: |

  | # | Status | Phase |
  |---|--------|-------|
row: "| {n} | {status} | [{title}](phase-{n}.md) |"
footer: |

?>

| #   | Status | Phase                                                                             |
| --- | ------ | --------------------------------------------------------------------------------- |
| 1   | ✅     | [The table opens on a header row, and a held-idle lane reads as idle](phase-1.md) |
<?/catalog?>

## Acceptance Criteria

- [x] `frit board`'s rendered table opens on a header row that names
      every selected column, so the hold column and the agent column are
      told apart at a glance
- [x] A held plan with no live agent reads as `idle` in the agent
      column, not the bare `-` an unheld plan shows — the two columns can
      never collapse to the same glyph
- [ ] A board carrying a `(stale …)` or `(dead)` hold prints a one-line
      legend explaining each marker present; a clean board prints none
- [x] `board --json` is unchanged, and every rendered table line still
      fits the terminal width
- [x] All tests pass: `go test ./...`
- [x] `go tool -modfile=tools/go.mod golangci-lint run` is clean
