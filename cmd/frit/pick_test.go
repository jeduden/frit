package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPickGoStartsTheTopCandidate is the Phase 1 gate: pick --go selects
// the top-ranked startable plan and runs start's claim-and-stand-up path
// on it — the same handshake start --go delegates to herdr — without the
// caller ever naming an id.
func TestPickGoStartsTheTopCandidate(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"pick", "--go", "--phase", "3", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t, rec.verb("worktree", "create"),
		"the worktree is herdr's to make")
	assert.True(t, rec.verb("agent", "start", "plan-7"),
		"the agent is herdr's to start")
	assert.True(t, rec.verb("agent", "prompt", "wZ:p1", "/plan-phase 7 3"),
		"the composed prompt goes to the new pane")
	assert.False(t, rec.verb("agent", "read"), "pick never reads a reply")
	assert.Contains(t, out.String(), "started plan 7")

	_, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7-shader-unit")
	require.NoError(t, err, "frit minted the claim itself")
}

// TestPickGoAdvancesPastALostRace: when the top candidate's claim loses
// its race to another machine, pick --go takes the next candidate rather
// than surfacing the race — the retry the skill used to spell out by
// hand. The contested ref is planted on origin only, so frit's local
// gather still offers plan 7 as a candidate, and the force-with-lease
// push is what loses.
func TestPickGoAdvancesPastALostRace(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	commitPlan(t, repo, 8, "🔲", "Vertex unit", nil, "")
	git(t, repo, "push", "-q", "origin",
		"main:refs/heads/plan/7-shader-unit")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"pick", "--go", "--phase", "2", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t, rec.verb("agent", "start", "plan-8"),
		"the race on 7 advances pick to the next candidate")
	assert.False(t, rec.verb("agent", "start", "plan-7"),
		"the contested plan is not started")
	assert.Contains(t, out.String(), "started plan 8")
}

// TestPickGoResumesAnUnheldInProgressPlan: with nothing fresh to start
// but a 🔳 plan nobody holds — a lane that merged away — pick --go
// resumes it, standing the lane back up on its deterministic branch.
func TestPickGoResumesAnUnheldInProgressPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	resumableRepo(t, root, "atlas", 7, "Shader unit")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"pick", "--go", "--phase", "2", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t, rec.verb("agent", "start", "plan-7"),
		"the merged-away lane is resumed")
	assert.Contains(t, out.String(), "started plan 7")
}

// TestPickGoIsQuietWhenNothingIsStartable: with no startable plan, pick
// --go gives the same empty answer bare pick does and mutates nothing.
func TestPickGoIsQuietWhenNothingIsStartable(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 1, "✅", "All done here", nil, "")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"pick", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "nothing startable")
	assert.False(t, rec.verb("worktree", "create"), "nothing is stood up")
}
