package plans_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/index"
	"github.com/jeduden/frit/internal/plans"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectKeepsAFolderPlanAndReportsAMislaidFile is Phase 1's RED:
// a folder's fixed plan.md is kept like a flat plan, its companion is
// dropped silently, and a plan-like file dropped anywhere is reported
// rather than lost.
func TestCollectKeepsAFolderPlanAndReportsAMislaidFile(t *testing.T) {
	dir := folderInitRepo(t)
	git(t, dir, "checkout", "-q", "-b", "plan/mixed")
	folderWrite(t, dir, "plan/2601010000_flat.md", minimalPlan(2601010000))
	folderWrite(t, dir, "plan/2601020000_folder/plan.md",
		minimalPlan(2601020000))
	folderWrite(t, dir, "plan/2601020000_folder/notes.md", "not a plan\n")
	folderWrite(t, dir, "plan/archive/2601050000_deep.md",
		minimalPlan(2601050000))
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "mixed")
	git(t, dir, "checkout", "-q", "main")

	got, ignored, err := plans.Collect(
		dir, plans.DefaultDir, gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	paths := make([]string, 0, len(got))
	for _, f := range got {
		paths = append(paths, f.Path)
	}
	assert.Contains(t, paths, "plan/2601010000_flat.md")
	assert.Contains(t, paths, "plan/2601020000_folder/plan.md")
	assert.NotContains(t, paths, "plan/2601020000_folder/notes.md")
	assert.NotContains(t, paths, "plan/archive/2601050000_deep.md")

	assert.Contains(t, ignored, "plan/archive/2601050000_deep.md")
	assert.NotContains(t, ignored, "plan/2601020000_folder/notes.md")

	entries, problems := index.Build("h", "repo", "", got)
	assert.Empty(t, problems, "no kept file should fail as not a plan")
	assert.Len(t, entries, 2, "the flat plan and the folder plan, once each")
}

func minimalPlan(id int64) string {
	return "---\nid: " + itoaFolder(id) +
		"\ntitle: t\nstatus: \"🔲\"\n---\n# t\n"
}

func itoaFolder(n int64) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

func folderInitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "frit-test")
	folderWrite(t, dir, "README.md", "# fixture\n")
	git(t, dir, "add", "README.md")
	git(t, dir, "commit", "-q", "-m", "initial")

	return dir
}

func folderWrite(t *testing.T, dir, rel, body string) {
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
