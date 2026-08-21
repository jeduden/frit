package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// yieldHerdr fakes a herdr socket that answers pane.current with the
// calling pane's own workspace and records every other call, so a
// test can assert yield tore its own lane down and read no agent back.
func yieldHerdr(workspace string) (herdr.Runner, *herdrCalls) {
	rec := &herdrCalls{}

	return func(args ...string) ([]byte, error) {
		rec.mu.Lock()
		rec.calls = append(rec.calls, append([]string(nil), args...))
		rec.mu.Unlock()
		if len(args) >= 2 && args[0] == "pane" && args[1] == "current" {
			return []byte(fmt.Sprintf(
				`{"result":{"pane":{"pane_id":"%s:p1","workspace_id":%q}}}`,
				workspace, workspace)), nil
		}

		return nil, nil
	}, rec
}

// fenceWithATakeover simulates another machine seizing a plan's work
// ref out from under repo's local copy: a fresh clone of the same
// origin takes the ref over at its current tip, and repo's own local
// ref is left exactly where it was — the local divergence a fenced
// lane still carries.
func fenceWithATakeover(t *testing.T, repo string, planID int64) {
	t.Helper()
	origin, err := gitCapture(t, repo, "config", "--get", "remote.origin.url")
	require.NoError(t, err)
	tip, err := gitCapture(t, repo, "rev-parse",
		fmt.Sprintf("refs/heads/plan/%d", planID))
	require.NoError(t, err)

	tmp := t.TempDir()
	other := filepath.Join(tmp, "elsewhere")
	git(t, tmp, "clone", "-q", origin, other)
	_, err = claim.Takeover(other, claim.LeaseOptions{
		PlanID: planID, Remote: "origin", Base: "origin/main",
		Holder: "elsewhere", Lane: "/lanes/x",
	}, tip, gitwt.Exec)
	require.NoError(t, err)
}

// TestYieldParksAFencedLaneAndTearsItDown: a lane fenced out by a
// takeover parks its local divergence to the rescue ref and hands its
// own pane's teardown to herdr — no agent read, just the workspace.
func TestYieldParksAFencedLaneAndTearsItDown(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	cr, _ := startHerdr()
	withHerdr(t, cr)
	var claimed bytes.Buffer
	code := run([]string{"claim", "7", "--root", root}, &claimed, &claimed)
	require.Equal(t, 0, code, claimed.String())

	fenceWithATakeover(t, repo, 7)

	runner, rec := yieldHerdr("w1A")
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code = run([]string{"yield", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "parked")
	assert.Contains(t, out.String(), "refs/frit/rescue/7/",
		"the rescue ref is named for the plan")
	rescue, err := gitCapture(t, repo, "ls-remote", "origin",
		"refs/frit/rescue/7/*")
	require.NoError(t, err)
	assert.NotEmpty(t, rescue, "the fenced lane's divergence was parked")
	assert.True(t, rec.verb("worktree", "remove"),
		"yield tears its own lane down through herdr")
	assert.True(t, rec.hasArg("w1A"),
		"the workspace torn down is the pane's own")
	assert.False(t, rec.verb("agent", "read"),
		"yield never reads an agent back")
}

// TestYieldRefusesTheCurrentHolder: a lane whose local tip still
// matches origin's is not fenced — yield refuses rather than treat
// itself as an alias for release, and nothing is parked or torn down.
func TestYieldRefusesTheCurrentHolder(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	cr, _ := startHerdr()
	withHerdr(t, cr)
	var claimed bytes.Buffer
	code := run([]string{"claim", "7", "--root", root}, &claimed, &claimed)
	require.Equal(t, 0, code, claimed.String())

	runner, rec := yieldHerdr("w1A")
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code = run([]string{"yield", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	rescue, err := gitCapture(t, repo, "ls-remote", "origin",
		"refs/frit/rescue/*")
	require.NoError(t, err)
	assert.Empty(t, rescue, "the live holder's lease is not parked")
	assert.False(t, rec.verb("worktree", "remove"),
		"a refusal tears nothing down")
}
