// Package ask keeps the record that ties a supervisor's question to
// the lane agent's answer.
//
// An ask is a fact local to the host that has the lane: the pane it
// went into and the answer written back live nowhere in the shared
// refs, so the record is a file beside the lane's lease token, not a
// ref. The asker reaches it through the lane's worktree root and the
// responder through its own cwd, and both resolve the same git dir. A
// write here touches no pane, no ref and no network, which is what
// lets the reply be pre-approved.
package ask

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
)

// State is where an ask stands: never made, sent and waiting, or
// answered.
type State string

const (
	// None is a lane nobody asked, or whose record cannot be read.
	None State = "none"
	// Pending is an ask sent and not yet answered.
	Pending State = "pending"
	// Answered is an ask whose answer was recorded.
	Answered State = "answered"
)

// ErrNoPending is why a reply was refused: there is no ask waiting
// for it, either because none was ever made or because the latest was
// already answered.
var ErrNoPending = errors.New("no ask is pending")

// Record is one ask: the question sent and, once given, its answer.
type Record struct {
	Text   string `json:"text"`
	Answer string `json:"answer"`
}

// Path is where a lane keeps its ask for a plan: the token's own
// directory, one file per plan, so a worktree carrying another plan
// later never reads this one's ask.
func Path(lane string, planID int64, run gitwt.Runner) (string, error) {
	token, err := claim.TokenPath(lane, planID, run)
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(token), fmt.Sprintf("ask-%d", planID)), nil
}

// Read reports the lane's ask for a plan. A missing or unreadable
// record is None: absence is the routine case, and a corrupt file is
// no ask rather than a fault a board should stop on.
func Read(lane string, planID int64, run gitwt.Runner) (Record, State) {
	path, err := Path(lane, planID, run)
	if err != nil {
		return Record{}, None
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, None
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, None
	}
	if rec.Answer != "" {
		return rec, Answered
	}

	return rec, Pending
}

// Open records a new pending ask, replacing whatever the lane held
// before: the latest question is the one a reply answers.
func Open(lane string, planID int64, text string, run gitwt.Runner) error {
	return write(lane, planID, Record{Text: text}, run)
}

// Reply stores the answer against the pending ask. It refuses with
// ErrNoPending when nothing waits for one, and never overwrites an
// answer already given.
func Reply(lane string, planID int64, answer string, run gitwt.Runner) error {
	rec, state := Read(lane, planID, run)
	if state != Pending {
		return ErrNoPending
	}
	rec.Answer = answer

	return write(lane, planID, rec, run)
}

// Remove drops the lane's ask for a plan, for a send that failed
// after its record was written. Best effort: a record that cannot be
// removed is a stale pending ask, which the next ask replaces.
func Remove(lane string, planID int64, run gitwt.Runner) {
	path, err := Path(lane, planID, run)
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

// write stores rec atomically — a temp file beside the record, then a
// rename — so a reader never sees a half-written record.
func write(lane string, planID int64, rec Record, run gitwt.Runner) error {
	path, err := Path(lane, planID, run)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	// A struct of two strings always marshals.
	data, _ := json.Marshal(rec)

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)

		return err
	}

	return nil
}
