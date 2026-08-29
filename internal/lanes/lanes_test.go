package lanes

import (
	"strings"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/repocfg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func canonical(t *testing.T) repocfg.Holds {
	t.Helper()
	h, err := repocfg.Default().Compiled()
	require.NoError(t, err)

	return h
}

func ref(name string) gitobj.Ref { return gitobj.Ref{Name: name} }

func wt(path, branch string) gitwt.Worktree {
	return gitwt.Worktree{
		Path:   path,
		Branch: branch,
		Head:   strings.Repeat("a", 40),
	}
}

func TestBuildPairsAClaimWithItsCheckout(t *testing.T) {
	got := Build(
		[]gitwt.Worktree{wt("/w/proj-fleet", "plan/42-fleet")},
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		nil, nil, canonical(t))

	require.Len(t, got, 1)
	assert.Equal(t, int64(42), got[0].PlanID)
	assert.Len(t, got[0].Holds, 1)
	assert.Len(t, got[0].Worktrees, 1)
	assert.False(t, got[0].Unstaffed())
}

// TestBuildDropsMergedRefs is the filter that keeps finished work out
// of the board: landing a plan does not delete its branch.
func TestBuildDropsMergedRefs(t *testing.T) {
	merged := map[string]bool{"refs/heads/plan/42-fleet": true}

	got := Build(nil,
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		merged, nil, canonical(t))

	assert.Empty(t, got, "a landed plan is not an active claim")
}

// TestBuildDropsLandedClaims closes the squash-merge gap: a claim whose
// plan is done on the default branch is landed work, even when its
// branch is no ancestor of that branch, so the merged set never lists
// it. The landed set drops it by plan id.
func TestBuildDropsLandedClaims(t *testing.T) {
	landed := map[int64]bool{42: true}

	got := Build(nil,
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		nil, landed, canonical(t))

	assert.Empty(t, got, "a plan done on the default branch is not a hold")
}

func TestBuildTreatsALocalAndRemoteClaimAsOneLane(t *testing.T) {
	got := Build(nil, []gitobj.Ref{
		ref("refs/heads/plan/42-fleet"),
		ref("refs/remotes/origin/plan/42-fleet"),
		ref("refs/remotes/peer/plan/42-fleet"),
	}, nil, nil, canonical(t))

	require.Len(t, got, 1, "one plan, three refs")
	assert.Len(t, got[0].Holds, 3)
}

func TestBuildIgnoresRefsNoPatternClaims(t *testing.T) {
	got := Build(nil, []gitobj.Ref{
		ref("refs/heads/main"),
		ref("refs/heads/backup/plan/42-fleet"),
		ref("refs/tags/plan/42-fleet"),
		ref("refs/heads/v0.42"),
	}, nil, nil, canonical(t))

	assert.Empty(t, got)
}

func TestBuildIgnoresAWorktreeOnNoClaimedBranch(t *testing.T) {
	got := Build(
		[]gitwt.Worktree{wt("/w/proj", "main")},
		nil, nil, nil, canonical(t))

	assert.Empty(t, got)
}

func TestBuildWithNoPatternsFindsNothing(t *testing.T) {
	none, err := repocfg.CompileAll(nil)
	require.NoError(t, err)

	got := Build(
		[]gitwt.Worktree{wt("/w/proj-fleet", "plan/42-fleet")},
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		nil, nil, none)

	assert.Empty(t, got, "a repo declaring no pattern has no lanes")
}

func TestBuildIsSortedByPlanID(t *testing.T) {
	got := Build(nil, []gitobj.Ref{
		ref("refs/heads/plan/90-b"),
		ref("refs/heads/plan/10-a"),
	}, nil, nil, canonical(t))

	require.Len(t, got, 2)
	assert.Equal(t, int64(10), got[0].PlanID)
	assert.Equal(t, int64(90), got[1].PlanID)
}

func TestFindReportsAClaimWithNoCheckout(t *testing.T) {
	built := Build(nil,
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		nil, nil, canonical(t))

	got := Find(built, nil)

	require.Len(t, got.Unstaffed, 1)
	assert.Equal(t, int64(42), got.Unstaffed[0].PlanID)
	assert.True(t, got.Any())
}

// TestFindFlagsADecoratedHoldAsMigratable: a legacy decorated branch
// still reads as a hold — nothing here refuses it — but it is also
// named as a migration candidate to the id-only ref the lease
// protocol writes, so a repository can move off it without a flag
// day.
func TestFindFlagsADecoratedHoldAsMigratable(t *testing.T) {
	built := Build(nil,
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		nil, nil, canonical(t))

	got := Find(built, nil)

	require.Len(t, got.Migratable, 1)
	assert.Equal(t, int64(42), got.Migratable[0].PlanID)
	assert.Equal(t, "plan/42-fleet", got.Migratable[0].From)
	assert.Equal(t, "plan/42", got.Migratable[0].To)
	assert.True(t, got.Any())
}

// TestFindLeavesAnIDOnlyHoldOffTheMigrationList: the lease protocol's
// own ref shape is already the target, so it is never its own
// migration candidate.
func TestFindLeavesAnIDOnlyHoldOffTheMigrationList(t *testing.T) {
	built := Build(nil,
		[]gitobj.Ref{ref("refs/heads/plan/42")},
		nil, nil, canonical(t))

	got := Find(built, nil)

	assert.Empty(t, got.Migratable)
}

// TestBuildLeavesAMergedBranchsCheckoutStranded is the shape this whole
// plan turns on: the ref merged and was dropped, but the worktree loop
// has no such filter, so the lane keeps a checkout with no hold.
func TestBuildLeavesAMergedBranchsCheckoutStranded(t *testing.T) {
	merged := map[string]bool{"refs/heads/plan/42-fleet": true}

	got := Build(
		[]gitwt.Worktree{wt("/w/proj-fleet", "plan/42-fleet")},
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		merged, nil, canonical(t))

	require.Len(t, got, 1)
	assert.Empty(t, got[0].Holds, "the merged ref was dropped")
	assert.Len(t, got[0].Worktrees, 1)
	assert.True(t, got[0].Stranded())
	assert.False(t, got[0].Unstaffed())
}

// TestStrandedIsNeitherAnUnstaffedNorAStaffedLane pins the predicate
// against the two shapes it must not claim: a hold with no checkout is
// unstaffed, and a hold with a checkout is healthy.
func TestStrandedIsNeitherAnUnstaffedNorAStaffedLane(t *testing.T) {
	unstaffed := Lane{Holds: []Hold{{PlanID: 1}}}
	staffed := Lane{
		Holds:     []Hold{{PlanID: 1}},
		Worktrees: []gitwt.Worktree{wt("/w/a", "plan/1-a")},
	}

	assert.False(t, unstaffed.Stranded())
	assert.False(t, staffed.Stranded())
}

func TestFindReportsACheckoutStrandedOnALandedBranch(t *testing.T) {
	built := Build(
		[]gitwt.Worktree{wt("/w/proj-fleet", "plan/42-fleet")},
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		map[string]bool{"refs/heads/plan/42-fleet": true}, nil, canonical(t))

	got := Find(built, nil)

	require.Len(t, got.Stranded, 1)
	assert.Equal(t, int64(42), got.Stranded[0].PlanID)
	assert.Empty(t, got.Unstaffed, "a stranded lane is not an unstaffed one")
	assert.True(t, got.Any())
}

// TestFindReportsAPrunableStrandedCheckoutOnceAsPrunable holds the
// "one worktree, one complaint" line for a landed branch: a checkout git
// already considers removable is prunable, not "still checked out", so it
// must not be reported as stranded as well.
func TestFindReportsAPrunableStrandedCheckoutOnceAsPrunable(t *testing.T) {
	gone := gitwt.Worktree{
		Path:     "/w/proj-fleet",
		Branch:   "plan/42-fleet",
		Head:     strings.Repeat("a", 40),
		Prunable: true,
	}
	built := Build(
		[]gitwt.Worktree{gone},
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		map[string]bool{"refs/heads/plan/42-fleet": true}, nil, canonical(t))

	got := Find(built, []gitwt.Worktree{gone})

	require.Len(t, got.Prunable, 1)
	assert.Empty(t, got.Stranded, "a prunable checkout is not still checked out")
}

// TestStandingKeepsOnlyTheCheckoutsStillOnDisk pins the helper that
// keeps stranded from double-counting a prunable worktree: the standing
// checkout survives, the prunable one is dropped.
func TestStandingKeepsOnlyTheCheckoutsStillOnDisk(t *testing.T) {
	lane := Lane{
		PlanID: 7,
		Worktrees: []gitwt.Worktree{
			wt("/w/here", "plan/7-a"),
			{Path: "/w/gone", Branch: "plan/7-a", Prunable: true},
		},
	}

	got := lane.standing()

	require.Len(t, got.Worktrees, 1)
	assert.Equal(t, "/w/here", got.Worktrees[0].Path)
	assert.Equal(t, int64(7), got.PlanID)
}

func TestFindReportsAWorktreeThatNeverStarted(t *testing.T) {
	empty := gitwt.Worktree{
		Path:   "/w/proj-fleet",
		Branch: "plan/42-fleet",
		Head:   strings.Repeat("0", 40),
	}

	got := Find(nil, []gitwt.Worktree{empty})

	require.Len(t, got.Empty, 1)
	assert.Equal(t, "/w/proj-fleet", got.Empty[0].Path)
}

func TestFindReportsPrunableAheadOfEmpty(t *testing.T) {
	gone := gitwt.Worktree{
		Path:     "/w/gone",
		Branch:   "plan/42-x",
		Head:     strings.Repeat("0", 40),
		Prunable: true,
	}

	got := Find(nil, []gitwt.Worktree{gone})

	require.Len(t, got.Prunable, 1)
	assert.Empty(t, got.Empty, "one worktree, one complaint")
}

func TestFindIgnoresABareRepository(t *testing.T) {
	got := Find(nil, []gitwt.Worktree{{Path: "/w/mirror", Bare: true}})

	assert.False(t, got.Any(), "a bare repo has no working tree")
}

func TestFindOnAHealthyRepositoryIsSilent(t *testing.T) {
	built := Build(
		[]gitwt.Worktree{wt("/w/proj-fleet", "plan/42-fleet")},
		[]gitobj.Ref{ref("refs/heads/plan/42-fleet")},
		nil, nil, canonical(t))

	got := Find(built, []gitwt.Worktree{wt("/w/proj-fleet", "plan/42-fleet")})

	assert.False(t, got.Any())
}

// TestFindNamesAForeignCheckoutOffTheRecordedLanePath pins gap 2: a
// lane with a live hold recording lane path A and a worktree checked
// out on the branch at path B — neither Unstaffed (it has a checkout)
// nor Stranded (it has a live hold) — is reported as a foreign
// checkout instead of swallowed as healthy.
func TestFindNamesAForeignCheckoutOffTheRecordedLanePath(t *testing.T) {
	worktrees := []gitwt.Worktree{wt("/lanes/b", "plan/42")}
	built := []Lane{{
		PlanID: 42,
		Holds: []Hold{{
			Ref: "refs/heads/plan/42", Branch: "plan/42", PlanID: 42,
			Lane: "/lanes/a",
		}},
		Worktrees: worktrees,
	}}

	got := Find(built, worktrees)

	require.Len(t, got.Foreign, 1)
	assert.Equal(t, int64(42), got.Foreign[0].PlanID)
	assert.Equal(t, "/lanes/b", got.Foreign[0].Worktree.Path)
	assert.Empty(t, got.Unstaffed, "the lane has a checkout")
	assert.Empty(t, got.Stranded, "the lane has a live hold")
	assert.True(t, got.Any())
}

// TestFindDoesNotFlagACheckoutAtItsOwnRecordedLanePath: the healthy
// shape — a checkout standing exactly where its own hold's marker
// recorded it — is not a foreign checkout.
func TestFindDoesNotFlagACheckoutAtItsOwnRecordedLanePath(t *testing.T) {
	worktrees := []gitwt.Worktree{wt("/lanes/a", "plan/42")}
	built := []Lane{{
		PlanID: 42,
		Holds: []Hold{{
			Ref: "refs/heads/plan/42", Branch: "plan/42", PlanID: 42,
			Lane: "/lanes/a",
		}},
		Worktrees: worktrees,
	}}

	got := Find(built, worktrees)

	assert.Empty(t, got.Foreign)
}

// TestFindDoesNotFlagAForeignCheckoutWhenTheLanePathIsUnknown: a hold
// whose marker was never read authorizes no path at all, so nothing is
// flagged — the fail-safe direction, since a blank read must not turn
// every checkout foreign.
func TestFindDoesNotFlagAForeignCheckoutWhenTheLanePathIsUnknown(t *testing.T) {
	worktrees := []gitwt.Worktree{wt("/lanes/b", "plan/42")}
	built := []Lane{{
		PlanID:    42,
		Holds:     []Hold{{Ref: "refs/heads/plan/42", Branch: "plan/42", PlanID: 42}},
		Worktrees: worktrees,
	}}

	got := Find(built, worktrees)

	assert.Empty(t, got.Foreign, "an unread marker authorizes no path")
}

// TestFindDoesNotFlagACheckoutAuthorizedByASiblingHoldOnTheSameLane:
// a lane migrating off a decorated hold carries two live holds at
// once (Migratable's own shape) — the canonical hold's own recorded
// Lane must not be checked against a checkout standing on the
// DECORATED hold's branch. Before this fix, foreignCheckouts pooled
// every hold's Lane into one set for the whole lane, so the decorated
// branch's own genuine checkout was flagged foreign — and reap --go
// would tear it down — merely because a sibling hold happened to
// record a different path.
func TestFindDoesNotFlagACheckoutAuthorizedByASiblingHoldOnTheSameLane(t *testing.T) {
	worktrees := []gitwt.Worktree{wt("/lanes/shader", "plan/42-shader")}
	built := []Lane{{
		PlanID: 42,
		Holds: []Hold{
			{Ref: "refs/heads/plan/42", Branch: "plan/42", PlanID: 42,
				Lane: "/lanes/canonical"},
			{Ref: "refs/heads/plan/42-shader", Branch: "plan/42-shader", PlanID: 42},
		},
		Worktrees: worktrees,
	}}

	got := Find(built, worktrees)

	assert.Empty(t, got.Foreign,
		"the decorated hold's own checkout is legitimate, "+
			"even though the canonical hold recorded a different lane")
}

// TestWithLanePathsFillsInHoldsFromTheGivenMap: the marker's lane:
// trailer, read wherever the caller has git access to do so, rides on
// the hold it belongs to by ref name. A hold missing from the map is
// left blank, the same as one whose marker was never read; Build
// itself stays pure and untouched.
func TestWithLanePathsFillsInHoldsFromTheGivenMap(t *testing.T) {
	built := []Lane{{
		PlanID: 42,
		Holds: []Hold{
			{Ref: "refs/heads/plan/42", Branch: "plan/42", PlanID: 42},
			{Ref: "refs/remotes/origin/plan/42", Branch: "plan/42", PlanID: 42},
		},
	}}

	got := WithLanePaths(built, map[string]string{
		"refs/heads/plan/42": "/lanes/a",
	})

	require.Len(t, got, 1)
	require.Len(t, got[0].Holds, 2)
	assert.Equal(t, "/lanes/a", got[0].Holds[0].Lane)
	assert.Equal(t, "", got[0].Holds[1].Lane, "a ref missing from the map is left blank")
}

func TestStaleMeasuresFromTheBranchTip(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	old := now.Add(-40 * 24 * time.Hour).Unix()
	fresh := now.Add(-1 * time.Hour).Unix()
	times := map[string]int64{
		"refs/heads/plan/1-old":   old,
		"refs/heads/plan/2-fresh": fresh,
	}

	got := Stale([]gitwt.Worktree{
		wt("/w/a", "plan/1-old"),
		wt("/w/b", "plan/2-fresh"),
	}, times, now, 30*24*time.Hour)

	require.Len(t, got, 1)
	assert.Equal(t, "/w/a", got[0].Worktree.Path)
	assert.Greater(t, got[0].Age, 39*24*time.Hour)
}

func TestStaleSortsOldestFirst(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	times := map[string]int64{
		"refs/heads/plan/1-a": now.Add(-100 * 24 * time.Hour).Unix(),
		"refs/heads/plan/2-b": now.Add(-50 * 24 * time.Hour).Unix(),
	}

	got := Stale([]gitwt.Worktree{
		wt("/w/b", "plan/2-b"),
		wt("/w/a", "plan/1-a"),
	}, times, now, time.Hour)

	require.Len(t, got, 2)
	assert.Equal(t, "/w/a", got[0].Worktree.Path)
}

func TestStaleSkipsWhatItCannotDate(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	noCommit := gitwt.Worktree{
		Path: "/w/empty", Branch: "plan/1-x",
		Head: strings.Repeat("0", 40),
	}
	detached := gitwt.Worktree{
		Path: "/w/det", Detached: true,
		Head: strings.Repeat("a", 40),
	}

	got := Stale([]gitwt.Worktree{
		noCommit,
		detached,
		wt("/w/unknown", "plan/9-untracked"),
	}, map[string]int64{}, now, time.Hour)

	assert.Empty(t, got, "the orphan report covers these instead")
}
