// Package scenario keeps the lease-protocol scenario matrix and its
// godog feature tags in bijection: every documented S-id has a tagged
// scenario, and every tag names a documented row.
package scenario

import (
	"fmt"
	"os"
	"regexp"

	"github.com/jeduden/frit/internal/planmeta"
)

// idNumber is how every id is numbered, on the matrix side and the
// feature side alike: from 1 with no leading zero, so "S01" and "S1"
// can never name one row twice.
const idNumber = `[1-9][0-9]*`

// rowID is the shape every id cell in a matrix table takes: an S
// scenario, or an F liveness / A safety attacker row, so numbered.
var rowID = regexp.MustCompile(`^[SFA]` + idNumber + `$`)

// MatrixIDs reads the S-scenario ids off the matrix tables in path —
// the tables whose header leads with "#", the shape every S, F and A
// table shares — keyed by each row's leading cell. Any other table, and
// all prose, is ignored, so a glossary whose first column happens to
// start with "S" is never mistaken for a malformed scenario. Within a
// matrix table every row must lead with a clean id: a lowercase,
// suffixed or missing id is reported with its line rather than silently
// dropped, as is an id repeated across rows, since a set would
// otherwise hide that one of the two rows has no scenario of its own.
// The tables are read through the markdown parser mdsmith itself uses,
// so a pipe row quoted in a fenced code block is not a row.
func MatrixIDs(path string) (map[string]bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scenario: read matrix: %w", err)
	}

	ids := map[string]bool{}
	for _, tbl := range planmeta.Tables(body) {
		if len(tbl.Header) == 0 || tbl.Header[0] != "#" {
			continue
		}
		for _, row := range tbl.Rows {
			if err := collectRowID(path, row, ids); err != nil {
				return nil, err
			}
		}
	}

	return ids, nil
}

// collectRowID records the S id a matrix row leads with, passes an F or
// A attacker id by, and reports a leading cell that is no id at all.
func collectRowID(path string, row planmeta.TableRow, ids map[string]bool) error {
	first := ""
	if len(row.Cells) > 0 {
		first = row.Cells[0]
	}
	if !rowID.MatchString(first) {
		return fmt.Errorf("scenario: %s:%d: malformed scenario id %q", path, row.Line, first)
	}
	if first[0] != 'S' {
		return nil
	}
	if ids[first] {
		return fmt.Errorf("scenario: %s:%d: duplicate scenario id %q", path, row.Line, first)
	}
	ids[first] = true

	return nil
}
