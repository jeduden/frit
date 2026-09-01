---
n: 1
title: The table opens on a header row, and a held-idle lane reads as idle
status: "✅"
result: true
summary: The board table opens on a header naming every column, a held lane with no live agent reads idle instead of a bare dash, and every rendered line — header included — fits the terminal width.
---
## Handoff

`frit board`'s table now opens on a header row naming every selected
column, and `agentLabel` reads a held lane with no live agent as
`idle` rather than the bare `-` an unheld lane shows — the hold and
agent columns can no longer collapse to the same glyph.

**Landed as specified.** `boardColLabel`/`boardHeader` build the header
row from `boardCols`, prepended to the data rows before the `width > 0`
fit in `printBoard`. `agentLabel` gained a `held bool` parameter;
`boardCell`'s agent case passes `p.Held`. The three width tests
(`TestPrintBoardFitsTheWidthWhenGiven`,
`TestPrintBoardTrimsTheLaneOnANarrowTerminal`,
`TestWidthFlagOverridesDetection`) now split on newlines and assert
every line fits, per the phase spec.

**Deviation: `printBoard` no longer uses `text/tabwriter`.** The spec
assumed the header would ride the existing `tabwriter` with no new
width machinery. It does not: `tabwriter` sizes a column by rune
count, not by the terminal columns a rune paints. The status column
carries a one-rune, two-column-wide emoji (`🔳`); once the header adds
the plain-ASCII word `status` (six runes) to that same column,
`tabwriter` pads the emoji cell to six *runes* rather than six
*display columns*, so the data row lands one column wider than the
header and than `fitBoard`'s budget — `TestPrintBoardFitsTheWidthWhenGiven`
et al. failed at width 81/80 before this was found. This was latent
before the header existed: a column's own single row was always
self-consistent, so the mismatch never surfaced.

Fixed by dropping `tabwriter` from `printBoard` in favor of a small
hand-rolled `alignRow(row []string, colw []int)`, padding every
column but the last with `textw.Width`-measured spaces. `colw` is
computed once over the header and every data row after `fitBoard`'s
trim, so alignment always matches the width `fitBoard` actually
measured. `tabwriter` is untouched everywhere else in
[cmd/frit/main.go](../../cmd/frit/main.go) — this is scoped to
`printBoard` alone.

**Other caller updated.** `printOpen` in
[cmd/frit/dispatch.go](../../cmd/frit/dispatch.go) also called
`agentLabel` — the phase's "one caller" assumption was off by one. It
reports a lane `open` just confirmed is focused, where held is not a
concept `OpenDoc` carries, so it passes `false`; behavior is
unchanged (a present agent is unaffected by `held`, and an absent one
still reads as `-`, matching the prior output exactly).

**For phase 2.** The stale/dead legend prints a line beneath the
table, not a table row, so it does not go through `alignRow`/`colw` —
but it must still respect `width` for the "every line fits" gate, the
same way the header row now does. `board --json` is untouched; the
model and JSON contract carry no change from this phase.

`go test ./...` and `go tool -modfile=tools/go.mod golangci-lint run`
are clean.
