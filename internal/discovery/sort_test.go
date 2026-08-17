package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func held(pl Plan) Plan {
	pl.Held = true
	return pl
}

func TestParseSortKey(t *testing.T) {
	for in, want := range map[string]SortKey{
		"status": SortStatus, "repo": SortRepo,
		"id": SortID, "age": SortID, "held": SortHeld,
	} {
		got, ok := ParseSortKey(in)
		require.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}

	_, ok := ParseSortKey("nonsense")
	assert.False(t, ok)
}

func TestSortByStatusFloatsInProgressFirst(t *testing.T) {
	plans := []Plan{
		p("atlas", 2, no), p("atlas", 1, wip), p("atlas", 3, done),
	}

	Sort(plans, SortStatus, false)

	assert.Equal(t, []int64{1, 2, 3}, ids(plans),
		"in-progress, then not-started, then done")
}

func TestSortByRepoGroups(t *testing.T) {
	plans := []Plan{
		p("orrery", 1, no), p("atlas", 9, no), p("atlas", 2, no),
	}

	Sort(plans, SortRepo, false)

	assert.Equal(t, []int64{2, 9, 1}, ids(plans))
}

func TestSortByIDIsOldestFirst(t *testing.T) {
	plans := []Plan{p("atlas", 30, no), p("atlas", 10, no), p("atlas", 20, no)}

	Sort(plans, SortID, false)

	assert.Equal(t, []int64{10, 20, 30}, ids(plans))
}

func TestSortByHeldPutsClaimedFirst(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, no), held(p("atlas", 2, no)), p("atlas", 3, no),
	}

	Sort(plans, SortHeld, false)

	assert.Equal(t, []int64{2, 1, 3}, ids(plans), "held plan leads")
}

func TestSortReverseFlipsTheOrder(t *testing.T) {
	plans := []Plan{p("atlas", 10, no), p("atlas", 20, no), p("atlas", 30, no)}

	Sort(plans, SortID, true)

	assert.Equal(t, []int64{30, 20, 10}, ids(plans), "newest first")
}

func TestReverseTurnsACommandsOwnOrder(t *testing.T) {
	plans := []Plan{p("a", 1, no), p("b", 2, no), p("c", 3, no)}

	Reverse(plans)

	assert.Equal(t, []int64{3, 2, 1}, ids(plans))
}
