package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func described(pl Plan, title, summary string) Plan {
	pl.Title = title
	pl.Summary = summary
	return pl
}

func TestFindMatchesTitleCaseInsensitively(t *testing.T) {
	plans := []Plan{
		described(p("atlas", 1, done), "Raymarch the gas giants", ""),
		described(p("atlas", 2, no), "Something else", ""),
	}

	got := Find("RAYMARCH", plans)

	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].ID)
}

func TestFindMatchesSummary(t *testing.T) {
	plans := []Plan{
		described(p("atlas", 1, no), "A title", "walk the signed distance field"),
		described(p("atlas", 2, no), "Another", "nothing to see"),
	}

	got := Find("signed distance", plans)

	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].ID)
}

func TestFindReturnsEveryMatchOrderedByRepoThenID(t *testing.T) {
	plans := []Plan{
		described(p("orrery", 5, no), "orbit math", ""),
		described(p("atlas", 9, no), "orbit control", ""),
		described(p("atlas", 2, no), "orbit view", ""),
	}

	got := ids(Find("orbit", plans))

	assert.Equal(t, []int64{2, 9, 5}, got)
}

func TestFindOnNoMatchIsEmptyNotNil(t *testing.T) {
	got := Find("absent", []Plan{described(p("atlas", 1, no), "x", "y")})

	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestFindOnAnEmptyQueryMatchesNothing(t *testing.T) {
	got := Find("   ", []Plan{described(p("atlas", 1, no), "anything", "")})

	assert.Empty(t, got, "a blank query is not a wildcard")
}
