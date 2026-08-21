package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/observe"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// claimableRepo builds a repository carrying a not-started plan on main,
// with a bare origin to push a lease to — what a plan looks like the
// moment before it is claimed: startable, held by nobody.
func claimableRepo(
	t *testing.T, root, name string, id int, title string,
) string {
	t.Helper()
	repo := initRepo(t, root, name)
	commitPlan(t, repo, id, "🔲", title, nil, "")
	// The origin lives outside root so the fleet walk does not index the
	// bare repository as one of its own.
	origin := filepath.Join(t.TempDir(), name+"-origin.git")
	git(t, repo, "init", "-q", "--bare", "-b", "main", origin)
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "-q", "origin", "main")

	return repo
}

// resumableRepo builds a repository whose plan is in progress on main
// with no hold branch — the state a plan is left in when its first phase
// merged: the 🔳 marker rode in on the merge and the lane that set it is
// gone. Nobody holds it, so it is resumable, not refused.
func resumableRepo(
	t *testing.T, root, name string, id int, title string,
) string {
	t.Helper()
	repo := initRepo(t, root, name)
	commitPlan(t, repo, id, "🔳", title, nil, "")
	origin := filepath.Join(t.TempDir(), name+"-origin.git")
	git(t, repo, "init", "-q", "--bare", "-b", "main", origin)
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "-q", "origin", "main")

	return repo
}

// gitCapture runs git in dir and returns its output and error, for
// asserting on refs a command wrote.
func gitCapture(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", full...).CombinedOutput()

	return strings.TrimSpace(string(out)), err
}

// TestClaimMintsAPickablePlan: a startable plan is leased on its hold
// branch, locally and on origin.
func TestClaimMintsAPickablePlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, _ := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed plan 7")
	assert.Contains(t, out.String(), "branch: plan/7\n",
		"the work ref is id-only")

	local, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err, "the claim ref was minted locally")
	remote, err := gitCapture(t, repo, "ls-remote", "origin",
		"refs/heads/plan/7")
	require.NoError(t, err)
	assert.Contains(t, remote, local, "the same lease is on origin")

	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", local)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: claim", "the tip is the claim marker")
	assert.Contains(t, body, "epoch:   1", "a fresh acquisition is epoch 1")
}

// TestClaimStandsUpItsWorktree is the isolation gate: a successful claim
// hands the lane's checkout to herdr — an isolated worktree, not a bare
// ref that tempts the shared clone. The agent stays start's rung.
func TestClaimStandsUpItsWorktree(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, rec := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.True(t, rec.verb("worktree", "create"),
		"a claim stands its lane's worktree up through herdr")
	assert.True(t, rec.hasArg("plan/7"),
		"the worktree is checked out on the id-only claim branch")
	assert.False(t, rec.verb("agent", "start"),
		"the agent is start's rung, not claim's")
	assert.Contains(t, out.String(), "worktree:",
		"the report names the isolated checkout to work in")
}

// TestClaimWarnsWhenTheWorktreeFails: the ref is atomic and minted first,
// so a herdr that cannot stand the worktree up is a warning, not a lost
// claim — the lease still stands, locally and on origin.
func TestClaimWarnsWhenTheWorktreeFails(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	withHerdr(t, func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return nil, errors.New("herdr down")
		}
		return nil, nil
	})
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed plan 7", "the lease stands")
	assert.Contains(t, out.String(), "warning", "a failed worktree warns")
	_, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err, "the atomic lease is minted before the worktree")
}

// TestLostRaceRefusalNamesTheHolder: the refusal wording distinguishes a
// landed work ref, a lease held by this machine, and one held elsewhere,
// and falls back to the original wording for an unknown or non-race
// error. The facts come off the winner's marker via the HeldError.
func TestLostRaceRefusalNamesTheHolder(t *testing.T) {
	assert.Equal(t,
		"the claim branch has already landed; its status is still open, "+
			"so set plan 7 to ✅",
		lostRaceRefusal(&claim.HeldError{
			PlanID: 7, Known: true, Landed: true}),
		"a merged holder is named as landed, not a competitor")

	assert.Equal(t, "already held on this host (box-a)",
		lostRaceRefusal(&claim.HeldError{
			PlanID: 7, Known: true, ThisHolder: true,
			Marker: claim.Marker{Holder: "box-a"}}),
		"a lease this machine holds names this host")

	assert.Equal(t, "lost the race to another machine (box-b)",
		lostRaceRefusal(&claim.HeldError{
			PlanID: 7, Known: true,
			Marker: claim.Marker{Holder: "box-b"}}),
		"a lease held elsewhere names the other machine")

	assert.Equal(t, "lost the race to another machine",
		lostRaceRefusal(&claim.HeldError{PlanID: 7}),
		"an unread marker falls back to the original wording")

	assert.Equal(t, "lost the race to another machine",
		lostRaceRefusal(&claim.HeldError{PlanID: 7, Known: true}),
		"a known marker with no holder names no machine, not empty parentheses")

	assert.Equal(t, "lost the race to another machine",
		lostRaceRefusal(errors.New("some other error")),
		"a non-HeldError falls back too")
}

