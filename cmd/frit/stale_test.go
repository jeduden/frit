package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitDated runs git with a fixed commit time, so a branch tip can be
// made old enough to read as stale without waiting.
func gitDated(t *testing.T, dir, when string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "commit.gpgsign=false"},
		args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_COMMITTER_DATE="+when, "GIT_AUTHOR_DATE="+when)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// agedWorktree adds a worktree on a plan branch whose tip is long past
// the staleness cutoff.
func agedWorktree(t *testing.T, repo, branch, path string) {
	t.Helper()
	git(t, repo, "worktree", "add", "-q", "-b", branch, path)
	require.NoError(t, os.WriteFile(
		filepath.Join(path, "work.txt"), []byte("wip\n"), 0o600))
	const longAgo = "2020-01-01T12:00:00"
	gitDated(t, path, longAgo, "add", "-A")
	gitDated(t, path, longAgo, "commit", "-q", "-m", "old work")
}

// TestStaleTellsALiveLaneFromAnAbandonedOne is the phase-4 acceptance
// criterion end to end: two idle branches, only one with an agent, come
// back labelled apart.
func TestStaleTellsALiveLaneFromAnAbandonedOne(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	live := filepath.Join(root, "atlas-live")
	cold := filepath.Join(root, "atlas-cold")
	agedWorktree(t, repo, "plan/2608161808-herdr-join", live)
	agedWorktree(t, repo, "plan/7-cold", cold)

	withHerdr(t, herdrReturning(map[string]any{
		"agent":        "claude",
		"agent_status": "working",
		"cwd":          live,
		"pane_id":      "wC:p1",
	}))
	var out, errb bytes.Buffer

	code := run([]string{"stale", "--root", root, "--days", "1"},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "atlas-live")
	assert.Contains(t, got, "atlas-cold")
	assert.Contains(t, got, "live")
	assert.Contains(t, got, "abandoned")
}

// TestStaleLeavesPresenceUnknownWithNoSocket is the read-only board's
// promise for stale: with no herdr reachable, the git answer still
// stands and no lane is called abandoned on a guess.
func TestStaleLeavesPresenceUnknownWithNoSocket(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	agedWorktree(t, repo, "plan/7-cold",
		filepath.Join(root, "atlas-cold"))

	withHerdr(t, func(...string) ([]byte, error) {
		return nil, errors.New("connect: no such file")
	})
	var out, errb bytes.Buffer

	code := run([]string{"stale", "--root", root, "--days", "1"},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "atlas-cold", "the git board still answers")
	assert.NotContains(t, got, "abandoned",
		"presence is unknown, not abandoned")
	assert.NotContains(t, got, "live")
}
