package fleet

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitOut runs git in dir for test setup and returns its trimmed stdout.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)

	return strings.TrimSpace(string(out))
}

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

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
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

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
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

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
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

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
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

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
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

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
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

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
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

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
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

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
	require.NoError(t, err)

	assert.True(t, planByID(t, res, 7).Held,
		"a legacy decorated hold still carries a marker")
}

// TestGatherLeavesHoldTipEmptyForADecoratedOnlyHold pins a deliberate
// limit: HoldTip is only ever the bare, id-only plan/<id> ref's tip —
// the one ref Takeover's CAS targets — never a decorated or legacy
// branch's, even when that branch alone is what makes Held true. A
// plan held only through such a branch stays outside the staleness
// window and the dead-session read (observeHolds's HoldTip == ""
// guard), because there is no id-only ref a takeover could seize
// anyway; seeding a tip from the decorated branch would let Stale or
// Dead mature and send claim's takeover at the bare ref regardless,
// which does not exist, turning the attempt into a raw push failure
// instead of a graceful refusal.
// TestGatherReadsASquashLandedPlanAsLandedThoughLocalMainLags pins
// S84/S85: a working checkout's local main advances only on an
// explicit merge or pull, so it routinely lags the remote-tracking
// default a squash merge actually lands on. origin/main carries plan
// 7 at done while the local main still carries it not-started, and
// origin/HEAD is unset. The plan's claim branch still exists — a
// squash merge does not delete it — but the plan must read as
// landed, not held.
func TestGatherReadsASquashLandedPlanAsLandedThoughLocalMainLags(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)

	gitCmd(t, repo, "checkout", "-q", "-b", "plan/7")
	gitCmd(t, repo, "commit", "--allow-empty", "-q", "-m", "plan 7: claim")
	gitCmd(t, repo, "checkout", "-q", "main")

	gitCmd(t, repo, "checkout", "-q", "--detach", "main")
	body := "---\nid: 7\ntitle: Shader unit\nstatus: \"✅\"\n---\n# Shader unit\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "plan", "plan.md"), []byte(body), 0o600))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-q", "-m", "land plan 7 (squash)")
	gitCmd(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitCmd(t, repo, "checkout", "-q", "main")

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
	require.NoError(t, err)

	assert.False(t, planByID(t, res, 7).Held,
		"a squash-landed plan reads as landed though local main lags")
}

func TestGatherLeavesHoldTipEmptyForADecoratedOnlyHold(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)
	gitCmd(t, repo, "checkout", "-q", "-b", "plan/7-shader")
	gitCmd(t, repo, "commit", "--allow-empty", "-q", "-m",
		"plan 7: claim shader")
	gitCmd(t, repo, "checkout", "-q", "main")

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
	require.NoError(t, err)

	p := planByID(t, res, 7)
	require.True(t, p.Held, "the decorated branch's marker still holds the plan")
	assert.Empty(t, p.HoldTip,
		"no bare id-only ref exists, so there is no tip a takeover CAS could target")
}

// TestGatherReportsALocalDefaultBranchLaggingItsFetchedRemote: a fetch
// updates refs/remotes/origin/main without touching the local main a
// worktree sits on — ordinary git, not a bug. Gather already has both
// refs from its own walk, so it names the gap rather than silently
// reading the stale local copy (S80).
func TestGatherReportsALocalDefaultBranchLaggingItsFetchedRemote(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)

	gitCmd(t, repo, "checkout", "-q", "-b", "tmp-ahead")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "extra.txt"), []byte("more\n"), 0o600))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-q", "-m", "landed on origin")
	ahead := gitOut(t, repo, "rev-parse", "HEAD")
	gitCmd(t, repo, "checkout", "-q", "main")
	gitCmd(t, repo, "branch", "-D", "tmp-ahead")
	gitCmd(t, repo, "update-ref", "refs/remotes/origin/main", ahead)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
	require.NoError(t, err)

	var found *Problem
	for i, p := range res.Problems {
		if p.Repo == "atlas" {
			found = &res.Problems[i]
		}
	}
	require.NotNil(t, found, "the lag is recorded as a problem")
	assert.Contains(t, found.Err.Error(), "1 commit")
}

// TestGatherLeavesAnInSyncDefaultBranchProblemless: a local default
// branch matching its remote-tracking ref exactly is not behind
// anything, so no problem is recorded.
func TestGatherLeavesAnInSyncDefaultBranchProblemless(t *testing.T) {
	root := t.TempDir()
	repo := repoWithPlan(t, root, "atlas", 7)
	head := gitOut(t, repo, "rev-parse", "HEAD")
	gitCmd(t, repo, "update-ref", "refs/remotes/origin/main", head)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
	require.NoError(t, err)

	for _, p := range res.Problems {
		assert.NotEqual(t, "atlas", p.Repo, "an in-sync branch is not a problem")
	}
}

