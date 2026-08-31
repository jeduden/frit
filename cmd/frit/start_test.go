package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startHerdr fakes a herdr that answers worktree.create and pane.current
// with the same pane, and agent.list with that pane bound to a session,
// and records every other call, so a test can assert the escalation ran
// the right handshake — and, just as important, never read an agent
// back. pane.current is what a resumed start reads instead of standing
// a fresh worktree up.
func startHerdr() (herdr.Runner, *herdrCalls) {
	rec := &herdrCalls{}

	return func(args ...string) ([]byte, error) {
		rec.mu.Lock()
		rec.calls = append(rec.calls, append([]string(nil), args...))
		rec.mu.Unlock()
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
		}
		if len(args) >= 2 && args[0] == "pane" && args[1] == "current" {
			return []byte(`{"result":{"pane":{"pane_id":"wZ:p1"}}}`), nil
		}
		if len(args) >= 2 && args[0] == "agent" && args[1] == "list" {
			return []byte(`{"result":{"agents":[{"agent":"claude",` +
				`"agent_status":"working","pane_id":"wZ:p1",` +
				`"agent_session":{"value":"sess-1"}}]}}`), nil
		}

		return nil, nil
	}, rec
}

// TestStartComposesTheEscalation: the dry run prints the whole plan —
// the claim, worktree, agent tier and typed prompt — and spawns nothing.
func TestStartComposesTheEscalation(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "dry run")
	assert.Contains(t, got, "claim:    plan/7  (base",
		"the claim branch is the id-only work ref")
	assert.Contains(t, got, "/plan-phase 7 3", "the typed prompt")
	assert.Contains(t, got, "--model", "the tier maps to an agent arg")
}

// TestStartGoDispatchesAPhaselessPlan: a plan small enough to land in
// one go carries no phase ledger. buildStart dispatches it whole,
// composing /plan-phase <id> with no --phase and no error, instead of
// demanding one.
func TestStartGoDispatchesAPhaselessPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "started plan 7")
	assert.True(t, rec.verb("agent", "prompt", "wZ:p1", "/plan-phase 7"),
		"the whole-plan prompt carries no phase token")
	assert.True(t, rec.hasArg("atlas plan 7"),
		"the pane label names the plan's repo, not the id alone")
	assert.True(t, rec.verb("worktree", "create"),
		"a fresh acquire, with no persisted token, still creates the worktree")
}

// leftoverWorktree reproduces the exact shape Release leaves behind:
// a plan claimed, worked and released. Release deletes nothing, so
// the branch and its worktree persist after the lease that authorized
// them ends. It returns the worktree's path and the branch's own tip
// right after release — a bare commit `Release` mints and CASes onto
// `refs/heads/<branch>` without ever touching the worktree's own
// checked-out files, so a worktree checked out *on* that branch
// reports it live rather than the leftover's last real commit.
func leftoverWorktree(t *testing.T, root, repo string, id int64) (path, tip string) {
	t.Helper()
	branch := claim.Branch(id)
	opts := claim.LeaseOptions{
		PlanID: id, Remote: "origin", Base: "origin/main", Holder: "elsewhere",
	}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)

	path = filepath.Join(root, "atlas-leftover")
	git(t, repo, "worktree", "add", "-q", path, branch)
	require.NoError(t, os.WriteFile(
		filepath.Join(path, "wip.txt"), []byte("wip\n"), 0o600))
	git(t, path, "add", "-A")
	git(t, path, "commit", "-q", "-m", "unlanded work")
	workTip, err := gitCapture(t, path, "rev-parse", "HEAD")
	require.NoError(t, err)
	git(t, repo, "push", "-q", "origin", branch)

	_, err = claim.Release(repo, opts, workTip, gitwt.Exec)
	require.NoError(t, err)
	tip, err = gitCapture(t, repo, "rev-parse", "refs/heads/"+branch)
	require.NoError(t, err)

	return path, tip
}

