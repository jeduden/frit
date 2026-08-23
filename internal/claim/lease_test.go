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

// TestAcquireRefusesToClobberUnpushedLocalBranch: a plan authored
// directly on its own lease branch, never pushed, must not be
// silently discarded by a fresh acquire that has never looked at the
// local ref (S82).
func TestAcquireRefusesToClobberUnpushedLocalBranch(t *testing.T) {
	work := originAndClone(t)
	gitCmd(t, work, "checkout", "-q", "-b", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(work, "draft.md"), []byte("x\n"), 0o600))
	gitCmd(t, work, "add", "-A")
	gitCmd(t, work, "commit", "-q", "-m", "local draft, never pushed")
	local := gitCmd(t, work, "rev-parse", "plan/7")
	gitCmd(t, work, "checkout", "-q", "main")

	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)

	require.Error(t, err)
	assert.Equal(t, local, gitCmd(t, work, "rev-parse", "refs/heads/plan/7"),
		"the local draft branch is untouched, not fast-forwarded past it")
}

// TestAcquireStillWinsWhenLocalBranchIsAncestorOfBase: the common
// case — no local branch of that name at all — still acquires
// exactly as before the guard.
func TestAcquireStillWinsWhenLocalBranchIsAncestorOfBase(t *testing.T) {
	work := originAndClone(t)

	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)

	require.NoError(t, err)
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
	assert.Contains(t, err.Error(), "yield",
		"a fenced-out session's next verb offers yield")
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

// TestTheLanePersistsItsLeaseToken: acquire and every renewal write
// the winning tip into the lane's own git dir, so the token survives
// the process (F9, S3). Release ends the lease, so it leaves the
// token exactly where the last renewal left it — writing one for a
// lease being given up would be a lie.
func TestTheLanePersistsItsLeaseToken(t *testing.T) {
	work := originAndClone(t)
	opts := leaseOptions("box-a", work)

	lease, err := Acquire(work, opts, gitwt.Exec)
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, ReadToken(work, opts.PlanID, gitwt.Exec))

	renewed, err := Renew(work, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	assert.Equal(t, renewed.Tip, ReadToken(work, opts.PlanID, gitwt.Exec))

	released, err := Release(work, opts, renewed.Tip, gitwt.Exec)
	require.NoError(t, err)
	assert.Equal(t, renewed.Tip, ReadToken(work, opts.PlanID, gitwt.Exec),
		"a release does not persist a token for the lease it just ended")
	_ = released
}

