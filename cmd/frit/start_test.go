package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startHerdr fakes a herdr that answers worktree.create with a pane and
// records every other call, so a test can assert the escalation ran the
// right handshake — and, just as important, never read an agent back.
func startHerdr() (herdr.Runner, *herdrCalls) {
	rec := &herdrCalls{}

	return func(args ...string) ([]byte, error) {
		rec.mu.Lock()
		rec.calls = append(rec.calls, append([]string(nil), args...))
		rec.mu.Unlock()
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
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
	assert.Contains(t, got, "plan/7-shader-unit", "the claim branch")
	assert.Contains(t, got, "/plan-phase 7 3", "the typed prompt")
	assert.Contains(t, got, "--model", "the tier maps to an agent arg")
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
	assert.Contains(t, got, "plan/7-shader-unit", "the claim branch is prescribed")
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
	assert.Equal(t, "plan/7-shader-unit", doc.Branch)
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
	assert.Contains(t, out.String(), "started plan 7")

	_, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7-shader-unit")
	require.NoError(t, err, "frit minted the claim itself")
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
		"rev-parse", "--verify", "refs/heads/plan/7-shader-unit")
	assert.Error(t, err, "no claim was pushed")
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

// TestReleaseClaimSurfacesADanglingClaim: when the unwind's remote delete
// fails, the claim is still on the remote, and that is reported — naming
// the ref and pointing at frit orphans — rather than swallowed.
func TestReleaseClaimSurfacesADanglingClaim(t *testing.T) {
	rt := &runtime{git: func(_ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "push" {
			return nil, errors.New("remote hung up")
		}
		return nil, nil // update-ref -d is best-effort and ignored
	}}
	sc := startContext{repoPath: "/repo", remote: "origin"}

	err := releaseClaim(rt, sc, "plan/7-shader-unit")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan/7-shader-unit")
	assert.Contains(t, err.Error(), "frit orphans")
}

// TestReleaseClaimIsSilentWhenTheUnwindTakes: a clean release reports
// nothing, so only a real leak is ever surfaced.
func TestReleaseClaimIsSilentWhenTheUnwindTakes(t *testing.T) {
	rt := &runtime{git: func(string, ...string) ([]byte, error) {
		return nil, nil
	}}
	err := releaseClaim(rt,
		startContext{repoPath: "/repo", remote: "origin"}, "plan/7-x")
	assert.NoError(t, err)
}