// TestStartGoReapsADeadLeftoverWorktreeBeforeRecreating is issue 118's
// own shape reaching start: Release deletes nothing, so a worktree
// already sits on the plan's own branch with no herdr pane on it.
// start --go parks the branch's unlanded chain, clears the worktree
// registration, and stands a fresh checkout up in its place instead
// of dying on herdr's raw worktree_create_failed.
func TestStartGoReapsADeadLeftoverWorktreeBeforeRecreating(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	leftover, _ := leftoverWorktree(t, root, repo, 7)
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "started plan 7")
	_, statErr := os.Stat(leftover)
	assert.ErrorIs(t, statErr, os.ErrNotExist,
		"the leftover checkout is removed from disk")
	rescue, err := gitCapture(t, repo, "ls-remote", "origin",
		"refs/frit/rescue/7/*")
	require.NoError(t, err)
	assert.NotEmpty(t, rescue, "the unlanded work is parked, not dropped")
	assert.True(t, rec.verb("worktree", "create"),
		"the reconcile clears the way, it does not skip the create")
}

// TestStartGoRefusesALiveHerdrPaneOnTheLeftover: the same leftover,
// but a herdr pane is still sitting on it — a lane a person or agent
// may genuinely be working in. start --go leaves it standing and
// refuses with a frit-authored message instead of reaping it out from
// under whoever is there, and never claims anything at all: the
// refusal runs ahead of startAcquire, so there is no lease to unwind.
func TestStartGoRefusesALiveHerdrPaneOnTheLeftover(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	leftover, releasedTip := leftoverWorktree(t, root, repo, 7)
	runner, rec := recordingHerdr(map[string]any{
		"agent": "claude", "agent_status": "working",
		"pane_id": "wLive:p9", "cwd": leftover,
	})
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "wLive:p9")
	assert.Contains(t, errb.String(), leftover)
	assert.False(t, rec.verb("worktree", "create"),
		"a refused reconcile never reaches herdr's own create")
	_, statErr := os.Stat(leftover)
	assert.NoError(t, statErr, "the live leftover is left standing")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, releasedTip, tip,
		"nothing was claimed, so the branch is exactly where release left it")
}

// TestStartGoRefusesALeftoverWhenHerdrCannotConfirmNoLivePaneIsOnIt:
// the same leftover, but herdr's own agent.list answers with an error
// rather than a pane list. reconcileLeftoverWorktree cannot tell a
// live pane from a dead one when it cannot ask at all, so it refuses
// on that uncertainty instead of reading the silence as "no pane" and
// parking and deleting a worktree that might genuinely be live.
func TestStartGoRefusesALeftoverWhenHerdrCannotConfirmNoLivePaneIsOnIt(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	leftover, releasedTip := leftoverWorktree(t, root, repo, 7)
	runner := func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "agent" && args[1] == "list" {
			return nil, errors.New("herdr: socket unreachable")
		}
		return nil, nil
	}
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), leftover)
	_, statErr := os.Stat(leftover)
	assert.NoError(t, statErr, "the leftover is left standing when herdr cannot be asked")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, releasedTip, tip,
		"nothing was parked or claimed while the live-pane question is unresolved")
}

// TestStartRefusesAnAllDonePhasedPlan: a phased ledger whose every
// phase is done has genuinely nothing left to send, unlike a plan
// with no ledger at all — it still refuses, and never mints a claim.
func TestStartRefusesAnAllDonePhasedPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPhasedPlan(t, repo, 7, "🔳", "Shader unit", "✅", "✅")
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--go", "--root", root}, &out, &errb)

	require.Equal(t, 1, code)
	assert.Contains(t, errb.String(), "has no open phase")
	_, err := gitCapture(t, repo, "rev-parse", "--verify", "refs/heads/plan/7")
	assert.Error(t, err, "no claim was pushed for the refused plan")
}

// TestStartFoldsInTheNote: a --note rides the composed prompt beneath the
// slash command, so the subject stays the tool's.
func TestStartFoldsInTheNote(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	var doc report.StartDoc

	emit(t, &doc, "start", "7", "--phase", "3",
		"--note", "skip the VRT case", "--root", root)

	assert.Equal(t, "/plan-phase 7 3\n\nskip the VRT case", doc.Prompt)
	assert.False(t, doc.Started, "a dry run spawns nothing")
}

// TestStartRefusesAnUnstartablePlan: start mints a claim, so a plan
// already held or blocked is refused rather than escalated.
func TestStartRefusesAnUnstartablePlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "already held")
}

