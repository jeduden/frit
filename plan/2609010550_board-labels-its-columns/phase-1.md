---
n: 1
title: The table opens on a header row, and a held-idle lane reads as idle
status: "🔲"
result: false
---
Give the board table the two things that keep held work from reading as
free: a header row that names every column, and an agent column that
says `idle` when a lane is held but no session is live on it. Both ride
the renderer already in
[cmd/frit/main.go](../../cmd/frit/main.go); no discovery, model or JSON
change is in this phase.

**Assumes.** `boardCols` is the ordered column set and `boardCell`
already renders one plan's value per column, so a header is the same
columns rendered from their names rather than a plan. `fitBoard` and the
`tabwriter` in `printBoard` measure every row handed to them, so a
header row prepended before the fit is trimmed and aligned like any
other. `BoardPlan` already carries `Held`, so the idle branch needs no
new data. `agentLabel` has one caller, `boardCell`, so its signature is
free to change.

**Value.** A reader can no longer mistake the hold column for the agent
column: the header names both, and a held-but-idle lane shows a slug
beside `idle` rather than a slug beside the same `-` an unheld plan
shows. The two columns can never collapse to one glyph, which is the
exact misread issue 121 reports.

**RED.** In [cmd/frit/board_test.go](../../cmd/frit/board_test.go).

- `TestPrintBoardOpensWithAHeaderRow`: build a one-row board (reuse
  `boardWith`), `printBoard(&buf, doc, 0, boardCols)`, split the output
  on newlines. Assert the first line names the columns — it contains
  `hold` and `agent` (the two that were ambiguous) and `title` — and
  that the data row, carrying the plan title, comes after it. A width of
  zero means nothing is trimmed, so the labels appear whole.
- `TestAgentLabelSaysIdleForAHeldLaneWithNoLiveAgent`: drive `agentLabel`
  directly. With presence true, no agent, and held true it is `idle`;
  with presence true, no agent, and held false it is `-`; with presence
  false it is `?` whatever the hold; with an agent present it names the
  agent regardless of held. This pins that only a held lane with no live
  agent — the dangerous case — reads as idle.
- `TestBoardRowShowsIdleForAHeldPlanWithNoAgent`: a CLI-level board over
  a held plan whose lane has no live herdr pane (reuse the held-plan
  fixture shape from
  [cmd/frit/discovery_test.go](../../cmd/frit/discovery_test.go)'s
  `TestBoardShowsUnfinishedWithHolderAndAgent`, but with
  `herdrReturning()` answering no pane). Assert the output contains
  `idle` and that the held lane's slug still shows — a held plan is not
  read as free.

**Update the width tests to a line-by-line measure.**
`TestPrintBoardFitsTheWidthWhenGiven`,
`TestPrintBoardTrimsTheLaneOnANarrowTerminal` and
`TestWidthFlagOverridesDetection` each collapse the whole buffer to one
line before a width check. That held only while the table was a single
data row. With a header above it, split the output on newlines. Assert
every non-empty line fits the width. Target the data row — the line
carrying the title, or the trimmed marker `…` — for the content checks.
`TestReadyTrimsTitleToWidth` is untouched: `ready` grows no header here.
`TestPrintBoardWidthZeroKeepsTheFullTitle`,
`TestBoardColumnsShowsOnlyWhatIsAsked` and the CLI board tests assert
with `Contains` over the whole buffer, so they stay green.

**GREEN.** In [cmd/frit/main.go](../../cmd/frit/main.go).

- Add `boardColLabel(name string) string`: the display word for a column
  key, mapping `held` to `hold` and returning the key itself for the
  rest. Keep it beside `boardCols` so the label set and the column set
  stay together.
- Add `boardHeader(cols []string) []string`: one cell per column, each
  `boardColLabel(name)`.
- In `printBoard`, build `rows` with the header row first —
  `append([][]string{boardHeader(cols)}, dataRows...)` — before the
  `width > 0` fit, so `fitBoard` measures and trims the header like any
  row and the `tabwriter` aligns the labels over their columns.
- Change `agentLabel` to `agentLabel(presence, held bool, agent, status
  string) string` with the idle branch: unknown presence is `?`; an
  agent present names it (with its status if any); then a held lane with
  no agent is `idle`; otherwise `-`. `boardCell`'s agent case passes
  `p.Held`.

**Guard the edges.** A `--columns` selection that omits `held` or
`agent` simply omits that header cell too, since the header is built
from the same `cols`. A width narrow enough to trim still fits, because
the header flows through the same `fitBoard`. An empty board still
prints `nothing outstanding` and no header, since the early return in
`printBoard` precedes the header build.

**Gate.** `frit board`'s rendered table opens on a header row naming
each column; a held plan with no live agent shows `idle` in the agent
column rather than `-`; every rendered line fits the width; `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are green.
