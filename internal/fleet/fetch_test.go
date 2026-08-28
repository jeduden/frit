package fleet

import (
	"errors"
	"testing"

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

	err := fetchRemote("/repo", "origin", run)

	require.Error(t, err)
	assert.ErrorIs(t, err, probeErr)
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

	err := fetchRemote("/repo", "origin", run)

	require.NoError(t, err)
}

// TestFetchRemoteFetchesAConfiguredRemote: a remote present in the
// configured list is fetched, and the fetch's own result is returned.
func TestFetchRemoteFetchesAConfiguredRemote(t *testing.T) {
	fetchErr := errors.New("connection refused")
	var fetchedArgs []string
	run := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "remote" {
			return []byte("origin\n"), nil
		}
		fetchedArgs = args

		return nil, fetchErr
	}

	err := fetchRemote("/repo", "origin", run)

	require.ErrorIs(t, err, fetchErr)
	assert.Equal(t, []string{"fetch", "--prune", "--quiet", "origin"},
		fetchedArgs)
}
