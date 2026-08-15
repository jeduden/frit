package report

import (
	"errors"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaleReportsAgeInBothUnits(t *testing.T) {
	doc := NewStale("/fleet", 30)
	doc.AddRepo("atlas", []lanes.Aged{{
		Worktree: gitwt.Worktree{
			Path:   "/fleet/atlas-lane",
			Branch: "plan/42-x",
			Head:   "a1b2c3",
		},
		Age: 41*24*time.Hour + 12*time.Hour,
	}})

	assert.Equal(t, Schema, doc.Schema)
	assert.Equal(t, "stale", doc.Command)
	assert.Equal(t, 30, doc.Days)
	require.Len(t, doc.Repos, 1)
	require.Len(t, doc.Repos[0].Stale, 1)

	aged := doc.Repos[0].Stale[0]
	assert.Equal(t, "atlas-lane", aged.Worktree.Name)
	// Days are what a person reads and seconds are what a consumer
	// applies its own threshold to, so the half day is dropped from
	// one and kept in the other.
	assert.Equal(t, 41, aged.AgeDays)
	assert.Equal(t, int64(3585600), aged.AgeSeconds)
}

func TestStaleKeepsFreshRepositories(t *testing.T) {
	doc := NewStale("/fleet", 30)
	doc.AddRepo("fresh", nil)
	doc.AddProblem("broken", errors.New("cannot read refs"))

	require.Len(t, doc.Repos, 1)
	assert.NotNil(t, doc.Repos[0].Stale)
	assert.Empty(t, doc.Repos[0].Stale)
	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "cannot read refs", doc.Problems[0].Message)
}

func TestStaleEmitsListsNeverNull(t *testing.T) {
	doc := NewStale("/fleet", 7)

	assert.NotNil(t, doc.Repos)
	assert.NotNil(t, doc.Problems)
}
