package discovery

import (
	"testing"

	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withPhases(pl Plan, phases ...planmeta.Phase) Plan {
	pl.Phases = phases
	return pl
}

// TestNextPhaseSkipsDoneAndStopsAtTheFirstOpen is the rule frit next
// follows over a plan's ledger.
func TestNextPhaseSkipsDoneAndStopsAtTheFirstOpen(t *testing.T) {
	pl := withPhases(p("atlas", 1, wip),
		planmeta.Phase{N: 1, Title: "one", Status: done},
		planmeta.Phase{N: 2, Title: "two", Status: wip},
		planmeta.Phase{N: 3, Title: "three", Status: no},
	)

	phase, ok := pl.NextPhase()

	require.True(t, ok)
	assert.Equal(t, 2, phase.N)
	assert.Equal(t, "two", phase.Title)
}

func TestNextPhaseReportsNoneWithoutALedger(t *testing.T) {
	_, ok := p("atlas", 1, no).NextPhase()

	assert.False(t, ok)
}

func TestDependenciesWalksTheUpstreamDAG(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, done),
		p("atlas", 2, done, 1),
		p("atlas", 3, no, 2),
	}

	tree := Dependencies(plans[2], plans)

	require.True(t, tree.Found)
	assert.Equal(t, int64(3), tree.Plan.ID)
	require.Len(t, tree.Deps, 1)
	assert.Equal(t, int64(2), tree.Deps[0].Plan.ID)
	require.Len(t, tree.Deps[0].Deps, 1)
	assert.Equal(t, int64(1), tree.Deps[0].Deps[0].Plan.ID)
}

// TestDependenciesCarriesAnUnknownEdge: an edge to a plan frit cannot
// see is shown as unresolved, not dropped.
func TestDependenciesCarriesAnUnknownEdge(t *testing.T) {
	plans := []Plan{p("atlas", 3, no, 999)}

	tree := Dependencies(plans[0], plans)

	require.Len(t, tree.Deps, 1)
	assert.False(t, tree.Deps[0].Found)
	assert.Equal(t, int64(999), tree.Deps[0].Plan.ID)
}

// TestDependenciesTerminatesOnACycle: a cycle in the edges must not
// loop forever.
func TestDependenciesTerminatesOnACycle(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, no, 2),
		p("atlas", 2, no, 1),
	}

	tree := Dependencies(plans[0], plans)

	require.Len(t, tree.Deps, 1)
	assert.Equal(t, int64(2), tree.Deps[0].Plan.ID)
	// 2 points back at 1, which is already expanded, so it stops.
	require.Len(t, tree.Deps[0].Deps, 1)
	assert.Empty(t, tree.Deps[0].Deps[0].Deps)
}

func TestDependenciesResolvesWithinTheRepository(t *testing.T) {
	plans := []Plan{
		p("orrery", 1, done), // same id, other repo
		p("atlas", 2, no, 1),
	}

	tree := Dependencies(plans[1], plans)

	require.Len(t, tree.Deps, 1)
	assert.False(t, tree.Deps[0].Found,
		"a cross-repo id does not resolve an edge")
}

// TestPickRanksTheMostUnblockingFirst: the plan that frees the most
// waiting work comes first, and -n trims the list.
func TestPickRanksTheMostUnblockingFirst(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, no), // nothing waits on 1
		p("atlas", 2, no), // 3 and 4 wait on 2, both still open
		p("atlas", 3, wip, 2),
		p("atlas", 4, no, 2),
	}

	got := Pick(plans, 0)

	require.Len(t, got, 2)
	assert.Equal(t, int64(2), got[0].ID, "the most unblocking is first")
	assert.Equal(t, int64(1), got[1].ID)
}

// TestPickDoesNotCountFinishedDependents: a plan whose dependents are
// all done frees no waiting work, so it must not outrank one with a
// genuinely blocked dependent.
func TestPickDoesNotCountFinishedDependents(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, no), // 3 waits on 1 and is still open
		p("atlas", 2, no), // 4 and 5 wait on 2 but are already done
		p("atlas", 3, no, 1),
		p("atlas", 4, done, 2),
		p("atlas", 5, done, 2),
	}

	got := Pick(plans, 0)

	require.Len(t, got, 2)
	assert.Equal(t, int64(1), got[0].ID,
		"one waiting dependent outranks two finished ones")
	assert.Equal(t, int64(2), got[1].ID)
}

func TestPickHonoursN(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, no),
		p("atlas", 2, no),
		p("atlas", 3, no),
	}

	got := Pick(plans, 2)

	assert.Len(t, got, 2)
}
