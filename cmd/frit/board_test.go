package main

import (
	"bytes"
	"strings"
	"testing"

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

// TestPrintBoardFitsTheWidthWhenGiven: with a width, no rendered line
// spills past it, and the trimmed title is marked.
func TestPrintBoardFitsTheWidthWhenGiven(t *testing.T) {
	longTitle := strings.Repeat("very long title ", 8)
	var buf bytes.Buffer

	printBoard(&buf, boardWith(longTitle), 80)

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

	printBoard(&buf, doc, 80)

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

	printBoard(&buf, boardWith(longTitle), 0)

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