// TestStartNotMaturedRefusalNamesTheSpanAndWindow: start speaks the
// same span-and-window phrasing claim and reap do for a live,
// not-yet-matured hold.
func TestStartNotMaturedRefusalNamesTheSpanAndWindow(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, lease.Tip, 42*time.Minute)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "unchanged for")
	assert.Contains(t, out.String(), "42m",
		"names the observed StaleFor span")
	assert.Contains(t, out.String(), "2h",
		"names the repo's configured takeover window")
}

// TestStartResumesAnUnheldInProgressPlan: an in-progress plan whose lane
// vanished — 🔳 on main, held by nobody — is escalated, not refused. The
// resume stands the lane back up on the plan's deterministic branch; the
// "already in progress" guard is not a refusal when nobody holds it.
func TestStartResumesAnUnheldInProgressPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	resumableRepo(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "2", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.NotContains(t, got, "refused")
	assert.Contains(t, got, "claim:    plan/7  (base",
		"the claim branch is prescribed and id-only")
	assert.Contains(t, got, "dry run")
}

// TestStartEmitsJSON decodes the escalation a consumer reads to see
// exactly what start would run.
func TestStartEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	var doc report.StartDoc

	emit(t, &doc, "start", "7", "--phase", "3", "--root", root)

	assert.Equal(t, "start", doc.Command)
	assert.Equal(t, int64(7), doc.Plan.ID)
	assert.Equal(t, "3", doc.Phase)
	assert.Equal(t, "claude", doc.Kind)
	assert.Equal(t, "plan/7", doc.Branch)
	assert.Equal(t, "/plan-phase 7 3", doc.Prompt)
	assert.NotEmpty(t, doc.Base)
	assert.False(t, doc.Started)
}

// TestStartGoRunsTheEscalation is the Phase 3 gate: with --go, start
// mints the claim and delegates the worktree, agent, prompt and focus to
// herdr — and never reads a reply.
func TestStartGoRunsTheEscalation(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t, rec.verb("worktree", "create"),
		"the worktree is herdr's to make")
	assert.True(t, rec.verb("agent", "start", "plan-7"),
		"the agent is herdr's to start")
	assert.True(t, rec.verb("agent", "prompt", "wZ:p1", "/plan-phase 7 3"),
		"the composed prompt goes to the new pane")
	assert.True(t, rec.verb("agent", "focus", "wZ:p1"))
	assert.False(t, rec.verb("agent", "read"), "start never reads a reply")
	got := out.String()
	assert.Contains(t, got, "started plan 7")
	assert.Contains(t, got, "running:  /plan-phase 7 3",
		"a live dispatch relabels the prompt as running")
	assert.NotContains(t, got, "prompt:",
		"a live dispatch does not read as a recipe")
	assert.Contains(t, got, "wZ:p1", "the directive names the pane")
	assert.Contains(t, got, "do not run it here",
		"the directive tells the reader not to run the prompt themselves")
	assert.Contains(t, got, "plan 7",
		"the don't-run-it-here directive names the plan id")
	assert.NotContains(t, got, "run again with --go",
		"a live dispatch does not invite a re-run")

	_, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err, "frit minted the claim itself")
}

// TestStartDryRunKeepsTheRecipe is the counter-case to
// TestStartGoRunsTheEscalation: without --go nothing is dispatched, so the
// output stays a recipe the reader is meant to run — prompt:, the
// run-again-with---go invitation, and no handoff directive.
func TestStartDryRunKeepsTheRecipe(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "prompt:   /plan-phase 7 3")
	assert.Contains(t, got, "run again with --go to execute")
	assert.NotContains(t, got, "running:")
	assert.NotContains(t, got, "do not run it here")
}

// TestStartBindsTheSessionOntoTheLease: once the agent is up, start
// reads its herdr session back and binds it onto the lease with a
// beat, so a later takeover can ask herdr whether this lease's holder
// is still alive (F3, S61).
func TestStartBindsTheSessionOntoTheLease(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, _ := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: beat")
	assert.Contains(t, body, "epoch:   1", "binding never bumps the epoch")
	assert.Contains(t, body, "session: sess-1")
}

// TestStartEditAmendsThePrompt: --edit hands the composed prompt to the
// editor and sends what comes back, the git-commit-message pattern.
func TestStartEditAmendsThePrompt(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	prev := openEditor
	openEditor = func(s string) (string, error) { return s + "\n\namended", nil }
	t.Cleanup(func() { openEditor = prev })
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go", "--edit",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t,
		rec.verb("agent", "prompt", "wZ:p1", "/plan-phase 7 3\n\namended"),
		"the edited prompt is what is sent")
}

