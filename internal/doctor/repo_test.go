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
		"https://x-access-token:secret@github.com/jeduden/frit.git",
		"ssh://git@github.com:22/jeduden/frit.git",
		"ssh://git@ssh.github.com:443/jeduden/frit.git",
		"https://github.com/jeduden/frit/",
		"https://GitHub.com/jeduden/frit.git",
		"org-1@github.com:jeduden/frit.git",
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
		"https://github.com.example.org/jeduden/frit.git",
		"git@gitlab.com:jeduden/frit.git",
		"https://github.com/%zz/frit.git",
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

// TestCheckAutoDeleteNamesWhyGhFailed: a gh that exits non-zero says
// why on its stderr, which exec carries beside a bare "exit status 1";
// the note quotes gh's own first line rather than the exit code.
func TestCheckAutoDeleteNamesWhyGhFailed(t *testing.T) {
	refused := func(...string) ([]byte, error) {
		return nil, &exec.ExitError{
			Stderr: []byte("gh: Not Found (HTTP 404)\nsecond line\n"),
		}
	}

	f, found := CheckAutoDelete("o", "r", refused)

	require.True(t, found)
	assert.Equal(t, "not checked: gh: Not Found (HTTP 404)", f.Message)
}

// TestCheckAutoDeleteNotesASettingGitHubDidNotShow: GitHub leaves the
// field out for a token without push access, which gh prints as null.
// That is a setting frit could not read, never one that is off.
func TestCheckAutoDeleteNotesASettingGitHubDidNotShow(t *testing.T) {
	for _, answer := range []string{"null\n", "", "maybe\n"} {
		gh := func(...string) ([]byte, error) { return []byte(answer), nil }

		f, found := CheckAutoDelete("o", "r", gh)

		require.True(t, found, "%q", answer)
		assert.Contains(t, f.Message, "not checked", "%q", answer)
		assert.NotContains(t, f.Message, "is off", "%q", answer)
	}
}

// TestGhFailureQuotesGhsOwnReason: gh's first stderr line is the
// reason, its "gh: " prefix dropped; a failure that said nothing, or
// one that never ran gh at all, keeps the error's own wording.
func TestGhFailureQuotesGhsOwnReason(t *testing.T) {
	said := &exec.ExitError{Stderr: []byte("gh: HTTP 403\nhint\n")}
	assert.Equal(t, "HTTP 403", ghFailure(said))

	silent := exec.Command("sh", "-c", "exit 3").Run()
	require.Error(t, silent)
	assert.Equal(t, "exit status 3", ghFailure(silent))

	assert.Equal(t, "timed out", ghFailure(errors.New("timed out")))
}

// TestIsGitHubHostMatchesTheWholeName: github.com under either name
// GitHub serves git from, in any case, and nothing that merely
// contains it.
func TestIsGitHubHostMatchesTheWholeName(t *testing.T) {
	for _, host := range []string{"github.com", "GitHub.com", "ssh.github.com"} {
		assert.True(t, isGitHubHost(host), host)
	}
	for _, host := range []string{"", "gitlab.com", "github.com.example.org", "mygithub.com"} {
		assert.False(t, isGitHubHost(host), host)
	}
}
