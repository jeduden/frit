package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The host-death and races vocabulary rides the same world the lease
// section built: a claimant races the holder, a machine renews a ref
// that was deleted out from under it, a takeover is released and
// re-claimed. It registers itself, like the lease section did.
func init() {
	registrars = append(registrars, (*world).registerHostDeathAndRaces)
}

// raceAttempt is one machine's Acquire, kept beside the world so a
// later step can read what it won or lost without re-running it.
type raceAttempt struct {
	lease claim.Lease
	err   error
}

// racesState holds the races section's own state: every claimant's
// attempt, keyed by holder.
type racesState struct {
	attempts map[string]raceAttempt
}

// hostDeathState holds S17's own state: the release marker box-b left
// and who re-claimed the plan from it.
type hostDeathState struct {
	released  claim.Lease
	reclaimed claim.Lease
	reclaimer string
}

func (w *world) registerHostDeathAndRaces(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]+)" claims plan (\d+)$`, w.attemptsClaim)
	sc.Step(`^"([^"]+)" retries plan (\d+)$`, w.attemptsClaim)
	sc.Step(`^"([^"]+)"'s claim loses, naming "([^"]+)" at epoch (\d+)$`, w.claimLosesNaming)
	sc.Step(`^"([^"]+)"'s retry acquires at epoch (\d+)$`, w.retryAcquiresAtEpoch)
	sc.Step(`^origin carries one work ref for plan (\d+)$`, w.originCarriesOneWorkRef)
	sc.Step(`^origin's tip is "([^"]+)"'s claim marker$`, w.originsTipIsClaimMarker)
	sc.Step(`^"([^"]+)" knows the plan file by a new name$`, w.knowsThePlanFileByANewName)
	sc.Step(`^a human deletes the work ref on origin$`, w.theWorkRefIsDeletedOnOrigin)
	sc.Step(`^the plan completes and its ref is deleted on origin$`, w.theWorkRefIsDeletedOnOrigin)
	sc.Step(`^the renewal fails$`, w.theRenewalFails)
	sc.Step(`^origin still has no work ref$`, w.originStillHasNoWorkRef)
	sc.Step(`^"([^"]+)" pushes its tip raw$`, w.pushesItsTipRaw)
	sc.Step(`^origin accepts it$`, w.originAcceptsIt)
	sc.Step(`^"([^"]+)"'s raw push is rejected as non-fast-forward$`, w.rawPushIsRejectedAsNonFastForward)
	sc.Step(`^"([^"]+)" releases the lease$`, w.releasesTheLease)
	sc.Step(`^"([^"]+)" claims the released plan$`, w.claimsTheReleasedPlan)
	sc.Step(`^the re-claim lands at epoch (\d+), a child of the release marker$`, w.reclaimLandsAtEpoch)
	sc.Step(`^yield parks "([^"]+)"'s work and leaves "([^"]+)"'s re-claim untouched$`, w.yieldParksLeavingReclaim)
}

// attemptsClaim is a fresh claimant's Acquire: the first time a holder
// is named it clones from the current holder's origin, and a later
// call — a retry after something on origin changed — reuses that same
// clone, so a local commit it carries (S27's rename) or a prior loss
// (S28's retry) stays exactly where the scenario put it. The result is
// kept, win or lose: a race is a fact to assert on, not a step to fail.
func (w *world) attemptsClaim(holder string, planID int) error {
	if planID != w.planID {
		return fmt.Errorf("this scenario set up plan %d, not %d", w.planID, planID)
	}
	repo, ok := w.clones[holder]
	if !ok {
		first, err := w.cloneOf(w.holder)
		if err != nil {
			return err
		}
		repo = cloneAgain(w.t, first)
		w.clones[holder] = repo
	}

	rs := section[racesState](w)
	if rs.attempts == nil {
		rs.attempts = map[string]raceAttempt{}
	}
	lease, err := claim.Acquire(repo, leaseFor(holder, planID), gitwt.Exec)
	rs.attempts[holder] = raceAttempt{lease: lease, err: err}

	return nil
}

// attemptOf reads back one holder's claim attempt, refusing a holder
// that never claimed rather than reading a zero value as a result.
func (w *world) attemptOf(holder string) (raceAttempt, error) {
	rs := section[racesState](w)
	a, ok := rs.attempts[holder]
	if !ok {
		return raceAttempt{}, fmt.Errorf("%q never attempted a claim", holder)
	}

	return a, nil
}

func (w *world) claimLosesNaming(holder, winner string, epoch int) error {
	a, err := w.attemptOf(holder)
	if err != nil {
		return err
	}
	var held *claim.HeldError
	if !errors.As(a.err, &held) {
		return fmt.Errorf("%q's claim was expected to lose, got %v", holder, a.err)
	}
	if !held.Known {
		return fmt.Errorf("%q's claim lost with no winning marker read", holder)
	}
	if held.Marker.Holder != winner {
		return fmt.Errorf("%q's claim names %q, want %q", holder, held.Marker.Holder, winner)
	}
	if held.Marker.Epoch != epoch {
		return fmt.Errorf("%q's claim names epoch %d, want %d", holder, held.Marker.Epoch, epoch)
	}

	return nil
}

