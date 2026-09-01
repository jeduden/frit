---
n: 2
title: The stale and dead hold markers carry a one-line legend
status: "✅"
result: true
summary: A board carrying a `(stale …)` or `(dead)` hold now prints a one-line legend explaining each marker present, on the same wording release's own refusal uses; a clean board, or `--columns` omitting `held`, prints none.
---
## Handoff

`printBoard` now prints a legend line beneath the table whenever the
`held` column is shown and at least one plan carries `(stale …)` or
`(dead)` — explaining both on one line when both appear, and nothing
at all on a clean board or a `--columns` selection that drops `held`.

**Landed as specified.** `boardLegend(cols []string, plans
[]report.BoardPlan) string` reads `doc.Plans` directly (not the
rendered `rows`), so it works off the same `Stale`/`Dead` flags
`heldCell` already reads. Its two clauses —
`"(stale …) = hold has matured"` and `"(dead) = the bound session is
confirmed gone"` — reuse the exact wording `foreignHoldRefusal` in
[cmd/frit/release.go](../../cmd/frit/release.go) already refuses a
foreign hold with, so a reader meets one phrasing for both facts. The
legend line runs through `textw.Truncate` when `width > 0`, the same
guard the header and data rows already answer to.

**No deviation this time.** Unlike phase 1, this phase's assumptions
held: the legend is a plain second `fmt.Fprintln` after the table's own
loop, no interaction with `alignRow`/`colw` since it is not a table
row, and `--json` untouched — `boardLegend` is only ever called from
`printBoard`'s table branch.

**Plan closed.** This was the plan's last phase. Every Acceptance
Criterion is met: the header row, the `idle` label, the stale/dead
legend, `board --json` unchanged, every rendered line fitting width,
`go test ./...` and `golangci-lint run` clean. `plan.md`'s `status:` is
`✅`.
