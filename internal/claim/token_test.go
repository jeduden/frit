package claim

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenPathIsBesideTheGitDir: the token file lives under the
// worktree's own git dir, keyed by plan id so one worktree can carry
// tokens for more than one plan over its lifetime without a parser to
// keep them apart.
func TestTokenPathIsBesideTheGitDir(t *testing.T) {
	repo := originAndClone(t)

	path, err := TokenPath(repo, 7, gitwt.Exec)

	require.NoError(t, err)
	gitDir, err := gitwt.GitDir(repo, gitwt.Exec)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(gitDir, "frit", "token-7"), path)
}

// TestPersistThenReadTokenRoundTrips: a token written for a lane is
// read back verbatim.
func TestPersistThenReadTokenRoundTrips(t *testing.T) {
	repo := originAndClone(t)
	opts := leaseOptions("box-a", repo)

	persistToken(opts, "abc123", gitwt.Exec)

	assert.Equal(t, "abc123", ReadToken(repo, opts.PlanID, gitwt.Exec))
}

// TestWriteTokenThenReadTokenRoundTrips: WriteToken is the exported
// call a lane-standing verb makes directly, off a lane path and a plan
// id alone — no LeaseOptions in hand yet at the moment herdr reports
// the worktree stood up.
func TestWriteTokenThenReadTokenRoundTrips(t *testing.T) {
	repo := originAndClone(t)

	err := WriteToken(repo, 7, "abc123", gitwt.Exec)

	require.NoError(t, err)
	assert.Equal(t, "abc123", ReadToken(repo, 7, gitwt.Exec))
}

// TestWriteTokenReturnsTheWriteFailure: a lane whose git dir already
// carries a plain file where the token directory would go cannot be
// written into — a caller that just stood the worktree up itself, not
// the routine "nothing on disk yet" skip, gets the failure back to
// warn on rather than a silent no-op.
func TestWriteTokenReturnsTheWriteFailure(t *testing.T) {
	repo := originAndClone(t)
	gitDir, err := gitwt.GitDir(repo, gitwt.Exec)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, tokenDir), []byte("blocked"), 0o600))

	err = WriteToken(repo, 7, "abc123", gitwt.Exec)

	assert.Error(t, err)
	assert.Equal(t, "", ReadToken(repo, 7, gitwt.Exec))
}

// TestTokenIsBestEffortOutsideAWorktree: a lane path that resolves to
// no git dir at all — the caller passed a bare directory, or the
// worktree does not exist on disk yet — writes nothing and never
// panics. Losing the shortcut only costs the resume path, never the
// lease itself.
func TestTokenIsBestEffortOutsideAWorktree(t *testing.T) {
	bare := t.TempDir()
	opts := leaseOptions("box-a", bare)

	persistToken(opts, "abc123", gitwt.Exec)

	assert.Equal(t, "", ReadToken(bare, opts.PlanID, gitwt.Exec))
	entries, err := os.ReadDir(bare)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing was written outside a worktree")
}

// TestPersistTokenSkipsAnEmptyOrUnboundLane: the lane path is "" or
// "-" for a lease minted with no worktree of its own yet, and an empty
// tip carries nothing worth remembering.
func TestPersistTokenSkipsAnEmptyOrUnboundLane(t *testing.T) {
	persistToken(leaseOptions("box-a", ""), "abc123", gitwt.Exec)
	persistToken(leaseOptions("box-a", "-"), "abc123", gitwt.Exec)
	repo := originAndClone(t)
	persistToken(leaseOptions("box-a", repo), "", gitwt.Exec)

	assert.Equal(t, "", ReadToken(repo, 7, gitwt.Exec))
}
