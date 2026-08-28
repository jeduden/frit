package plans

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectReadsAPlanFromABranchWithNoWorktree is the whole point
// of the package: the branch is never checked out, has no worktree,
// and its plan is still read.
func TestCollectReadsAPlanFromABranchWithNoWorktree(t *testing.T) {
	dir := initRepo(t)
	addPlanOnBranch(t, dir, "plan/2608142306-fleet-index",
		"plan/2608142306_fleet-index.md", "# The fleet index\n")
	// Leave main checked out; the plan branch has no worktree.
	git(t, dir, "checkout", "-q", "main")

	got, _, err := Collect(dir, DefaultDir, gitwt.Exec, gitwt.ExecPipe)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "refs/heads/plan/2608142306-fleet-index",
		got[0].Ref)
	assert.Equal(t, "plan/2608142306-fleet-index", got[0].Short())
	assert.Equal(t, "plan/2608142306_fleet-index.md", got[0].Path)
	assert.Equal(t, "# The fleet index\n", string(got[0].Content))
}

func TestCollectReadsContentWithNewlinesIntact(t *testing.T) {
	body := "---\nid: 2608142306\nstatus: \"🔳\"\n---\n# Plan\n\nBody.\n"
	dir := initRepo(t)
	addPlanOnBranch(t, dir, "plan/multi", "plan/a.md", body)

	got, _, err := Collect(dir, DefaultDir, gitwt.Exec, gitwt.ExecPipe)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, body, string(got[0].Content),
		"payloads are sliced by byte count, not by line")
}

func TestCollectSeesEveryRefIncludingRemotesAndTags(t *testing.T) {
	dir := initRepo(t)
	addPlanOnBranch(t, dir, "plan/one", "plan/one.md", "# one\n")
	git(t, dir, "tag", "v1.0.0", "plan/one")
	// A remote-tracking ref, written directly the way a fetch would.
	git(t, dir, "update-ref", "refs/remotes/peer/plan/one",
		"refs/heads/plan/one")

	got, _, err := Collect(dir, DefaultDir, gitwt.Exec, gitwt.ExecPipe)

	require.NoError(t, err)
	refs := map[string]bool{}
	for _, f := range got {
		refs[f.Ref] = true
	}
	assert.True(t, refs["refs/heads/plan/one"])
	assert.True(t, refs["refs/remotes/peer/plan/one"], "remotes count")
	assert.True(t, refs["refs/tags/v1.0.0"], "tags count")
}

func TestCollectSharesContentBetweenIdenticalRefs(t *testing.T) {
	dir := initRepo(t)
	addPlanOnBranch(t, dir, "plan/one", "plan/one.md", "# one\n")
	git(t, dir, "update-ref", "refs/remotes/peer/plan/one",
		"refs/heads/plan/one")

	got, _, err := Collect(dir, DefaultDir, gitwt.Exec, gitwt.ExecPipe)

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(got), 2)
	assert.Equal(t, got[0].OID, got[1].OID,
		"identical content is one object, read once")
}

func TestCollectIgnoresRefsWithoutAPlanDirectory(t *testing.T) {
	dir := initRepo(t)

	got, _, err := Collect(dir, DefaultDir, gitwt.Exec, gitwt.ExecPipe)

	require.NoError(t, err)
	assert.Empty(t, got, "main carries no plans and that is not a fault")
}

func TestCollectKeepsOnlyMarkdown(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "plan/mixed")
	write(t, dir, "plan/a.md", "# a\n")
	write(t, dir, "plan/notes.txt", "not a plan\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "mixed")
	git(t, dir, "checkout", "-q", "main")

	got, _, err := Collect(dir, DefaultDir, gitwt.Exec, gitwt.ExecPipe)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "plan/a.md", got[0].Path)
}

func TestCollectIsSortedByRefThenPath(t *testing.T) {
	dir := initRepo(t)
	git(t, dir, "checkout", "-q", "-b", "plan/zeta")
	write(t, dir, "plan/b.md", "# b\n")
	write(t, dir, "plan/a.md", "# a\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "two")
	git(t, dir, "checkout", "-q", "main")

	got, _, err := Collect(dir, DefaultDir, gitwt.Exec, gitwt.ExecPipe)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "plan/a.md", got[0].Path)
	assert.Equal(t, "plan/b.md", got[1].Path)
}

func TestCollectFailsOnANonRepository(t *testing.T) {
	_, _, err := Collect(t.TempDir(), DefaultDir,
		gitwt.Exec, gitwt.ExecPipe)

	require.Error(t, err)
}

func TestMarkdownOnlyDropsTreesAndRejoinsThePrefix(t *testing.T) {
	// ls-tree reports paths relative to the tree it was given, so
	// the inputs here carry no "plan/" prefix and the outputs do.
	got, ignored := markdownOnly("plan", []gitobj.TreeEntry{
		{Type: "blob", Path: "a.md"},
		{Type: "blob", Path: "notes.txt"},
		{Type: "tree", Path: "sub"},
	})

	require.Len(t, got, 1)
	assert.Equal(t, "plan/a.md", got[0].Path)
	assert.Empty(t, ignored)
}

func TestBlobOIDsAreDistinctAndSorted(t *testing.T) {
	got := blobOIDs(map[string][]gitobj.TreeEntry{
		"t1": {{OID: "bbbb"}, {OID: "aaaa"}},
		"t2": {{OID: "aaaa"}},
	})

	assert.Equal(t, []string{"aaaa", "bbbb"}, got)
}

// --- fixtures ---

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "frit-test")
	write(t, dir, "README.md", "# fixture\n")
	git(t, dir, "add", "README.md")
	git(t, dir, "commit", "-q", "-m", "initial")

	return dir
}

// addPlanOnBranch commits one plan file on its own branch and
// returns to where it started.
func addPlanOnBranch(t *testing.T, dir, branch, path, body string) {
	t.Helper()
	git(t, dir, "checkout", "-q", "-b", branch)
	write(t, dir, path, body)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "add "+path)
	git(t, dir, "checkout", "-q", "main")
}

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
	require.NoError(t, os.WriteFile(full, []byte(body), 0o600))
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "commit.gpgsign=false"},
		args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
}
