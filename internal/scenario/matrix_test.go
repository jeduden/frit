package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeMatrix(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "matrix.md")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

// TestMatrixIDsReadsOnlyTheIDTables: S ids come off tables headed by
// "#". An F attacker row in such a table is passed by; a table under
// any other header is not a matrix table even when its first column
// starts with "S"; a pipe row quoted in a fenced code block is not a
// row; and a bare id in prose is never counted.
func TestMatrixIDsReadsOnlyTheIDTables(t *testing.T) {
	path := writeMatrix(t, ""+
		"| #  | Scenario | Outcome |\n"+
		"| -- | -------- | ------- |\n"+
		"| S1 | first    | as S9, never actually a row |\n"+
		"| S2 | second   | see S1 |\n"+
		"| F1 | attacker | not a scenario |\n\n"+
		"| State | Meaning |\n"+
		"| ----- | ------- |\n"+
		"| SCAV  | a glossary row, not an id |\n\n"+
		"```\n| #  | Scenario |\n| -- | -------- |\n| S9 | quoted   |\n```\n\n"+
		"S9 mentioned only in prose, never a table row.\n")

	ids, err := MatrixIDs(path)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"S1": true, "S2": true}, ids)
}

// TestMatrixIDsFailsOnARowWithoutACleanID: inside an id table every
// row must lead with S, F or A and a number. A lowercase id, a
// suffixed one, a leading zero, and an id shifted out of the first
// column are each reported with the row's line, never dropped.
func TestMatrixIDsFailsOnARowWithoutACleanID(t *testing.T) {
	for _, bad := range []string{"s3", "S3a", "S03", ""} {
		path := writeMatrix(t, "| #  | Scenario |\n| -- | -------- |\n| "+bad+" | row |\n")

		_, err := MatrixIDs(path)
		require.Error(t, err, bad)
		assert.Contains(t, err.Error(), ":3:", bad)
		assert.Contains(t, err.Error(), fmt.Sprintf("%q", bad), bad)
	}
}

// TestMatrixIDsFailsOnDuplicateID: the same id on two rows is reported
// rather than silently collapsed into one, since a set would otherwise
// hide that one of the two rows has no scenario of its own.
func TestMatrixIDsFailsOnDuplicateID(t *testing.T) {
	path := writeMatrix(t, ""+
		"| #  | Scenario |\n"+
		"| -- | -------- |\n"+
		"| S1 | first    |\n"+
		"| S1 | second   |\n")

	_, err := MatrixIDs(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ":4:")
	assert.Contains(t, err.Error(), "S1")
}

// TestMatrixIDsRejectsAMissingFile surfaces the open error rather than
// reporting an empty matrix.
func TestMatrixIDsRejectsAMissingFile(t *testing.T) {
	_, err := MatrixIDs(filepath.Join(t.TempDir(), "missing.md"))
	require.Error(t, err)
}

// TestCollectRowIDKeepsSAndPassesAttackersBy pins the three answers a
// row can get: an S id is recorded, an F or A id is passed by without
// being recorded, and an empty cell is malformed.
func TestCollectRowIDKeepsSAndPassesAttackersBy(t *testing.T) {
	ids := map[string]bool{}
	require.NoError(t, collectRowID("m.md", planmeta.TableRow{Line: 3, Cells: []string{"S7"}}, ids))
	require.NoError(t, collectRowID("m.md", planmeta.TableRow{Line: 4, Cells: []string{"F2"}}, ids))
	require.NoError(t, collectRowID("m.md", planmeta.TableRow{Line: 5, Cells: []string{"A1"}}, ids))
	assert.Equal(t, map[string]bool{"S7": true}, ids)

	err := collectRowID("m.md", planmeta.TableRow{Line: 6}, ids)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "m.md:6:")
}
