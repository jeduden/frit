package fleet

import (
	"errors"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFetchRemoteSurfacesAProbeFailureRatherThanSilentlySkipping: the
// preliminary check for whether a remote is configured must not fold a
// probe failure — a stalled or corrupt local git call, indistinguishable
// from "not configured" by error alone — into "nothing to fetch". Doing
// so hides a real fault: staleFetch never fires, and Gather trusts a
// remote-tracking view it never actually confirmed.
func TestFetchRemoteSurfacesAProbeFailureRatherThanSilentlySkipping(t *testing.T) {
	probeErr := errors.New("git: timed out after 1ms")
	run := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "remote" {
			return nil, probeErr
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	fetched, err := fetchRemote("/repo", "origin", run)

	require.Error(t, err)
	assert.ErrorIs(t, err, probeErr)
	assert.False(t, fetched, "a failed probe fetched nothing")
}

// TestFetchRemoteSkipsANotConfiguredRemote: a remote absent from the
// configured list is genuinely nothing to fetch, so it is skipped
// without error, same as before.
func TestFetchRemoteSkipsANotConfiguredRemote(t *testing.T) {
	run := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "remote" {
			return []byte("upstream\n"), nil
		}
		t.Fatalf("unexpected git call: %v", args)

		return nil, nil
	}

	fetched, err := fetchRemote("/repo", "origin", run)

	require.NoError(t, err)
	assert.False(t, fetched, "a remote not configured fetched nothing")
}

// TestFetchRemoteFetchesAConfiguredRemote: a remote present in the
// configured list is fetched, and the fetch's own result is returned.
// A fetch that failed did not refresh anything, so it reports fetched
// false alongside the error.
func TestFetchRemoteFetchesAConfiguredRemote(t *testing.T) {
	fetchErr := errors.New("connection refused")
	var fetchedArgs []string
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "remote":
			return []byte("origin\n"), nil
		case "config":
			return nil, errors.New("exit status 1")
		}
		fetchedArgs = args

		return nil, fetchErr
	}

	fetched, err := fetchRemote("/repo", "origin", run)

	require.ErrorIs(t, err, fetchErr)
	assert.False(t, fetched, "a failed fetch refreshed nothing")
	assert.Equal(t, []string{"fetch", "--prune", "--quiet", "origin"},
		fetchedArgs)
}

// TestFetchRemoteReportsASuccessfulFetch: a fetch against a configured
// remote that succeeds reports fetched true, the signal the gather's
// status summary counts.
func TestFetchRemoteReportsASuccessfulFetch(t *testing.T) {
	run := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "remote" {
			return []byte("origin\n"), nil
		}

		return nil, nil
	}

	fetched, err := fetchRemote("/repo", "origin", run)

	require.NoError(t, err)
	assert.True(t, fetched, "a successful fetch reports it refreshed")
}

// TestFetchRemoteSkipsRescueBranches: the fetch keeps every refspec the
// remote is configured with and adds one negative refspec, so frit's
// own rescue branches are never downloaded into the clone.
func TestFetchRemoteSkipsRescueBranches(t *testing.T) {
	var fetchedArgs []string
	run := func(dir string, args ...string) ([]byte, error) {
		switch args[0] {
		case "remote":
			return []byte("origin\n"), nil
		case "config":
			assert.Equal(t, []string{"config", "--get-all", "remote.origin.fetch"}, args)
			return []byte("+refs/heads/*:refs/remotes/origin/*\n+refs/notes/*:refs/notes/*\n"), nil
		}
		fetchedArgs = args

		return nil, nil
	}

	_, err := fetchRemote("/repo", "origin", run)

	require.NoError(t, err)
	assert.Equal(t, []string{"fetch", "--prune", "--quiet", "origin",
		"+refs/heads/*:refs/remotes/origin/*", "+refs/notes/*:refs/notes/*",
		"^refs/heads/frit-rescue/*"}, fetchedArgs)
}

// TestFetchRemoteLeavesRescueBranchesOnOrigin: against real git, a
// rescue branch on origin stays there and is not copied into the
// clone, while an ordinary branch is fetched as usual.
func TestFetchRemoteLeavesRescueBranchesOnOrigin(t *testing.T) {
	origin := t.TempDir()
	gitIn(t, origin, "init", "-q", "--bare", "-b", "main")
	clone := t.TempDir()
	gitIn(t, clone, "init", "-q", "-b", "main")
	gitIn(t, clone, "commit", "-q", "--allow-empty", "-m", "base")
	gitIn(t, clone, "remote", "add", "origin", origin)
	gitIn(t, clone, "push", "-q", "origin",
		"main", "main:plan/7", "main:frit-rescue/7/box-abc")
	// A push updates the pusher's own tracking refs; start the fetch
	// from a clone that has never seen either branch.
	gitIn(t, clone, "update-ref", "-d", "refs/remotes/origin/plan/7")
	gitIn(t, clone, "update-ref", "-d", "refs/remotes/origin/frit-rescue/7/box-abc")

	fetched, err := fetchRemote(clone, "origin", gitwt.Exec)

	require.NoError(t, err)
	assert.True(t, fetched)
	refs := gitIn(t, clone, "for-each-ref", "--format=%(refname)", "refs/remotes/origin/")
	assert.Contains(t, refs, "refs/remotes/origin/plan/7")
	assert.NotContains(t, refs, "frit-rescue")
}

// gitIn runs git in dir for a fixture, failing the test on error.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitwt.Exec(dir, append([]string{"-c", "commit.gpgsign=false",
		"-c", "user.name=frit", "-c", "user.email=frit@example.com"}, args...)...)
	require.NoError(t, err, "git %v: %s", args, out)

	return string(out)
}
