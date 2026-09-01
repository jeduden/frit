package main

import (
	"bytes"
	"os"
	"path/filepath"
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

// TestBoardCellNamesADeadSessionsHold: a hold whose window has not
// matured but whose bound session herdr confirms gone still reads as
// a takeover candidate, told apart from a live hold at a glance.
func TestBoardCellNamesADeadSessionsHold(t *testing.T) {
	doc := report.NewBoard("/x", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:100", Repo: "atlas", ID: 100, Status: "🔳",
		Title: "Underway", Held: true, Holds: []string{"plan/100"},
		Dead: true,
	}, "", "")

	cell := boardCell("held", doc, doc.Plans[0])

	assert.Contains(t, cell, "plan/100")
	assert.Contains(t, cell, "dead", "a confirmed-dead session is not a live hold")
}

// TestBoardColLabelRendersHoldForHeld: the held column's header reads
// as "hold" — the word a reader expects over the lane-slug cell —
// while every other column keeps its own key as its header word.
func TestBoardColLabelRendersHoldForHeld(t *testing.T) {
	assert.Equal(t, "hold", boardColLabel("held"), "held renders as hold")
	assert.Equal(t, "title", boardColLabel("title"),
		"an ordinary column keeps its own key")
}

// TestBoardHeaderNamesEveryColumnInOrder: the header row carries one
// label per selected column, in the order given.
func TestBoardHeaderNamesEveryColumnInOrder(t *testing.T) {
	got := boardHeader([]string{"id", "held", "title"})

	assert.Equal(t, []string{"id", "hold", "title"}, got)
}

// TestAlignRowPadsEveryColumnButTheLast: each cell is padded out to
// its column's width plus a two-space gap, and the last column carries
// no trailing padding.
func TestAlignRowPadsEveryColumnButTheLast(t *testing.T) {
	got := alignRow([]string{"a", "bb", "c"}, []int{3, 3, 3})

	assert.Equal(t, "a    bb   c", got)
}

// TestAlignRowAccountsForAWideGlyph: a two-column glyph is padded by
// the terminal columns it paints, not its rune count — unlike
// tabwriter, which this replaced for exactly this reason.
func TestAlignRowAccountsForAWideGlyph(t *testing.T) {
	got := alignRow([]string{"🔳", "x"}, []int{2, 1})

	assert.Equal(t, "🔳  x", got)
}

// TestPrintBoardOpensWithAHeaderRow: the table names every column before
// any data, so the hold column and the agent column are told apart at
// a glance rather than read by position.
func TestPrintBoardOpensWithAHeaderRow(t *testing.T) {
	var buf bytes.Buffer

	printBoard(&buf, boardWith("Underway"), 0, boardCols)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 2, "a header row, then one data row")
	assert.Contains(t, lines[0], "hold", "the hold column is named")
	assert.Contains(t, lines[0], "agent", "the agent column is named")
	assert.Contains(t, lines[0], "title", "the title column is named")
	assert.Contains(t, lines[1], "Underway", "the data row follows the header")
}

// TestAgentLabelSaysIdleForAHeldLaneWithNoLiveAgent: only a held lane
// with no live agent — the dangerous, easily-misread case — reads as
// idle. Every other combination is unchanged.
func TestAgentLabelSaysIdleForAHeldLaneWithNoLiveAgent(t *testing.T) {
	assert.Equal(t, "idle", agentLabel(true, true, "", ""),
		"held with no live agent reads as idle, not a bare dash")
	assert.Equal(t, "-", agentLabel(true, false, "", ""),
		"unheld with no live agent is still a bare dash")
	assert.Equal(t, "?", agentLabel(false, true, "", ""),
		"herdr unreachable is unknown regardless of hold")
	assert.Equal(t, "?", agentLabel(false, false, "", ""),
		"herdr unreachable is unknown regardless of hold")
	assert.Equal(t, "claude", agentLabel(true, true, "claude", ""),
		"a live agent names itself regardless of hold")
	assert.Equal(t, "claude (working)", agentLabel(true, false, "claude", "working"),
		"a live agent's status rides beside it regardless of hold")
}

// TestBoardRowShowsIdleForAHeldPlanWithNoAgent: a held plan whose lane
// has no live session reads as idle, and the held lane's slug still
// shows — a held plan is never read as free.
func TestBoardRowShowsIdleForAHeldPlanWithNoAgent(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 100, "🔳", "Underway", nil, "")

	wt := filepath.Join(root, "atlas-100")
	git(t, repo, "worktree", "add", "-q", "-b", "plan/100-underway", wt)
	git(t, wt, "commit", "--allow-empty", "-q", "-m", "plan 100: claim")
	require.NoError(t, os.WriteFile(
		filepath.Join(wt, "work.txt"), []byte("wip\n"), 0o600))
	git(t, wt, "add", "-A")
	git(t, wt, "commit", "-q", "-m", "wip")

	withHerdr(t, herdrReturning())
	var out, errb bytes.Buffer

	code := run([]string{"board", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "idle", "a held lane with no live agent is idle")
	assert.Contains(t, got, "underway",
		"the held lane's slug still shows — held work is not read as free")
}