// TestTakeoverPersistsTheTakingLanesToken: the taking lane's own token
// is left at the takeover marker, not the tip it seized from.
func TestTakeoverPersistsTheTakingLanesToken(t *testing.T) {
	first := originAndClone(t)
	lease, err := Acquire(first, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	second := cloneAgain(t, first)
	taken, err := Takeover(second,
		leaseOptions("box-b", second), lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, taken.Tip, ReadToken(second, 7, gitwt.Exec))
}

// TestResumeIsARenewalWithNoWindow: resume mints the same transition a
// renewal does — a beat, same epoch, CASed from the lane's own
// recorded tip — and persists the token like any other renewal. The
// caller is what skips the staleness window; Resume itself is
// mechanically Renew.
func TestResumeIsARenewalWithNoWindow(t *testing.T) {
	work := originAndClone(t)
	opts := leaseOptions("box-a", work)
	lease, err := Acquire(work, opts, gitwt.Exec)
	require.NoError(t, err)

	resumed, err := Resume(work, opts, lease.Tip, gitwt.Exec)

	require.NoError(t, err)
	assert.Equal(t, lease.Epoch, resumed.Epoch, "resume never bumps the epoch")
	body := gitCmd(t, work, "log", "-1", "--format=%B", resumed.Tip)
	assert.Contains(t, body, "plan 7: beat")
	assert.Equal(t, resumed.Tip, ReadToken(work, opts.PlanID, gitwt.Exec))
}

// TestReadMarkerReadsTheMarkerAtATip: ReadMarker is the exported
// reader a caller outside the package uses to learn who a tip's
// marker names — the veto check's way in.
func TestReadMarkerReadsTheMarkerAtATip(t *testing.T) {
	first := originAndClone(t)
	opts := leaseOptions("box-a", "/lanes/a")
	opts.Session = "wS:p1"
	lease, err := Acquire(first, opts, gitwt.Exec)
	require.NoError(t, err)

	second := cloneAgain(t, first)
	m, ok := ReadMarker(second, opts, lease.Tip, gitwt.Exec)

	require.True(t, ok)
	assert.Equal(t, "box-a", m.Holder)
	assert.Equal(t, "wS:p1", m.Session)
}

// TestRemoteTipReadsOriginsCurrentTip: RemoteTip is the one ls-remote
// self-resume needs to compare a persisted token against origin's
// current view, not this clone's possibly-stale local one.
func TestRemoteTipReadsOriginsCurrentTip(t *testing.T) {
	first := originAndClone(t)
	assert.Equal(t, "", RemoteTip(first, "origin", 7, gitwt.Exec),
		"no ref, no tip")

	lease, err := Acquire(first, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, lease.Tip, RemoteTip(first, "origin", 7, gitwt.Exec))
}

// TestOwnAdvanceRecognizesRawCommitsOnTopOfTheToken: an ordinary run
// of TDD commits on plan/<id> advances the ref past a lane's persisted
// token with no frit transition between. OwnAdvance still reads this
// as the lane's own advance, because the token stays an ancestor and
// the marker governing the new tip still carries the same epoch and
// holder.
func TestOwnAdvanceRecognizesRawCommitsOnTopOfTheToken(t *testing.T) {
	first := originAndClone(t)
	lease, err := Acquire(first, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	gitCmd(t, first, "checkout", "-q", "plan/7")
	gitCmd(t, first, "commit", "--allow-empty", "-q", "-m", "red")
	gitCmd(t, first, "commit", "--allow-empty", "-q", "-m", "green")
	gitCmd(t, first, "push", "-q", "origin", "plan/7")
	tip := gitCmd(t, first, "rev-parse", "HEAD")

	assert.True(t, OwnAdvance(first, 7, lease.Tip, tip, gitwt.Exec))
}

// TestOwnAdvanceRefusesAForeignTakeover: a takeover marker minted at a
// new epoch from the observed tip descends from the token too, so
// ancestry alone cannot tell the two apart — OwnAdvance still refuses
// it because the epoch/holder half of the guard changed.
func TestOwnAdvanceRefusesAForeignTakeover(t *testing.T) {
	first := originAndClone(t)
	lease, err := Acquire(first, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	over, err := Takeover(
		first, leaseOptions("box-b", "/lanes/b"), lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	assert.False(t, OwnAdvance(first, 7, lease.Tip, over.Tip, gitwt.Exec))
}

// TestOwnAdvanceRefusesATokenThatIsNotAnAncestor: a token unreachable
// from tip is not an advance at all, whatever its marker says.
func TestOwnAdvanceRefusesATokenThatIsNotAnAncestor(t *testing.T) {
	first := originAndClone(t)
	tip := gitCmd(t, first, "rev-parse", "main")

	assert.False(t, OwnAdvance(first, 7, "deadbeef", tip, gitwt.Exec))
}

// TestTakeoverCountReadsTheMarkersAlreadyInTheChain: the backoff
// factor k is read straight off the chain, so every observer computes
// the same one (F3) — a fresh claim carries none, and each seized
// takeover adds one more.
func TestTakeoverCountReadsTheMarkersAlreadyInTheChain(t *testing.T) {
	first := originAndClone(t)
	lease, err := Acquire(first, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	base := gitCmd(t, first, "rev-parse", "origin/main")
	assert.Equal(t, 0, TakeoverCount(first, 7, base, lease.Tip, gitwt.Exec),
		"a fresh claim carries no takeover marker")

	taken, err := Takeover(
		first, leaseOptions("box-b", "/lanes/b"), lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	assert.Equal(t, 1, TakeoverCount(first, 7, base, taken.Tip, gitwt.Exec))

	twice, err := Takeover(
		first, leaseOptions("box-c", "/lanes/c"), taken.Tip, gitwt.Exec)
	require.NoError(t, err)
	assert.Equal(t, 2, TakeoverCount(first, 7, base, twice.Tip, gitwt.Exec))
}

// TestTakeoverCountIsBestEffort: a bad base or repo dir answers zero
// rather than fail the caller — a rough count costs nothing to be
// quietly wrong, and the CAS stays the safety net either way.
func TestTakeoverCountIsBestEffort(t *testing.T) {
	assert.Equal(t, 0,
		TakeoverCount("/does/not/exist", 7, "main", "HEAD", gitwt.Exec))
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

// TestHeldFindsAClaimOrTakeoverReachableFromTip: the ordinary shapes a
// live lease takes all read as held — a bare claim, a claim beneath
// later work, a takeover, and the legacy decorated claim whose subject
// carries a lane slug behind the kind.
func TestHeldFindsAClaimOrTakeoverReachableFromTip(t *testing.T) {
	work := originAndClone(t)
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "plan 7: claim")
	tip := gitCmd(t, work, "rev-parse", "HEAD")
	assert.True(t, Held(work, tip, 7, gitwt.Exec), "a bare claim is held")

	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "real work")
	tip = gitCmd(t, work, "rev-parse", "HEAD")
	assert.True(t, Held(work, tip, 7, gitwt.Exec),
		"a claim beneath later work is still held")

	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "plan 7: takeover")
	tip = gitCmd(t, work, "rev-parse", "HEAD")
	assert.True(t, Held(work, tip, 7, gitwt.Exec), "a takeover is held")
}

// TestHeldReadsALegacyDecoratedClaimAsHeld: the old claim design's
// subject carries a lane slug behind the kind; Held tolerates it the
// same way markerSubject and Released's callers already do.
func TestHeldReadsALegacyDecoratedClaimAsHeld(t *testing.T) {
	work := originAndClone(t)
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m",
		"plan 7: claim shader")
	tip := gitCmd(t, work, "rev-parse", "HEAD")

	assert.True(t, Held(work, tip, 7, gitwt.Exec))
}

// TestHeldIsNotFooledByAReleaseThatSupersedesAnOldClaim: the bug this
// test pins is a release marker read as transparent because an older
// claim was still reachable from tip. A release is the nearest
// terminal marker here, so it must win over the claim beneath it —
// Released alone cannot tell this apart from a live hold, since
// Released only reads the tip's own subject, and the tip here is a
// plain commit pushed on top of the release with no new claim.
func TestHeldIsNotFooledByAReleaseThatSupersedesAnOldClaim(t *testing.T) {
	work := originAndClone(t)
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "plan 7: claim")
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "plan 7: release")
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m",
		"some other manual commit, not a marker")
	tip := gitCmd(t, work, "rev-parse", "HEAD")

	assert.False(t, Held(work, tip, 7, gitwt.Exec),
		"a release after the claim ends the hold, even with later work")
}