// TestStartEditEmptyAbortsBeforeAnythingRuns: an editor that leaves the
// prompt empty aborts with no claim pushed and no lane stood up, the way
// git aborts an empty commit message.
func TestStartEditEmptyAbortsBeforeAnythingRuns(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	prev := openEditor
	openEditor = func(string) (string, error) { return "   \n", nil }
	t.Cleanup(func() { openEditor = prev })
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go", "--edit",
		"--root", root}, &out, &errb)

	require.Equal(t, 1, code, "an empty prompt aborts")
	assert.Contains(t, errb.String(), "empty")
	assert.False(t, rec.verb("worktree", "create"), "nothing was spawned")
	_, err := gitCapture(t, repo,
		"rev-parse", "--verify", "refs/heads/plan/7")
	assert.Error(t, err, "no claim was pushed")
}

// TestStartUnwindReleasesTheLeaseAndNamesTheLane: a handoff that dies
// after the lane stood up tears the worktree and pane down before it
// releases the lease with a pushed marker — never a delete, so the
// next acquire reads epoch E+1 — and, since the teardown itself
// succeeded, the reported error stays the plain cause: the abort is
// clean, nothing was left behind.
func TestStartUnwindTearsDownTheLaneOnAFailedHandoff(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	rec := &herdrCalls{}
	withHerdr(t, func(args ...string) ([]byte, error) {
		rec.mu.Lock()
		rec.calls = append(rec.calls, append([]string(nil), args...))
		rec.mu.Unlock()
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
		}
		if len(args) >= 2 && args[0] == "agent" && args[1] == "start" {
			return nil, errors.New(
				"agent target pane wZ:p1 is not an available shell")
		}
		return nil, nil
	})
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 1, code, "a dead handoff is a failure")
	assert.True(t, rec.verb("worktree", "remove"),
		"the abort tears the worktree it stood up back down")
	assert.True(t, rec.hasArg("wZ"),
		"the workspace is derived from the pane herdr opened")
	assert.NotContains(t, errb.String(), "left behind",
		"a clean teardown reports the plain cause, not what stood up")

	remote, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.NotEmpty(t, remote, "the unwind deletes nothing")
	subject, err := gitCapture(t, repo,
		"log", "-1", "--format=%s", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, "plan 7: release", subject,
		"the unwind pushes the release marker")
}

// TestStartUnwindNamesWhatTeardownLeftBehindWhenItFails: when the
// teardown itself errors, the failure it could not clean up after is
// surfaced by naming the worktree and pane still standing — so
// `frit orphans` finds them — and the lease is still released.
func TestStartUnwindNamesWhatTeardownLeftBehindWhenItFails(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	withHerdr(t, func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
		}
		if len(args) >= 2 && args[0] == "agent" && args[1] == "start" {
			return nil, errors.New(
				"agent target pane wZ:p1 is not an available shell")
		}
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "remove" {
			return nil, errors.New("workspace busy")
		}
		return nil, nil
	})
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 1, code, "a dead handoff is a failure")
	assert.Contains(t, errb.String(), "atlas-shader-unit",
		"the error names the worktree left behind")
	assert.Contains(t, errb.String(), "wZ:p1",
		"the error names the pane left behind")

	remote, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.NotEmpty(t, remote, "the unwind deletes nothing")
	subject, err := gitCapture(t, repo,
		"log", "-1", "--format=%s", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, "plan 7: release", subject,
		"the lease still releases even though the teardown failed")
}

// TestStartTakesOverAStaleLease: start's escalation meets a stale-held
// plan the same way claim does — it seizes the lease by takeover
// rather than losing the race claim.Acquire alone would report, and
// still stands the lane up.
func TestStartTakesOverAStaleLease(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, lease.Tip, 3*time.Hour)
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "started plan 7",
		"a matured stale hold is taken over, not refused")
	assert.True(t, rec.verb("worktree", "create"),
		"the lane is still stood up on the seized lease")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: beat",
		"the session bind rides atop the takeover marker")
	parent, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	takeoverBody, err := gitCapture(t, repo, "log", "-1", "--format=%B", parent)
	require.NoError(t, err)
	assert.Contains(t, takeoverBody, "plan 7: takeover")
	assert.Contains(t, takeoverBody, "epoch:   2", "the takeover reads E+1")
}

