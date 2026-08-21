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

// persistToken records the tip a winning transition left the lane's
// lease at, so the token survives the process (F9, S3).
//
// Best-effort, like syncLocalRef: start acquires the lease before
// herdr creates the worktree, so opts.Lane routinely names a path with
// nothing on disk yet, and claim stands its worktree up after minting
// too. A token that could not be written costs the lane only its
// resume shortcut — the lease itself is already on the remote, and the
// CAS, not the file, is the fence.
func persistToken(opts LeaseOptions, tip string, run gitwt.Runner) {
	if opts.Lane == "" || opts.Lane == "-" || tip == "" {
		return
	}
	path, err := TokenPath(opts.Lane, opts.PlanID, run)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(tip+"\n"), 0o600)
}
