package doctor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckHolderInRepoConfigFlagsACommittedHolder: a .frit.yml that
// names a holder is reported, since frit ignores it there.
func TestCheckHolderInRepoConfigFlagsACommittedHolder(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".frit.yml"),
		[]byte("plan-dir: plan\nholder: box\n"), 0o600))

	f, found, err := CheckHolderInRepoConfig(root)

	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "holder", f.Check)
	assert.Equal(t, ".frit.yml", f.Path)
	assert.Contains(t, f.Message, "ignored")
}

// TestCheckHolderInRepoConfigPassesWithoutOne: no holder key, or no
// file at all, is nothing to report.
func TestCheckHolderInRepoConfigPassesWithoutOne(t *testing.T) {
	root := t.TempDir()
	_, found, err := CheckHolderInRepoConfig(root)
	require.NoError(t, err)
	assert.False(t, found, "no file")

	require.NoError(t, os.WriteFile(filepath.Join(root, ".frit.yml"),
		[]byte("plan-dir: plan\n"), 0o600))
	_, found, err = CheckHolderInRepoConfig(root)
	require.NoError(t, err)
	assert.False(t, found, "no holder key")
}

// TestCheckHolderInRepoConfigSurfacesAnUnreadableFile: a file that is
// there but not YAML, or not readable, is a fault, not a pass.
func TestCheckHolderInRepoConfigSurfacesAnUnreadableFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".frit.yml"),
		[]byte("holder: [\n"), 0o600))
	_, _, err := CheckHolderInRepoConfig(root)
	require.Error(t, err)

	require.NoError(t, os.Remove(filepath.Join(root, ".frit.yml")))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".frit.yml"), 0o750))
	_, _, err = CheckHolderInRepoConfig(root)
	require.Error(t, err, "a directory where the file should be")
}

// TestGitHubRepoReadsEveryURLShape: https, scp-like ssh and ssh://
// origins on github.com all name owner and repository; anything else
// is not a GitHub origin.
func TestGitHubRepoReadsEveryURLShape(t *testing.T) {
	for _, url := range []string{
		"https://github.com/jeduden/frit",
		"https://github.com/jeduden/frit.git",
		"git@github.com:jeduden/frit.git",
		"ssh://git@github.com/jeduden/frit.git",
	} {
		owner, name, ok := GitHubRepo(url)
		require.True(t, ok, url)
		assert.Equal(t, "jeduden", owner, url)
		assert.Equal(t, "frit", name, url)
	}
	for _, url := range []string{
		"https://gitlab.com/jeduden/frit.git",
		"/tmp/origin.git",
		"https://github.com/jeduden",
		"https://github.com/jeduden/frit/extra",
	} {
		_, _, ok := GitHubRepo(url)
		assert.False(t, ok, url)
	}
}

// TestCheckAutoDeleteFlagsASettingThatIsOff: GitHub answering false is
// a finding naming the setting.
func TestCheckAutoDeleteFlagsASettingThatIsOff(t *testing.T) {
	var asked []string
	gh := func(args ...string) ([]byte, error) {
		asked = args
		return []byte("false\n"), nil
	}

	f, found := CheckAutoDelete("jeduden", "frit", gh)

	require.True(t, found)
	assert.Equal(t, []string{"api", "repos/jeduden/frit", "--jq", ".delete_branch_on_merge"}, asked)
	assert.Equal(t, "auto-delete", f.Check)
	assert.Contains(t, f.Message, "Automatically delete head branches")
}

// TestCheckAutoDeletePassesWhenOn: true is nothing to report.
func TestCheckAutoDeletePassesWhenOn(t *testing.T) {
	gh := func(...string) ([]byte, error) { return []byte("true\n"), nil }

	_, found := CheckAutoDelete("jeduden", "frit", gh)

	assert.False(t, found)
}

// TestCheckAutoDeleteNotesWhenItCannotCheck: no gh, or a gh call that
// fails, is a note that the check was skipped, never a pass.
func TestCheckAutoDeleteNotesWhenItCannotCheck(t *testing.T) {
	missing := func(...string) ([]byte, error) { return nil, exec.ErrNotFound }
	f, found := CheckAutoDelete("o", "r", missing)
	require.True(t, found)
	assert.Contains(t, f.Message, "gh is not installed")

	failing := func(...string) ([]byte, error) { return nil, errors.New("HTTP 401") }
	f, found = CheckAutoDelete("o", "r", failing)
	require.True(t, found)
	assert.Contains(t, f.Message, "HTTP 401")
}