// TestClaimReacquiresAReleasedLease: a work ref whose tip is a release
// marker is a lease that ended, not a live hold — the plan is claimable
// again, and the new claim CASes on the release marker at epoch E+1.
func TestClaimReacquiresAReleasedLease(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	_, err = claim.Release(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	runner, _ := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed plan 7",
		"a released lease does not read as held")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: claim", "the tip is a fresh claim")
	assert.Contains(t, body, "epoch:   2", "the re-acquisition reads E+1")
}

// TestClaimRefusesAHeldPlan: a plan a lane already holds is not
// re-claimed, and nothing is pushed.
func TestClaimRefusesAHeldPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	heldPlan(t, root, "atlas", 7, "Shader unit")
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "already held")
}

// TestClaimRefusesABlockedPlan: a plan with an unfinished dependency is
// not claimable, and says why.
func TestClaimRefusesABlockedPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "🔲", "Upstream", nil, "")
	commitPlan(t, repo, 8, "🔲", "Downstream", []int{7}, "")
	var out, errb bytes.Buffer

	code := run([]string{"claim", "8", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "blocked")
}

// TestClaimResumesAnUnheldInProgressPlan: an in-progress plan whose lane
// vanished — 🔳 on main, held by nobody — is resumable, not refused. The
// branch, lane and tier are already prescribed; only the "already in
// progress" guard blocked a resume. frit re-mints the hold on the
// deterministic branch, and Mint's force-with-lease stays the arbiter of
// a live hold.
func TestClaimResumesAnUnheldInProgressPlan(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := resumableRepo(t, root, "atlas", 7, "Shader unit")
	runner, _ := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed plan 7")
	assert.NotContains(t, out.String(), "already in progress")
	_, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err, "the resume mints the hold on the deterministic branch")
}

// TestClaimEmitsJSON decodes the document a consumer reads back to learn
// the branch it now holds.
func TestClaimEmitsJSON(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	claimableRepo(t, root, "atlas", 7, "Shader unit")
	runner, _ := startHerdr()
	withHerdr(t, runner)
	var doc report.ClaimDoc

	emit(t, &doc, "claim", "7", "--root", root)

	assert.Equal(t, "claim", doc.Command)
	assert.True(t, doc.Claimed)
	assert.Equal(t, "plan/7", doc.Branch)
	assert.Equal(t, int64(7), doc.Plan.ID)
	assert.NotEmpty(t, doc.Base, "the lease is dated against a base commit")
	assert.Empty(t, doc.Refused)
	assert.NotEmpty(t, doc.Worktree, "the isolated checkout is reported")
	assert.Empty(t, doc.Warning, "the worktree stood up, so no warning")
}

// TestClaimRefusesAnAmbiguousRepoName: when two checkouts under the root
// share a basename, the fleet cannot tell which one a plan lives in, so
// claim refuses rather than mint the lease into the wrong repository. No
// ref is pushed in either checkout.
func TestClaimRefusesAnAmbiguousRepoName(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repoA := initRepo(t, filepath.Join(root, "a"), "frontend")
	commitPlan(t, repoA, 7, "🔲", "Shader unit", nil, "")
	repoB := initRepo(t, filepath.Join(root, "b"), "frontend")
	commitPlan(t, repoB, 9, "🔲", "Other work", nil, "")
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "shared by another checkout")

	_, err := gitCapture(t, repoA, "rev-parse", "refs/heads/plan/7")
	assert.Error(t, err, "no lease was minted in either checkout")
}

// seedWindow writes an observation state whose window over tip has
// been maturing for span, last confirmed a minute ago — the state a
// faithful observer holds over a dead holder's unmoving ref.
func seedWindow(
	t *testing.T, repo string, id int64, tip string, span time.Duration,
) {
	t.Helper()
	path, err := observe.Path()
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, observe.Save(path, observe.State{
		observe.Key(repo, id): discovery.Window{
			Tip:     tip,
			First:   now.Add(-span),
			Last:    now.Add(-time.Minute),
			Samples: 9,
		},
	}))
}