// TestStartRefusesATakeoverVetoedByALiveSession: a matured window would
// normally open the takeover door for start too, but herdr positively
// confirms the holder's bound session is still live, so start refuses
// and beats the holder's own lease instead of seizing it (F3, S61).
func TestStartRefusesATakeoverVetoedByALiveSession(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wS:p9"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, lease.Tip, 3*time.Hour)
	withHerdr(t, herdrReturning(map[string]any{
		"agent":        "claude",
		"agent_status": "working",
		"cwd":          repo,
		"pane_id":      "wS:p9",
		"agent_session": map[string]any{
			"value": "wS:p9",
		},
	}))
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "live agent session")

	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: beat",
		"the holder's own lease was renewed, not seized")
	assert.Contains(t, body, "holder:  elsewhere")
}

// TestStartResumesItsOwnLeaseFromThePersistedToken: run from the
// lane's own worktree, a restarted fleet of one resumes its lease by
// its persisted token and still stands a fresh agent up on it, with no
// staleness window consulted (F9, F11, S3).
func TestStartResumesItsOwnLeaseFromThePersistedToken(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane,
		Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.NotContains(t, got, "refused")
	assert.Contains(t, got, "resumed plan 7")
	assert.True(t, rec.verb("agent", "start", "plan-7"),
		"a resumed lease still stands a fresh agent up")
	assert.False(t, rec.verb("worktree", "create"),
		"a resume drives the pane it is already in, not a fresh worktree")
	assert.True(t, rec.verb("pane", "current"),
		"the pane to drive is read the way currentSession already does")

	// The chain now carries two beats: the resume itself, CASed from the
	// lane's own persisted token, and the session bind riding atop it
	// once the fresh agent is up.
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: beat")
	assert.Contains(t, body, "epoch:   1", "a resume never bumps the epoch")
	resumeTip, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	resumeBody, err := gitCapture(t, repo, "log", "-1", "--format=%B", resumeTip)
	require.NoError(t, err)
	assert.Contains(t, resumeBody, "plan 7: beat")
	parent, err := gitCapture(t, repo, "rev-parse", resumeTip+"^")
	require.NoError(t, err)
	assert.Equal(t, renewed.Tip, parent,
		"the resume is CASed from the lane's own persisted token")
}

// TestStartResumeRenewsWithTheLaneItIsActuallyRunningFrom: a resumed
// lease's renewal must record where the checkout genuinely is, not
// defaultLanePath's naming convention — the two diverge whenever the
// lane was set up somewhere other than the convention (here, a plain
// tempdir), and orphans/reap now trust the marker's lane: trailer to
// tell a foreign checkout apart from the real one (gap 2). Recording
// the wrong path would make reap tear down this very lane.
func TestStartResumeRenewsWithTheLaneItIsActuallyRunningFrom(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane,
		Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	_, err = claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	runner, _ := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "lane:    "+lane,
		"the renewed marker names the lane's real path, "+
			"not the naming convention's")
}

// TestStartRefusesWhenAResumeCannotFindItsCurrentPane: a resume that
// cannot read the pane it is running in must not fall through to
// worktree.create — that would fail anyway, since the lane's own
// checkout already occupies the path. It is a stand-up failure, not a
// lease problem: the resume's own renewal above already stands, so
// nothing here is released.
func TestStartRefusesWhenAResumeCannotFindItsCurrentPane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane,
		Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	_, err = claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	t.Chdir(lane)
	rec := &herdrCalls{}
	withHerdr(t, func(args ...string) ([]byte, error) {
		rec.mu.Lock()
		rec.calls = append(rec.calls, append([]string(nil), args...))
		rec.mu.Unlock()
		if len(args) >= 2 && args[0] == "pane" && args[1] == "current" {
			return nil, errors.New("dial unix .herdr.sock: no such file")
		}
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
		}

		return nil, nil
	})
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 1, code, "a resume that cannot find its pane refuses")
	assert.Contains(t, errb.String(), "no such file",
		"the refusal names the cause")
	assert.False(t, rec.verb("worktree", "create"),
		"a resume never falls through to create")

	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	subject, err := gitCapture(t, repo, "log", "-1", "--format=%s", tip)
	require.NoError(t, err)
	assert.Equal(t, "plan 7: beat", subject,
		"the resume's own renewal stands; nothing is released")
}

