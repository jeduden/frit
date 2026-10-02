// Package ask keeps the record that lets a supervisor's question to a
// lane's agent carry its answer back.
//
// `frit message --ask` poses a question and `frit reply` answers it.
// The two meet on one small file under the repository's common git
// dir, so the asker in the main checkout and the responder in the
// lane's worktree read and write the same record. An answer is a local
// fact, like a live pane: it never reaches origin, and writing it
// touches no pane, no ref and no network — which is what makes a reply
// safe to pre-approve.
package ask

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/jeduden/frit/internal/gitwt"
)

// ErrNoPending reports that a plan has no ask awaiting an answer:
// nothing was asked, or the last ask is already answered.
var ErrNoPending = errors.New("no ask is pending")

// The three states an ask reads as, on the board and in --json.
const (
	StateNone     = "none"
	StatePending  = "pending"
	StateAnswered = "answered"
)

// dir is the directory the records live under, inside the common git
// dir — the same name the lease token's own directory uses.
const dir = "frit"

// Record is one plan's latest ask and, once given, its answer.
type Record struct {
	Question   string    `json:"question"`
	AskedAt    time.Time `json:"asked_at"`
	Answer     string    `json:"answer"`
	AnsweredAt time.Time `json:"answered_at"`
}

// Path is where a repository keeps the ask record for a plan: one file
// per plan under the common git dir, so every checkout of the
// repository — the main one and each linked lane — names the same
// file. A lane in a separate clone has its own common dir, so an ask
// and its reply meet only through worktrees of one clone.
func Path(checkout string, planID int64, run gitwt.Runner) (string, error) {
	d, err := Dir(checkout, run)
	if err != nil {
		return "", err
	}

	return File(d, planID), nil
}

// Dir is the directory a repository keeps its ask records in, found
// once with one git call — a caller reading many plans of one
// repository finds it once and names each record with File.
func Dir(checkout string, run gitwt.Runner) (string, error) {
	common, err := gitwt.CommonDir(checkout, run)
	if err != nil {
		return "", err
	}

	return filepath.Join(common, dir), nil
}

// File names a plan's record inside a repository's ask directory.
func File(askDir string, planID int64) string {
	return filepath.Join(askDir, fmt.Sprintf("ask-%d.json", planID))
}

// Read reads the record at path. ok is false, with no error, when no
// ask was ever recorded; a file that cannot be read or parsed is an
// error, never a silent "none".
func Read(path string) (rec Record, ok bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, false, fmt.Errorf("%s: %w", path, err)
	}

	return rec, true, nil
}

// Write stores rec at path, creating its directory. The write is a
// temp file renamed into place, so a reader on the other side of the
// ask sees the old record or the new one, never half of one. The temp
// name carries the process id, so two frit runs never share one.
func Write(path string, rec Record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp := tempPath(path)
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	return nil
}

// tempPath is the name Write stages a record under before renaming it
// into place: beside it, so the rename never crosses a filesystem.
func tempPath(path string) string {
	return fmt.Sprintf("%s.%d.tmp", path, os.Getpid())
}

// StateOf names a record's state: none when there is no record,
// pending until it carries an answer, answered after.
func StateOf(rec Record, ok bool) string {
	switch {
	case !ok:
		return StateNone
	case rec.Answer == "":
		return StatePending
	default:
		return StateAnswered
	}
}

// Pose records a fresh, pending ask for a plan from any checkout of
// its repository, replacing whatever was asked before, and returns the
// record's path.
func Pose(
	checkout string, planID int64, question string, now time.Time,
	run gitwt.Runner,
) (string, error) {
	path, err := Path(checkout, planID, run)
	if err != nil {
		return "", err
	}

	return path, Write(path, Record{Question: question, AskedAt: now})
}

// Answer records answer against a plan's pending ask and returns the
// answered record. It refuses with ErrNoPending when nothing was asked
// or the ask is already answered, so a second reply never overwrites
// an answer the asker may already have read.
func Answer(
	checkout string, planID int64, answer string, now time.Time,
	run gitwt.Runner,
) (Record, error) {
	path, err := Path(checkout, planID, run)
	if err != nil {
		return Record{}, err
	}
	rec, ok, err := Read(path)
	if err != nil {
		return Record{}, err
	}
	if StateOf(rec, ok) != StatePending {
		return Record{}, ErrNoPending
	}
	rec.Answer = answer
	rec.AnsweredAt = now

	return rec, Write(path, rec)
}

// Clear removes a plan's ask, answered or not, when its lane ends —
// released, yielded, or replaced by a fresh claim — so no later lane
// inherits a question it never saw. No ask is fine; a record that will
// not go is an error, so a stale ask is never left silently.
func Clear(checkout string, planID int64, run gitwt.Runner) error {
	path, err := Path(checkout, planID, run)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}

// Withdraw removes a posed ask whose question never reached the lane,
// so the lane is not left owing a reply. A record already gone is
// fine; there is nothing to report to a caller already failing.
func Withdraw(path string) {
	_ = os.Remove(path)
}

// Envelope wraps a supervisor's text so the agent that reads it knows
// a reply is wanted and how to give one: the plan-reply skill, whose
// approval covers the reply, or the raw command for a repository
// without the bundle. It stays on one line, so a pane that submits on
// a newline still receives it whole.
func Envelope(text string) string {
	return text + " — frit: a reply is wanted. " +
		`Load the plan-reply skill to answer, or run: frit reply "<answer>"`
}
