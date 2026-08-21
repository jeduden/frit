package fleet

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/discovery"
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

// TestGatherWithholdsAnAmbiguousCoordinate: two repositories under the
// root sharing a basename cannot be told apart by the name the fleet
// keys on, so the gather carries no coordinate for that name and records
// the collision — a mutating verb then refuses rather than mint a lease
// into whichever checkout the walk reached last. The plans of both are
// still gathered, so the read-only board stays useful.
func TestGatherWithholdsAnAmbiguousCoordinate(t *testing.T) {
	root := t.TempDir()
	repoWithPlan(t, filepath.Join(root, "a"), "frontend", 7)
	repoWithPlan(t, filepath.Join(root, "b"), "frontend", 9)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	_, ok := res.Coords["frontend"]
	assert.False(t, ok,
		"an ambiguous name carries no coordinate, so no lease is minted blind")

	var recorded bool
	for _, p := range res.Problems {
		if p.Repo == "frontend" && strings.Contains(p.Err.Error(), "not unique") {
			recorded = true
		}
	}
	assert.True(t, recorded, "the collision is recorded as a problem")
	assert.Len(t, res.Plans, 2, "both repos' plans are still gathered")
}

// TestGatherKeepsAUniqueCoordinate: a repository whose basename is
// unique under the root keeps its coordinate even when the fleet holds
// other repos — the collision guard withholds only the name it shares.
func TestGatherKeepsAUniqueCoordinate(t *testing.T) {
	root := t.TempDir()
	repoWithPlan(t, filepath.Join(root, "a"), "frontend", 7)
	repoWithPlan(t, filepath.Join(root, "b"), "frontend", 9)
	repoWithPlan(t, root, "atlas", 3)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	_, ok := res.Coords["atlas"]
	assert.True(t, ok, "the unique name keeps its coordinate")
	_, ok = res.Coords["frontend"]
	assert.False(t, ok, "the shared name does not")
}

// planByID finds the gathered plan with the given id. The test that
// calls it has already set up a fixture where the id must be present,
// so a miss is a fixture bug, not a case under test.
func planByID(t *testing.T, res Result, id int64) discovery.Plan {
	t.Helper()
	for _, p := range res.Plans {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("no gathered plan with id %d", id)

	return discovery.Plan{}
}

// TestGatherLeavesAMarkerlessBranchUnheld: a hand-made plan/<id>
// branch of plain commits matches the holds pattern by name alone. No
// frit marker is reachable from its tip, so it is not a hold.
func TestGatherLeavesAMarkerlessBranchUnheld(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)
	gitCmd(t, repo, "checkout", "-q", "-b", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "work.txt"), []byte("wip\n"), 0o600))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-q", "-m", "hand-made branch, no marker")
	gitCmd(t, repo, "checkout", "-q", "main")

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	assert.False(t, planByID(t, res, 7).Held,
		"a name match with no marker is not a hold")
}

// TestGatherReadsAClaimMarkerBeneathLaterWorkAsHeld: real work landed
// on top of a minted claim still reads as held — the marker only has
// to be reachable, not the tip.
func TestGatherReadsAClaimMarkerBeneathLaterWorkAsHeld(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)
	gitCmd(t, repo, "checkout", "-q", "-b", "plan/7")
	gitCmd(t, repo, "commit", "--allow-empty", "-q", "-m", "plan 7: claim")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "work.txt"), []byte("wip\n"), 0o600))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-q", "-m", "real work")
	gitCmd(t, repo, "checkout", "-q", "main")

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	assert.True(t, planByID(t, res, 7).Held,
		"the claim marker beneath the tip still counts")
}

// TestGatherReadsAMarkerOnlyBranchAsHeld: the claim marker is the
// whole branch, with no work commit on top of it yet.
func TestGatherReadsAMarkerOnlyBranchAsHeld(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)
	gitCmd(t, repo, "checkout", "-q", "-b", "plan/7")
	gitCmd(t, repo, "commit", "--allow-empty", "-q", "-m", "plan 7: claim")
	gitCmd(t, repo, "checkout", "-q", "main")

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	assert.True(t, planByID(t, res, 7).Held,
		"a bare claim marker is still a hold")
}

// TestGatherLeavesAReleasedTipUnheld pins the existing rule unchanged:
// a tip that is a release marker is a lease that ended, not a hold.
func TestGatherLeavesAReleasedTipUnheld(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)
	gitCmd(t, repo, "checkout", "-q", "-b", "plan/7")
	gitCmd(t, repo, "commit", "--allow-empty", "-q", "-m", "plan 7: claim")
	gitCmd(t, repo, "commit", "--allow-empty", "-q", "-m", "plan 7: release")
	gitCmd(t, repo, "checkout", "-q", "main")

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	assert.False(t, planByID(t, res, 7).Held,
		"a released tip stays not held")
}

// TestGatherReadsALegacyDecoratedHoldAsHeld: the old claim design's
// slug-carrying branch still carries a claim marker, so the migration
// path off it is not broken by the marker gate.
func TestGatherReadsALegacyDecoratedHoldAsHeld(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)
	gitCmd(t, repo, "checkout", "-q", "-b", "plan/7-shader")
	gitCmd(t, repo, "commit", "--allow-empty", "-q", "-m",
		"plan 7: claim shader")
	gitCmd(t, repo, "checkout", "-q", "main")

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe)
	require.NoError(t, err)

	assert.True(t, planByID(t, res, 7).Held,
		"a legacy decorated hold still carries a marker")
}