// TestClaimTakesOverAStaleLease: a held plan whose takeover window has
// matured is not refused — claim executes the takeover CAS, and the
// new tip is a takeover marker, child of the stale tip, epoch E+1.
func TestClaimTakesOverAStaleLease(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, lease.Tip, 3*time.Hour)
	runner, _ := startHerdr()
	withHerdr(t, runner)
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed plan 7",
		"a matured stale hold is taken over, not refused")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: takeover")
	assert.Contains(t, body, "epoch:   2", "the takeover reads E+1")
	parent, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, parent,
		"the marker is a child of the observed stale tip")
}

// TestClaimStillRefusesALiveLease: the same held plan with no matured
// window keeps today's refusal — staleness is the only door in.
func TestClaimStillRefusesALiveLease(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
}

// TestClaimRefusesATakeoverVetoedByALiveSession: a matured window
// would normally open the takeover door, but herdr positively
// confirms the holder's bound session is still live, so the takeover
// is refused and a beat is pushed on the holder's own behalf instead
// (F3, S61).
func TestClaimRefusesATakeoverVetoedByALiveSession(t *testing.T) {
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

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "live agent session")

	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: beat",
		"the holder's own lease was renewed, not seized")
	assert.Contains(t, body, "epoch:   1")
	assert.Contains(t, body, "holder:  elsewhere",
		"the beat renews the holder's lease, not this run's own")
	assert.Contains(t, body, "session: wS:p9")
	parent, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, parent)
}

// TestClaimTakesOverWhenHerdrCannotAnswer: no answer is no veto (F3) —
// an unreachable herdr must not protect a matured stale hold forever.
func TestClaimTakesOverWhenHerdrCannotAnswer(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wS:p9"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	seedWindow(t, "atlas", 7, lease.Tip, 3*time.Hour)
	withHerdr(t, func(...string) ([]byte, error) {
		return nil, errors.New("dial unix .herdr.sock: no such file")
	})
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed plan 7")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: takeover")
	assert.Contains(t, body, "epoch:   2")
	parent, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, parent)
}

// TestClaimTakesOverWhenTheBoundSessionIsGone: herdr answers, but the
// holder's bound session is not among the live panes — the session is
// unknown to herdr, which is no veto either.
func TestClaimTakesOverWhenTheBoundSessionIsGone(t *testing.T) {
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
		"pane_id":      "wOther:p1",
		"agent_session": map[string]any{
			"value": "wOther:session",
		},
	}))
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed plan 7")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: takeover")
	assert.Contains(t, body, "epoch:   2")
}

// TestClaimTakesOverADeadSessionWithNoMaturedWindow: herdr positively
// confirms the holder's bound session is gone, so the plan is a
// takeover candidate at once — no staleness window is seeded at all
// (2608212203, S76).
func TestClaimTakesOverADeadSessionWithNoMaturedWindow(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wS:p9"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	withHerdr(t, herdrReturning(map[string]any{
		"agent":        "claude",
		"agent_status": "working",
		"pane_id":      "wOther:p1",
		"agent_session": map[string]any{
			"value": "wOther:session",
		},
	}))
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "claimed plan 7",
		"a confirmed-dead session is a candidate with no matured window")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: takeover")
	assert.Contains(t, body, "epoch:   2")
	parent, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, parent,
		"the takeover is a child of exactly the observed tip")
}

// TestClaimStillRefusesALiveSessionWithNoMaturedWindow pins the
// baseline: a live bound session is never taken over, whether or not
// a window has matured.
func TestClaimStillRefusesALiveSessionWithNoMaturedWindow(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wS:p9"}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	withHerdr(t, herdrReturning(map[string]any{
		"agent":        "claude",
		"agent_status": "working",
		"pane_id":      "wS:p9",
		"agent_session": map[string]any{
			"value": "wS:p9",
		},
	}))
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused",
		"a live session is not a candidate on its own")
	_, err = gitCapture(t, repo, "rev-parse", "refs/heads/plan/7^2")
	assert.Error(t, err, "nothing was taken over")
}

// TestClaimStillRefusesAnUnreachableHerdrWithNoMaturedWindow pins the
// other baseline: an unreachable herdr cannot confirm a death, so it
// falls back to the window rule exactly as before this signal existed
// — not a candidate, since the window has not matured either.
func TestClaimStillRefusesAnUnreachableHerdrWithNoMaturedWindow(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x",
		Session: "wS:p9"}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	withHerdr(t, func(...string) ([]byte, error) {
		return nil, errors.New("dial unix .herdr.sock: no such file")
	})
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused",
		"an unreachable herdr falls back to the window rule")
}

