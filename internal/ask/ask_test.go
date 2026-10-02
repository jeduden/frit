package ask

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoWithLane builds a real repository and one linked worktree beside
// it — the shape an ask meets: the asker stands in the main checkout,
// the responder in the lane.
func repoWithLane(t *testing.T) (main, lane string) {
	t.Helper()
	root := t.TempDir()
	main = filepath.Join(root, "atlas")
	lane = filepath.Join(root, "atlas-7")
	gitRun(t, root, "init", "-q", "-b", "main", main)
	gitRun(t, main, "-c", "user.name=t", "-c", "user.email=t@t",
		"commit", "-q", "--allow-empty", "-m", "init")
	gitRun(t, main, "worktree", "add", "-q", "-b", "plan/7-lane", lane)

	return main, lane
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

var asked = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

// TestPathIsSharedByTheMainCheckoutAndItsLane: the asker in the main
// checkout and the responder in the lane's worktree resolve one file,
// because it sits under the repository's common git dir rather than
// either checkout's own.
func TestPathIsSharedByTheMainCheckoutAndItsLane(t *testing.T) {
	main, lane := repoWithLane(t)

	fromMain, err := Path(main, 7, gitwt.Exec)
	require.NoError(t, err)
	fromLane, err := Path(lane, 7, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, fromMain, fromLane)
	common, err := gitwt.CommonDir(main, gitwt.Exec)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(common, "frit", "ask-7.json"), fromMain)
}

// TestPathFailsOutsideARepository: a directory git cannot place has no
// record to name.
func TestPathFailsOutsideARepository(t *testing.T) {
	_, err := Path(t.TempDir(), 7, gitwt.Exec)

	assert.Error(t, err)
}

// TestReadAnAbsentRecordIsNotAFault: no record is the "none" state,
// not an error.
func TestReadAnAbsentRecordIsNotAFault(t *testing.T) {
	_, ok, err := Read(filepath.Join(t.TempDir(), "ask-7.json"))

	require.NoError(t, err)
	assert.False(t, ok)
}

// TestReadReportsAnUnparsableRecord: a torn or foreign file is a
// fault the caller hears about, never a silent "none".
func TestReadReportsAnUnparsableRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ask-7.json")
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

	_, _, err := Read(path)

	assert.Error(t, err)
}

// TestReadReportsAnUnreadableRecord: a path that exists but cannot be
// read as a file is a fault, distinct from an absent one.
func TestReadReportsAnUnreadableRecord(t *testing.T) {
	_, _, err := Read(t.TempDir())

	assert.Error(t, err)
}

// TestWriteThenReadRoundTrips: a record written is read back whole,
// and the write creates the directory it lands in.
func TestWriteThenReadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frit", "ask-7.json")
	want := Record{Question: "status?", AskedAt: asked}

	require.NoError(t, Write(path, want))
	got, ok, err := Read(path)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, want.Question, got.Question)
	assert.True(t, want.AskedAt.Equal(got.AskedAt))
}

// TestWriteLeavesNoTempFileBehind: the write is a temp file renamed
// into place, so a reader never sees half a record and the directory
// carries only the record afterward.
func TestWriteLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, Write(filepath.Join(dir, "ask-7.json"), Record{Question: "q"}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "ask-7.json", entries[0].Name())
}

// TestWriteFailsWhereNoDirectoryCanBeMade: a plain file where the
// record's directory would go is a write failure handed back.
func TestWriteFailsWhereNoDirectoryCanBeMade(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "frit")
	require.NoError(t, os.WriteFile(blocker, nil, 0o600))

	err := Write(filepath.Join(blocker, "ask-7.json"), Record{})

	assert.Error(t, err)
}

// TestWriteFailsWhenTheRenameIsRefused: a directory standing where the
// record goes refuses the rename, and the temp file is cleaned up.
func TestWriteFailsWhenTheRenameIsRefused(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ask-7.json")
	require.NoError(t, os.MkdirAll(filepath.Join(target, "x"), 0o750))

	err := Write(target, Record{})

	assert.Error(t, err)
	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	assert.Len(t, entries, 1, "the temp file does not linger")
}

// TestWriteFailsWhenTheTempFileCannotBeWritten: a directory standing
// where the staged record goes refuses the write, and nothing lands.
func TestWriteFailsWhenTheTempFileCannotBeWritten(t *testing.T) {
	target := filepath.Join(t.TempDir(), "ask-7.json")
	require.NoError(t, os.MkdirAll(tempPath(target), 0o750))

	err := Write(target, Record{})

	assert.Error(t, err)
	_, ok, readErr := Read(target)
	require.NoError(t, readErr)
	assert.False(t, ok)
}

// TestWriteFailsOnARecordJSONCannotCarry: a time outside JSON's range
// is refused before anything touches disk.
func TestWriteFailsOnARecordJSONCannotCarry(t *testing.T) {
	target := filepath.Join(t.TempDir(), "ask-7.json")

	err := Write(target, Record{AskedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)})

	assert.Error(t, err)
}

// TestTempPathSitsBesideTheRecord: staged beside the record, so the
// rename stays on one filesystem, and named per process.
func TestTempPathSitsBesideTheRecord(t *testing.T) {
	got := tempPath("/x/frit/ask-7.json")

	assert.Equal(t, "/x/frit", filepath.Dir(got))
	assert.Contains(t, got, strconv.Itoa(os.Getpid()))
}

// TestStateOfNamesEachShape: no record is none, a record with no
// answer is pending, and one carrying an answer is answered.
func TestStateOfNamesEachShape(t *testing.T) {
	assert.Equal(t, StateNone, StateOf(Record{}, false))
	assert.Equal(t, StatePending, StateOf(Record{Question: "q"}, true))
	assert.Equal(t, StateAnswered,
		StateOf(Record{Question: "q", Answer: "in PR #9"}, true))
}

