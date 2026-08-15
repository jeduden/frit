package report

import (
	"errors"
	"testing"

	"github.com/jeduden/frit/internal/index"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// entry builds a plan entry carried by two versions, the first of
// which Build would have ranked authoritative.
func entry(id int64, status string) index.Entry {
	return index.Entry{
		Key: index.Key{Host: "forge", Repo: "atlas", ID: id},
		Versions: []index.Version{
			{
				OID:  "0a1",
				Path: "plan/2608142306_fleet-index.md",
				Plan: planmeta.Plan{
					ID:        id,
					Title:     "The fleet index",
					Status:    status,
					Summary:   "Walk a root for repositories",
					Model:     "opus",
					DependsOn: []int64{6},
				},
				Refs: []string{"refs/heads/main"},
			},
			{
				OID:  "0b2",
				Path: "plan/2608142306_fleet-index.md",
				Plan: planmeta.Plan{ID: id, Title: "The fleet index"},
				Refs: []string{"refs/heads/old", "refs/remotes/o/old"},
			},
		},
	}
}

func TestPlansReportsThePrimaryVersionOfEachPlan(t *testing.T) {
	doc := NewPlans("/fleet", "forge")
	doc.AddRepo("atlas", []index.Entry{entry(2608142306, planmeta.StatusInProgress)})

	assert.Equal(t, Schema, doc.Schema)
	assert.Equal(t, "plans", doc.Command)
	assert.Equal(t, "forge", doc.Host)
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Plans, 1)

	p := doc.Repos[0].Plans[0]
	assert.Equal(t, "forge:atlas:2608142306", p.Key)
	assert.Equal(t, int64(2608142306), p.ID)
	assert.Equal(t, planmeta.StatusInProgress, p.Status)
	assert.Equal(t, "The fleet index", p.Title)
	assert.Equal(t, "opus", p.Model)
	assert.Equal(t, []int64{6}, p.DependsOn)
	assert.Equal(t, "plan/2608142306_fleet-index.md", p.Path)
	assert.Equal(t, []string{"refs/heads/main"}, p.Refs)
}

// TestPlansCountsEveryRefAndVersion keeps the two numbers apart: the
// refs on the authoritative version are not the refs that carry the
// plan, and a reader deciding whether a lane is behind needs both.
func TestPlansCountsEveryRefAndVersion(t *testing.T) {
	doc := NewPlans("/fleet", "forge")
	doc.AddRepo("atlas", []index.Entry{entry(42, planmeta.StatusDone)})

	p := doc.Repos[0].Plans[0]
	assert.Equal(t, 3, p.RefCount)
	assert.Equal(t, 2, p.Versions)
	assert.Len(t, p.Refs, 1)
}

// TestPlansAlwaysCarriesEveryPlan is the one place the JSON and the
// table part company: --detail decides how much a person is shown,
// while a consumer is always given the whole list.
func TestPlansAlwaysCarriesEveryPlan(t *testing.T) {
	doc := NewPlans("/fleet", "forge")
	doc.AddRepo("atlas", []index.Entry{
		entry(1, planmeta.StatusDone),
		entry(2, planmeta.StatusNotStarted),
	})

	assert.Len(t, doc.Repos[0].Plans, 2)
}

func TestPlansRecordsARepositoryItCouldNotRead(t *testing.T) {
	doc := NewPlans("/fleet", "forge")
	doc.AddProblem("atlas", errors.New("not a git repository"))

	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "atlas", doc.Problems[0].Repo)
	assert.Equal(t, "not a git repository", doc.Problems[0].Message)
}

func TestPlansEmitsListsNeverNull(t *testing.T) {
	doc := NewPlans("/fleet", "forge")
	doc.AddRepo("empty", nil)

	assert.NotNil(t, doc.Repos)
	assert.NotNil(t, doc.Problems)
	assert.NotNil(t, doc.Repos[0].Plans)
}

func TestPlanCountsBreakDownByStatus(t *testing.T) {
	repo := PlanRepo{Plans: []Plan{
		{Status: planmeta.StatusDone},
		{Status: planmeta.StatusDone},
		{Status: planmeta.StatusInProgress},
	}}

	counts := repo.Counts()

	assert.Equal(t, 2, counts[planmeta.StatusDone])
	assert.Equal(t, 1, counts[planmeta.StatusInProgress])
	assert.Zero(t, counts[planmeta.StatusNotStarted])
}
