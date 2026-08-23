package claim

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitCmd runs git in dir for test setup, failing loudly. Signing is off
// so a developer's global commit.gpgsign does not stall the marker.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)

	return strings.TrimSpace(string(out))
}

// gitCapture runs git in dir and returns its output and error, for
// asserting on a ref that should or should not exist.
func gitCapture(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()

	return strings.TrimSpace(string(out)), err
}

// originAndClone builds a bare origin.git with a working clone that has
// one commit on main pushed to it, and returns the working repo dir.
func originAndClone(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	gitCmd(t, root, "init", "-q", "--bare", "-b", "main", origin)

	work := filepath.Join(root, "work")
	require.NoError(t, os.Mkdir(work, 0o750))
	gitCmd(t, work, "init", "-q", "-b", "main")
	gitCmd(t, work, "config", "user.email", "t@example.com")
	gitCmd(t, work, "config", "user.name", "frit-test")
	gitCmd(t, work, "config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(
		filepath.Join(work, "README.md"), []byte("x\n"), 0o600))
	gitCmd(t, work, "add", "-A")
	gitCmd(t, work, "commit", "-q", "-m", "init")
	gitCmd(t, work, "remote", "add", "origin", origin)
	gitCmd(t, work, "push", "-q", "origin", "main")

	return work
}

// cloneAgain makes a second working clone of the same origin the first
// clone in work points at, so two machines can race for one plan.
func cloneAgain(t *testing.T, work string) string {
	t.Helper()
	origin := gitCmd(t, work, "config", "--get", "remote.origin.url")
	dst := t.TempDir()
	gitCmd(t, dst, "clone", "-q", origin, dst)
	gitCmd(t, dst, "config", "user.email", "t2@example.com")
	gitCmd(t, dst, "config", "user.name", "frit-test-2")
	gitCmd(t, dst, "config", "commit.gpgsign", "false")

	return dst
}

// TestBranchIsTheIdOnlyWorkRef: the hold branch is plan/<id> and
// carries nothing derived from local state, so a renamed plan file
// still names the same ref on every machine.
func TestBranchIsTheIdOnlyWorkRef(t *testing.T) {
	assert.Equal(t, "plan/7", Branch(7))
	assert.Equal(t, "plan/2608161810", Branch(2608161810))
}

// TestMarkerHost reads the host line from a marker body, and reports "" for
// a plain work commit that carries none.
func TestMarkerHost(t *testing.T) {
	body := "plan 7: claim shader-unit\n\n" +
		"lane:     /work/lanes/shader-unit\n" +
		"host:     mm-box\n" +
		"base:     abc123\n" +
		"plan:     plan/7-shader-unit.md"
	assert.Equal(t, "mm-box", markerHost(body))
	assert.Empty(t, markerHost("real work, no host line"))

	lease := "plan 7: claim\n\n" +
		"epoch:   1\n" +
		"nonce:   cafe\n" +
		"holder:  box-a\n" +
		"lane:    /lanes/a\n" +
		"session: -\n" +
		"base:    abc123"
	assert.Equal(t, "box-a", markerHost(lease),
		"a lease marker's holder trailer reads as the host")
}

// TestBaseBranch reduces every base-ref shape to the remote branch name a
// fresh landed check fetches, and leaves a bare name unchanged.
func TestBaseBranch(t *testing.T) {
	assert.Equal(t, "main", baseBranch("refs/remotes/origin/main", "origin"))
	assert.Equal(t, "main", baseBranch("origin/main", "origin"))
	assert.Equal(t, "main", baseBranch("refs/heads/main", "origin"))
	assert.Equal(t, "main", baseBranch("main", "origin"))
}

// TestIsAncestor: a zero exit reads as merged, any non-zero exit as not.
func TestIsAncestor(t *testing.T) {
	yes := func(_ string, _ ...string) ([]byte, error) { return nil, nil }
	no := func(_ string, _ ...string) ([]byte, error) {
		return nil, errors.New("not an ancestor")
	}
	assert.True(t, isAncestor("/r", "a", "b", yes))
	assert.False(t, isAncestor("/r", "a", "b", no))
}

