package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withHerdr installs a fake herdr socket for one test and restores the
// real one after. Unlike git, there is no throwaway server to stand
// up, so the seam is a package variable rather than a temp directory.
func withHerdr(t *testing.T, runner herdr.Runner) {
	t.Helper()
	prev := herdrRunner
	herdrRunner = runner
	t.Cleanup(func() { herdrRunner = prev })
}

// herdrReturning fakes `herdr agent list` with a canned set of panes.
func herdrReturning(agents ...map[string]any) herdr.Runner {
	body, err := json.Marshal(map[string]any{
		"result": map[string]any{"agents": agents},
	})
	if err != nil {
		panic(err)
	}

	return func(...string) ([]byte, error) { return body, nil }
}

// repoOnPlan builds a repository parked on a plan branch, which is what
// a lane under active work looks like.
func repoOnPlan(t *testing.T, parent, name, branch string) string {
	t.Helper()
	repo := initRepo(t, parent, name)
	git(t, repo, "checkout", "-q", "-b", branch)

	return repo
}

func TestWhoListsALiveAgentOnItsPlan(t *testing.T) {
	isolate(t)
	repo := repoOnPlan(t, t.TempDir(), "atlas",
		"plan/2608161808-herdr-join")
	withHerdr(t, herdrReturning(map[string]any{
		"agent":                   "claude",
		"agent_status":            "working",
		"cwd":                     repo,
		"pane_id":                 "wC:p1",
		"workspace_id":            "wC",
		"terminal_title_stripped": "Land the join",
	}))
	var out, errb bytes.Buffer

	code := run([]string{"who", "--root", repo}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "atlas")
	assert.Contains(t, got, "2608161808")
	assert.Contains(t, got, "claude")
	assert.Contains(t, got, "working")
	assert.Contains(t, got, "Land the join")
}

// TestWhoReportsUnknownNeverIdle is the acceptance criterion end to
// end: a pane whose status frit cannot read shows as unknown, and the
// word idle never appears for it.
func TestWhoReportsUnknownNeverIdle(t *testing.T) {
	isolate(t)
	repo := repoOnPlan(t, t.TempDir(), "atlas",
		"plan/2608161808-herdr-join")
	withHerdr(t, herdrReturning(map[string]any{
		"agent":                   "pi",
		"agent_status":            "unknown",
		"cwd":                     repo,
		"pane_id":                 "wP:p2",
		"terminal_title_stripped": "off the record",
	}))
	var out, errb bytes.Buffer

	code := run([]string{"who", "--root", repo}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "unknown")
	assert.NotContains(t, out.String(), "idle")
}

// TestWhoKeepsAPaneOffTheConvention: an agent working in a repository
// on a branch that claims no plan is listed, not dropped.
func TestWhoKeepsAPaneOffTheConvention(t *testing.T) {
	isolate(t)
	repo := repoOnPlan(t, t.TempDir(), "atlas", "feature/side-quest")
	withHerdr(t, herdrReturning(map[string]any{
		"agent":                   "claude",
		"agent_status":            "idle",
		"cwd":                     repo,
		"pane_id":                 "wX:p1",
		"terminal_title_stripped": "wandering",
	}))
	var out, errb bytes.Buffer

	code := run([]string{"who", "--root", repo}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "atlas")
	assert.Contains(t, got, "wandering")
	assert.Contains(t, got, "-", "a lane with no plan is marked, not hidden")
}

// TestWhoSurvivesAMissingSocket is the read-only board's promise: with
// no herdr reachable the command still exits clean, saying so.
func TestWhoSurvivesAMissingSocket(t *testing.T) {
	isolate(t)
	withHerdr(t, func(...string) ([]byte, error) {
		return nil, errors.New("dial unix .herdr.sock: connect: no such file")
	})
	var out, errb bytes.Buffer

	code := run([]string{"who", "--root", t.TempDir()}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "no live agents")
	assert.Contains(t, errb.String(), "herdr")
}

// TestWhoEmitsJSON decodes the document a consumer is written against.
func TestWhoEmitsJSON(t *testing.T) {
	isolate(t)
	repo := repoOnPlan(t, t.TempDir(), "atlas",
		"plan/2608161808-herdr-join")
	withHerdr(t, herdrReturning(map[string]any{
		"agent":                   "claude",
		"agent_status":            "working",
		"cwd":                     repo,
		"pane_id":                 "wC:p1",
		"workspace_id":            "wC",
		"terminal_title_stripped": "Land the join",
	}))
	var doc report.WhoDoc

	emit(t, &doc, "who", "--root", repo)

	assert.Equal(t, "who", doc.Command)
	require.Len(t, doc.Lanes, 1)
	assert.Equal(t, "claude", doc.Lanes[0].Agent)
	assert.Equal(t, herdr.StatusWorking, doc.Lanes[0].Status)
	assert.Equal(t, int64(2608161808), doc.Lanes[0].PlanID)
	assert.Equal(t, "atlas", doc.Lanes[0].Repo)
}
