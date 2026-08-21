package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCandidatesOrdersFreshThenResume: a fresh startable plan is a real
// new pick, so it precedes the resume tail — an in-progress plan nobody
// holds, whose lane merged away — which is the fallback for when nothing
// fresh is startable.
func TestCandidatesOrdersFreshThenResume(t *testing.T) {
	plans := []Plan{
		p("atlas", 1, wip), // in progress, unheld: a resume
		p("atlas", 2, no),  // fresh, startable
	}

	got := ids(Candidates(plans))

	require.Equal(t, []int64{2, 1}, got,
		"the fresh pick ranks before the resume")
}

// TestCandidatesExcludesAHeldResume: an in-progress plan a lane already
// holds is being worked, so it is not offered as a resume.
func TestCandidatesExcludesAHeldResume(t *testing.T) {
	held := p("atlas", 1, wip)
	held.Held = true
	plans := []Plan{held}

	got := ids(Candidates(plans))

	assert.NotContains(t, got, int64(1), "a held lane is not a resume")
}