// landedPlanWithStaleOrigin builds a real clone of an origin whose main
// has since landed the plan and deleted its lease branch, while the
// clone's own refs/remotes/origin/* still carries the pre-merge state
// — a checkout that has not fetched since, not a faked ref. It returns
// the root Gather walks, holding only the clone.
func landedPlanWithStaleOrigin(t *testing.T, name string, id int) string {
	t.Helper()
	branch := "plan/" + strconv.Itoa(id)

	origin := repoWithPlan(t, t.TempDir(), name, id)
	gitCmd(t, origin, "checkout", "-q", "-b", branch)
	gitCmd(t, origin, "commit", "--allow-empty", "-q", "-m",
		"plan "+strconv.Itoa(id)+": claim")
	gitCmd(t, origin, "checkout", "-q", "main")

	root := t.TempDir()
	gitCmd(t, root, "clone", "-q", origin, filepath.Join(root, name))

	body := "---\nid: " + strconv.Itoa(id) +
		"\ntitle: Shader unit\nstatus: \"✅\"\n---\n# Shader unit\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(origin, "plan", "plan.md"), []byte(body), 0o600))
	gitCmd(t, origin, "add", "-A")
	gitCmd(t, origin, "commit", "-q", "-m", "land plan (squash)")
	gitCmd(t, origin, "branch", "-D", branch)

	return root
}

// TestGatherFetchesBeforeReadingLandedEvidence pins the fix this plan
// exists for: a squash-landed, branch-deleted plan reads landed only
// once Gather fetches. Before the fetch this reads as held; after, as
// landed.
func TestGatherFetchesBeforeReadingLandedEvidence(t *testing.T) {
	root := landedPlanWithStaleOrigin(t, "atlas", 7)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe,
		Options{Fetch: true})
	require.NoError(t, err)

	assert.False(t, planByID(t, res, 7).Held,
		"the lease branch is gone on origin; a fetch must see that "+
			"before the plan reads held")
}

// TestGatherWithoutFetchStillReadsHeld pins the contrast: the very
// same stale-origin fixture, read without a fetch, still reads the
// deleted lease branch as held off its stale remote-tracking copy.
func TestGatherWithoutFetchStillReadsHeld(t *testing.T) {
	root := landedPlanWithStaleOrigin(t, "atlas", 7)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe,
		Options{})
	require.NoError(t, err)

	assert.True(t, planByID(t, res, 7).Held,
		"without a fetch, the stale remote-tracking lease branch "+
			"still reads as held")
}

// TestGatherFetchSkipsARepositoryWithNoRemoteConfigured: a repository
// that has never had a remote added has nothing for a fetch to
// refresh, so asking Gather to fetch does not fail the whole walk —
// it reads the repo exactly as an unfetched one always has.
func TestGatherFetchSkipsARepositoryWithNoRemoteConfigured(t *testing.T) {
	root := t.TempDir()
	repoWithPlan(t, root, "atlas", 7)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe,
		Options{Fetch: true})
	require.NoError(t, err)

	assert.False(t, planByID(t, res, 7).Held,
		"a repo with no remote gathers normally; a requested fetch is "+
			"simply skipped")
}

// recordingRunner wraps Exec, appending each subcommand's verb to
// calls, so a test can assert which git subprocesses Gather issued.
func recordingRunner(calls *[]string) gitwt.Runner {
	return func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 {
			*calls = append(*calls, args[0])
		}

		return gitwt.Exec(dir, args...)
	}
}

// repoWithUnreachableRemote builds a repository whose origin is
// configured but points nowhere, and whose refs/remotes/origin/* still
// carry a stale copy — the mark that a remote is configured. A fetch
// against it always fails, standing in for an offline checkout.
func repoWithUnreachableRemote(t *testing.T, root, name string, id int) string {
	t.Helper()
	dir := repoWithPlan(t, root, name, id)
	gitCmd(t, dir, "remote", "add", "origin",
		filepath.Join(t.TempDir(), "gone.git"))
	gitCmd(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")

	return dir
}

// TestGatherWithoutFetchIssuesNoFetchSubprocess: with Fetch off, Gather
// reads the local view and issues no fetch at all.
func TestGatherWithoutFetchIssuesNoFetchSubprocess(t *testing.T) {
	root := landedPlanWithStaleOrigin(t, "atlas", 7)

	var calls []string
	_, err := Gather(root, "testhost", recordingRunner(&calls),
		gitwt.ExecPipe, Options{})
	require.NoError(t, err)

	assert.NotContains(t, calls, "fetch",
		"with Fetch off, Gather issues no fetch subprocess")
}

// TestGatherFetchErrorFallsBackAndNamesStaleness: an unreachable remote
// does not fail the walk. Gather still reads the plan off the local
// view and records one staleness problem naming the repo, because a
// refs/remotes/origin/* ref shows a remote is configured.
func TestGatherFetchErrorFallsBackAndNamesStaleness(t *testing.T) {
	root := t.TempDir()
	repoWithUnreachableRemote(t, root, "atlas", 7)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe,
		Options{Fetch: true})
	require.NoError(t, err)

	assert.Equal(t, int64(7), planByID(t, res, 7).ID,
		"the failed fetch falls back to the local view, not off the walk")

	var found *Problem
	for i, p := range res.Problems {
		if p.Repo == "atlas" {
			found = &res.Problems[i]
		}
	}
	require.NotNil(t, found,
		"an unreachable remote records a staleness problem")
	assert.Contains(t, found.Err.Error(), "stale")
}

