package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/report"
	"github.com/jeduden/frit/internal/textw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLaneShortsDropsTheRedundantPrefix(t *testing.T) {
	got := laneShorts([]string{"plan/100-shader-unit", "feature/x"}, 100)

	assert.Equal(t, []string{"shader-unit", "feature/x"}, got,
		"the plan/<id>- prefix goes; a foreign convention stays whole")
}

// boardWith builds a one-row board carrying a long title, to exercise
// the width trimming.
func boardWith(title string) *report.BoardDoc {
	doc := report.NewBoard("/x", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:100", Repo: "atlas", ID: 100, Status: "🔳",
		Title: title, Held: true, Holds: []string{"plan/100-underway"},
	}, "claude", "working")

	return doc
}

// TestBoardCellNamesAMaturedHoldsAge: the held-stale cell of the
// verb-state table — a matured hold reads as stale with its age, not
// merely as held, so a takeover candidate is told apart from a live
// one at a glance.
func TestBoardCellNamesAMaturedHoldsAge(t *testing.T) {
	doc := report.NewBoard("/x", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:100", Repo: "atlas", ID: 100, Status: "🔳",
		Title: "Underway", Held: true, Holds: []string{"plan/100"},
		Stale: true, StaleFor: 3 * time.Hour,
	}, "", "")

	cell := boardCell("held", doc, doc.Plans[0])

	assert.Contains(t, cell, "plan/100")
	assert.Contains(t, cell, "stale", "a matured hold reads as stale")
	assert.Contains(t, cell, "3h", "the age rides beside it")
}

// TestBoardCellLeavesALiveHoldUnmarked: a fresh hold, window not yet
// matured, reads as plainly held — the stale marker names only a
// takeover candidate.
func TestBoardCellLeavesALiveHoldUnmarked(t *testing.T) {
	doc := report.NewBoard("/x", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:100", Repo: "atlas", ID: 100, Status: "🔳",
		Title: "Underway", Held: true, Holds: []string{"plan/100"},
	}, "", "")

	cell := boardCell("held", doc, doc.Plans[0])

	assert.Equal(t, "plan/100", cell,
		"a live hold carries no stale marker or age")
}

// TestPrintBoardFitsTheWidthWhenGiven: with a width, no rendered line
// spills past it, and the trimmed title is marked.
func TestPrintBoardFitsTheWidthWhenGiven(t *testing.T) {
	longTitle := strings.Repeat("very long title ", 8)
	var buf bytes.Buffer

	printBoard(&buf, boardWith(longTitle), 80, boardCols)

	line := strings.TrimRight(buf.String(), "\n")
	assert.LessOrEqual(t, textw.Width(line), 80, "the row fits the terminal")
	assert.Contains(t, line, "…", "the trimmed title is marked")
	assert.Contains(t, line, "underway",
		"the lane shows without its plan/<id>- prefix")
}

// TestPrintBoardTrimsTheLaneOnANarrowTerminal: when a long lane would
// crowd out the title, the lane gives up its tail too so the row still
// fits.
func TestPrintBoardTrimsTheLaneOnANarrowTerminal(t *testing.T) {
	doc := report.NewBoard("/x", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:smalt:100", Repo: "smalt", ID: 100, Status: "🔳",
		Title: "A title that also wants room",
		Held:  true, Holds: []string{"plan/100-render-consistency-loop-gas-giants"},
	}, "", "")
	var buf bytes.Buffer

	printBoard(&buf, doc, 80, boardCols)

	line := strings.TrimRight(buf.String(), "\n")
	assert.LessOrEqual(t, textw.Width(line), 80,
		"a long lane is trimmed rather than spilling the row")
	assert.Contains(t, line, "…")
}

// TestPrintBoardWidthZeroKeepsTheFullTitle: a pipe or a test gets the
// whole title, never trimmed to a window it does not have.
func TestPrintBoardWidthZeroKeepsTheFullTitle(t *testing.T) {
	longTitle := strings.Repeat("very long title ", 8)
	var buf bytes.Buffer

	printBoard(&buf, boardWith(longTitle), 0, boardCols)

	assert.Contains(t, buf.String(), strings.TrimSpace(longTitle))
	assert.NotContains(t, buf.String(), "…")
}

// TestWidthFlagOverridesDetection: --width fits a table even with no
// terminal to measure, and works given before the verb like any global
// flag.
func TestWidthFlagOverridesDetection(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "🔳", strings.Repeat("very long title ", 6), nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"--root", root, "--width", "50", "board"},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	line := strings.TrimRight(out.String(), "\n")
	assert.LessOrEqual(t, textw.Width(line), 50,
		"--width fits the table though stdout is not a terminal")
	assert.Contains(t, line, "…")
}

func TestSelectBoardColumns(t *testing.T) {
	all, err := selectBoardColumns("")
	require.NoError(t, err)
	assert.Equal(t, boardCols, all, "empty is every column")

	got, err := selectBoardColumns("id, description")
	require.NoError(t, err)
	assert.Equal(t, []string{"id", "title"}, got,
		"description is an alias for title, whitespace ignored")

	_, err = selectBoardColumns("id,bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown column")
}

// TestBoardColumnsShowsOnlyWhatIsAsked: --columns id,description prints
// the two columns and nothing else.
func TestBoardColumnsShowsOnlyWhatIsAsked(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 100, "🔳", "The description here", nil, "")
	withHerdr(t, herdrReturning())
	var out, errb bytes.Buffer

	code := run([]string{"board", "--columns", "id,description", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "100")
	assert.Contains(t, got, "The description here")
	assert.NotContains(t, got, "🔳", "the status column was not asked for")
	assert.NotContains(t, got, "atlas", "nor the repo column")
}

func TestBoardColumnsRejectsUnknown(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	initRepo(t, root, "atlas")
	var out, errb bytes.Buffer

	code := run([]string{"board", "--columns", "id,bogus", "--root", root},
		&out, &errb)

	assert.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "unknown column")
}

func TestTerminalWidthIsZeroForANonTerminal(t *testing.T) {
	require.Equal(t, 0, terminalWidth(&bytes.Buffer{}),
		"a buffer is not a terminal, so no width is imposed")
}

// TestFitLastColumnCapsTheFlexibleCell: the last column is trimmed to
// what the fixed columns and gaps leave, and the shorter fixed cells
// are untouched.
func TestFitLastColumnCapsTheFlexibleCell(t *testing.T) {
	rows := [][]string{
		{"atlas", "100", "opus", strings.Repeat("word ", 20)},
		{"orrery", "7", "sonnet", "short"},
	}

	fitLastColumn(40, rows)

	for _, r := range rows {
		assert.LessOrEqual(t, textw.Width(strings.Join(r, "  ")), 40)
	}
	assert.Equal(t, "short", rows[1][3], "a short cell is left whole")
}

// TestReadyTrimsTitleToWidth: the ready table fits a terminal, and full
// when there is none.
func TestReadyTrimsTitleToWidth(t *testing.T) {
	doc := report.NewReady("/x", "h")
	doc.SetPlans([]discovery.Plan{{
		Repo: "atlas", ID: 100, Status: "🔲", Model: "opus",
		Title: strings.Repeat("very long title ", 6),
	}})

	var fitted bytes.Buffer
	printReady(&fitted, doc, 50)
	line := strings.TrimRight(fitted.String(), "\n")
	assert.LessOrEqual(t, textw.Width(line), 50)
	assert.Contains(t, line, "…")

	var full bytes.Buffer
	printReady(&full, doc, 0)
	assert.NotContains(t, full.String(), "…", "no width means no trim")
}