func (w *world) retryAcquiresAtEpoch(holder string, epoch int) error {
	a, err := w.attemptOf(holder)
	if err != nil {
		return err
	}
	if a.err != nil {
		return fmt.Errorf("%q's retry failed: %w", holder, a.err)
	}
	if a.lease.Epoch != epoch {
		return fmt.Errorf("%q's retry landed at epoch %d, want %d", holder, a.lease.Epoch, epoch)
	}

	return nil
}

func (w *world) originsTipIsClaimMarker(holder string) error {
	a, err := w.attemptOf(holder)
	if err != nil {
		return err
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	got := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if got != a.lease.Tip {
		return fmt.Errorf("origin holds %s, want %q's claim marker %s", got, holder, a.lease.Tip)
	}

	return nil
}

// originCarriesOneWorkRef counts the work refs a race left on origin:
// exactly one, however many machines contended for it.
func (w *world) originCarriesOneWorkRef(planID int) error {
	if planID != w.planID {
		return fmt.Errorf("this scenario set up plan %d, not %d", w.planID, planID)
	}
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	refs, err := gitCapture(w.t, repo, "ls-remote", "--heads", "origin", "refs/heads/plan/*")
	if err != nil {
		return fmt.Errorf("%s: %w", refs, err)
	}
	if count := strings.Count(refs, "refs/heads/plan/"); count != 1 {
		return fmt.Errorf("origin carries %d work refs for plan %d, want 1", count, planID)
	}

	return nil
}

// knowsThePlanFileByANewName gives a claimant its own clone with the
// plan file renamed and committed locally — never pushed. The work
// ref is minted from the plan id alone (claim.Branch), so nothing of
// this rename can reach it: the point S27 proves.
func (w *world) knowsThePlanFileByANewName(holder string) error {
	first, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	repo := cloneAgain(w.t, first)
	w.clones[holder] = repo

	matches, err := filepath.Glob(
		filepath.Join(repo, "plan", fmt.Sprintf("%d_*.md", w.planID)))
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return fmt.Errorf(
			"expected one plan file for plan %d, found %d", w.planID, len(matches))
	}
	renamed := filepath.Join(
		filepath.Dir(matches[0]), fmt.Sprintf("%d_renamed.md", w.planID))
	git(w.t, repo, "mv", matches[0], renamed)
	git(w.t, repo, "commit", "-q", "-m", "rename plan file")

	return nil
}

// theWorkRefIsDeletedOnOrigin drops the work ref straight from the
// remote, the shape a human's `git push --delete` or a completed
// plan's teardown both take: nothing local moves, only origin's ref.
func (w *world) theWorkRefIsDeletedOnOrigin() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, repo, "push", "origin", ":"+w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}

	return nil
}

func (w *world) theRenewalFails() error {
	if w.err == nil {
		return fmt.Errorf("the renewal succeeded, want a failure")
	}

	return nil
}

func (w *world) originStillHasNoWorkRef() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	remote, err := gitCapture(w.t, repo, "ls-remote", "origin", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", remote, err)
	}
	if remote != "" {
		return fmt.Errorf("origin carries a work ref: %q", remote)
	}

	return nil
}

func (w *world) pushesItsTipRaw(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	out, pushErr := gitCapture(w.t, repo, "push", "origin", w.lease.Tip+":"+w.branch())
	if pushErr != nil {
		return fmt.Errorf("origin refused the raw push: %s: %w", out, pushErr)
	}

	return nil
}

// originAcceptsIt is the TRUST observable: a fact read back off origin,
// never inferred from the push command's own exit status.
func (w *world) originAcceptsIt() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	got := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if got != w.lease.Tip {
		return fmt.Errorf("origin holds %s, want %s's tip %s", got, w.holder, w.lease.Tip)
	}

	return nil
}

// rawPushIsRejectedAsNonFastForward is S16's pushIsRejected with one
// more fact pinned: the rejection is specifically non-fast-forward,
// the shape git gives a push that cannot be replayed onto the ref's
// current tip, not merely any refusal.
func (w *world) rawPushIsRejectedAsNonFastForward(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	local, err := w.unpushedWork(holder)
	if err != nil {
		return err
	}
	out, pushErr := gitCapture(w.t, repo, "push", "origin", local+":"+w.branch())
	if pushErr == nil {
		return fmt.Errorf("origin accepted the raw push: %s", out)
	}
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "non-fast-forward") && !strings.Contains(lower, "fetch first") {
		return fmt.Errorf("the push was rejected, but not as non-fast-forward: %s", out)
	}

	return w.originHoldsTheTakeover()
}

