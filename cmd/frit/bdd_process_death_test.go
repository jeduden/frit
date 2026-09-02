package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/observe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The process-death vocabulary: a claim that dies before it writes
// anything, a local write that never reaches origin, an observer
// whose window resets under it, a takeover that inherits whatever
// actually reached origin. It registers itself, like every section's
// step file, so a section adds a file and never a line to bdd_test.go.
func init() {
	registrars = append(registrars, (*world).registerProcessDeath)
}

// deathState is this section's own state beside the shared world: the
// machine that won a fresh claim, the tip a push actually landed on
// origin, and the fixed instant an observation reset was driven at —
// kept here, not on world, so this section adds a file and never a
// field to it.
type deathState struct {
	retryErr    error
	winner      string
	winnerLease claim.Lease
	pushedTip   string
	resetAt     time.Time
}

func (w *world) registerProcessDeath(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]+)" has a claimable plan (\d+)$`, w.hasAClaimablePlan)
	sc.Step(`^"([^"]+)"'s claim dies before writing anything$`, w.claimDiesBeforeWritingAnything)
	sc.Step(`^origin has no work ref for plan (\d+)$`, w.originHasNoWorkRefForPlan)
	sc.Step(`^"([^"]+)" retries and acquires the lease at epoch 1$`, w.retriesAndAcquiresAtEpoch1)
	sc.Step(`^"([^"]+)" mints a local claim it never pushes for plan (\d+)$`, w.mintsLocalClaimNeverPushed)
	sc.Step(`^"([^"]+)" retries the claim$`, w.retriesTheClaim)
	sc.Step(`^"([^"]+)" claims plan (\d+)$`, w.claimsPlan)
	sc.Step(`^"([^"]+)" wins the lease at epoch 1$`, w.winsTheLeaseAtEpoch1)
	sc.Step(`^"([^"]+)"'s retry is refused: the local branch diverges$`, w.retryIsRefusedLocalDiverges)
	sc.Step(`^the observer has matured a window on "([^"]+)"'s tip$`, w.observerHasMaturedAWindowOnTip)
	sc.Step(`^the observer resets the window on the new tip$`, w.observerResetsTheWindowOnTheNewTip)
	sc.Step(`^the observation restarts fresh on the new tip$`, w.observationRestartsFreshOnTheNewTip)
	sc.Step(`^"([^"]+)" pushes a work commit on the lane$`, w.pushesAWorkCommit)
	sc.Step(`^"([^"]+)" takes over the current lease$`, w.takesOverTheCurrentLease)
	sc.Step(`^"([^"]+)"'s takeover is a child of the tip that actually reached origin$`,
		w.takeoverIsChildOfTheReachedTip)
	sc.Step(`^"([^"]+)"'s pushed work is in "([^"]+)"'s takeover history$`, w.pushedWorkIsInTakeoverHistory)
	sc.Step(`^"([^"]+)"'s local-only work is in no history on origin$`, w.localWorkInNoOriginHistory)
}

// hasAClaimablePlan sets up a startable plan, held by nobody — the
// state before any claim on it has begun.
func (w *world) hasAClaimablePlan(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = holder
	w.clones[holder] = claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")

	return nil
}

// claimDiesBeforeWritingAnything is a claim that never started: no
// local ref, no push, nothing shared. The step exists so the
// scenario's own words appear in the report; the world has nothing to
// change, since nothing happened.
func (w *world) claimDiesBeforeWritingAnything(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never started a claim; %q did", holder, w.holder)
	}

	return nil
}

// originHasNoWorkRefForPlan reads origin directly, so a claim that
// died before writing is verified by what the remote shows, not a
// local guess.
func (w *world) originHasNoWorkRefForPlan(planID int) error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	if got := claim.RemoteTip(repo, "origin", int64(planID), gitwt.Exec); got != "" {
		return fmt.Errorf("origin carries %s for plan %d, want no ref", got, planID)
	}

	return nil
}

// retriesAndAcquiresAtEpoch1 is the retry itself: a clean acquire on
// an origin that never saw the dead claim lands at epoch 1, the same
// as a first attempt would have.
func (w *world) retriesAndAcquiresAtEpoch1(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	lease, err := claim.Acquire(repo, leaseFor(holder, w.planID), gitwt.Exec)
	if err != nil {
		return err
	}
	if lease.Epoch != 1 {
		return fmt.Errorf("retry acquired at epoch %d, want 1", lease.Epoch)
	}
	w.lease = lease

	return nil
}

