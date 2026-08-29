package report

import (
	"errors"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func found() lanes.Orphans {
	return lanes.Orphans{
		Unstaffed: []lanes.Lane{{
			PlanID: 42,
			Holds: []lanes.Hold{{
				Ref:    "refs/heads/plan/42-x",
				Branch: "plan/42-x",
				PlanID: 42,
			}},
		}},
		Stranded: []lanes.Lane{{
			PlanID: 99,
			Worktrees: []gitwt.Worktree{{
				Path:   "/fleet/atlas-landed",
				Branch: "plan/99-z",
				Head:   "d00dfeed",
			}},
		}},
		Empty: []gitwt.Worktree{
			{Path: "/fleet/atlas-empty", Branch: "plan/7-y", Head: zero},
		},
		Prunable: []gitwt.Worktree{
			{
				Path:        "/fleet/atlas-gone",
				Head:        "a1b2c3",
				Prunable:    true,
				PruneReason: "gitdir file points to non-existent location",
			},
		},
		Migratable: []lanes.Migratable{
			{PlanID: 42, From: "plan/42-x", To: "plan/42"},
		},
		Foreign: []lanes.ForeignCheckout{
			{
				PlanID: 2608291751,
				Worktree: gitwt.Worktree{
					Path:   "/fleet/atlas-off-lane",
					Branch: "plan/2608291751",
					Head:   "cafef00d",
				},
			},
		},
	}
}

func TestOrphansKeepsTheKindsApart(t *testing.T) {
	doc := NewOrphans("/fleet")
	doc.AddRepo("atlas", found())

	assert.Equal(t, Schema, doc.Schema)
	assert.Equal(t, "orphans", doc.Command)
	require.Len(t, doc.Repos, 1)

	repo := doc.Repos[0]
	require.Len(t, repo.Unstaffed, 1)
	assert.Equal(t, int64(42), repo.Unstaffed[0].PlanID)
	require.Len(t, repo.Unstaffed[0].Holds, 1)
	assert.Equal(t, "plan/42-x", repo.Unstaffed[0].Holds[0].Branch)
	require.Len(t, repo.Stranded, 1)
	assert.Equal(t, int64(99), repo.Stranded[0].PlanID)
	require.Len(t, repo.Stranded[0].Worktrees, 1)
	assert.Equal(t, "atlas-landed", repo.Stranded[0].Worktrees[0].Name)
	assert.Equal(t, "plan/99-z", repo.Stranded[0].Worktrees[0].Branch)
	require.Len(t, repo.Empty, 1)
	assert.Equal(t, "atlas-empty", repo.Empty[0].Name)
	require.Len(t, repo.Prunable, 1)
	assert.Equal(t, "atlas-gone", repo.Prunable[0].Name)
	require.Len(t, repo.Migratable, 1)
	assert.Equal(t, int64(42), repo.Migratable[0].PlanID)
	assert.Equal(t, "plan/42-x", repo.Migratable[0].From)
	assert.Equal(t, "plan/42", repo.Migratable[0].To)
	require.Len(t, repo.Foreign, 1)
	assert.Equal(t, int64(2608291751), repo.Foreign[0].PlanID)
	assert.Equal(t, "atlas-off-lane", repo.Foreign[0].Worktree.Name)
	assert.Equal(t, "plan/2608291751", repo.Foreign[0].Worktree.Branch)
}

// TestOrphansKeepsCleanRepositories is what the table cannot say. A
// repository with nothing wrong is skipped in print, so a consumer
// reading the table cannot tell a clean repository from one that was
// never walked. The document lists both, one with empty lists.
func TestOrphansKeepsCleanRepositories(t *testing.T) {
	doc := NewOrphans("/fleet")
	doc.AddRepo("clean", lanes.Orphans{})
	doc.AddProblem("broken", errors.New("no such worktree"))

	require.Len(t, doc.Repos, 1)
	assert.Equal(t, "clean", doc.Repos[0].Name)
	assert.False(t, doc.Repos[0].Any())
	assert.NotNil(t, doc.Repos[0].Unstaffed)
	assert.NotNil(t, doc.Repos[0].Stranded)
	assert.NotNil(t, doc.Repos[0].Empty)
	assert.NotNil(t, doc.Repos[0].Prunable)
	assert.NotNil(t, doc.Repos[0].Migratable)
	assert.NotNil(t, doc.Repos[0].Foreign)
	assert.NotNil(t, doc.Repos[0].StaleHolds)
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "broken", doc.Problems[0].Repo)
	assert.NotNil(t, doc.Repos[0].Deserted)
	assert.NotNil(t, doc.Repos[0].Rescued)
}