// TestPoseRecordsAPendingAskFromTheMainCheckout: the asker's write is
// a pending record the lane can read back.
func TestPoseRecordsAPendingAskFromTheMainCheckout(t *testing.T) {
	main, lane := repoWithLane(t)

	path, err := Pose(main, 7, "status?", asked, gitwt.Exec)
	require.NoError(t, err)

	fromLane, err := Path(lane, 7, gitwt.Exec)
	require.NoError(t, err)
	assert.Equal(t, fromLane, path)
	rec, ok, err := Read(path)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, StatePending, StateOf(rec, ok))
	assert.Equal(t, "status?", rec.Question)
}

// TestPoseReplacesAnEarlierAsk: a fresh ask supersedes the last one,
// answered or not, so a reply always answers the latest question.
func TestPoseReplacesAnEarlierAsk(t *testing.T) {
	main, lane := repoWithLane(t)
	_, err := Pose(main, 7, "first?", asked, gitwt.Exec)
	require.NoError(t, err)
	_, err = Answer(lane, 7, "first answer", asked, gitwt.Exec)
	require.NoError(t, err)

	path, err := Pose(main, 7, "second?", asked, gitwt.Exec)
	require.NoError(t, err)

	rec, ok, err := Read(path)
	require.NoError(t, err)
	assert.Equal(t, StatePending, StateOf(rec, ok))
	assert.Equal(t, "second?", rec.Question)
	assert.Empty(t, rec.Answer)
}

// TestPoseFailsOutsideARepository: no repository, no record.
func TestPoseFailsOutsideARepository(t *testing.T) {
	_, err := Pose(t.TempDir(), 7, "status?", asked, gitwt.Exec)

	assert.Error(t, err)
}

// TestAnswerRecordsTheReplyFromTheLane: the responder's write lands on
// the record the asker posed, and reads back answered.
func TestAnswerRecordsTheReplyFromTheLane(t *testing.T) {
	main, lane := repoWithLane(t)
	_, err := Pose(main, 7, "status?", asked, gitwt.Exec)
	require.NoError(t, err)
	answered := asked.Add(time.Minute)

	rec, err := Answer(lane, 7, "in PR #9", answered, gitwt.Exec)

	require.NoError(t, err)
	assert.Equal(t, "status?", rec.Question)
	assert.Equal(t, "in PR #9", rec.Answer)
	path, err := Path(main, 7, gitwt.Exec)
	require.NoError(t, err)
	got, ok, err := Read(path)
	require.NoError(t, err)
	assert.Equal(t, StateAnswered, StateOf(got, ok))
	assert.True(t, answered.Equal(got.AnsweredAt))
}

// TestAnswerRefusesWithNoAskPending: nothing asked, nothing to answer.
func TestAnswerRefusesWithNoAskPending(t *testing.T) {
	_, lane := repoWithLane(t)

	_, err := Answer(lane, 7, "in PR #9", asked, gitwt.Exec)

	assert.ErrorIs(t, err, ErrNoPending)
}

// TestAnswerRefusesAnAlreadyAnsweredAsk: an answered ask is not
// pending, so a second reply is refused rather than overwriting the
// answer the asker may already have read.
func TestAnswerRefusesAnAlreadyAnsweredAsk(t *testing.T) {
	main, lane := repoWithLane(t)
	_, err := Pose(main, 7, "status?", asked, gitwt.Exec)
	require.NoError(t, err)
	_, err = Answer(lane, 7, "first", asked, gitwt.Exec)
	require.NoError(t, err)

	_, err = Answer(lane, 7, "second", asked, gitwt.Exec)

	assert.ErrorIs(t, err, ErrNoPending)
}

// TestAnswerFailsOutsideARepository: no repository, no record to
// answer — a fault, not "no ask pending".
func TestAnswerFailsOutsideARepository(t *testing.T) {
	_, err := Answer(t.TempDir(), 7, "x", asked, gitwt.Exec)

	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoPending)
}

// TestAnswerSurfacesAnUnreadableRecord: a torn record is a fault the
// responder hears about, not "no ask pending".
func TestAnswerSurfacesAnUnreadableRecord(t *testing.T) {
	_, lane := repoWithLane(t)
	path, err := Path(lane, 7, gitwt.Exec)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("{torn"), 0o600))

	_, err = Answer(lane, 7, "x", asked, gitwt.Exec)

	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoPending)
}

// TestWithdrawRemovesThePosedRecord: an ask whose send failed is taken
// back, so the lane is not left owing a reply to a question it never
// saw.
func TestWithdrawRemovesThePosedRecord(t *testing.T) {
	main, _ := repoWithLane(t)
	path, err := Pose(main, 7, "status?", asked, gitwt.Exec)
	require.NoError(t, err)

	Withdraw(path)

	_, ok, err := Read(path)
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestEnvelopeAsksForAReplyAndNamesBothWaysToGiveIt: the envelope
// carries the operator's text, says a reply is wanted, names the
// plan-reply skill, and gives the raw reply command for a repository
// without the bundle — all on one line, so a pane that submits on a
// newline still receives it whole.
func TestEnvelopeAsksForAReplyAndNamesBothWaysToGiveIt(t *testing.T) {
	got := Envelope("are you in a PR?")

	assert.True(t, strings.HasPrefix(got, "are you in a PR?"), got)
	assert.Contains(t, got, "reply is wanted")
	assert.Contains(t, got, "plan-reply")
	assert.Contains(t, got, `frit reply "<answer>"`)
	assert.NotContains(t, got, "\n")
}