// TestHeldIsNotFooledByAMarkerLineBuriedInAnUnrelatedBody: git's
// --grep matches anywhere in a commit's full multi-line message, not
// just its subject, so a commit whose body merely contains a line that
// looks like a marker — the shape a squash-merge leaves when it
// concatenates an old marker subject into a new commit's body — must
// not read as the marker itself. Held is expected to re-validate the
// candidate's actual subject before trusting it.
func TestHeldIsNotFooledByAMarkerLineBuriedInAnUnrelatedBody(t *testing.T) {
	work := originAndClone(t)
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m",
		"unrelated subject\n\nplan 7: claim\nepoch:   9")
	tip := gitCmd(t, work, "rev-parse", "HEAD")

	assert.False(t, Held(work, tip, 7, gitwt.Exec),
		"a marker line buried in another commit's body is not a marker")
}

// TestHeldStillCountsAnActiveRenewalWhoseTipIsABeatMarker pins the
// baseline the fix above must not break: a beat marker is a routine
// renewal, not a terminal state, so Held must see past it to the claim
// or takeover it renews rather than stopping at the first marker of
// any kind it meets.
func TestHeldStillCountsAnActiveRenewalWhoseTipIsABeatMarker(t *testing.T) {
	work := originAndClone(t)
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "plan 7: claim")
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "plan 7: beat")
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "plan 7: beat")
	tip := gitCmd(t, work, "rev-parse", "HEAD")

	assert.True(t, Held(work, tip, 7, gitwt.Exec),
		"a renewed lease is held even though its tip is a beat marker")
}