// mintsLocalClaimNeverPushed writes the local hold-branch commit a
// real claim mints, but never pushes it — the state a process is left
// in when it is killed after the local ref write, before the push.
func (w *world) mintsLocalClaimNeverPushed(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = holder
	repo := claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")
	w.clones[holder] = repo

	git(w.t, repo, "checkout", "-q", "-b", claim.Branch(int64(planID)))
	git(w.t, repo, "commit", "-q", "--allow-empty", "-m", fmt.Sprintf("plan %d: claim", planID))
	git(w.t, repo, "checkout", "-q", "main")

	return nil
}

// claimsPlan is a fresh machine's own acquire against origin, in a
// clone that never saw the first machine's unpushed local ref — the
// only copy of that ref, so this claim sees an absent ref and wins
// clean.
func (w *world) claimsPlan(holder string, planID int) error {
	first, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	repo := cloneAgain(w.t, first)
	w.clones[holder] = repo

	lease, err := claim.Acquire(repo, leaseFor(holder, planID), gitwt.Exec)
	if err != nil {
		return err
	}
	ds := section[deathState](w)
	ds.winner, ds.winnerLease = holder, lease

	return nil
}

func (w *world) winsTheLeaseAtEpoch1(holder string) error {
	ds := section[deathState](w)
	if ds.winner != holder {
		return fmt.Errorf("%q did not claim the plan; %q did", holder, ds.winner)
	}
	if ds.winnerLease.Epoch != 1 {
		return fmt.Errorf("%q won at epoch %d, want 1", holder, ds.winnerLease.Epoch)
	}

	return nil
}

// retriesTheClaim replays the first machine's own acquire from its
// own clone: the local branch it minted before dying still carries
// commits the base does not have, so a fresh acquire refuses it
// rather than clobber it. Origin is untouched either way — the guard
// fires before any push — so a claim from elsewhere still finds it
// bare, which the next step exercises.
func (w *world) retriesTheClaim(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never minted the local claim; %q did", holder, w.holder)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	_, err = claim.Acquire(repo, leaseFor(holder, w.planID), gitwt.Exec)
	section[deathState](w).retryErr = err

	return nil
}

// retryIsRefusedLocalDiverges reads back the retry's own recorded
// error: a local branch diverging from the base refuses as this
// exact shape, not a generic failure.
func (w *world) retryIsRefusedLocalDiverges(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never retried the claim; %q did", holder, w.holder)
	}
	var diverges *claim.LocalDivergesError
	err := section[deathState](w).retryErr
	if !errors.As(err, &diverges) {
		return fmt.Errorf("expected a local-diverges refusal, got %v", err)
	}

	return nil
}

// observerHasMaturedAWindowOnTip seeds an observation that has
// watched one tip unchanged for hours — the state a faithful observer
// holds just before whatever it was watching moves.
func (w *world) observerHasMaturedAWindowOnTip(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q holds no tip to observe; %q does", holder, w.holder)
	}
	seedWindow(w.t, "atlas", int64(w.planID), w.lease.Tip, 3*time.Hour)

	return nil
}

// observerResetsTheWindowOnTheNewTip drives resetWindow directly, the
// same call a lost takeover's CAS makes, at a fixed instant rather
// than time.Now — the reset is a pure function of the tip and the
// clock it is handed, and no step here reads a real clock.
func (w *world) observerResetsTheWindowOnTheNewTip() error {
	ds := section[deathState](w)
	ds.resetAt = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	resetWindow(discovery.Plan{Repo: "atlas", ID: int64(w.planID)}, w.taken.Tip, ds.resetAt)

	return nil
}

// observationRestartsFreshOnTheNewTip reads the observation store
// back and checks it holds exactly what a first-ever look would
// record: one sample, first and last both the reset instant, no
// voided reason — never whatever span the stale window had matured.
func (w *world) observationRestartsFreshOnTheNewTip() error {
	ds := section[deathState](w)
	path, err := observe.Path()
	if err != nil {
		return err
	}
	state := observe.Load(path)
	got, ok := state[observe.Key("atlas", int64(w.planID))]
	if !ok {
		return fmt.Errorf("no observation recorded for plan %d", w.planID)
	}
	if got.Tip != w.taken.Tip {
		return fmt.Errorf("the observation watches %s, want the new tip %s", got.Tip, w.taken.Tip)
	}
	if got.Samples != 1 {
		return fmt.Errorf("the observation carries %d samples, want 1 (fresh)", got.Samples)
	}
	if !got.First.Equal(ds.resetAt) || !got.Last.Equal(ds.resetAt) {
		return fmt.Errorf("the observation spans %s..%s, want both at the reset instant %s",
			got.First, got.Last, ds.resetAt)
	}
	if got.Voided != "" {
		return fmt.Errorf("the observation carries a voided reason %q, want none", got.Voided)
	}

	return nil
}

