package discovery

import (
	"testing"

	"github.com/jeduden/frit/internal/planmeta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// p is a terse plan builder for the DAG fixtures.
func p(repo string, id int64, status string, deps ...int64) Plan {
	return Plan{
		Key: repo, Repo: repo, ID: id, Status: status, DependsOn: deps,
		Title: "plan " + repo,
	}
}

const (
	no   = planmeta.StatusNotStarted
	wip  = planmeta.StatusInProgress
	done = planmeta.StatusDone
	gone = planmeta.StatusSuperseded
)

func ids(plans []Plan) []int64 {
	out := make([]int64, 0, len(plans))
	for _, pl := range plans {
		out = append(out, pl.ID)
	}

	return out
}

// TestReadyWithdrawsAPlanWithOneUnmetDependency is the gate: a single
// upstream that is not done withholds the plan.
func TestReadyWithdrawsAPlanWithOneUnmetDependency(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, done),
		p("atlas", 2, wip),
		p("atlas", 3, no, 1, 2), // waits on 2, which is not done
	}

	got := Ready(plans)

	assert.NotContains(t, ids(got), int64(3),
		"an unmet dependency withholds the plan")
}

func TestReadyListsAPlanWhenEveryDependencyIsDone(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, done),
		p("atlas", 2, done),
		p("atlas", 3, no, 1, 2),
	}

	got := Ready(plans)

	require.Len(t, got, 1)
	assert.Equal(t, int64(3), got[0].ID)
}

func TestReadyListsAPlanWithNoDependencies(t *testing.T) {
	got := Ready([]Plan{p("atlas", 5, no)})

	require.Len(t, got, 1)
	assert.Equal(t, int64(5), got[0].ID)
}

func TestReadyWithholdsHeldStartedAndFinishedPlans(t *testing.T) {
	held := p("atlas", 1, no)
	held.Held = true
	plans := []Plan{
		held,               // startable but claimed
		p("atlas", 2, wip), // already in progress
		p("atlas", 3, done),
		p("atlas", 4, gone),
	}

	got := Ready(plans)

	assert.Empty(t, got)
}

// TestReadyWithholdsOnAnUnknownDependency: a dependency edge frit cannot
// resolve to a done plan is treated as unmet, not assumed satisfied.
func TestReadyWithholdsOnAnUnknownDependency(t *testing.T) {
	got := Ready([]Plan{p("atlas", 3, no, 999)})

	assert.Empty(t, got)
}

// TestReadyResolvesDependenciesWithinTheRepository: a done plan in
// another repository does not satisfy an edge, because ids are only
// unique per repo.
func TestReadyResolvesDependenciesWithinTheRepository(t *testing.T) {
	plans := []Plan{
		p("orrery", 1, done), // same id, wrong repo
		p("atlas", 2, no, 1),
	}

	got := Ready(plans)

	assert.Empty(t, got, "a cross-repo id must not satisfy a dependency")
}

// TestReadyOffersAHeldPlanWithAConfirmedDeadSession: a held plan whose
// bound session herdr confirms gone is a candidate at once, with no
// staleness window matured (2608212203, S76).
func TestReadyOffersAHeldPlanWithAConfirmedDeadSession(t *testing.T) {
	dead := p("atlas", 1, no)
	dead.Held = true
	dead.Dead = true

	got := Ready([]Plan{dead})

	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].ID)
}

// TestReadyWithholdsAHeldPlanNeitherStaleNorDead pins the baseline
// this phase must not regress: a held plan with no matured window and
// no confirmed-dead session stays withheld.
func TestReadyWithholdsAHeldPlanNeitherStaleNorDead(t *testing.T) {
	held := p("atlas", 1, no)
	held.Held = true

	got := Ready([]Plan{held})

	assert.Empty(t, got)
}

func TestReadyOrdersByRepoThenID(t *testing.T) {
	plans := []Plan{
		p("orrery", 5, no),
		p("atlas", 9, no),
		p("atlas", 2, no),
	}

	got := ids(Ready(plans))

	assert.Equal(t, []int64{2, 9, 5}, got)
}