// TestHeldAnsweresFalseForAMarkerlessBranch: a ref matching the holds
// pattern by name alone, with no marker of any kind reachable, is not
// a lease frit ever minted.
func TestHeldAnswersFalseForAMarkerlessBranch(t *testing.T) {
	work := originAndClone(t)
	gitCmd(t, work, "commit", "--allow-empty", "-q", "-m", "hand-made commit")
	tip := gitCmd(t, work, "rev-parse", "HEAD")

	assert.False(t, Held(work, tip, 7, gitwt.Exec))
}

// TestHeldAnswersFalseForAnUnreadableTip fails safe: an object Held
// cannot read costs a plan wrongly offered as startable, never a live
// lease wrongly seized.
func TestHeldAnswersFalseForAnUnreadableTip(t *testing.T) {
	assert.False(t, Held("/r", "not-a-real-sha", 7,
		func(string, ...string) ([]byte, error) {
			return nil, errors.New("bad object")
		}))
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

// squashLandOnMain simulates a squash-merge landing: main gains a
// fresh commit carrying the same content, with no merge relationship
// to the lane's own commits — the shape a squash-merge PR leaves
// behind, where ancestry can never see the lane's work as landed.
func squashLandOnMain(t *testing.T, repo, content string) {
	t.Helper()
	gitCmd(t, repo, "checkout", "-q", "main")
	require.NoError(t, os.WriteFile(
		filepath.Join(repo, "w.txt"), []byte(content), 0o600))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-q", "-m", "squash-merge plan 7")
	gitCmd(t, repo, "push", "-q", "origin", "main")
}

// TestScavengeParksSquashLandedWorkWithNoRescue: a lane's work reached
// the default branch by squash-merge — same content, no ancestry to
// the lane's own tip — so it is landed by content though ancestry
// alone would call it unlanded. Scavenge deletes it clean, no rescue
// ref parked for content already on main.
func TestScavengeParksSquashLandedWorkWithNoRescue(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	tip := workOn(t, work)
	squashLandOnMain(t, work, "wip\n")

	sc, err := Scavenge(
		work, leaseOptions("box-b", "/lanes/b"), tip, gitwt.Exec)
	require.NoError(t, err)

	assert.Empty(t, sc.Rescue, "the content is already on main; nothing to park")
	rescue := gitCmd(t, work, "ls-remote", "origin", "refs/frit/rescue/*")
	assert.Empty(t, rescue, "no rescue ref was created")
	gone := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Empty(t, gone, "the work ref is still deleted")
}

// TestScavengeParksOnContentConflict: the lane's tip and main both
// edited the same line differently — merge-tree conflicts — so the
// chain still reads as unlanded and parks. The conflict is evidence,
// not a fault: Scavenge returns no error.
func TestScavengeParksOnContentConflict(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	gitCmd(t, work, "checkout", "-q", "plan/7")
	require.NoError(t, os.WriteFile(
		filepath.Join(work, "README.md"), []byte("lane-edit\n"), 0o600))
	gitCmd(t, work, "add", "-A")
	gitCmd(t, work, "commit", "-q", "-m", "lane edits README")
	gitCmd(t, work, "push", "-q", "origin", "plan/7")
	tip := gitCmd(t, work, "rev-parse", "refs/heads/plan/7")
	gitCmd(t, work, "checkout", "-q", "main")
	require.NoError(t, os.WriteFile(
		filepath.Join(work, "README.md"), []byte("main-edit\n"), 0o600))
	gitCmd(t, work, "add", "-A")
	gitCmd(t, work, "commit", "-q", "-m", "main edits README")
	gitCmd(t, work, "push", "-q", "origin", "main")

	sc, err := Scavenge(
		work, leaseOptions("box-b", "/lanes/b"), tip, gitwt.Exec)
	require.NoError(t, err, "a content conflict is evidence, not a fault")

	assert.Equal(t, "refs/frit/rescue/7/box-b", sc.Rescue,
		"conflicting content still reads as unlanded work")
	rescue := gitCmd(t, work, "ls-remote", "origin", sc.Rescue)
	assert.Contains(t, rescue, tip, "the rescue ref carries the old tip")
	gone := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Empty(t, gone, "the work ref is still deleted")
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
	var conflict *RescueConflictError
	require.ErrorAs(t, err, &conflict,
		"the refusal is a typed RescueConflictError, not a plain error")
	assert.Equal(t, int64(7), conflict.PlanID)
	assert.Equal(t, "refs/frit/rescue/7/box-b", conflict.Rescue)
	remote := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Contains(t, remote, tip, "the work ref is not deleted")
	rescue := gitCmd(t, work, "ls-remote", "origin", "refs/frit/rescue/7/box-b")
	assert.Contains(t, rescue, other, "the foreign rescue is untouched")
}

// TestScavengeSparesABranchCheckedOutInALinkedWorktree: the remote
// delete and the rescue park happen exactly as always, but the local
// refs/heads/plan/7 must survive where a linked worktree still has it
// checked out — deleting it would leave that worktree's HEAD dangling.
func TestScavengeSparesABranchCheckedOutInALinkedWorktree(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	tip := workOn(t, work)
	linked := filepath.Join(t.TempDir(), "linked")
	gitCmd(t, work, "worktree", "add", "-q", linked, "plan/7")

	sc, err := Scavenge(work, leaseOptions("box-b", "/lanes/b"), tip, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, "refs/frit/rescue/7/box-b", sc.Rescue,
		"the rescue still parks the unlanded work")
	gone := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Empty(t, gone, "the remote ref is still deleted")
	local := gitCmd(t, work, "rev-parse", "--verify", "refs/heads/plan/7")
	assert.Equal(t, tip, local,
		"the local ref survives at its pre-scavenge tip; a worktree stands on it")
	head := gitCmd(t, linked, "rev-parse", "HEAD")
	assert.Equal(t, tip, head, "the linked worktree's HEAD still resolves")
}

// TestCheckedOutFailsClosedOnAnUnreadableWorktreeList: an unreadable
// `git worktree list` must not read as "nobody is standing on this
// branch" — that would let a transient git fault delete a branch a
// worktree genuinely has checked out, the exact hazard the guard
// exists to prevent. checkedOut answers true instead, so a caller
// skips the delete on the safe side, same direction isAncestor
// already takes for an unreadable read.
func TestCheckedOutFailsClosedOnAnUnreadableWorktreeList(t *testing.T) {
	failingList := func(_ string, _ ...string) ([]byte, error) {
		return nil, errors.New("git worktree list: exit status 128")
	}

	assert.True(t, checkedOut("/repo", "plan/7", failingList))
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

// TestYieldWithNoLocalRefIsANoOp: a checkout that never fetched or
// minted the plan's branch has nothing of its own to rescue. Yield
// must not read that as "still held" — an absent remote ref reads as
// "" too, and "" == "" would misreport a plan nobody holds — and must
// not call park with an empty tip either: park's push turns an empty
// tip into a delete of the rescue ref, which git accepts as a no-op
// whether or not the ref exists, so a naive call would silently
// "succeed" without ever creating a rescue.
func TestYieldWithNoLocalRefIsANoOp(t *testing.T) {
	first := originAndClone(t)
	opts := leaseOptions("box-a", "/lanes/a")

	sc, err := Yield(first, opts, "", gitwt.Exec)

	require.NoError(t, err, "no local ref is a no-op, not a refusal")
	assert.Empty(t, sc.Rescue, "nothing was parked")
	rescue, lsErr := gitCapture(t, first, "ls-remote", "origin",
		"refs/frit/rescue/*")
	require.NoError(t, lsErr)
	assert.Empty(t, rescue, "no rescue ref was ever created")
}

// TestYieldWithNoLocalRefAndAForeignHolderIsStillANoOp: the same "no
// local ref" no-op holds even when the plan is genuinely held live by
// someone else — this checkout simply never fetched it, so it still
// has nothing of its own to park, and must not fabricate a "parked"
// rescue for that other holder's work.
func TestYieldWithNoLocalRefAndAForeignHolderIsStillANoOp(t *testing.T) {
	first := originAndClone(t)
	_, err := Acquire(first, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	sc, err := Yield(first, leaseOptions("box-b", "/lanes/b"), "", gitwt.Exec)

	require.NoError(t, err)
	assert.Empty(t, sc.Rescue, "nothing was parked for a ref never fetched")
	rescue, lsErr := gitCapture(t, first, "ls-remote", "origin",
		"refs/frit/rescue/*")
	require.NoError(t, lsErr)
	assert.Empty(t, rescue, "no rescue ref was fabricated")
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

// TestAllRescueRefsBucketsByPlanID: the batched sibling of RescueRefs
// reads every plan's rescue refs in one ls-remote, bucketed by the id
// segment in the ref name — what orphans' sweep needs instead of one
// call per plan.
func TestAllRescueRefsBucketsByPlanID(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	tip := workOn(t, work)
	_, err = Scavenge(work, leaseOptions("box-b", "/lanes/b"), tip, gitwt.Exec)
	require.NoError(t, err)
	gitCmd(t, work, "push", "-q", "origin", tip+":refs/frit/rescue/8/box-c")

	buckets, err := AllRescueRefs(work, "origin", gitwt.Exec)

	require.NoError(t, err)
	assert.Equal(t, []string{"refs/frit/rescue/7/box-b"}, buckets[7])
	assert.Equal(t, []string{"refs/frit/rescue/8/box-c"}, buckets[8])
}

// TestAllRescueRefsIsEmptyWithNoRemoteConfigured: a repository that has
// never pushed anywhere has parked nothing there either — reading as
// an empty sweep, not a fault, is what tells this apart from a remote
// that is configured but cannot be reached.
func TestAllRescueRefsIsEmptyWithNoRemoteConfigured(t *testing.T) {
	work := t.TempDir()
	gitCmd(t, work, "init", "-q", "-b", "main")

	buckets, err := AllRescueRefs(work, "origin", gitwt.Exec)

	require.NoError(t, err)
	assert.Empty(t, buckets)
}

// TestAllRescueRefsErrsWhenTheRemoteCannotBeRead: unlike RescueRefs'
// per-plan cousin, which swallows an unreadable remote to an empty
// list, the batched sweep surfaces the fault — orphans records it as a
// Problem rather than silently reporting a clean repository.
func TestAllRescueRefsErrsWhenTheRemoteCannotBeRead(t *testing.T) {
	work := originAndClone(t)
	deadRemote := func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ls-remote" {
			return nil, errors.New("could not resolve host")
		}

		return gitwt.Exec(dir, args...)
	}

	_, err := AllRescueRefs(work, "origin", deadRemote)

	require.Error(t, err)
}

// TestScavengeErrsWhenTheRemoteCannotBeRead: an ls-remote failure is a
// fault, not an absent ref. Reading it as "already gone" would delete
// the local copy of a lease the remote still carries and report a
// clean no-op that never happened — so the scavenge must surface the
// fault and touch nothing.
func TestScavengeErrsWhenTheRemoteCannotBeRead(t *testing.T) {
	work := originAndClone(t)
	lease, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	deadRemote := func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ls-remote" {
			return nil, errors.New("could not resolve host")
		}

		return gitwt.Exec(dir, args...)
	}

	_, err = Scavenge(
		work, leaseOptions("box-b", "/lanes/b"), lease.Tip, deadRemote)

	require.Error(t, err)
	local := gitCmd(t, work, "rev-parse", "--verify", "refs/heads/plan/7")
	assert.NotEmpty(t, local, "the local ref survives an unreadable remote")
}

