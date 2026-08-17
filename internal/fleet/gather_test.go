package fleet

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoWithPlan builds a one-commit repository under root carrying a
// single not-started plan on its default branch — the shape the gather
// walks and the mutating verbs date a lease against.
func repoWithPlan(t *testing.T, root, name string, id int) string {
	t.Helper()
	dir := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "plan"), 0o750))
	gitCmd(t, dir, "init", "-q", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "t@example.com")
	gitCmd(t, dir, "config", "user.name", "frit-test")
	body := "---\nid: " + strconv.Itoa(id) +
		"\ntitle: Shader unit\nstatus: \"🔲\"\n---\n# Shader unit\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "plan", "plan.md"), []byte(body), 0o600))
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-q", "-m", "plan")

	return dir
}

// TestGatherCarriesRepoCoordinates: the gather hands back each
// repository's path, remote and base beside the plans, so a claim reads
// them off the walk it already ran rather than walking the fleet again.
func TestGatherCarriesRepoCoordinates(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	coord, ok := res.Coords["atlas"]
	require.True(t, ok, "the gather carries a coordinate for the repo")
	// git reports the worktree's realpath, so resolve the expected path
	// too: on a host where the temp root is a symlink the raw join and
	// git's answer would differ though the coordinate is right.
	want, err := filepath.EvalSymlinks(repo)
	require.NoError(t, err)
	assert.Equal(t, want, coord.Path)
	assert.Equal(t, "origin", coord.Remote)
	assert.Equal(t, "refs/heads/main", coord.Base,
		"the base is the default-ref cascade when the config sets none")
}

// TestGatherCarriesTheConfiguredBase: when a repository pins its base in
// .frit.yml, that is the base the gather carries — not the default-ref
// cascade, which only fills in when the config sets none.
func TestGatherCarriesTheConfiguredBase(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, ".frit.yml"), []byte("base: refs/heads/dev\n"), 0o600))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-q", "-m", "pin base")

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	coord, ok := res.Coords["atlas"]
	require.True(t, ok)
	assert.Equal(t, "refs/heads/dev", coord.Base,
		"the base is the config's when it sets one")
}
