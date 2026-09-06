// Package scenario keeps one or more scenario-matrix documents and
// their godog feature tags in bijection: every documented S- or C-id
// has a tagged scenario, and every tag names a documented row.
package scenario

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jeduden/frit/internal/planmeta"
)

// idNumber is how every id is numbered, on the matrix side and the
// feature side alike: from 1 with no leading zero, so "S01" and "S1"
// can never name one row twice.
const idNumber = `[1-9][0-9]*`

// scenarioLetters are the matrix-row and feature-tag leading letters
// that name an id needing a scenario — as opposed to an F liveness or
// A safety attacker row, which passes by uncounted. This is the one
// place a new catalog's letter is added: rowID below, collectRowID's
// guard, and featureTag in features.go all read it, so a third
// catalog is one constant, not three independent letter classes.
const scenarioLetters = "SC"

// attackerLetters are matrix-row-only letters: F liveness and A safety
// attacker ids, numbered like a scenario but never tagged in
// features/.
const attackerLetters = "FA"

// rowID is the shape every id cell in a matrix table takes: a
// scenarioLetters id, or an attackerLetters row, so numbered.
var rowID = regexp.MustCompile(`^[` + scenarioLetters + attackerLetters + `]` + idNumber + `$`)

// MatrixIDs reads the S- and C-scenario ids off the matrix tables in
// path — the tables whose header leads with "#", the shape every S,
// F, A and C table shares — keyed by each row's leading cell. Any
// other table, and all prose, is ignored, so a glossary whose first
// column happens to start with "S" is never mistaken for a malformed
// scenario. Within a
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

// collectRowID records a scenarioLetters id a matrix row leads with,
// passes an attackerLetters id by, and reports a leading cell that is
// no id at all.
func collectRowID(path string, row planmeta.TableRow, ids map[string]bool) error {
	first := ""
	if len(row.Cells) > 0 {
		first = row.Cells[0]
	}
	if !rowID.MatchString(first) {
		return fmt.Errorf("scenario: %s:%d: malformed scenario id %q", path, row.Line, first)
	}
	if !strings.ContainsRune(scenarioLetters, rune(first[0])) {
		return nil
	}
	if ids[first] {
		return fmt.Errorf("scenario: %s:%d: duplicate scenario id %q", path, row.Line, first)
	}
	ids[first] = true

	return nil
}

// MatrixIDsAll merges the scenario ids MatrixIDs reads off every
// document in paths, so more than one catalog — the lease protocol's
// and a command's own — can share one bijection gate. An id two
// documents both claim is reported rather than silently kept from
// whichever was read first: two catalogs must never overlap.
func MatrixIDsAll(paths ...string) (map[string]bool, error) {
	ids := map[string]bool{}
	for _, path := range paths {
		got, err := MatrixIDs(path)
		if err != nil {
			return nil, err
		}
		for id := range got {
			if ids[id] {
				return nil, fmt.Errorf(
					"scenario: %s: id %q already claimed by another matrix document", path, id)
			}
			ids[id] = true
		}
	}

	return ids, nil
}