// TestParkUnlandedParksAChainCarryingWork: the park half of a scavenge
// on its own — a tip carrying work commits is parked to the plan's
// rescue ref, and the work ref itself is untouched. It exists for a
// teardown that deletes through git porcelain rather than a ref CAS
// (reap's branch delete) and must still honor park-before-delete.
func TestParkUnlandedParksAChainCarryingWork(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	tip := workOn(t, work)

	sc, err := ParkUnlanded(
		work, leaseOptions("box-b", "/lanes/b"), tip, gitwt.Exec)
	require.NoError(t, err)

	assert.Equal(t, "refs/frit/rescue/7/box-b", sc.Rescue)
	rescue := gitCmd(t, work, "ls-remote", "origin", sc.Rescue)
	assert.Contains(t, rescue, tip, "the rescue ref carries the tip")
	still := gitCmd(t, work, "ls-remote", "origin", "refs/heads/plan/7")
	assert.Contains(t, still, tip, "parking deletes nothing")
}

// TestParkUnlandedIsANoOpForAMarkerOnlyChain: markers are not work, so
// there is nothing to park and no rescue ref is minted.
func TestParkUnlandedIsANoOpForAMarkerOnlyChain(t *testing.T) {
	work := originAndClone(t)
	lease, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)

	sc, err := ParkUnlanded(
		work, leaseOptions("box-b", "/lanes/b"), lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	assert.Empty(t, sc.Rescue)
	rescue := gitCmd(t, work, "ls-remote", "origin", "refs/frit/rescue/*")
	assert.Empty(t, rescue, "no rescue ref was created")
}

