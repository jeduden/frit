package claim

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leaseOptions is a lease acquisition for plan 7, parameterised by who
// is acquiring and from where — the two things a race varies.
func leaseOptions(holder, lane string) LeaseOptions {
	return LeaseOptions{
		PlanID: 7,
		Remote: "origin",
		Base:   "origin/main",
		Holder: holder,
		Lane:   lane,
	}
}

// TestAcquireRaceHasOneWinnerAndNamesTheLease: two machines acquire one
// plan id; the server-side CAS picks exactly one winner, and the
// loser's error carries the winner's epoch, machine id and lane read
// off the winning marker — the facts a refusal needs, not a guess.
func TestAcquireRaceHasOneWinnerAndNamesTheLease(t *testing.T) {
	first := originAndClone(t)
	second := cloneAgain(t, first)

	won, err := Acquire(first, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	assert.Equal(t, "plan/7", won.Branch, "the work ref is id-only")
	assert.Equal(t, 1, won.Epoch, "a fresh acquisition is epoch 1")

	_, err = Acquire(second, leaseOptions("box-b", "/lanes/b"), gitwt.Exec)

	var held *HeldError
	require.ErrorAs(t, err, &held)
	assert.True(t, errors.Is(err, ErrLostRace),
		"the sentinel still tells a lost race from a git fault")
	require.True(t, held.Known, "the winner's marker was read")
	assert.Equal(t, 1, held.Marker.Epoch)
	assert.Equal(t, "box-a", held.Marker.Holder)
	assert.Equal(t, "/lanes/a", held.Marker.Lane)
	assert.False(t, held.ThisHolder)
	assert.False(t, held.Landed)
}

// TestAcquireMarkersNeverShareASHA: two acquisitions identical in
// everything else — same plan, holder, lane, base and commit
// timestamps — still mint distinct marker SHAs. The nonce is what makes
// SHA-based CAS ABA-proof (A3): a deterministic marker could be
// recreated at an old SHA, and a pending takeover would then fire
// against the fresh lease.
func TestAcquireMarkersNeverShareASHA(t *testing.T) {
	t.Setenv("GIT_AUTHOR_DATE", "2026-08-21T00:00:00Z")
	t.Setenv("GIT_COMMITTER_DATE", "2026-08-21T00:00:00Z")
	first := originAndClone(t)
	second := originAndClone(t) // a twin: same history, its own origin
	require.Equal(t,
		gitCmd(t, first, "rev-parse", "origin/main"),
		gitCmd(t, second, "rev-parse", "origin/main"),
		"the twin fixtures are identical up to the marker")

	a, err := Acquire(first, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	b, err := Acquire(second, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	assert.NotEqual(t, a.Tip, b.Tip,
		"the nonce keeps every marker SHA unique")
}

// TestAcquireIsRenameProof: the work ref embeds the plan id and nothing
// else, so a plan file renamed between two acquires still funnels both
// machines onto the same ref and the same single winner — a rename can
// no longer mint a second hold (S27).
func TestAcquireIsRenameProof(t *testing.T) {
	first := originAndClone(t)
	second := cloneAgain(t, first)

	_, err := Acquire(first, leaseOptions("box-a", "/lanes/old-name"), gitwt.Exec)
	require.NoError(t, err)

	// The second machine knows the plan file by a new name; nothing of
	// that name reaches the ref, which is the point.
	_, err = Acquire(second, leaseOptions("box-b", "/lanes/new-name"), gitwt.Exec)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrLostRace))

	refs := gitCmd(t, first, "ls-remote", "--heads", "origin", "refs/heads/plan/*")
	assert.Equal(t, 1, strings.Count(refs, "refs/heads/plan/"),
		"one work ref carries the plan, whatever the file is called")
}

// TestRenewAdvancesTheTipAndNothingElse: a renewal CASes from the
// holder's recorded tip to a beat marker that is its child — same
// epoch, fresh nonce — so the lease stays live without a second
// acquisition.
func TestRenewAdvancesTheTipAndNothingElse(t *testing.T) {
	work := originAndClone(t)
	opts := leaseOptions("box-a", "/lanes/a")
	lease, err := Acquire(work, opts, gitwt.Exec)
	require.NoError(t, err)

	renewed, err := Renew(work, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, lease.Epoch, renewed.Epoch,
		"a renewal never bumps the epoch")
	assert.Equal(t, lease.Tip,
		gitCmd(t, work, "rev-parse", renewed.Tip+"^"),
		"the beat is a child of the recorded tip")
	remote := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Contains(t, remote, renewed.Tip, "origin advanced to the beat")
	body := gitCmd(t, work, "log", "-1", "--format=%B", renewed.Tip)
	assert.Contains(t, body, "plan 7: beat")
	assert.Contains(t, body, "epoch:   1")
}

// TestRenewAfterAForeignMoveIsFenced: a renewal from a tip the remote
// no longer holds is rejected by the CAS — the holder is fenced — and
// the error names the machine that moved the ref, read off the mover's
// marker. The loser moves nothing.
func TestRenewAfterAForeignMoveIsFenced(t *testing.T) {
	first := originAndClone(t)
	opts := leaseOptions("box-a", "/lanes/a")
	lease, err := Acquire(first, opts, gitwt.Exec)
	require.NoError(t, err)

	// Another machine takes the ref over: a child of the tip carrying
	// its own marker, pushed the way a matured takeover lands.
	second := cloneAgain(t, first)
	gitCmd(t, second, "fetch", "-q", "origin",
		"refs/heads/plan/7:refs/heads/plan/7")
	tree := gitCmd(t, second, "rev-parse", "plan/7^{tree}")
	foreign := gitCmd(t, second, "commit-tree", tree, "-p", lease.Tip, "-m",
		"plan 7: takeover\n\nepoch:   2\nnonce:   feed\nholder:  box-b\n"+
			"lane:    /lanes/b\nsession: -")
	gitCmd(t, second, "push", "-q", "-f", "origin",
		foreign+":refs/heads/plan/7")

	_, err = Renew(first, opts, lease.Tip, gitwt.Exec)

	var fenced *FenceError
	require.ErrorAs(t, err, &fenced)
	require.True(t, fenced.Known, "the mover's marker was read")
	assert.Equal(t, "box-b", fenced.Marker.Holder, "the fence names the mover")
	assert.Contains(t, err.Error(), "box-b")
	remote := gitCmd(t, first, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Contains(t, remote, foreign, "the fenced holder moved nothing")
	assert.Equal(t, lease.Tip,
		gitCmd(t, first, "rev-parse", "refs/heads/plan/7"),
		"the local ref is rolled back to the recorded tip")
}

// TestReleaseLeavesAMarkerAndReacquireBumpsTheEpoch: a release pushes a
// marker and deletes nothing — the history stays for the next holder —
// and a later acquire CASes exactly on that marker, reading epoch E+1
// so the old lease can never be confused with the new one.
func TestReleaseLeavesAMarkerAndReacquireBumpsTheEpoch(t *testing.T) {
	first := originAndClone(t)
	opts := leaseOptions("box-a", "/lanes/a")
	lease, err := Acquire(first, opts, gitwt.Exec)
	require.NoError(t, err)

	released, err := Release(first, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	remote := gitCmd(t, first, "ls-remote", "origin", "refs/heads/plan/7")
	require.Contains(t, remote, released.Tip,
		"the ref survives on origin, marked released")
	assert.Equal(t, "plan 7: release",
		gitCmd(t, first, "log", "-1", "--format=%s", released.Tip))
	assert.Equal(t, lease.Tip,
		gitCmd(t, first, "rev-parse", released.Tip+"^"),
		"the release marker is a child of the released tip")
	assert.Equal(t, 1, released.Epoch, "a release keeps the epoch it ends")

	second := cloneAgain(t, first)
	re, err := Acquire(second, leaseOptions("box-b", "/lanes/b"), gitwt.Exec)
	require.NoError(t, err, "a released plan is acquirable again")
	assert.Equal(t, 2, re.Epoch, "the new lease reads epoch E+1")
	assert.Equal(t, released.Tip,
		gitCmd(t, second, "rev-parse", re.Tip+"^"),
		"the new claim is a child of the release marker")
}

// TestLeaseMessage pins the marker body: the kind in the subject, the
// trailers beneath it, "-" for an empty lane or session, and the base
// trailer only where a claim carries one.
func TestLeaseMessage(t *testing.T) {
	claim := leaseMessage("claim",
		LeaseOptions{PlanID: 7, Holder: "box-a", Lane: "/lanes/a"},
		1, "cafe", "basesha")
	want := "plan 7: claim\n\n" +
		"epoch:   1\n" +
		"nonce:   cafe\n" +
		"holder:  box-a\n" +
		"lane:    /lanes/a\n" +
		"session: -\n" +
		"base:    basesha"
	assert.Equal(t, want, claim)

	beat := leaseMessage("beat",
		LeaseOptions{PlanID: 7, Holder: "box-a"}, 3, "f00d", "")
	assert.Contains(t, beat, "plan 7: beat")
	assert.Contains(t, beat, "epoch:   3")
	assert.Contains(t, beat, "lane:    -", "an empty lane records -")
	assert.NotContains(t, beat, "base:", "only a claim carries a base")
}

// TestParseMarker round-trips the message builder, and reports not-a-
// marker for a plain work commit and for another plan's marker.
func TestParseMarker(t *testing.T) {
	body := leaseMessage("claim",
		LeaseOptions{PlanID: 7, Holder: "box-a", Lane: "/lanes/a",
			Session: "w0:p1"}, 2, "cafe", "basesha")

	m, ok := parseMarker(7, body)
	require.True(t, ok)
	assert.Equal(t, Marker{Kind: "claim", Epoch: 2, Nonce: "cafe",
		Holder: "box-a", Lane: "/lanes/a", Session: "w0:p1",
		Base: "basesha"}, m)

	_, ok = parseMarker(7, "fix the shader unit\n\nreal work")
	assert.False(t, ok, "a work commit is not a marker")
	_, ok = parseMarker(8, body)
	assert.False(t, ok, "another plan's marker does not read as this one's")
}

// TestReleased reads the tip's subject and nothing else: a release
// marker for this plan answers true; work commits, other kinds, other
// plans and unreadable objects answer false.
func TestReleased(t *testing.T) {
	subject := func(s string) gitwt.Runner {
		return func(string, ...string) ([]byte, error) {
			return []byte(s + "\n"), nil
		}
	}

	assert.True(t, Released("/r", "tip", 7, subject("plan 7: release")))
	assert.False(t, Released("/r", "tip", 7, subject("plan 7: claim")))
	assert.False(t, Released("/r", "tip", 7, subject("plan 8: release")))
	assert.False(t, Released("/r", "tip", 7, subject("real work")))
	assert.False(t, Released("/r", "tip", 7,
		func(string, ...string) ([]byte, error) {
			return nil, errors.New("bad object")
		}))
}

// TestNewNonce: every nonce is fresh and non-empty; two calls never
// agree, which is what keeps two otherwise-identical markers apart.
func TestNewNonce(t *testing.T) {
	a, err := newNonce()
	require.NoError(t, err)
	b, err := newNonce()
	require.NoError(t, err)
	assert.NotEmpty(t, a)
	assert.NotEqual(t, a, b)
}

// TestMarkerHostReadsALeaseMarker: the current-worktree guard reads
// the holder off a lease marker too — the id-only subject with the
// holder trailer — so standing in a checkout another machine leases
// still refuses the inference.
func TestMarkerHostReadsALeaseMarker(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, "box-a", MarkerHost(work, "plan/7", 7, gitwt.Exec))
}

// TestTakeoverSeizesAStaleTip: the takeover marker is a child of
// exactly the observed tip at epoch E+1, so the taker inherits every
// pushed commit and the old lease can never be confused with the new.
func TestTakeoverSeizesAStaleTip(t *testing.T) {
	first := originAndClone(t)
	lease, err := Acquire(first, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	second := cloneAgain(t, first)
	taken, err := Takeover(second,
		leaseOptions("box-b", "/lanes/b"), lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, 2, taken.Epoch, "the takeover reads epoch E+1")
	assert.Equal(t, lease.Tip,
		gitCmd(t, second, "rev-parse", taken.Tip+"^"),
		"the marker is a child of the observed stale tip")
	assert.Equal(t, "plan 7: takeover",
		gitCmd(t, second, "log", "-1", "--format=%s", taken.Tip))
	remote := gitCmd(t, second, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Contains(t, remote, taken.Tip, "origin carries the takeover")
}

// TestTakeoverRacesARenewalOneCASWins: the holder renews between the
// observation and the takeover; the takeover CASes on the stale tip,
// loses to the server, re-reads, and names the live holder. The
// renewal stands untouched.
func TestTakeoverRacesARenewalOneCASWins(t *testing.T) {
	first := originAndClone(t)
	a := leaseOptions("box-a", "/lanes/a")
	lease, err := Acquire(first, a, gitwt.Exec)
	require.NoError(t, err)
	second := cloneAgain(t, first) // observes lease.Tip, then goes quiet

	renewed, err := Renew(first, a, lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	_, err = Takeover(second,
		leaseOptions("box-b", "/lanes/b"), lease.Tip, gitwt.Exec)

	var held *HeldError
	require.ErrorAs(t, err, &held)
	require.True(t, held.Known, "the loser re-read the live lease")
	assert.Equal(t, "box-a", held.Marker.Holder,
		"the refusal names the holder whose renewal won")
	remote := gitCmd(t, second, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Contains(t, remote, renewed.Tip,
		"the renewal stands; the loser moved nothing")
}

// workOn commits one file on the plan's work ref and pushes it, so
// the lease carries real work beyond its markers.
func workOn(t *testing.T, repo string) string {
	t.Helper()
	gitCmd(t, repo, "checkout", "-q", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "w.txt"), []byte("wip\n"), 0o600))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-q", "-m", "unlanded work")
	gitCmd(t, repo, "push", "-q", "origin", "plan/7")
	gitCmd(t, repo, "checkout", "-q", "main")

	return gitCmd(t, repo, "rev-parse", "refs/heads/plan/7")
}

// TestScavengeParksUnlandedWorkThenDeletes: a chain carrying commits
// that never landed is parked to the rescue ref before the delete, so
// scavenge never destroys work.
func TestScavengeParksUnlandedWorkThenDeletes(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	tip := workOn(t, work)

	sc, err := Scavenge(work, leaseOptions("box-b", "/lanes/b"), tip, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, "refs/frit/rescue/7/box-b", sc.Rescue,
		"the rescue ref is named for the plan and the scavenger")
	rescue := gitCmd(t, work, "ls-remote", "origin", sc.Rescue)
	assert.Contains(t, rescue, tip, "the rescue ref carries the old tip")
	gone := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Empty(t, gone, "the work ref is deleted from origin")
	_, localErr := gitCapture(t, work,
		"rev-parse", "--verify", "refs/heads/plan/7")
	assert.Error(t, localErr, "the local copy is cleaned up too")
}

// TestScavengeMarkerOnlyDeletesWithoutParking: a ref carrying nothing
// but frit's own markers has no work to lose — no rescue ref is
// created, the delete alone suffices.
func TestScavengeMarkerOnlyDeletesWithoutParking(t *testing.T) {
	work := originAndClone(t)
	lease, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	sc, err := Scavenge(
		work, leaseOptions("box-b", "/lanes/b"), lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	assert.Empty(t, sc.Rescue, "markers are not work; nothing is parked")
	rescue := gitCmd(t, work, "ls-remote", "origin", "refs/frit/rescue/*")
	assert.Empty(t, rescue, "no rescue ref was created")
	gone := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Empty(t, gone)
}

// TestScavengeRefusesAMovedTip: the evidence is tied to the tip it
// deletes. A holder that renewed moved the tip, so the scavenge fails
// and deletes nothing — a renewing holder is never scavenged (A2).
func TestScavengeRefusesAMovedTip(t *testing.T) {
	first := originAndClone(t)
	opts := leaseOptions("box-a", "/lanes/a")
	lease, err := Acquire(first, opts, gitwt.Exec)
	require.NoError(t, err)
	second := cloneAgain(t, first) // observed lease.Tip, then went quiet

	renewed, err := Renew(first, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	_, err = Scavenge(
		second, leaseOptions("box-b", "/lanes/b"), lease.Tip, gitwt.Exec)

	require.Error(t, err)
	remote := gitCmd(t, second, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Contains(t, remote, renewed.Tip, "nothing was deleted")
	rescue := gitCmd(t, second, "ls-remote", "origin", "refs/frit/rescue/*")
	assert.Empty(t, rescue, "nothing was parked over")
}

// TestScavengeIsIdempotent: a second scavenge of a ref already gone is
// a clean no-op, and a retry after a half-done run — rescue parked,
// delete never landed — parks nothing again and finishes the delete.
func TestScavengeIsIdempotent(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	tip := workOn(t, work)
	opts := leaseOptions("box-b", "/lanes/b")

	// A half-done earlier run already parked the tip.
	gitCmd(t, work, "push", "-q", "origin", tip+":refs/frit/rescue/7/box-b")

	sc, err := Scavenge(work, opts, tip, gitwt.Exec)
	require.NoError(t, err, "an existing rescue at the same tip is already parked")
	assert.Equal(t, "refs/frit/rescue/7/box-b", sc.Rescue)
	gone := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Empty(t, gone, "the retry finished the delete")

	sc, err = Scavenge(work, opts, tip, gitwt.Exec)
	require.NoError(t, err, "a ref already gone is a clean no-op")
	assert.Empty(t, sc.Rescue)
	rescue := gitCmd(t, work, "ls-remote", "origin", "refs/frit/rescue/7/box-b")
	assert.Contains(t, rescue, tip, "the rescue ref is never clobbered")
}

// TestScavengeRefusesAForeignRescue: a rescue ref that already exists
// at a different tip is somebody's parked work — scavenge refuses to
// clobber it and leaves the work ref alone rather than delete work it
// could not park.
func TestScavengeRefusesAForeignRescue(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	tip := workOn(t, work)
	other := gitCmd(t, work, "rev-parse", "origin/main")
	gitCmd(t, work, "push", "-q", "origin", other+":refs/frit/rescue/7/box-b")

	_, err = Scavenge(
		work, leaseOptions("box-b", "/lanes/b"), tip, gitwt.Exec)

	require.Error(t, err)
	remote := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Contains(t, remote, tip, "the work ref is not deleted")
	rescue := gitCmd(t, work, "ls-remote", "origin", "refs/frit/rescue/7/box-b")
	assert.Contains(t, rescue, other, "the foreign rescue is untouched")
}

// localWork commits one file on the plan's work ref without pushing —
// the local divergence a fenced lane still carries after it lost a
// race it never fetched.
func localWork(t *testing.T, repo string) string {
	t.Helper()
	gitCmd(t, repo, "checkout", "-q", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "w.txt"), []byte("wip\n"), 0o600))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-q", "-m", "unlanded work")
	tip := gitCmd(t, repo, "rev-parse", "refs/heads/plan/7")
	gitCmd(t, repo, "checkout", "-q", "main")

	return tip
}

// TestYieldParksLocalDivergenceOfAFencedLane: a lane fenced out by a
// takeover still carries the local commits it made before losing the
// race; yield parks that divergence to the rescue ref, create-only,
// without touching the work ref the takeover now holds.
func TestYieldParksLocalDivergenceOfAFencedLane(t *testing.T) {
	first := originAndClone(t)
	opts := leaseOptions("box-a", "/lanes/a")
	lease, err := Acquire(first, opts, gitwt.Exec)
	require.NoError(t, err)
	local := localWork(t, first)

	second := cloneAgain(t, first)
	_, err = Takeover(
		second, leaseOptions("box-b", "/lanes/b"), lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	sc, err := Yield(first, opts, local, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, "refs/frit/rescue/7/box-a", sc.Rescue,
		"the rescue ref is named for the plan and the fenced lane")
	rescue := gitCmd(t, first, "ls-remote", "origin", sc.Rescue)
	assert.Contains(t, rescue, local, "the divergence is parked")
	remote := gitCmd(t, first, "ls-remote", "origin", "refs/heads/plan/7")
	assert.NotContains(t, remote, local,
		"the takeover still holds the work ref; yield never CASes it")
}

// TestYieldRefusesTheCurrentHolder: a lane whose local tip still
// matches origin's is not fenced — yield is for the fenced, not an
// alias for release — so it refuses and parks nothing.
func TestYieldRefusesTheCurrentHolder(t *testing.T) {
	first := originAndClone(t)
	opts := leaseOptions("box-a", "/lanes/a")
	lease, err := Acquire(first, opts, gitwt.Exec)
	require.NoError(t, err)

	_, err = Yield(first, opts, lease.Tip, gitwt.Exec)

	var still *StillHeldError
	require.ErrorAs(t, err, &still)
	rescue := gitCmd(t, first, "ls-remote", "origin", "refs/frit/rescue/*")
	assert.Empty(t, rescue, "the live holder's lease is not parked")
}

// TestRescueRefsListsEveryMachinesParkedWork: rescue is per plan and
// per machine, so two parked runs both come back, another plan's
// rescue ref does not bleed in, and a plan with nothing parked reads
// as empty.
func TestRescueRefsListsEveryMachinesParkedWork(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	tip := workOn(t, work)
	_, err = Scavenge(
		work, leaseOptions("box-b", "/lanes/b"), tip, gitwt.Exec)
	require.NoError(t, err)
	gitCmd(t, work, "push", "-q", "origin", tip+":refs/frit/rescue/7/box-c")

	refs := RescueRefs(work, "origin", 7, gitwt.Exec)

	assert.Equal(t,
		[]string{"refs/frit/rescue/7/box-b", "refs/frit/rescue/7/box-c"},
		refs)
	assert.Empty(t, RescueRefs(work, "origin", 8, gitwt.Exec),
		"another plan's rescue refs do not bleed in")
}