// fakeMergeTree returns a Runner that answers merge-tree with treeOID
// (or mergeErr, when set) and rev-parse ...^{tree} with baseTreeOID (or
// rpErr) — the two calls landedByContent makes, told apart by args[0].
func fakeMergeTree(
	treeOID string, mergeErr error, baseTreeOID string, rpErr error,
) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "merge-tree":
			if mergeErr != nil {
				return nil, mergeErr
			}
			return []byte(treeOID + "\n"), nil
		case "rev-parse":
			if rpErr != nil {
				return nil, rpErr
			}
			return []byte(baseTreeOID + "\n"), nil
		default:
			return nil, errors.New("unexpected git subcommand: " + args[0])
		}
	}
}

// TestLandedByContent drives every branch directly against a fake
// Runner: a merge-no-op reports landed, a differing tree or a conflict
// (merge-tree's non-zero exit) reports not landed, and a git fault on
// either call fails toward the same safe "not landed" answer.
func TestLandedByContent(t *testing.T) {
	same := "1111111111111111111111111111111111111111"
	other := "2222222222222222222222222222222222222222"

	assert.True(t, landedByContent(
		"/r", "base", "tip", fakeMergeTree(same, nil, same, nil)),
		"merging tip into base reproduces base's own tree: landed")

	assert.False(t, landedByContent(
		"/r", "base", "tip", fakeMergeTree(other, nil, same, nil)),
		"a differing resulting tree means real, unlanded divergence")

	assert.False(t, landedByContent(
		"/r", "base", "tip",
		fakeMergeTree("", errors.New("CONFLICT"), same, nil)),
		"merge-tree's conflict exit is evidence of divergence, not landed")

	assert.False(t, landedByContent(
		"/r", "base", "tip",
		fakeMergeTree(same, nil, "", errors.New("bad revision"))),
		"a git fault reading the base's own tree fails toward not-landed")
}

// TestHasWork tells a marker-only chain from one carrying real work and
// surfaces a git fault reading the chain — the gate that keeps a bare
// claim marker's trivial no-op merge from ever reading as landed.
func TestHasWork(t *testing.T) {
	chain := func(out string) func(string, ...string) ([]byte, error) {
		return func(_ string, _ ...string) ([]byte, error) {
			return []byte(out), nil
		}
	}

	work, err := hasWork("/r", 7, "base", "tip",
		chain("plan 7: beat\nplan 7: claim\n"))
	require.NoError(t, err)
	assert.False(t, work, "a chain of only frit's markers carries no work")

	work, err = hasWork("/r", 7, "base", "tip",
		chain("a real change\nplan 7: claim\n"))
	require.NoError(t, err)
	assert.True(t, work, "a non-marker subject is work a delete would lose")

	_, err = hasWork("/r", 7, "base", "tip",
		func(_ string, _ ...string) ([]byte, error) {
			return nil, errors.New("bad revision")
		})
	require.Error(t, err, "a git fault reading the chain is surfaced")
}

// TestHolderMarker returns the marker body on a match and reports not-found
// when the grep selects nothing or the object is missing.
func TestHolderMarker(t *testing.T) {
	found := func(_ string, _ ...string) ([]byte, error) {
		return []byte("plan 7: claim x\n\nhost:     mm-box\n"), nil
	}
	body, ok := holderMarker("/r", 7, "tip", found)
	require.True(t, ok)
	assert.Contains(t, body, "host:     mm-box")

	missing := func(_ string, _ ...string) ([]byte, error) {
		return nil, errors.New("bad object tip")
	}
	_, ok = holderMarker("/r", 7, "tip", missing)
	assert.False(t, ok)

	empty := func(_ string, _ ...string) ([]byte, error) { return nil, nil }
	_, ok = holderMarker("/r", 7, "tip", empty)
	assert.False(t, ok, "an empty body is not a marker")
}