// TestParkUnlandedRefusesAForeignRescue: a rescue ref already holding
// a different tip is somebody's parked work; the park refuses rather
// than clobber it, so the caller knows not to delete.
func TestParkUnlandedRefusesAForeignRescue(t *testing.T) {
	work := originAndClone(t)
	_, err := Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)
	require.NoError(t, err)
	tip := workOn(t, work)
	other := gitCmd(t, work, "rev-parse", "origin/main")
	gitCmd(t, work, "push", "-q", "origin", other+":refs/frit/rescue/7/box-b")

	_, err = ParkUnlanded(
		work, leaseOptions("box-b", "/lanes/b"), tip, gitwt.Exec)

	require.Error(t, err)
	rescue := gitCmd(t, work, "ls-remote", "origin", "refs/frit/rescue/7/box-b")
	assert.Contains(t, rescue, other, "the foreign rescue is untouched")
}

// TestHasUnlandedTellsWorkFromMarkers: a marker-only chain has nothing
// a delete could destroy; a chain with a work commit does. Exported so
// a dry run can say whether a teardown would park before it acts.
func TestHasUnlandedTellsWorkFromMarkers(t *testing.T) {
	work := originAndClone(t)
	opts := leaseOptions("box-a", "/lanes/a")
	lease, err := Acquire(work, opts, gitwt.Exec)
	require.NoError(t, err)

	markersOnly, err := HasUnlanded(work, opts, lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	assert.False(t, markersOnly)

	tip := workOn(t, work)
	carried, err := HasUnlanded(work, opts, tip, gitwt.Exec)
	require.NoError(t, err)
	assert.True(t, carried)
}

// TestRescueRefNamesThePlanAndTheHolder pins the exported name a dry
// run previews: the same per-plan, per-machine ref park writes.
func TestRescueRefNamesThePlanAndTheHolder(t *testing.T) {
	assert.Equal(t, "refs/frit/rescue/7/box-a", RescueRef(7, "box-a"))
}
