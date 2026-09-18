package ask

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lane builds a real repository with a linked worktree and returns
// both checkouts, so a test proves the asker in the main checkout and
// the responder in the lane's worktree meet on one file.
func lane(t *testing.T) (main, linked string) {
	t.Helper()
	main = filepath.Join(t.TempDir(), "atlas")
	require.NoError(t, os.MkdirAll(main, 0o750))
	gitIn(t, main, "init", "-q", "-b", "main")
	gitIn(t, main, "config", "user.email", "t@example.com")
	gitIn(t, main, "config", "user.name", "t")
	gitIn(t, main, "commit", "--allow-empty", "-q", "-m", "initial")
	linked = filepath.Join(t.TempDir(), "lane")
	gitIn(t, main, "worktree", "add", "-q", "-b", "plan/7-x", linked)

	return main, linked
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir,
		"-c", "commit.gpgsign=false"}, args...)...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

func TestPathSitsBesideTheLaneToken(t *testing.T) {
	_, linked := lane(t)

	tok, err := claim.TokenPath(linked, 7, gitwt.Exec)
	require.NoError(t, err)
	got, err := Path(linked, 7, gitwt.Exec)

	require.NoError(t, err)
	assert.Equal(t, filepath.Dir(tok), filepath.Dir(got),
		"the record shares the token's directory")
	assert.Equal(t, "ask-7", filepath.Base(got))
}

func TestPathFailsOutsideARepository(t *testing.T) {
	_, err := Path(t.TempDir(), 7, gitwt.Exec)

	assert.Error(t, err)
}

func TestReadIsNoneWhenNothingWasAsked(t *testing.T) {
	_, linked := lane(t)

	rec, state := Read(linked, 7, gitwt.Exec)

	assert.Equal(t, None, state)
	assert.Equal(t, Record{}, rec)
}

func TestOpenRecordsAPendingAsk(t *testing.T) {
	_, linked := lane(t)

	require.NoError(t, Open(linked, 7, "are you in a PR?", gitwt.Exec))
	rec, state := Read(linked, 7, gitwt.Exec)

	assert.Equal(t, Pending, state)
	assert.Equal(t, "are you in a PR?", rec.Text)
	assert.Empty(t, rec.Answer)
}

func TestReplyAnswersThePendingAsk(t *testing.T) {
	_, linked := lane(t)
	require.NoError(t, Open(linked, 7, "status?", gitwt.Exec))

	require.NoError(t, Reply(linked, 7, "in a PR, merging", gitwt.Exec))
	rec, state := Read(linked, 7, gitwt.Exec)

	assert.Equal(t, Answered, state)
	assert.Equal(t, "status?", rec.Text, "the question stays beside its answer")
	assert.Equal(t, "in a PR, merging", rec.Answer)
}

func TestReplyRefusesWithNoAskPending(t *testing.T) {
	_, linked := lane(t)

	err := Reply(linked, 7, "hello", gitwt.Exec)

	assert.ErrorIs(t, err, ErrNoPending)
}

func TestReplyRefusesAnAlreadyAnsweredAsk(t *testing.T) {
	_, linked := lane(t)
	require.NoError(t, Open(linked, 7, "status?", gitwt.Exec))
	require.NoError(t, Reply(linked, 7, "first", gitwt.Exec))

	err := Reply(linked, 7, "second", gitwt.Exec)

	assert.ErrorIs(t, err, ErrNoPending)
	rec, _ := Read(linked, 7, gitwt.Exec)
	assert.Equal(t, "first", rec.Answer, "an answer is never overwritten")
}

func TestOpenReplacesAnAnsweredAsk(t *testing.T) {
	_, linked := lane(t)
	require.NoError(t, Open(linked, 7, "one", gitwt.Exec))
	require.NoError(t, Reply(linked, 7, "yes", gitwt.Exec))

	require.NoError(t, Open(linked, 7, "two", gitwt.Exec))
	rec, state := Read(linked, 7, gitwt.Exec)

	assert.Equal(t, Pending, state, "a new ask supersedes the old answer")
	assert.Equal(t, "two", rec.Text)
}

func TestAskerAndResponderShareOneFile(t *testing.T) {
	main, linked := lane(t)
	require.NoError(t, Open(linked, 7, "status?", gitwt.Exec))

	// The asker names the lane by its worktree root; a responder
	// standing in a subdirectory of that worktree reaches the same file.
	sub := filepath.Join(linked, "sub")
	require.NoError(t, os.MkdirAll(sub, 0o750))
	require.NoError(t, Reply(sub, 7, "answered", gitwt.Exec))

	_, state := Read(linked, 7, gitwt.Exec)
	assert.Equal(t, Answered, state)
	_, mainState := Read(main, 7, gitwt.Exec)
	assert.Equal(t, None, mainState,
		"the main checkout keeps its own git dir, not the lane's")
}

func TestReadIsNoneForACorruptRecord(t *testing.T) {
	_, linked := lane(t)
	path, err := Path(linked, 7, gitwt.Exec)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

	_, state := Read(linked, 7, gitwt.Exec)

	assert.Equal(t, None, state, "an unreadable record is no ask, not a fault")
}

func TestRemoveDropsTheRecord(t *testing.T) {
	_, linked := lane(t)
	require.NoError(t, Open(linked, 7, "status?", gitwt.Exec))

	Remove(linked, 7, gitwt.Exec)

	_, state := Read(linked, 7, gitwt.Exec)
	assert.Equal(t, None, state)
}

func TestOpenLeavesNoTempFileBehind(t *testing.T) {
	_, linked := lane(t)
	require.NoError(t, Open(linked, 7, "status?", gitwt.Exec))
	path, err := Path(linked, 7, gitwt.Exec)
	require.NoError(t, err)

	assert.NoFileExists(t, path+".tmp", "the atomic write renames its temp file away")
}

func TestOpenFailsWhenTheRecordDirCannotBeMade(t *testing.T) {
	_, linked := lane(t)
	path, err := Path(linked, 7, gitwt.Exec)
	require.NoError(t, err)
	// A file where the frit directory belongs blocks the mkdir.
	require.NoError(t, os.WriteFile(filepath.Dir(path), []byte("x"), 0o600))

	assert.Error(t, Open(linked, 7, "status?", gitwt.Exec))
}

func TestOpenFailsWhenTheTempFileCannotBeWritten(t *testing.T) {
	_, linked := lane(t)
	path, err := Path(linked, 7, gitwt.Exec)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(path+".tmp", 0o750))

	assert.Error(t, Open(linked, 7, "status?", gitwt.Exec))
}

func TestOpenFailsAndCleansUpWhenTheRenameFails(t *testing.T) {
	_, linked := lane(t)
	path, err := Path(linked, 7, gitwt.Exec)
	require.NoError(t, err)
	// A non-empty directory where the record belongs blocks the rename.
	require.NoError(t, os.MkdirAll(filepath.Join(path, "keep"), 0o750))

	assert.Error(t, Open(linked, 7, "status?", gitwt.Exec))
	assert.NoFileExists(t, path+".tmp", "a failed write leaves no temp file")
}

func TestOpenFailsOutsideARepository(t *testing.T) {
	assert.Error(t, Open(t.TempDir(), 7, "status?", gitwt.Exec))
}

func TestRemoveIsQuietOutsideARepository(t *testing.T) {
	assert.NotPanics(t, func() { Remove(t.TempDir(), 7, gitwt.Exec) })
}
