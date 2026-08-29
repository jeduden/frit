// Package headroom answers one question: does a plan file have room
// left to grow another "## Phase N" section before mdsmith's own
// max-file-length rule would fire?
//
// It never reads the configured cap directly — mdsmith does not
// expose it through pkg/mdsmith's public API, and a second copy of
// the number here would drift the moment the repository's own
// .mdsmith.yml changes it. Instead it asks the oracle: pad an
// in-memory copy of the plan's source and let the session that
// already validates it answer.
package headroom

import (
	"bytes"

	"github.com/jeduden/mdsmith/pkg/markdown"
	"github.com/jeduden/mdsmith/pkg/mdsmith"
)

// padLine is a comment, not a blank line: a blank line would trip
// no-multiple-blanks and the oracle would answer about the wrong
// rule.
const padLine = "<!-- headroom padding -->\n"

// maxFileLength is the mdsmith rule name this package's whole
// question is about.
const maxFileLength = "max-file-length"

// ReserveLines is how many lines a percent%-of-body reserve holds for
// source, rounded up. Front matter is excluded: it grows with a
// plan's metadata, not its phases, so it must not dilute or inflate
// the reserve a phase section actually needs. percent <= 0 disables
// the reserve entirely.
func ReserveLines(source []byte, percent int) int {
	if percent <= 0 {
		return 0
	}

	_, body := markdown.StripFrontMatter(source)
	lines := markdown.CountLines(body)

	return (lines*percent + 99) / 100
}

// Room reports how many of reserve padding lines fit onto rel's
// source before mdsmith's max-file-length rule fires against sess,
// narrowing over [0, reserve]. A plan short of its full reserve has
// no headroom; the shortfall is reserve - room.
func Room(sess *mdsmith.Session, rel string, source []byte, reserve int) (int, error) {
	lo, hi := 0, reserve
	for lo < hi {
		mid := lo + (hi-lo+1)/2

		ok, err := fits(sess, rel, source, mid)
		if err != nil {
			return 0, err
		}
		if ok {
			lo = mid
		} else {
			hi = mid - 1
		}
	}

	return lo, nil
}

// fits reports whether source padded with n reserve lines still
// passes max-file-length.
func fits(sess *mdsmith.Session, rel string, source []byte, n int) (bool, error) {
	diags, err := sess.Check(rel, pad(source, n))
	if err != nil {
		return false, err
	}

	for _, d := range diags {
		if d.Name == maxFileLength {
			return false, nil
		}
	}

	return true, nil
}

// pad appends n padding lines to source, adding a newline first if
// source's last line is not already terminated — otherwise the first
// pad line would merge into it, one line short of what the reserve
// claims.
func pad(source []byte, n int) []byte {
	var buf bytes.Buffer
	buf.Write(source)
	if len(source) > 0 && source[len(source)-1] != '\n' {
		buf.WriteByte('\n')
	}
	for range n {
		buf.WriteString(padLine)
	}

	return buf.Bytes()
}
