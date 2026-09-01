---
n: 2
title: The stale and dead hold markers carry a one-line legend
status: "🔲"
result: false
---
Print what `(stale …)` and `(dead)` mean, once, beneath the table —
only when the hold column is shown and only when a marker is actually
on screen. A clean board, or one with `--columns` omitting `held`,
prints nothing extra.

**Assumes.** `heldCell` in [cmd/frit/main.go](../../cmd/frit/main.go)
already stamps `(stale <age>)` on a matured hold and `(dead)` on a
confirmed-gone session; `report.BoardPlan` already carries `Stale` and
`Dead` per row. `printBoard`'s `rows`/`cols` loop from phase 1 already
has every plan and the selected columns in hand when it finishes
printing the table, so the legend is a second, short write after it —
no new data, no change to `report.BoardDoc` or the `--json` output.

**Value.** `(stale 3h)` and `(dead)` are terse by design — they ride
inside the already-narrow hold column. A reader who has not met them
before has no way to learn what they mean from the table alone. One
line beneath the table, present only when a marker actually appears,
closes that without taxing the common case where nothing is stale or
dead.

**RED.** In [cmd/frit/board_test.go](../../cmd/frit/board_test.go).

- `TestPrintBoardLegendsAStaleHold`: a one-row board (reuse
  `boardWith`'s shape, or build one directly) with `Stale: true`,
  `printBoard(&buf, doc, 0, boardCols)`. Assert the output names
  `stale` and says the window matured, and does not mention `dead`.
- `TestPrintBoardLegendsADeadHold`: same shape with `Dead: true`
  instead. Assert the output names `dead` and says the session is
  confirmed gone, and does not mention `stale`.
- `TestPrintBoardLegendsBothWhenBothAppear`: two rows, one stale, one
  dead. Assert a single legend line carries both explanations — not
  one line each.
- `TestPrintBoardOmitsTheLegendWhenClean`: `boardWith`'s plain fixture
  (no stale, no dead). Assert neither `stale` nor `dead` appears
  outside the table's own hold cell content (i.e., no legend line is
  present — `boardWith` carries neither marker, so the assertion is
  just that the output has no extra line naming either word).
- `TestPrintBoardOmitsTheLegendWhenHeldIsNotShown`: a stale plan,
  `printBoard(&buf, doc, 0, []string{"id", "title"})` — `held` outside
  `cols`. Assert no legend line appears; the marker text lives only in
  the hold column, which is not on screen to explain.

**GREEN.** In [cmd/frit/main.go](../../cmd/frit/main.go).

- Add `boardLegend(cols []string, plans []report.BoardPlan) string`:
  "" unless `held` is in `cols` and at least one plan is `Stale` or
  `Dead`. Built from up to two clauses, joined `"; "`, each present
  only when its marker appears across `plans`:
  `"(stale …) = the hold's takeover window has matured"` and
  `"(dead) = the bound session is confirmed gone"` — the same wording
  `foreignHoldRefusal` in [cmd/frit/release.go](../../cmd/frit/release.go)
  already uses for both, so the two places name the same fact the same
  way.
- In `printBoard`, after the row-printing loop, compute the legend from
  `cols` and `doc.Plans` (not the header-prefixed `rows`, which carries
  rendered strings, not the `Stale`/`Dead` flags) and, if non-empty,
  print it — `textw.Truncate`d to `width` when `width > 0`, the same
  guard the header and data rows already answer to.

**Guard the edges.** `--columns` without `held` never legends, since a
marker the reader cannot see needs no key. `--json` is untouched:
`boardLegend` is called only from the table branch of `printBoard`,
never from `report.WriteJSON`'s path.

**Gate.** A board carrying a `(stale …)` or `(dead)` hold prints a
legend line explaining each marker present; a clean board, or a
`--columns` selection omitting `held`, prints none; `board --json` is
unchanged; the legend line fits `width` like any other; `go test
./...` and `go tool -modfile=tools/go.mod golangci-lint run` are
green.

This is the plan's last phase: tick the plan's Acceptance Criteria,
flip `plan.md`'s `status:` to `✅`, and `mdsmith fix PLAN.md`.