func (w *world) releasesTheLease(holder string) error {
	if holder != w.taker {
		return fmt.Errorf("%q did not take the lease over; %q did", holder, w.taker)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	released, err := claim.Release(repo, leaseFor(holder, w.planID), w.taken.Tip, gitwt.Exec)
	if err != nil {
		return err
	}
	section[hostDeathState](w).released = released

	return nil
}

func (w *world) claimsTheReleasedPlan(holder string) error {
	first, err := w.cloneOf(w.taker)
	if err != nil {
		return err
	}
	repo := cloneAgain(w.t, first)
	w.clones[holder] = repo

	lease, err := claim.Acquire(repo, leaseFor(holder, w.planID), gitwt.Exec)
	if err != nil {
		return err
	}
	hs := section[hostDeathState](w)
	hs.reclaimed = lease
	hs.reclaimer = holder

	return nil
}

func (w *world) reclaimLandsAtEpoch(epoch int) error {
	hs := section[hostDeathState](w)
	if hs.reclaimer == "" {
		return fmt.Errorf("no plan has been re-claimed yet")
	}
	if hs.reclaimed.Epoch != epoch {
		return fmt.Errorf("the re-claim landed at epoch %d, want %d", hs.reclaimed.Epoch, epoch)
	}
	repo, err := w.cloneOf(hs.reclaimer)
	if err != nil {
		return err
	}
	parent, err := gitCapture(w.t, repo, "rev-parse", hs.reclaimed.Tip+"^")
	if err != nil {
		return fmt.Errorf("%s: %w", parent, err)
	}
	if parent != hs.released.Tip {
		return fmt.Errorf(
			"the re-claim's parent is %s, want the release marker %s", parent, hs.released.Tip)
	}

	return nil
}

// yieldParksLeavingReclaim is S17's own final step: unlike S16's
// yieldParks, origin no longer holds the takeover this fenced holder
// lost to — box-b released it and reclaimer re-claimed it fresh — so
// the check is against the re-claim's tip, not the takeover's.
func (w *world) yieldParksLeavingReclaim(holder, reclaimer string) error {
	if holder != w.holder {
		return fmt.Errorf("%q is not the fenced holder, %q is", holder, w.holder)
	}
	hs := section[hostDeathState](w)
	if reclaimer != hs.reclaimer {
		return fmt.Errorf("%q did not re-claim the plan; %q did", reclaimer, hs.reclaimer)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	local, err := w.unpushedWork(holder)
	if err != nil {
		return err
	}

	sc, err := claim.Yield(repo, leaseFor(holder, w.planID), local, gitwt.Exec)
	if err != nil {
		return err
	}
	rescue, err := gitCapture(w.t, repo, "ls-remote", "origin", sc.Rescue)
	if err != nil {
		return fmt.Errorf("%s: %w", rescue, err)
	}
	if fields := strings.Fields(rescue); len(fields) == 0 || fields[0] != local {
		return fmt.Errorf("the rescue ref %s does not point at %s: %q", sc.Rescue, local, rescue)
	}

	got := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if got != hs.reclaimed.Tip {
		return fmt.Errorf("origin holds %s, want the re-claim %s", got, hs.reclaimed.Tip)
	}

	return nil
}

// TestAttemptsClaimRefusesAPlanTheScenarioNeverSetUp: a scenario names
// one plan in its opening Given; a step naming another id is a typo in
// the feature file, not a second plan, so it fails before touching git.
func TestAttemptsClaimRefusesAPlanTheScenarioNeverSetUp(t *testing.T) {
	w := newWorld(t)
	w.planID = 7

	require.Error(t, w.attemptsClaim("box-b", 8))
}

// TestAttemptOfRefusesAHolderThatNeverClaimed: reading back a claim
// attempt before any claims step ran, or for a machine that never
// claimed, fails rather than reading a zero-value win.
func TestAttemptOfRefusesAHolderThatNeverClaimed(t *testing.T) {
	w := newWorld(t)

	_, err := w.attemptOf("box-b")
	require.Error(t, err)
}

// TestClaimLosesNamingChecksTheWinnerAndTheEpoch: the Then step reads
// the loser's own HeldError, so it fails on a claim that actually won,
// one whose winner marker was never read, and one whose winner or
// epoch does not match what the scenario asserts.
func TestClaimLosesNamingChecksTheWinnerAndTheEpoch(t *testing.T) {
	w := newWorld(t)
	rs := section[racesState](w)
	rs.attempts = map[string]raceAttempt{}

	rs.attempts["box-b"] = raceAttempt{err: nil}
	require.Error(t, w.claimLosesNaming("box-b", "box-a", 1),
		"a claim that won is not a loss")

	rs.attempts["box-b"] = raceAttempt{err: &claim.HeldError{Known: false}}
	require.Error(t, w.claimLosesNaming("box-b", "box-a", 1),
		"an unread winner marker cannot name anyone")

	rs.attempts["box-b"] = raceAttempt{
		err: &claim.HeldError{Known: true, Marker: claim.Marker{Holder: "box-c", Epoch: 1}},
	}
	require.Error(t, w.claimLosesNaming("box-b", "box-a", 1),
		"the loser's marker names another machine")

	rs.attempts["box-b"] = raceAttempt{
		err: &claim.HeldError{Known: true, Marker: claim.Marker{Holder: "box-a", Epoch: 2}},
	}
	require.Error(t, w.claimLosesNaming("box-b", "box-a", 1),
		"the loser's marker names the right winner at the wrong epoch")

	rs.attempts["box-b"] = raceAttempt{
		err: &claim.HeldError{Known: true, Marker: claim.Marker{Holder: "box-a", Epoch: 1}},
	}
	assert.NoError(t, w.claimLosesNaming("box-b", "box-a", 1))
}

// TestRetryAcquiresAtEpochChecksTheLease: a retry that failed, or one
// that won at a different epoch than the scenario asserts, both fail
// the step.
func TestRetryAcquiresAtEpochChecksTheLease(t *testing.T) {
	w := newWorld(t)
	rs := section[racesState](w)
	rs.attempts = map[string]raceAttempt{
		"box-b": {err: errors.New("still contended")},
	}
	require.Error(t, w.retryAcquiresAtEpoch("box-b", 1))

	rs.attempts["box-b"] = raceAttempt{lease: claim.Lease{Epoch: 2}}
	require.Error(t, w.retryAcquiresAtEpoch("box-b", 1),
		"the retry landed at an epoch the scenario did not assert")

	rs.attempts["box-b"] = raceAttempt{lease: claim.Lease{Epoch: 1}}
	assert.NoError(t, w.retryAcquiresAtEpoch("box-b", 1))
}

// TestOriginCarriesOneWorkRefRefusesAPlanTheScenarioNeverSetUp: like
// attemptsClaim, a mismatched plan id in the step text is refused
// before any ls-remote runs.
func TestOriginCarriesOneWorkRefRefusesAPlanTheScenarioNeverSetUp(t *testing.T) {
	w := newWorld(t)
	w.planID = 7

	require.Error(t, w.originCarriesOneWorkRef(8))
}

// TestTheRenewalFailsReadsWErr: the step reads whatever the last
// renewal step left in w.err — nothing when it succeeded, an error
// when it did not.
func TestTheRenewalFailsReadsWErr(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theRenewalFails(), "no error means the renewal succeeded")

	w.err = errors.New("push rejected")
	assert.NoError(t, w.theRenewalFails())
}

// TestPushesItsTipRawRefusesAMachineThatNeverHeldTheLease: only the
// original holder's own tip is meaningful to push raw; the step
// refuses standing in for a machine that was never the holder.
func TestPushesItsTipRawRefusesAMachineThatNeverHeldTheLease(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"

	require.Error(t, w.pushesItsTipRaw("box-b"))
}

// TestReleasesTheLeaseRefusesAMachineThatNeverTookOver: release is the
// taking machine's own transition; a scenario naming any other machine
// is refused rather than releasing on its behalf.
func TestReleasesTheLeaseRefusesAMachineThatNeverTookOver(t *testing.T) {
	w := newWorld(t)
	w.taker = "box-b"

	require.Error(t, w.releasesTheLease("box-c"))
}

// TestReclaimLandsAtEpochRefusesBeforeAReclaim: the epoch and ancestry
// check both stand on a re-claim already recorded; asking before one
// happened, or with the wrong epoch, fails without touching git.
func TestReclaimLandsAtEpochRefusesBeforeAReclaim(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.reclaimLandsAtEpoch(3), "no plan has been re-claimed yet")

	hs := section[hostDeathState](w)
	hs.reclaimer = "box-c"
	hs.reclaimed = claim.Lease{Epoch: 2}
	require.Error(t, w.reclaimLandsAtEpoch(3), "the re-claim landed at the wrong epoch")
}

// TestYieldParksLeavingReclaimRefusesTheWrongRoles: the fenced holder
// and the re-claimer are both checked against the scenario's own
// setup, so the step cannot pass on a swapped or invented machine.
func TestYieldParksLeavingReclaimRefusesTheWrongRoles(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	section[hostDeathState](w).reclaimer = "box-c"

	require.Error(t, w.yieldParksLeavingReclaim("box-b", "box-c"),
		"box-b never held the lease box-a did")
	require.Error(t, w.yieldParksLeavingReclaim("box-a", "box-d"),
		"box-d never re-claimed the plan")
}
