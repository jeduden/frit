package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoardCarriesEveryUnfinishedPlan(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, done),
		p("atlas", 2, wip),
		p("atlas", 3, no),
		p("atlas", 4, gone),
	}

	got := ids(Board(plans, false))

	assert.Equal(t, []int64{2, 3}, got,
		"done and superseded are off the board")
}

func TestBoardWipOnlyKeepsInProgress(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, wip),
		p("atlas", 2, no),
		p("atlas", 3, wip),
	}

	got := ids(Board(plans, true))

	assert.Equal(t, []int64{1, 3}, got, "not-started is excluded by --wip")
}

// TestBoardFloatsInProgressToTheTop: active work reads first, then
// not-started, ordered within each by repo and id.
func TestBoardFloatsInProgressToTheTop(t *testing.T) {
	plans := []Plan{
		p("atlas", 5, no),
		p("atlas", 9, wip),
		p("orrery", 2, wip),
		p("atlas", 1, no),
	}

	got := ids(Board(plans, false))

	assert.Equal(t, []int64{9, 2, 1, 5}, got,
		"in-progress (9, 2) before not-started, each by repo then id")
}

func TestBoardIsEmptyNotNilWhenNothingIsOutstanding(t *testing.T) {
	got := Board([]Plan{p("atlas", 1, done)}, false)

	require.NotNil(t, got)
	assert.Empty(t, got)
}