// TestStartResumesALaneWhoseOwnCommitsAdvancedTheTip: the prescribed
// workflow is raw git commit/push on plan/<id>, with no frit
// transition between — origin's tip ends up a descendant of the
// lane's persisted token under the same epoch. `start` still resumes
// it and stands a fresh agent up, CASing the beat from the fresh tip
// rather than the stale token (S86).
func TestStartResumesALaneWhoseOwnCommitsAdvancedTheTip(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane,
		Session: "wOld:p1"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	_, err = claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	git(t, lane, "commit", "--allow-empty", "-q", "-m", "red: add failing test")
	git(t, lane, "commit", "--allow-empty", "-q", "-m", "green: make it pass")
	git(t, lane, "push", "-q", "origin", "plan/7")
	rawTip, err := gitCapture(t, lane, "rev-parse", "HEAD")
	require.NoError(t, err)

	t.Chdir(lane)
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.NotContains(t, got, "refused")
	assert.Contains(t, got, "resumed plan 7")
	assert.True(t, rec.verb("agent", "start", "plan-7"),
		"a resumed lease still stands a fresh agent up")

	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: beat")
	resumeTip, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	resumeBody, err := gitCapture(t, repo, "log", "-1", "--format=%B", resumeTip)
	require.NoError(t, err)
	assert.Contains(t, resumeBody, "plan 7: beat")
	assert.Contains(t, resumeBody, "epoch:   1", "a resume never bumps the epoch")
	parent, err := gitCapture(t, repo, "rev-parse", resumeTip+"^")
	require.NoError(t, err)
	assert.Equal(t, rawTip, parent,
		"the resume is CASed from origin's fresh tip, not the stale token")
}

// TestStartScavengesALandedRef: the landed cell of the verb-state
// table for start — a claim lost to a ref whose work already merged
// keeps the refusal claim gives, and cleans the leftover ref up the
// same way, rather than merely refusing and leaving it behind.
func TestStartScavengesALandedRef(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, _ := landedLeaseRepo(t, root)
	runner, _ := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "already landed")
	gone, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Empty(t, gone, "the landed ref is scavenged from origin")
}

// TestStartScavengesADoneGlyphOnlyWhenStale: start's refusal of a done
// plan whose lingering ref has matured cleans it up too, the same
// glyph evidence claim scavenges.
func TestStartScavengesADoneGlyphOnlyWhenStale(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, tip := doneGlyphRepo(t, root)
	seedWindow(t, "atlas", 7, tip, 3*time.Hour)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	gone, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Empty(t, gone,
		"a done plan's lingering ref is scavenged under a matured window")
}

// TestStartWarnsRatherThanFailsOnAParkConflict: a scavenge blocked by
// an earlier park at a different tip is a warning, not a dropped
// message — printStart used to render nothing beyond "refused" on this
// path, the same rendering hole printClaim and printRelease had
// already closed.
func TestStartWarnsRatherThanFailsOnAParkConflict(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, _ := doneGlyphRepo(t, root)
	// The glyph lease needs unlanded work for the scavenge to park at
	// all — a marker-only chain has nothing to park and deletes clean.
	git(t, repo, "checkout", "-q", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "w.txt"), []byte("wip\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "work on plan 7")
	git(t, repo, "push", "-q", "origin", "plan/7")
	git(t, repo, "checkout", "-q", "main")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, tip, 3*time.Hour)
	foreignTip, err := gitCapture(t, repo, "rev-parse", "origin/main")
	require.NoError(t, err)
	_, err = gitCapture(t, repo, "push", "-q", "origin",
		foreignTip+":refs/frit/rescue/7/"+hostname()+"-"+tip)
	require.NoError(t, err)
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--root", root},
		&out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "warning",
		"the park conflict reaches start's table output")
	gone, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.NotEmpty(t, gone, "a blocked park leaves the lingering ref alone")
}

// TestEditInEditorRunsAMultiWordEditor: a $EDITOR carrying a flag still
// launches, so the value is split into a command and its arguments rather
// than treated as one binary name.
func TestEditInEditorRunsAMultiWordEditor(t *testing.T) {
	script := filepath.Join(t.TempDir(), "ed.sh")
	require.NoError(t, os.WriteFile(script,
		[]byte("#!/bin/sh\nprintf ' amended' >> \"$1\"\n"), 0o600))
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "sh "+script)

	got, err := editInEditor("/plan-phase 7 3")

	require.NoError(t, err)
	assert.Equal(t, "/plan-phase 7 3 amended", got)
}

