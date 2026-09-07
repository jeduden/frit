package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withHerdr installs a fake herdr socket for one test and restores the
// real one after. Unlike git, there is no throwaway server to stand
// up, so the seam is a package variable rather than a temp directory.
// The fake stays a plain Runner — it is a closure, not a real
// subprocess, so it has no context to honor — and is adapted to the
// context-aware herdrRunner here, the one place that matters.
func withHerdr(t *testing.T, runner herdr.Runner) {
	t.Helper()
	prev := herdrRunner
	herdrRunner = func(ctx context.Context, args ...string) ([]byte, error) {
		return runner(args...)
	}
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

// herdrReturningWithWorktree is herdrReturning plus a working
// worktree.create — the two combined a claim test needs to exercise
// its veto or takeover mechanics and still reach a stood-up worktree,
// since a bare herdrReturning answers every call with the same
// agent-list body and worktree.create then fails parsing it.
func herdrReturningWithWorktree(agents ...map[string]any) herdr.Runner {
	agentList := herdrReturning(agents...)

	return func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
		}

		return agentList(args...)
	}
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

// TestWhoNamesAnUnreadHostAsAProblem: fleetPresence can succeed for
// the local socket while a configured host still goes unread — that
// per-host failure travels as a problem alongside whatever the local
// read found.
func TestWhoNamesAnUnreadHostAsAProblem(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	withHerdr(t, herdrReturning())
	var doc report.WhoDoc

	emit(t, &doc, "who", "--root", t.TempDir(), "--hosts", "box")

	require.Len(t, doc.Problems, 1)
	assert.Equal(t, "host box", doc.Problems[0].Repo)
}

// TestWhoLanesTieBreaksOnPaneIDWithinTheSamePlan: two staffed panes on
// the same repository and plan sort by pane id, so the board reads
// the same way twice.
func TestWhoLanesTieBreaksOnPaneIDWithinTheSamePlan(t *testing.T) {
	isolate(t)
	repo := repoOnPlan(t, t.TempDir(), "atlas", "plan/7-shader")
	panes := []herdr.Pane{
		{Agent: "claude", CWD: repo, PaneID: "wB:p1"},
		{Agent: "claude", CWD: repo, PaneID: "wA:p1"},
	}

	lanes := whoLanes(panes, gitwt.Exec)

	require.Len(t, lanes, 2)
	assert.Equal(t, "wA:p1", lanes[0].Pane.PaneID)
	assert.Equal(t, "wB:p1", lanes[1].Pane.PaneID)
}

// TestWhoLanesSortsByPlanIDWithinTheSameRepo: two staffed panes whose
// checkouts share a repository name — the ambiguous-basename shape,
// here two different plans rather than two claims of the same one —
// sort by plan id, the return line the same-repo, same-plan tie-break
// never reaches.
func TestWhoLanesSortsByPlanIDWithinTheSameRepo(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repoA := repoOnPlan(t, filepath.Join(root, "a"), "atlas", "plan/9-later")
	repoB := repoOnPlan(t, filepath.Join(root, "b"), "atlas", "plan/7-first")
	panes := []herdr.Pane{
		{Agent: "claude", CWD: repoA, PaneID: "wB:p1"},
		{Agent: "claude", CWD: repoB, PaneID: "wA:p1"},
	}

	lanes := whoLanes(panes, gitwt.Exec)

	require.Len(t, lanes, 2)
	assert.Equal(t, int64(7), lanes[0].PlanID)
	assert.Equal(t, int64(9), lanes[1].PlanID)
}

// TestHoldsForRootIsNilWhenTheConfigCannotBeRead: holdsForRoot's own
// repocfg.Load error, called directly against a broken .frit.yml.
func TestHoldsForRootIsNilWhenTheConfigCannotBeRead(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\n"), 0o600))

	assert.Nil(t, holdsForRoot(repo))
}

// TestHoldsForRootIsNilWhenThePatternCannotCompile: holdsForRoot's own
// cfg.Compiled error, called directly against an uncompilable glob.
func TestHoldsForRootIsNilWhenThePatternCannotCompile(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".frit.yml"),
		[]byte("holds: [\"plan/[\"]\n"), 0o600))

	assert.Nil(t, holdsForRoot(repo))
}
