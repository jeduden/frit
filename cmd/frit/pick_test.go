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