// pushesAWorkCommit lands a work commit on origin's copy of the hold
// branch — the phase work a real lane pushes as it goes, as opposed
// to the unpushed commit "commits work on the lane it never pushes"
// leaves only local.
func (w *world) pushesAWorkCommit(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	git(w.t, repo, "checkout", "-q", claim.Branch(int64(w.planID)))
	writeFile(w.t, repo, "w.txt", "landed\n")
	git(w.t, repo, "add", "-A")
	git(w.t, repo, "commit", "-q", "-m", "pushed work")
	tip, err := gitCapture(w.t, repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	if out, err := gitCapture(w.t, repo, "push", "-q", "origin", w.branch()); err != nil {
		return fmt.Errorf("push work: %s: %w", out, err)
	}
	git(w.t, repo, "checkout", "-q", "main")

	section[deathState](w).pushedTip = tip

	return nil
}

// takesOverTheCurrentLease is a takeover CASed from whatever origin's
// work ref actually holds right now, read fresh — unlike "takes the
// lease over", which CASes from the tip the world's own earlier
// acquire recorded, this sees a push the holder made afterward.
func (w *world) takesOverTheCurrentLease(holder string) error {
	if holder == w.holder {
		return fmt.Errorf("%q already holds the lease; a takeover comes from another machine", holder)
	}
	first, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	second := cloneAgain(w.t, first)
	w.clones[holder] = second

	from := claim.RemoteTip(second, "origin", int64(w.planID), gitwt.Exec)
	if from == "" {
		return fmt.Errorf("origin carries no lease for plan %d to take over", w.planID)
	}
	taken, err := claim.Takeover(second, leaseFor(holder, w.planID), from, gitwt.Exec)
	if err != nil {
		return err
	}
	w.taker, w.taken = holder, taken

	return nil
}

// takeoverIsChildOfTheReachedTip checks the takeover's parent against
// the tip that actually landed on origin: a pushed work commit when
// there was one, else the plain claim tip from the original acquire —
// what "reached origin" means when nothing further was pushed.
func (w *world) takeoverIsChildOfTheReachedTip(taker string) error {
	if taker != w.taker {
		return fmt.Errorf("%q did not take the lease over; %q did", taker, w.taker)
	}
	want := section[deathState](w).pushedTip
	if want == "" {
		want = w.lease.Tip
	}
	repo := w.clones[taker]
	parent, err := gitCapture(w.t, repo, "rev-parse", w.taken.Tip+"^1")
	if err != nil {
		return fmt.Errorf("%s: %w", parent, err)
	}
	if parent != want {
		return fmt.Errorf("the takeover's parent is %s, want the reached tip %s", parent, want)
	}

	return nil
}

// pushedWorkIsInTakeoverHistory checks the pushed work commit is
// reachable from the takeover — the promise that a takeover inherits
// what was actually pushed, not merely that it landed as the parent.
func (w *world) pushedWorkIsInTakeoverHistory(holder, taker string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never pushed the work; %q did", holder, w.holder)
	}
	if taker != w.taker {
		return fmt.Errorf("%q did not take the lease over; %q did", taker, w.taker)
	}
	pushedTip := section[deathState](w).pushedTip
	if pushedTip == "" {
		return fmt.Errorf("%q never pushed work; the push step comes first", holder)
	}
	repo := w.clones[taker]
	if out, err := gitCapture(w.t, repo, "merge-base", "--is-ancestor", pushedTip, w.taken.Tip); err != nil {
		return fmt.Errorf("the pushed work %s is not reachable from the takeover %s: %s",
			pushedTip, w.taken.Tip, out)
	}

	return nil
}

// localWorkInNoOriginHistory checks the holder's unpushed commit is
// unreachable from origin's current work ref — the loss S11 names: a
// takeover inherits only what was pushed, and this commit never was.
func (w *world) localWorkInNoOriginHistory(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q committed no unpushed work; %q did", holder, w.holder)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	local, err := w.unpushedWork(holder)
	if err != nil {
		return err
	}
	if out, err := gitCapture(w.t, repo, "fetch", "-q", "origin"); err != nil {
		return fmt.Errorf("fetch origin: %s: %w", out, err)
	}
	remote := "origin/" + claim.Branch(int64(w.planID))
	if _, err := gitCapture(w.t, repo, "merge-base", "--is-ancestor", local, remote); err == nil {
		return fmt.Errorf("the local commit %s is reachable from origin, want it absent", local)
	}

	return nil
}