// TestPrintBoardLegendsAStaleHold: a matured hold's `(stale …)` marker
// is explained beneath the table, and `dead` is not mentioned since no
// row carries it.
func TestPrintBoardLegendsAStaleHold(t *testing.T) {
	doc := report.NewBoard("/x", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:100", Repo: "atlas", ID: 100, Status: "🔳",
		Title: "Underway", Held: true, Holds: []string{"plan/100"},
		Stale: true, StaleFor: 3 * time.Hour,
	}, "", "")
	var buf bytes.Buffer

	printBoard(&buf, doc, 0, boardCols)

	got := buf.String()
	assert.Contains(t, got, "stale", "the marker is named")
	assert.Contains(t, got, "matured", "and explained")
	assert.NotContains(t, got, "dead", "no row carries the dead marker")
}

// TestPrintBoardLegendsADeadHold: a confirmed-dead session's `(dead)`
// marker is explained beneath the table, and `stale` is not mentioned
// since no row carries it.
func TestPrintBoardLegendsADeadHold(t *testing.T) {
	doc := report.NewBoard("/x", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:100", Repo: "atlas", ID: 100, Status: "🔳",
		Title: "Underway", Held: true, Holds: []string{"plan/100"},
		Dead: true,
	}, "", "")
	var buf bytes.Buffer

	printBoard(&buf, doc, 0, boardCols)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	legend := lines[len(lines)-1]
	assert.Contains(t, legend, "dead", "the marker is named")
	assert.Contains(t, legend, "confirmed gone", "and explained")
	assert.NotContains(t, legend, "stale", "no row carries the stale marker")
}

// TestPrintBoardLegendsBothWhenBothAppear: a board with one stale and
// one dead hold explains both, on a single legend line rather than one
// per row.
func TestPrintBoardLegendsBothWhenBothAppear(t *testing.T) {
	doc := report.NewBoard("/x", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:100", Repo: "atlas", ID: 100, Status: "🔳",
		Title: "Underway", Held: true, Holds: []string{"plan/100"},
		Stale: true, StaleFor: 3 * time.Hour,
	}, "", "")
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:101", Repo: "atlas", ID: 101, Status: "🔳",
		Title: "Also underway", Held: true, Holds: []string{"plan/101"},
		Dead: true,
	}, "", "")
	var buf bytes.Buffer

	printBoard(&buf, doc, 0, boardCols)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	legend := lines[len(lines)-1]
	assert.Contains(t, legend, "stale")
	assert.Contains(t, legend, "dead")

	legendLines := 0
	for _, l := range lines {
		if strings.Contains(l, "matured") || strings.Contains(l, "confirmed gone") {
			legendLines++
		}
	}
	assert.Equal(t, 1, legendLines, "one legend line, not one per marker")
}

// TestPrintBoardOmitsTheLegendWhenClean: nothing stale or dead means no
// legend line — the common case pays nothing extra.
func TestPrintBoardOmitsTheLegendWhenClean(t *testing.T) {
	var buf bytes.Buffer

	printBoard(&buf, boardWith("Underway"), 0, boardCols)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 2, "header and one data row, no legend")
}

// TestPrintBoardOmitsTheLegendWhenHeldIsNotShown: --columns without
// held drops the marker itself, so there is nothing left to explain.
func TestPrintBoardOmitsTheLegendWhenHeldIsNotShown(t *testing.T) {
	doc := report.NewBoard("/x", true)
	doc.AddPlan(discovery.Plan{
		Key: "forge:atlas:100", Repo: "atlas", ID: 100, Status: "🔳",
		Title: "Underway", Held: true, Holds: []string{"plan/100"},
		Stale: true, StaleFor: 3 * time.Hour,
	}, "", "")
	var buf bytes.Buffer

	printBoard(&buf, doc, 0, []string{"id", "title"})

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 2, "header and one data row, no legend")
}

// TestPrintBoardFitsTheWidthWhenGiven: with a width, no rendered line
// spills past it, and the trimmed title is marked.
func TestPrintBoardFitsTheWidthWhenGiven(t *testing.T) {
	longTitle := strings.Repeat("very long title ", 8)
	var buf bytes.Buffer

	printBoard(&buf, boardWith(longTitle), 80, boardCols)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	for _, line := range lines {
		assert.LessOrEqual(t, textw.Width(line), 80, "every line fits the terminal")
	}
	dataLine := lines[len(lines)-1]
	assert.Contains(t, dataLine, "…", "the trimmed title is marked")
	assert.Contains(t, dataLine, "underway",
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

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	for _, line := range lines {
		assert.LessOrEqual(t, textw.Width(line), 80,
			"a long lane is trimmed rather than spilling the row")
	}
	assert.Contains(t, lines[len(lines)-1], "…")
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
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	for _, line := range lines {
		assert.LessOrEqual(t, textw.Width(line), 50,
			"--width fits the table though stdout is not a terminal")
	}
	assert.Contains(t, lines[len(lines)-1], "…")
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
