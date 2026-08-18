package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// claimableRepo builds a repository carrying a not-started plan on main,
// with a bare origin to push a lease to — what a plan looks like the
// moment before it is claimed: startable, held by nobody.
func claimableRepo(
	t *testing.T, root, name string, id int, title string,
) string {
	t.Helper()
	repo := initRepo(t, root, name)
	commitPlan(t, repo, id, "🔲", title, nil, "")
	// The origin lives outside root so the fleet walk does not index the
	// bare repository as one of its own.
	origin := filepath.Join(t.TempDir(), name+"-origin.git")
	git(t, repo, "init", "-q", "--bare", "-b", "main", origin)
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "-q", "origin", "main")

	return repo
}

// gitCapture runs git in dir and returns its output and error, for
// asserting on refs a command wrote.
func gitCapture(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()

	return strings.TrimSpace(string(out)), err
}

// TestClaimMintsAPickablePlan: a startable plan is leased on its hold
// branch, locally and on origin.
func TestClaimMintsAPickablePlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed plan 7")
	assert.Contains(t, out.String(), "plan/7-shader-unit")

	local, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7-shader-unit")
	require.NoError(t, err, "the claim ref was minted locally")
	remote, err := gitCapture(t, repo, "ls-remote", "origin",
		"refs/heads/plan/7-shader-unit")
	require.NoError(t, err)
	assert.Contains(t, remote, local, "the same lease is on origin")
}

// TestClaimRefusesAHeldPlan: a plan a lane already holds is not
// re-claimed, and nothing is pushed.
func TestClaimRefusesAHeldPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "already held")
}

// TestClaimRefusesABlockedPlan: a plan with an unfinished dependency is
// not claimable, and says why.
func TestClaimRefusesABlockedPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔲", "Upstream", nil, "")
	commitPlan(t, repo, 8, "🔲", "Downstream", []int{7}, "")
	var out, errb bytes.Buffer

	code := run([]string{"claim", "8", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "blocked")
}

// TestClaimEmitsJSON decodes the document a consumer reads back to learn
// the branch it now holds.
func TestClaimEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	var doc report.ClaimDoc

	emit(t, &doc, "claim", "7", "--root", root)

	assert.Equal(t, "claim", doc.Command)
	assert.True(t, doc.Claimed)
	assert.Equal(t, "plan/7-shader-unit", doc.Branch)
	assert.Equal(t, int64(7), doc.Plan.ID)
	assert.NotEmpty(t, doc.Base, "the lease is dated against a base commit")
	assert.Empty(t, doc.Refused)
}

// TestClaimRefusesAnAmbiguousRepoName: when two checkouts under the root
// share a basename, the fleet cannot tell which one a plan lives in, so
// claim refuses rather than mint the lease into the wrong repository. No
// ref is pushed in either checkout.
func TestClaimRefusesAnAmbiguousRepoName(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repoA := initRepo(t, filepath.Join(root, "a"), "frontend")
	commitPlan(t, repoA, 7, "🔲", "Shader unit", nil, "")
	repoB := initRepo(t, filepath.Join(root, "b"), "frontend")
	commitPlan(t, repoB, 9, "🔲", "Other work", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "shared by another checkout")

	_, err := gitCapture(t, repoA, "rev-parse",
		"refs/heads/plan/7-shader-unit")
	assert.Error(t, err, "no lease was minted in either checkout")
}