// unwindGit fakes the git calls a release transition makes — the
// marker read, the tree, the marker commit — with the push's outcome
// injectable, so the unwind's two endings can both be driven.
func unwindGit(push func() ([]byte, error)) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "log":
			return []byte("plan 7: claim\n\nepoch:   1\nnonce:   c\n" +
				"holder:  h\nlane:    -\nsession: -\n"), nil
		case "rev-parse":
			return []byte("treesha\n"), nil
		case "commit-tree":
			return []byte("markersha\n"), nil
		case "push":
			return push()
		}
		return nil, nil
	}
}

// TestReleaseLeaseSurfacesADanglingLease: when the unwind's release
// push fails, the lease is still live on the remote, and that is
// reported — naming the ref and pointing at frit orphans — rather than
// swallowed.
func TestReleaseLeaseSurfacesADanglingLease(t *testing.T) {
	rt := &runtime{git: unwindGit(func() ([]byte, error) {
		return nil, errors.New("remote hung up")
	})}
	coord := fleet.Coord{Path: "/repo", Remote: "origin"}

	err := releaseLease(rt, coord, discovery.Plan{ID: 7},
		"plan/7", "origin/main", "", "tipsha")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan/7")
	assert.Contains(t, err.Error(), "frit orphans")
}

// TestReleaseLeaseIsSilentWhenTheUnwindTakes: a clean release reports
// nothing, so only a real leak is ever surfaced.
func TestReleaseLeaseIsSilentWhenTheUnwindTakes(t *testing.T) {
	rt := &runtime{git: unwindGit(func() ([]byte, error) {
		return nil, nil
	})}
	coord := fleet.Coord{Path: "/repo", Remote: "origin"}

	err := releaseLease(rt, coord, discovery.Plan{ID: 7},
		"plan/7", "origin/main", "", "tipsha")

	assert.NoError(t, err)
}

// TestHandoffError: a failure after the pane stood up names the
// worktree and the pane; one before it leaves the cause untouched.
func TestHandoffError(t *testing.T) {
	cause := errors.New("prompt: boom")

	named := handoffError("/lanes/atlas-x", "wZ:p1", cause)
	assert.Contains(t, named.Error(), "/lanes/atlas-x")
	assert.Contains(t, named.Error(), "wZ:p1")
	assert.True(t, errors.Is(named, cause), "the cause still unwraps")

	assert.Equal(t, cause, handoffError("/lanes/atlas-x", "", cause),
		"no pane stood up, so there is nothing to name")
}

// TestStartAgentStartRetriesAPaneNotReadyFailure: the pane herdr just
// opened losing the race with its own shell settling — the transient
// Phase 1's teardown cleans up after — is retried to success, so the
// lane comes up started with no teardown and no lease release.
func TestStartAgentStartRetriesAPaneNotReadyFailure(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	prevPause := agentStartPause
	agentStartPause = func() {}
	t.Cleanup(func() { agentStartPause = prevPause })
	rec := &herdrCalls{}
	withHerdr(t, func(args ...string) ([]byte, error) {
		rec.mu.Lock()
		rec.calls = append(rec.calls, append([]string(nil), args...))
		attempt := 0
		for _, c := range rec.calls {
			if len(c) >= 2 && c[0] == "agent" && c[1] == "start" {
				attempt++
			}
		}
		rec.mu.Unlock()
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
		}
		if len(args) >= 2 && args[0] == "agent" && args[1] == "start" {
			if attempt == 1 {
				return nil, errors.New(`{"error":{"code":"agent_pane_busy",` +
					`"message":"agent target pane wZ:p1 is not an available shell"}}`)
			}
			return nil, nil
		}
		if len(args) >= 2 && args[0] == "agent" && args[1] == "list" {
			return []byte(`{"result":{"agents":[{"agent":"claude",` +
				`"agent_status":"working","pane_id":"wZ:p1",` +
				`"agent_session":{"value":"sess-1"}}]}}`), nil
		}
		return nil, nil
	})
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Equal(t, 2, rec.count("agent", "start"),
		"agent start was retried past the transient failure")
	assert.False(t, rec.verb("worktree", "remove"),
		"a retry that succeeds tears nothing down")
	assert.Contains(t, out.String(), "started plan 7")

	subject, err := gitCapture(t, repo,
		"log", "-1", "--format=%s", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Equal(t, "plan 7: beat", subject,
		"the lease is bound to the started session, never released")
}