// TestGatherFetchErrorOnALocalOnlyRepoRecordsNoStaleness: a repository
// with no remote at all is local-only. A failed fetch there is
// expected, not stale, so no problem is recorded.
func TestGatherFetchErrorOnALocalOnlyRepoRecordsNoStaleness(t *testing.T) {
	root := t.TempDir()
	repoWithPlan(t, root, "atlas", 7)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe,
		Options{Fetch: true})
	require.NoError(t, err)

	assert.Equal(t, int64(7), planByID(t, res, 7).ID,
		"a local-only repo gathers from its own view")
	for _, p := range res.Problems {
		assert.NotEqual(t, "atlas", p.Repo,
			"a local-only repo's failed fetch is expected, not stale")
	}
}

// TestGatherLeavesAnUnfetchedDefaultBranchProblemless: a repository
// with no remote-tracking ref at all — never fetched, or no remote
// configured — has nothing to compare against, so it is not flagged.
func TestGatherLeavesAnUnfetchedDefaultBranchProblemless(t *testing.T) {
	root := t.TempDir()
	repoWithPlan(t, root, "atlas", 7)

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{Fetch: true})
	require.NoError(t, err)

	for _, p := range res.Problems {
		assert.NotEqual(t, "atlas", p.Repo,
			"no remote-tracking ref means nothing to compare")
	}
}

// TestGatherReportsASkippedRepositoryAsAProblem: a candidate git
// refuses to answer for does not vanish from the walk — it reaches
// Gather's own Problems, the same channel a fetch failure already
// reports through, and every other repository still gathers.
func TestGatherReportsASkippedRepositoryAsAProblem(t *testing.T) {
	root := t.TempDir()
	repoWithPlan(t, root, "atlas", 7)

	broken := filepath.Join(root, "broken")
	require.NoError(t, os.MkdirAll(broken, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(broken, ".git"),
		[]byte("gitdir: /nonexistent\n"), 0o600))

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{})
	require.NoError(t, err)

	assert.Equal(t, int64(7), planByID(t, res, 7).ID,
		"the good repo still gathers")

	var found bool
	for _, p := range res.Problems {
		if p.Repo == "broken" {
			found = true
		}
	}
	assert.True(t, found,
		"a repo git refuses to answer for is a problem, not a silent drop")
}

// TestGatherReportsAMislaidPlanAsAProblem: a plan-like file dropped
// somewhere plans.Collect will not read it — one directory too deep —
// surfaces as a Problem the gather carries, not a silent drop, and it
// is not the benign NotPlan kind: a mislaid plan is a real problem.
func TestGatherReportsAMislaidPlanAsAProblem(t *testing.T) {
	root := t.TempDir()
	dir := repoWithPlan(t, root, "atlas", 7)

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "plan", "archive"), 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "plan", "archive", "8_mislaid.md"),
		[]byte("---\nid: 8\ntitle: Mislaid\nstatus: \"🔲\"\n---\n# Mislaid\n"),
		0o600))
	gitCmd(t, dir, "add", "-A")
	gitCmd(t, dir, "commit", "-q", "-m", "mislay a plan")

	res, err := Gather(root, "testhost", gitwt.Exec, gitwt.ExecPipe, Options{})
	require.NoError(t, err)

	var found *Problem
	for i, p := range res.Problems {
		if p.Repo == "atlas" &&
			strings.Contains(p.Err.Error(), "archive/8_mislaid.md") {
			found = &res.Problems[i]
		}
	}
	require.NotNil(t, found, "a mislaid plan is reported, not lost")
	assert.False(t, found.NotPlan,
		"a mislaid plan is a real problem, not the benign not-a-plan kind")
}
