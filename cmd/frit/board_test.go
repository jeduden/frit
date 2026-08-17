package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateCellsMarksACut(t *testing.T) {
	assert.Equal(t, "hello", truncateCells("hello", 10))
	assert.Equal(t, "hel…", truncateCells("hello world", 4))
	assert.Equal(t, "…", truncateCells("hello", 1))
	assert.Equal(t, "", truncateCells("hello", 0))
}

func TestLaneShortsDropsTheRedundantPrefix(t *testing.T) {
	got := laneShorts([]string{"plan/100-shader-unit", "feature/x"}, 100)

	assert.Equal(t, []string{"shader-unit", "feature/x"}, got,
		"the plan/<id>- prefix goes; a foreign convention stays whole")
}

func TestCellWidthCountsAStatusGlyphAsTwo(t *testing.T) {
	assert.Equal(t, 2, cellWidth("🔳"))
	assert.Equal(t, 5, cellWidth("hello"))
	// An em dash stays one column.
	assert.Equal(t, 3, cellWidth("a—b"))
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
	assert.LessOrEqual(t, cellWidth(line), 80, "the row fits the terminal")
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
	assert.LessOrEqual(t, cellWidth(line), 80,
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

func TestTerminalWidthIsZeroForANonTerminal(t *testing.T) {
	require.Equal(t, 0, terminalWidth(&bytes.Buffer{}),
		"a buffer is not a terminal, so no width is imposed")
}
