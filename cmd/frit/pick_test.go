package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	_, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err, "frit minted the claim itself")
}

// TestPickGoDispatchesAPhaselessTopCandidate: pick's "one verb, never
// ask" contract must hold even when the top-ranked candidate carries no
// phase ledger — it claims and starts it, composing /plan-phase <id>,
// with no --phase and no error.
func TestPickGoDispatchesAPhaselessTopCandidate(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"pick", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t, rec.verb("agent", "prompt", "wZ:p1", "/plan-phase 7"),
		"the whole-plan prompt carries no phase token")
	assert.Contains(t, out.String(), "started plan 7")
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
		"main:refs/heads/plan/7")
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

// TestPickGoRefusesADivergingLocalBranch: the fresh-acquire guard
// reaches pick --go through the same mintOrTakeOver path claim uses —
// a local plan/<id> branch with unpushed commits refuses instead of
// being silently clobbered (S82).
func TestPickGoRefusesADivergingLocalBranch(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	git(t, repo, "checkout", "-q", "-b", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "draft.md"), []byte("x\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "local draft, never pushed")
	local, err := gitCapture(t, repo, "rev-parse", "plan/7")
	require.NoError(t, err)
	git(t, repo, "checkout", "-q", "main")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"pick", "--go", "--root", root}, &out, &errb)

	assert.NotEqual(t, 0, code)
	assert.Contains(t, errb.String(), "plan 7")
	assert.False(t, rec.verb("worktree", "create"), "nothing is stood up")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, local, tip,
		"the local draft branch is untouched, not clobbered")
}

// TestPickGoRefusesWhenALiveAgentAlreadyHoldsTheTopLane: the same
// live-but-unbound lane guard start --go meets, reached through pick's
// own claim-and-stand-up path. The refusal is surfaced, not skipped —
// pick --go only retries a lost race, and a live lane is not one.
func TestPickGoRefusesWhenALiveAgentAlreadyHoldsTheTopLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, lease, runner, rec := liveLeaseFixture(t, root)
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"pick", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "plan/7", "the reason names the live lane")
	assert.False(t, rec.verb("worktree", "create"),
		"a live lane is refused before a takeover ever runs")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, tip, "nothing was taken over")
}

// TestPickGoDoesNotReattachAHeldLane: the from-outside resume is an
// explicit `start <id>` on a lane the caller names. pick --go ranks
// ready plans, and a held lane herdr confirms dead is ready for a
// takeover (S76) — but pick promises to resume only an unheld plan, so
// it must not quietly reopen a held checkout on the way to the top
// candidate; the takeover it always ran is what it still runs.
func TestPickGoDoesNotReattachAHeldLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, _, _ := heldLaneOwnedBy(t, root, hostname(), "wOld:p1")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"pick", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.NotContains(t, out.String(), "resumed plan 7",
		"pick never resumes a held lane")
	assert.False(t, rec.verb("worktree", "open"),
		"pick never reopens a held checkout")
	tip := remoteWorkTip(t, repo)
	body, err := gitCapture(t, repo, "log", "--format=%s", tip, "^origin/main")
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: takeover",
		"a dead hold reached through pick is taken over, as before")
}

// TestPickGoAdvancesPastALiveHold: a hold gone stale by its clock
// while its agent is still genuinely working — plausible for a phase
// that runs long between beats, the same shape
// TestPickListsAStaleLeaseAsTakeover ranks as a takeover candidate —
// reaches pick --go's own claim attempt, not start's phase-2
// live-agent wording (plan 2609011941): that wording is for an
// operator's explicit `start <id>`, not a candidate pick --go is
// silently walking past. mintOrTakeOver's own live-session veto fires
// exactly as it always did, classified a lost race, and the walk
// advances — here there is nothing left to advance to.
func TestPickGoAdvancesPastALiveHold(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, _, held := heldLaneOwnedBy(t, root, hostname(), "wOld:p1")
	seedWindow(t, "atlas", 7, held, 3*time.Hour)
	withHerdr(t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "working",
		"cwd":                     t.TempDir(),
		"pane_id":                 "wOld:p1",
		"agent_session":           map[string]any{"value": "wOld:p1"},
		"terminal_title_stripped": "elsewhere",
	}))
	var out, errb bytes.Buffer

	code := run([]string{"pick", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.NotContains(t, got, "live agent",
		"pick's own walk is not the operator staring at a named refusal")
	assert.Contains(t, got, "nothing startable",
		"the lost race advances past the only candidate, same as before")
	tip := remoteWorkTip(t, repo)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: beat",
		"the veto renews the live holder's own lease; it is never taken over")
	assert.NotContains(t, body, "plan 7: takeover")
}