// TestClaimResumesItsOwnLeaseFromThePersistedToken: a lane whose
// persisted token matches origin's current tip, with no live session
// bound to it, resumes on the spot — no matured window is seeded, so
// the ordinary "already held" refusal would otherwise fire (F9, F11,
// S3, S21).
func TestClaimResumesItsOwnLeaseFromThePersistedToken(t *testing.T) {
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
	withHerdr(t, herdrReturning())
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	got := out.String()
	assert.Contains(t, got, "resumed plan 7")
	assert.NotContains(t, got, "refused")

	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", tip)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 7: beat")
	assert.Contains(t, body, "epoch:   1", "a resume never bumps the epoch")
	parent, err := gitCapture(t, repo, "rev-parse", tip+"^")
	require.NoError(t, err)
	assert.Equal(t, renewed.Tip, parent)
}

// TestResumeIgnoresATokenFromAnotherLane: `claim` run from anywhere
// other than the plan's own worktree must not go looking for a local
// token — the ordinary "already held" refusal stands, and no beat is
// pushed.
func TestResumeIgnoresATokenFromAnotherLane(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	lane := filepath.Join(t.TempDir(), "atlas-lane")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: hostname(), Lane: lane}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "worktree", "add", "-q", lane, "plan/7")
	_, err = claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	// Deliberately not chdir'd into lane: an unrelated directory.
	withHerdr(t, herdrReturning())
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	assert.Contains(t, out.String(), "already held")
}

// landedLeaseRepo is a claimable repo whose lease landed: work on the
// ref merged into main and pushed, the ref left behind, the plan's
// status still open — landed work with a stale status.
func landedLeaseRepo(t *testing.T, root string) (string, string) {
	t.Helper()
	repo := claimableRepo(t, root, "atlas", 7, "Shader unit")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	_, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)
	git(t, repo, "checkout", "-q", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "w.txt"), []byte("done\n"), 0o600))
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-q", "-m", "work on plan 7")
	git(t, repo, "push", "-q", "origin", "plan/7")
	git(t, repo, "checkout", "-q", "main")
	git(t, repo, "merge", "-q", "--no-ff", "-m", "land plan 7", "plan/7")
	git(t, repo, "push", "-q", "origin", "main")
	tip, err := gitCapture(t, repo, "rev-parse", "refs/heads/plan/7")
	require.NoError(t, err)

	return repo, tip
}

// TestClaimScavengesALandedRef: a claim lost to a ref whose work
// already merged keeps today's refusal wording — and cleans the
// leftover ref up, ancestry evidence tied to the very tip it deletes.
func TestClaimScavengesALandedRef(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, _ := landedLeaseRepo(t, root)
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "already landed",
		"the refusal keeps today's wording")
	assert.Contains(t, out.String(), "set plan 7 to ✅")
	gone, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Empty(t, gone, "the landed ref is scavenged from origin")
	rescue, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/frit/rescue/*")
	require.NoError(t, err)
	assert.Empty(t, rescue, "everything landed; there is nothing to park")
}

// doneGlyphRepo builds a repo whose plan is ✅ on main while a
// live-looking lease ref lingers — the squash-merge shape ancestry
// evidence cannot see. Returns the repo and the lingering tip.
func doneGlyphRepo(t *testing.T, root string) (string, string) {
	t.Helper()
	repo := initRepo(t, root, "atlas")
	commitPlan(t, repo, 7, "✅", "Shader unit", nil, "")
	origin := filepath.Join(t.TempDir(), "atlas-origin.git")
	git(t, repo, "init", "-q", "--bare", "-b", "main", origin)
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "-q", "origin", "main")
	opts := claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	require.NoError(t, err)

	return repo, lease.Tip
}

// TestClaimScavengesADoneGlyphOnlyWhenStale: glyph evidence — ✅ on
// the default branch — is not tied to the tip, so the scavenge fires
// only under a matured window. A fresh window leaves the ref alone,
// so a live, renewing holder is never scavenged.
func TestClaimScavengesADoneGlyphOnlyWhenStale(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, tip := doneGlyphRepo(t, root)
	seedWindow(t, "atlas", 7, tip, 3*time.Hour)
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	gone, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Empty(t, gone,
		"a done plan's lingering ref is scavenged under a matured window")
}

// TestClaimLeavesADoneGlyphWithAFreshWindow: the same shape without a
// matured window is left alone — the holder may just be quiet.
func TestClaimLeavesADoneGlyphWithAFreshWindow(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo, tip := doneGlyphRepo(t, root)
	var out, errb bytes.Buffer

	code := run([]string{"claim", "7", "--root", root}, &out, &errb)

	require.Equal(t, 0, code, errb.String())
	assert.Contains(t, out.String(), "refused")
	remote, err := gitCapture(t, repo,
		"ls-remote", "origin", "refs/heads/plan/7")
	require.NoError(t, err)
	assert.Contains(t, remote, tip, "a fresh window leaves the ref alone")
}
