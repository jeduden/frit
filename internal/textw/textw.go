// Package textw measures and trims text by the columns it paints on a
// terminal, not by the runes or bytes it holds.
//
// A status glyph paints two columns from one rune, an em dash one from
// three bytes; a table that lines up or trims by rune or byte count
// gets both wrong. go-runewidth carries the width table, so this is a
// thin, named seam over it rather than a hand-rolled approximation that
// drifts on the awkward runes.
package textw

import "github.com/mattn/go-runewidth"

// ellipsis marks a trimmed string, so a cut cell reads as cut rather
// than as a shorter value.
const ellipsis = "…"

// Width is the number of terminal columns a string paints.
func Width(s string) int {
	return runewidth.StringWidth(s)
}

// Truncate caps a string to max columns, marking a cut with an
// ellipsis. A string already within the cap is returned untouched, and
// a non-positive cap yields the empty string.
func Truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if Width(s) <= max {
		return s
	}

	return runewidth.Truncate(s, max, ellipsis)
}