func TestOrphanRepoAnyReportsWhateverWasFound(t *testing.T) {
	assert.False(t, OrphanRepo{}.Any())
	assert.True(t, OrphanRepo{Unstaffed: []Lane{{}}}.Any())
	assert.True(t, OrphanRepo{Stranded: []StrandedLane{{}}}.Any())
	assert.True(t, OrphanRepo{Empty: []Worktree{{}}}.Any())
	assert.True(t, OrphanRepo{Prunable: []Worktree{{}}}.Any())
	assert.True(t, OrphanRepo{Migratable: []Migratable{{}}}.Any())
	assert.True(t, OrphanRepo{Foreign: []ForeignCheckout{{}}}.Any())
	assert.True(t, OrphanRepo{StaleHolds: []StaleHold{{}}}.Any())
	assert.True(t, OrphanRepo{Deserted: []Deserted{{}}}.Any())
	assert.True(t, OrphanRepo{Rescued: []Rescued{{}}}.Any(),
		"a repository whose only finding is a rescue ref still renders")
}

// TestOrphansAddDesertedRecordsADeadEnd: the deserted cell of the
// verb-state table — a held plan herdr confirms lost its session,
// surfaced beside orphans' other kinds, distinct from a matured
// StaleHold.
func TestOrphansAddDesertedRecordsADeadEnd(t *testing.T) {
	doc := NewOrphans("/fleet")
	doc.AddRepo("atlas", lanes.Orphans{})

	doc.AddDeserted("atlas", []discovery.Plan{
		{ID: 42, Holds: []string{"plan/42"}},
	})

	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Deserted, 1)
	assert.Equal(t, int64(42), doc.Repos[0].Deserted[0].PlanID)
	assert.Equal(t, "plan/42", doc.Repos[0].Deserted[0].Branch)
	assert.True(t, doc.Repos[0].Any())
}

// TestOrphansAddStaleRecordsAMaturedHold: the held-stale cell of the
// verb-state table — a matured lease reads as a takeover candidate,
// beside orphans' other kinds, sourced from the same observation fold
// board and claim read rather than lanes.Find's git-ref sweep.
func TestOrphansAddStaleRecordsAMaturedHold(t *testing.T) {
	doc := NewOrphans("/fleet")
	doc.AddRepo("atlas", lanes.Orphans{})

	doc.AddStale("atlas", []discovery.Plan{
		{ID: 42, Holds: []string{"plan/42"}, StaleFor: 3 * time.Hour},
	})

	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].StaleHolds, 1)
	assert.Equal(t, int64(42), doc.Repos[0].StaleHolds[0].PlanID)
	assert.Equal(t, "plan/42", doc.Repos[0].StaleHolds[0].Branch)
	assert.Equal(t, int64(3*60*60), doc.Repos[0].StaleHolds[0].StaleSeconds)
	assert.True(t, doc.Repos[0].Any())
}

// TestOrphansAddRescuedRecordsLeftoverParks: the rescued cell of the
// verb-state table — a rescue ref found before anyone triggers the
// blocked park it stands for, beside orphans' other kinds. A
// superseded plan's parked work must read as superseded, never landed
// (the ByPlanID bool a squash-merge landed check collapses both into
// cannot tell them apart on its own).
func TestOrphansAddRescuedRecordsLeftoverParks(t *testing.T) {
	doc := NewOrphans("/fleet")
	doc.AddRepo("atlas", lanes.Orphans{})

	doc.AddRescued("atlas", []Rescued{
		{PlanID: 42, State: "⛔", Refs: []string{"refs/frit/rescue/42/box-a"}},
	})

	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Rescued, 1)
	assert.Equal(t, int64(42), doc.Repos[0].Rescued[0].PlanID)
	assert.Equal(t, "⛔", doc.Repos[0].Rescued[0].State)
	assert.Equal(t, []string{"refs/frit/rescue/42/box-a"},
		doc.Repos[0].Rescued[0].Refs)
	assert.True(t, doc.Repos[0].Any())
}

func TestOrphansEmitsListsNeverNull(t *testing.T) {
	doc := NewOrphans("/fleet")

	assert.NotNil(t, doc.Repos)
	assert.NotNil(t, doc.Problems)
}

// TestOrphansAddDesertedIsANoOpForAnUnknownRepo mirrors AddStale's own
// guard: a plan for a repository AddRepo never recorded is dropped
// rather than fabricating a repo entry out of order.
func TestOrphansAddDesertedIsANoOpForAnUnknownRepo(t *testing.T) {
	doc := NewOrphans("/fleet")

	doc.AddDeserted("ghost", []discovery.Plan{{ID: 1}})

	assert.Empty(t, doc.Repos)
}

// TestOrphansAddRescuedIsANoOpForAnUnknownRepo mirrors AddStale's own
// guard.
func TestOrphansAddRescuedIsANoOpForAnUnknownRepo(t *testing.T) {
	doc := NewOrphans("/fleet")

	doc.AddRescued("ghost", []Rescued{{PlanID: 1}})

	assert.Empty(t, doc.Repos)
}