// TestStartAgentStartDoesNotRetryANonTransientFailure: an agent-start
// error that is not herdr's pane-not-ready signal fails after a
// single attempt and drops straight into Phase 1's teardown.
func TestStartAgentStartDoesNotRetryANonTransientFailure(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	prevPause := agentStartPause
	agentStartPause = func() {}
	t.Cleanup(func() { agentStartPause = prevPause })
	rec := &herdrCalls{}
	withHerdr(t, func(args ...string) ([]byte, error) {
		rec.mu.Lock()
		rec.calls = append(rec.calls, append([]string(nil), args...))
		rec.mu.Unlock()
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
		}
		if len(args) >= 2 && args[0] == "agent" && args[1] == "start" {
			return nil, errors.New("the agent binary is missing")
		}
		return nil, nil
	})
	var out, errb bytes.Buffer

	code := run([]string{"start", "7", "--phase", "3", "--go",
		"--root", root}, &out, &errb)

	require.Equal(t, 1, code, "a real fault is a failure")
	assert.Equal(t, 1, rec.count("agent", "start"),
		"a non-transient error is not retried")
	assert.True(t, rec.verb("worktree", "remove"),
		"the single failed attempt drops into phase 1's teardown")
}

// TestPaneNotReadyMatchesHerdrsSignal: only herdr's own transient —
// code agent_pane_busy, message "not an available shell" — reads as
// retriable; any other agent-start failure is a real fault.
func TestPaneNotReadyMatchesHerdrsSignal(t *testing.T) {
	assert.True(t, paneNotReady(errors.New(`{"error":{"code":"agent_pane_busy",`+
		`"message":"agent target pane wZ:p1 is not an available shell"}}`)))
	assert.False(t, paneNotReady(errors.New("the agent binary is missing")))
	assert.False(t, paneNotReady(nil))
}

// TestStartAgentDoesNotPauseAfterTheFinalAttempt: once every retry is
// spent, startAgent returns straight to the caller's teardown rather
// than pausing once more for an attempt it will never make.
func TestStartAgentDoesNotPauseAfterTheFinalAttempt(t *testing.T) {
	prevPause := agentStartPause
	pauses := 0
	agentStartPause = func() { pauses++ }
	t.Cleanup(func() { agentStartPause = prevPause })
	rt := &runtime{herdr: func(...string) ([]byte, error) {
		return nil, errors.New(`{"error":{"code":"agent_pane_busy",` +
			`"message":"agent target pane wZ:p1 is not an available shell"}}`)
	}}

	err := startAgent(rt, discovery.Plan{ID: 7}, report.StartPlan{}, "wZ:p1")

	require.Error(t, err)
	assert.Equal(t, agentStartAttempts-1, pauses,
		"the pause between attempts never fires after the last one")
}

// TestTeardownHandoffDerivesTheWorkspaceFromThePane: herdr names a
// pane <workspace>:<pane>, so the workspace worktree.remove takes is
// the segment before the colon.
func TestTeardownHandoffDerivesTheWorkspaceFromThePane(t *testing.T) {
	var got []string
	rt := &runtime{herdr: func(args ...string) ([]byte, error) {
		got = args

		return nil, nil
	}}

	err := teardownHandoff(rt, "wZ:p1")

	require.NoError(t, err)
	assert.Equal(t,
		[]string{"worktree", "remove", "--workspace", "wZ", "--json"}, got)
}

// TestTeardownHandoffSurfacesTheRunnerError: a teardown that itself
// fails is reported rather than swallowed, so buildStart's failure
// branch can fall back to naming what was left behind.
func TestTeardownHandoffSurfacesTheRunnerError(t *testing.T) {
	want := errors.New("workspace busy")
	rt := &runtime{herdr: func(...string) ([]byte, error) {
		return nil, want
	}}

	err := teardownHandoff(rt, "wZ:p1")

	assert.ErrorIs(t, err, want)
}
