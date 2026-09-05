package claim

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeduden/frit/internal/gitwt"
)

// tokenDir is the directory a lane's tokens live under, inside its own
// git dir.
const tokenDir = "frit"

// TokenPath is where a lane records its lease token for a plan: one
// file per plan inside the lane's own git directory, because a
// worktree can carry a different plan over its lifetime and one file
// per plan needs no parser to keep them apart.
func TokenPath(laneDir string, planID int64, run gitwt.Runner) (string, error) {
	gitDir, err := gitwt.GitDir(laneDir, run)
	if err != nil {
		return "", err
	}

	return filepath.Join(gitDir, tokenDir, fmt.Sprintf("token-%d", planID)), nil
}

// ReadToken reads the token a lane recorded for a plan, "" when it
// keeps none — never held, or laneDir is not a worktree at all.
// Absence is not a fault: it only costs this lane the resume shortcut
// (A1).
func ReadToken(laneDir string, planID int64, run gitwt.Runner) string {
	path, err := TokenPath(laneDir, planID, run)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// WriteToken records the tip a winning transition left a lane's lease
// at, so the token survives the process (F9, S3). An empty or unbound
// lane, or an empty tip, is skipped in silence — that shape is
// routine: start acquires the lease before herdr creates the
// worktree, and claim mints its lease before herdr stands the
// worktree up too, so lane often names a path with nothing on disk
// yet at that point. Both verbs call this again once herdr reports
// the worktree stood up, so neither one's token depends on a later
// renewal to exist at all.
//
// A non-nil error here means the lane's checkout already exists and
// the write still failed — worth a caller's warning, unlike the
// routine skip above. Either way it costs the lane only its resume
// shortcut: the lease itself is already on the remote, and the CAS,
// not the file, is the fence.
func WriteToken(lane string, planID int64, tip string, run gitwt.Runner) error {
	if lane == "" || lane == "-" || tip == "" {
		return nil
	}
	path, err := TokenPath(lane, planID, run)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(tip+"\n"), 0o600)
}

// persistToken is WriteToken read off a lease's own options — the form
// every transition in this file already holds. Silent on failure, like
// WriteToken's own routine skip: none of these callers stand a
// worktree up themselves, so none has a report to warn on.
func persistToken(opts LeaseOptions, tip string, run gitwt.Runner) {
	_ = WriteToken(opts.Lane, opts.PlanID, tip, run)
}
