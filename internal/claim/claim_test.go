package claim

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
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

func sampleOptions() Options {
	return Options{
		Branch:   "plan/7-shader-unit",
		Base:     "origin/main",
		Remote:   "origin",
		PlanID:   7,
		PlanFile: "plan/7-shader-unit.md",
		Lane:     "/work/lanes/shader-unit",
		Host:     "mm-box",
	}
}

// TestBranchDerivesTheHoldName: the lease branch is plan/<id>-<slug>,
// with the slug taken from the plan file after its id prefix. A file
// with no underscore contributes its whole stem.
func TestBranchDerivesTheHoldName(t *testing.T) {
	assert.Equal(t, "plan/7-shader-unit",
		Branch(7, "plan/7_shader-unit.md"))
	assert.Equal(t, "plan/2608161810-dispatch-ladder",
		Branch(2608161810, "plan/2608161810_dispatch-ladder.md"))
	assert.Equal(t, "plan/42-notes",
		Branch(42, "docs/notes.md"),
		"a file with no id prefix contributes its whole stem")
}

// TestMintCreatesTheLease: the claim ref is minted both locally and on
// origin, and the Result is dated against the base commit.
func TestMintCreatesTheLease(t *testing.T) {
	work := originAndClone(t)
	baseSHA := gitCmd(t, work, "rev-parse", "origin/main")

	res, err := Mint(work, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, "plan/7-shader-unit", res.Branch)
	assert.Equal(t, baseSHA, res.BaseSHA)

	local := gitCmd(t, work, "rev-parse", "refs/heads/plan/7-shader-unit")
	remote := gitCmd(t, work,
		"ls-remote", "origin", "refs/heads/plan/7-shader-unit")
	assert.NotEmpty(t, local)
	assert.Contains(t, remote, local,
		"the same marker is on origin as locally")
}

// TestMintIsAnEmptyMarker: the marker's tree equals the base tree, so
// the claim touched no file — it is only a lease.
func TestMintIsAnEmptyMarker(t *testing.T) {
	work := originAndClone(t)
	baseTree := gitCmd(t, work, "rev-parse", "origin/main^{tree}")

	_, err := Mint(work, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)

	markerTree := gitCmd(t, work,
		"rev-parse", "refs/heads/plan/7-shader-unit^{tree}")
	assert.Equal(t, baseTree, markerTree)
}

// TestMintLosesTheRaceAndRollsBack: once the first clone has planted the
// ref on origin, the second clone's Mint must lose and leave no local
// ref behind, so a retry starts clean.
func TestMintLosesTheRaceAndRollsBack(t *testing.T) {
	first := originAndClone(t)
	second := cloneAgain(t, first)

	_, err := Mint(first, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)

	_, err = Mint(second, sampleOptions(), gitwt.Exec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lost the race for plan 7")

	verify := exec.Command("git", "-C", second,
		"rev-parse", "--verify", "-q", "refs/heads/plan/7-shader-unit")
	require.Error(t, verify.Run(),
		"the local ref was rolled back after losing the race")
}

// TestMintReportsANonRaceFailure: a push that fails for a reason other
// than a pre-existing ref is a real fault, not a lost race, and the local
// ref is still rolled back.
func TestMintReportsANonRaceFailure(t *testing.T) {
	work := originAndClone(t)
	opts := sampleOptions()
	opts.Remote = "no-such-remote" // never configured, so the push faults

	_, err := Mint(work, opts, gitwt.Exec)

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrLostRace),
		"an unreachable remote is not another machine winning the race")

	_, refErr := gitCapture(t, work,
		"rev-parse", "--verify", "refs/heads/plan/7-shader-unit")
	assert.Error(t, refErr, "the local ref was rolled back")
}

// TestReleaseDropsTheClaim: a minted claim can be unwound, leaving no ref
// locally or on the remote for a lane that never stood up.
func TestReleaseDropsTheClaim(t *testing.T) {
	work := originAndClone(t)
	_, err := Mint(work, sampleOptions(), gitwt.Exec)
	require.NoError(t, err)

	require.NoError(t,
		Release(work, "plan/7-shader-unit", "origin", gitwt.Exec))

	_, localErr := gitCapture(t, work,
		"rev-parse", "--verify", "refs/heads/plan/7-shader-unit")
	assert.Error(t, localErr, "the local ref is gone")
	remote, err := gitCapture(t, work,
		"ls-remote", "origin", "refs/heads/plan/7-shader-unit")
	require.NoError(t, err)
	assert.Empty(t, remote, "the remote ref is gone")
}

// TestMarkerMessage pins the exact commit body, including the empty-Lane
// case that records "-" for both the title and the lane line.
func TestMarkerMessage(t *testing.T) {
	got := markerMessage(sampleOptions(), "abc123")
	want := "plan 7: claim shader-unit\n\n" +
		"lane:     /work/lanes/shader-unit\n" +
		"host:     mm-box\n" +
		"base:     abc123\n" +
		"plan:     plan/7-shader-unit.md"
	assert.Equal(t, want, got)

	opts := sampleOptions()
	opts.Lane = ""
	gotEmpty := markerMessage(opts, "abc123")
	wantEmpty := "plan 7: claim -\n\n" +
		"lane:     -\n" +
		"host:     mm-box\n" +
		"base:     abc123\n" +
		"plan:     plan/7-shader-unit.md"
	assert.Equal(t, wantEmpty, gotEmpty)
}
