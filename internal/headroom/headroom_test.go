package headroom

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeduden/mdsmith/pkg/mdsmith"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSession builds a session against a default config — max-file-length
// keeps its built-in cap of 300 lines, which is all the oracle needs.
func newSession(t *testing.T) *mdsmith.Session {
	t.Helper()
	sess, err := mdsmith.NewSession(mdsmith.SessionOptions{
		Workspace: mdsmith.NewMemWorkspace(nil),
		Config:    mdsmith.ConfigYAML(""),
	})
	require.NoError(t, err)

	return sess
}

// plan renders a minimal front-matter block followed by n body lines, so
// the front matter never counts toward ReserveLines but always counts
// toward mdsmith's own line cap.
func plan(bodyLines int) []byte {
	var b strings.Builder
	b.WriteString("---\nid: 1\ntitle: x\n---\n")
	for range bodyLines {
		b.WriteString("body line\n")
	}

	return []byte(b.String())
}

func TestReserveLinesIsCeilOfPercentOfBodyLinesOnly(t *testing.T) {
	src := plan(50)

	assert.Equal(t, 5, ReserveLines(src, 10))
	assert.Equal(t, 15, ReserveLines(src, 30))
	assert.Equal(t, 0, ReserveLines(src, 0))
}

func TestRoomWithPlentyOfRoomReturnsTheFullReserve(t *testing.T) {
	sess := newSession(t)
	src := plan(50)
	reserve := ReserveLines(src, 10)
	require.Equal(t, 5, reserve)

	room, err := Room(sess, "plan/1_x.md", src, reserve)

	require.NoError(t, err)
	assert.Equal(t, reserve, room, "50-line body plus 5 lines of padding sits well under the 300-line cap")
}

func TestRoomPaddedToTheCapReturnsLessThanTheReserve(t *testing.T) {
	sess := newSession(t)
	// max-file-length counts body lines only — mdsmith strips front
	// matter before the rule ever sees the file, so 290 body lines
	// leaves only 10 lines of headroom under the 300-line cap.
	src := plan(290)
	reserve := ReserveLines(src, 10)
	require.Equal(t, 29, reserve)

	room, err := Room(sess, "plan/1_x.md", src, reserve)

	require.NoError(t, err)
	assert.Less(t, room, reserve,
		"290 body lines plus the full 29-line reserve would exceed the 300-line cap")
	assert.Equal(t, 10, room, "300 - 290 lines of headroom actually fit")
}

// TestSessionUsesTheRepositoryOwnMdsmithYML pins that a repository's own
// .mdsmith.yml is honored — a custom max-file-length would otherwise be
// silently ignored by the oracle.
func TestSessionUsesTheRepositoryOwnMdsmithYML(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".mdsmith.yml"),
		[]byte("rules:\n  max-file-length:\n    max: 5\n"), 0o600))

	sess, err := Session(root)
	require.NoError(t, err)

	room, err := Room(sess, "plan/1_x.md", plan(3), 10)

	require.NoError(t, err)
	assert.Equal(t, 2, room, "3 body lines already sit 2 short of the 5-line cap")
}

// TestSessionToleratesAMissingMdsmithYML pins that a repository with no
// .mdsmith.yml still opens a session — mdsmith itself runs on its own
// built-in defaults with no config file at all, and the oracle must not
// refuse to open just because a repository indexed by frit ships none.
func TestSessionToleratesAMissingMdsmithYML(t *testing.T) {
	root := t.TempDir()

	sess, err := Session(root)
	require.NoError(t, err)

	room, err := Room(sess, "plan/1_x.md", plan(50), 5)

	require.NoError(t, err)
	assert.Equal(t, 5, room, "the built-in 300-line default leaves plenty of room")
}

// TestPadInsertsANewlineBeforePaddingAnUnterminatedSource is Phase 6's
// RED for pad's trailing-newline branch: without the inserted newline
// the first pad line would merge into source's own last line, one line
// short of what n claims.
func TestPadInsertsANewlineBeforePaddingAnUnterminatedSource(t *testing.T) {
	src := []byte("line one\nline two") // no trailing newline

	got := pad(src, 2)

	want := "line one\nline two\n" +
		"<!-- headroom padding -->\n<!-- headroom padding -->\n"
	assert.Equal(t, want, string(got))
}

// TestPadDoesNotDoubleTheBlankOnAnAlreadyTerminatedSource pins the
// other side of the same branch: a source already ending in a newline
// gets no inserted blank, so it pads identically to the unterminated
// case above once normalized.
func TestPadDoesNotDoubleTheBlankOnAnAlreadyTerminatedSource(t *testing.T) {
	src := []byte("line one\nline two\n")

	got := pad(src, 2)

	want := "line one\nline two\n" +
		"<!-- headroom padding -->\n<!-- headroom padding -->\n"
	assert.Equal(t, want, string(got))
}

// TestPadOnEmptySourceAddsNoLeadingNewline pins the guard against an
// empty source: len(source) > 0 must be checked before indexing its
// last byte, or an empty plan file would panic pad rather than just
// receiving its n padding lines.
func TestPadOnEmptySourceAddsNoLeadingNewline(t *testing.T) {
	got := pad(nil, 2)

	want := "<!-- headroom padding -->\n<!-- headroom padding -->\n"
	assert.Equal(t, want, string(got))
}

// TestFitsReportsWhetherThePaddedSourceStillPassesTheCap is Phase 6's
// RED for fits: the same 290-body-line plan TestRoomPaddedToTheCap...
// already pins at a 10-line room, checked directly here at the
// boundary rather than through Room's search.
func TestFitsReportsWhetherThePaddedSourceStillPassesTheCap(t *testing.T) {
	sess := newSession(t)
	src := plan(290)

	ok, err := fits(sess, "plan/1_x.md", src, 10)
	require.NoError(t, err)
	assert.True(t, ok, "290 body lines + 10 padding lines sits right at the 300-line cap")

	ok, err = fits(sess, "plan/1_x.md", src, 11)
	require.NoError(t, err)
	assert.False(t, ok, "290 body lines + 11 padding lines trips the 300-line cap")
}