// TestProcessDeathStepsRefuseAMachineTheyNeverMet: every quoted role
// this section's steps read is checked against the world, the same
// guard bdd_lease_test.go's own steps carry, so a scenario cannot
// pass by naming a machine it never introduced or the wrong role for
// one it did.
func TestProcessDeathStepsRefuseAMachineTheyNeverMet(t *testing.T) {
	w := newWorld(t)
	w.holder, w.taker = "box-a", "box-b"
	w.clones["box-a"] = t.TempDir()

	require.Error(t, w.claimDiesBeforeWritingAnything("ghost"))
	require.Error(t, w.retriesTheClaim("ghost"))
	require.Error(t, w.retryIsRefusedLocalDiverges("ghost"))
	require.Error(t, w.observerHasMaturedAWindowOnTip("ghost"))
	require.Error(t, w.pushedWorkIsInTakeoverHistory("ghost", "box-b"), "wrong holder")
	require.Error(t, w.pushedWorkIsInTakeoverHistory("box-a", "ghost"), "wrong taker")
	require.Error(t, w.localWorkInNoOriginHistory("ghost"))
	require.Error(t, w.takeoverIsChildOfTheReachedTip("ghost"))
	require.Error(t, w.takesOverTheCurrentLease("box-a"), "the holder cannot take over from itself")
}

// TestWinsTheLeaseAtEpoch1ReadsTheRecordedClaim: the step reads back
// what "claims plan" recorded rather than the world's shared lease
// fields, so it fails on the wrong winner or the wrong epoch instead
// of passing on an empty or unrelated world.
func TestWinsTheLeaseAtEpoch1ReadsTheRecordedClaim(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.winsTheLeaseAtEpoch1("box-b"), "nobody has claimed yet")

	ds := section[deathState](w)
	ds.winner, ds.winnerLease = "box-b", claim.Lease{Epoch: 2}
	require.Error(t, w.winsTheLeaseAtEpoch1("box-a"), "wrong winner")
	require.Error(t, w.winsTheLeaseAtEpoch1("box-b"), "wrong epoch")

	ds.winnerLease.Epoch = 1
	assert.NoError(t, w.winsTheLeaseAtEpoch1("box-b"))
}

// TestRetryIsRefusedLocalDivergesWantsTheRightErrorShape: a retry that
// failed for any other reason, or did not fail at all, is not the
// promise this row makes.
func TestRetryIsRefusedLocalDivergesWantsTheRightErrorShape(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"

	require.Error(t, w.retryIsRefusedLocalDiverges("box-a"), "no retry ran yet")

	section[deathState](w).retryErr = errors.New("some other fault")
	require.Error(t, w.retryIsRefusedLocalDiverges("box-a"), "wrong error shape")

	section[deathState](w).retryErr = &claim.LocalDivergesError{PlanID: 7}
	assert.NoError(t, w.retryIsRefusedLocalDiverges("box-a"))
}

// TestPushedWorkIsInTakeoverHistoryRefusesWithNoPush: the step stands
// on the pushed tip "pushes a work commit" records — without it a
// merge-base check against an empty string would misreport rather
// than name the missing step.
func TestPushedWorkIsInTakeoverHistoryRefusesWithNoPush(t *testing.T) {
	w := newWorld(t)
	w.holder, w.taker = "box-a", "box-b"

	err := w.pushedWorkIsInTakeoverHistory("box-a", "box-b")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "never pushed work")
}

// TestResetWindowWritesAFreshWindow: driven with an explicit instant,
// resetWindow always writes a one-sample window dated exactly then —
// never a stale span carried over, and never the real clock.
func TestResetWindowWritesAFreshWindow(t *testing.T) {
	isolate(t)
	at := time.Date(2031, 6, 7, 8, 9, 10, 0, time.UTC)

	resetWindow(discovery.Plan{Repo: "atlas", ID: 7}, "deadbeef", at)

	path, err := observe.Path()
	require.NoError(t, err)
	state := observe.Load(path)
	got, ok := state[observe.Key("atlas", 7)]
	require.True(t, ok, "the reset is recorded")
	assert.Equal(t, "deadbeef", got.Tip)
	assert.Equal(t, 1, got.Samples)
	assert.True(t, got.First.Equal(at))
	assert.True(t, got.Last.Equal(at))
	assert.Empty(t, got.Voided)
}
