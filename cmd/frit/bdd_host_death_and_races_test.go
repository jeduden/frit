package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/report"
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

// cliState holds what a verb-level row needs beside the shared world:
// this row family drives `frit claim` and `frit orphans` through the
// CLI rather than the lease API directly, so a scenario's Given sets
// up a lane and a bound session, its When captures one CLI run's
// output, and its Then reads both back.
type cliState struct {
	lane     string // "this host"'s own worktree, set for S14 and S18
	session  string // the session a bound lease's marker names
	token    string // the tip persisted at setup; a successful resume's parent
	rawTip   string // the tip a mid-scenario raw commit and push leaves
	herdrSet bool   // a herdr fake was installed explicitly this scenario
	out      bytes.Buffer
	errb     bytes.Buffer
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
	sc.Step(`^"this host" holds the lease for plan (\d+), bound in its own lane$`, w.thisHostHoldsBoundLease)
	sc.Step(`^"([^"]+)" holds the lease for plan (\d+), bound to a session$`, w.holdsBoundLease)
	sc.Step(`^"this host" commits raw work on its own lane and pushes it$`, w.thisHostCommitsRawWorkAndPushes)
	sc.Step(`^this host claims plan (\d+)$`, w.thisHostClaimsPlan)
	sc.Step(`^a live agent sits on that lane's own session$`, w.aLiveAgentSitsOnThatLanesSession)
	sc.Step(`^the live agent goes quiet$`, w.theLiveAgentGoesQuiet)
	sc.Step(`^"([^"]+)"'s bound session wakes and answers live$`, w.boundSessionWakesAndAnswersLive)
	sc.Step(`^the claim is refused, naming the lease already held$`, w.theClaimIsRefusedAlreadyHeld)
	sc.Step(`^the takeover is refused, naming a live agent session$`, w.theTakeoverIsRefusedNamingALiveSession)
	sc.Step(`^origin's hold is left exactly as it stood$`, w.originsHoldIsLeftExactlyAsItStood)
	sc.Step(`^this host resumes its own lease from the persisted token$`, w.thisHostResumesFromThePersistedToken)
	sc.Step(`^this host resumes its own lease from origin's fresh tip, not the stale token$`,
		w.thisHostResumesFromFreshTip)
	sc.Step(`^the hold's takeover window has matured$`, w.theHoldsTakeoverWindowHasMatured)
	sc.Step(`^this host takes the lease over, epoch (\d+), child of the stale tip$`, w.thisHostTakesTheLeaseOver)
	sc.Step(`^origin's orphan report lists plan (\d+) as neither stale nor deserted$`,
		w.originsOrphanReportListsNeither)
	sc.Step(`^"([^"]+)"'s lease is renewed by a beat instead of seized$`, w.leaseIsRenewedByABeatInsteadOfSeized)
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
		var err error
		repo, err = w.cloneAs(w.holder, holder)
		if err != nil {
			return err
		}
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

// originsTipIsClaimMarker reads back the holder's own claim attempt:
// a holder whose last attempt lost carries no lease of its own to
// compare against, so that is refused by name rather than read as a
// zero-value tip that would only fail later with a confusing message.
func (w *world) originsTipIsClaimMarker(holder string) error {
	a, err := w.attemptOf(holder)
	if err != nil {
		return err
	}
	if a.err != nil {
		return fmt.Errorf("%q's claim never won, so it minted no marker: %w", holder, a.err)
	}

	return w.originTipIs(holder, a.lease.Tip)
}

// originCarriesOneWorkRef counts the work refs a race left on origin
// for this plan: exactly one, however many machines contended for it.
// The ls-remote pattern names the plan's own ref rather than every
// plan/* ref on the remote, so a second plan sharing the same origin
// can never be counted as if it were this one's contention.
func (w *world) originCarriesOneWorkRef(planID int) error {
	if planID != w.planID {
		return fmt.Errorf("this scenario set up plan %d, not %d", w.planID, planID)
	}
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	refs, err := gitCapture(w.t, repo, "ls-remote", "--heads", "origin", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", refs, err)
	}
	if count := strings.Count(refs, w.branch()); count != 1 {
		return fmt.Errorf("origin carries %d work refs for plan %d, want 1", count, planID)
	}

	return nil
}

// knowsThePlanFileByANewName gives a claimant its own clone with the
// plan file renamed and committed locally — never pushed. The work
// ref is minted from the plan id alone (claim.Branch), so nothing of
// this rename can reach it: the point S27 proves.
func (w *world) knowsThePlanFileByANewName(holder string) error {
	repo, err := w.cloneAs(w.holder, holder)
	if err != nil {
		return err
	}

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
	return w.originTipIs(w.holder, w.lease.Tip)
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
	out, pushErr := gitCaptureEnglish(w.t, repo, "push", "origin", local+":"+w.branch())
	if pushErr == nil {
		return fmt.Errorf("origin accepted the raw push: %s", out)
	}
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "non-fast-forward") && !strings.Contains(lower, "fetch first") {
		return fmt.Errorf("the push was rejected, but not as non-fast-forward: %s", out)
	}

	return w.originHoldsTheTakeover()
}

// gitCaptureEnglish is gitCapture pinned to the "C" locale: this step
// asserts on the specific English wording of git's own rejection
// message, so it must not depend on the host's locale producing a
// translated one.
func gitCaptureEnglish(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	out, err := cmd.CombinedOutput()

	return strings.TrimSpace(string(out)), err
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
	repo, err := w.cloneAs(w.taker, holder)
	if err != nil {
		return err
	}

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
	if err := w.verifyRescue(repo, holder, local); err != nil {
		return err
	}

	return w.originTipIs(holder, hs.reclaimed.Tip)
}

// thisHostHoldsBoundLease is S14's and S18's setup: this host holds
// the lease bound to a real worktree lane and a session, the token
// persisted by a renewal — the shape a lane that has actually started
// work leaves, which self-resume proves itself against.
func (w *world) thisHostHoldsBoundLease(planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = "this host"
	repo := claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")
	w.clones["this host"] = repo

	cs := section[cliState](w)
	cs.lane = filepath.Join(w.t.TempDir(), "atlas-lane")
	cs.session = "wOld:p1"
	opts := claim.LeaseOptions{
		PlanID: int64(planID), Remote: "origin", Base: "origin/main",
		Holder: hostname(), Lane: cs.lane, Session: cs.session,
	}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	if err != nil {
		return err
	}
	git(w.t, repo, "worktree", "add", "-q", cs.lane, claim.Branch(int64(planID)))
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	if err != nil {
		return err
	}
	cs.token = renewed.Tip
	w.t.Chdir(cs.lane)

	return nil
}

// holdsBoundLease is S15's and S31's setup: a lease held by another
// machine, bound to a session, with no local lane of this host's own
// — the shape S31's takeover veto and its beat-on-behalf-of both read
// off the marker rather than a worktree.
func (w *world) holdsBoundLease(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	repo := claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")
	w.clones[holder] = repo

	cs := section[cliState](w)
	cs.session = "wS:p9"
	opts := claim.LeaseOptions{
		PlanID: int64(planID), Remote: "origin", Base: "origin/main",
		Holder: holder, Lane: "/lanes/" + holder, Session: cs.session,
	}
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	if err != nil {
		return err
	}
	w.holder, w.lease = holder, lease

	return nil
}

// thisHostCommitsRawWorkAndPushes is the ordinary raw-TDD workflow on
// this host's own lane, landed on origin with no frit transition
// between — S14's "the push did land" half.
func (w *world) thisHostCommitsRawWorkAndPushes() error {
	cs := section[cliState](w)
	if cs.lane == "" {
		return fmt.Errorf("this host has no lane; the bound-lease step comes first")
	}
	git(w.t, cs.lane, "commit", "--allow-empty", "-q", "-m", "red: add failing test")
	git(w.t, cs.lane, "commit", "--allow-empty", "-q", "-m", "green: make it pass")
	git(w.t, cs.lane, "push", "-q", "origin", claim.Branch(int64(w.planID)))
	tip, err := gitCapture(w.t, cs.lane, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	cs.rawTip = tip

	return nil
}

// thisHostClaimsPlan drives the CLI directly, capturing its report for
// the Then steps to read: `frit claim` is where the resume, the
// window's takeover and the live-session veto all actually live,
// never mocked at the lease-API level. Absent a herdr fake this
// scenario already set explicitly, it installs the standing one every
// takeover needs to stand a worktree up; a resume never calls it.
func (w *world) thisHostClaimsPlan(planID int) error {
	if planID != w.planID {
		return fmt.Errorf("this scenario set up plan %d, not %d", w.planID, planID)
	}
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	cs := section[cliState](w)
	if !cs.herdrSet {
		runner, _ := startHerdr()
		withHerdr(w.t, runner)
		cs.herdrSet = true
	}
	cs.out.Reset()
	cs.errb.Reset()
	run([]string{"claim", strconv.Itoa(planID), "--root", filepath.Dir(repo)},
		&cs.out, &cs.errb)

	return nil
}

// aLiveAgentSitsOnThatLanesSession fakes herdr showing the setup
// step's own bound session live — S18's VETO branch.
func (w *world) aLiveAgentSitsOnThatLanesSession() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	cs := section[cliState](w)
	if cs.session == "" {
		return fmt.Errorf("no bound session recorded for %q", w.holder)
	}
	withHerdr(w.t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "working",
		"cwd": repo, "pane_id": cs.session,
		"agent_session": map[string]any{"value": cs.session},
	}))
	cs.herdrSet = true

	return nil
}

// theLiveAgentGoesQuiet drops the herdr fake to no live agents at
// all, so a later claim reads no veto.
func (w *world) theLiveAgentGoesQuiet() error {
	withHerdr(w.t, herdrReturning())
	section[cliState](w).herdrSet = true

	return nil
}

// boundSessionWakesAndAnswersLive is S31's own live-session fake: the
// same shape aLiveAgentSitsOnThatLanesSession builds, for the machine
// a matured takeover is about to veto rather than this host's own
// lane.
func (w *world) boundSessionWakesAndAnswersLive(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	cs := section[cliState](w)
	if cs.session == "" {
		return fmt.Errorf("no bound session recorded for %q", holder)
	}
	withHerdr(w.t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "working",
		"cwd": repo, "pane_id": cs.session,
		"agent_session": map[string]any{"value": cs.session},
	}))
	cs.herdrSet = true

	return nil
}

// theClaimIsRefusedAlreadyHeld is S18's refusal: claimCmd's own
// claimRefusal, fired because resumeOwnLease's veto fell through to a
// held, not-yet-matured plan.
func (w *world) theClaimIsRefusedAlreadyHeld() error {
	got := section[cliState](w).out.String()
	if !strings.Contains(got, "refused") {
		return fmt.Errorf("expected a refusal, got: %s", got)
	}
	if !strings.Contains(got, "already held") {
		return fmt.Errorf("the refusal does not name the lease already held: %s", got)
	}

	return nil
}

// theTakeoverIsRefusedNamingALiveSession is S31's refusal: distinct
// wording from S18's, since a matured window routes the attempt
// through mintOrTakeOver's own veto rather than claimRefusal.
func (w *world) theTakeoverIsRefusedNamingALiveSession() error {
	got := section[cliState](w).out.String()
	if !strings.Contains(got, "refused") {
		return fmt.Errorf("expected a refusal, got: %s", got)
	}
	if !strings.Contains(got, "live agent session") {
		return fmt.Errorf("the refusal does not name a live agent session: %s", got)
	}

	return nil
}

// originsHoldIsLeftExactlyAsItStood confirms a refused claim moved
// nothing: origin's tip is still the one the setup step persisted.
func (w *world) originsHoldIsLeftExactlyAsItStood() error {
	cs := section[cliState](w)
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	got := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if got != cs.token {
		return fmt.Errorf("origin holds %s, want the unchanged hold %s", got, cs.token)
	}

	return nil
}

// resumedBeatParent reads back the beat a successful resume pushed
// and its parent tip — the fact both S14 assertions and S18's own
// resume reduce to, over two different parents.
func (w *world) resumedBeatParent() (string, error) {
	got := section[cliState](w).out.String()
	if strings.Contains(got, "refused") {
		return "", fmt.Errorf("expected a resume, got a refusal: %s", got)
	}
	if !strings.Contains(got, "resumed plan") {
		return "", fmt.Errorf("expected a resume, got: %s", got)
	}
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return "", err
	}
	tip, err := gitCapture(w.t, repo, "rev-parse", "refs/heads/"+claim.Branch(int64(w.planID)))
	if err != nil {
		return "", fmt.Errorf("%s: %w", tip, err)
	}

	return gitCapture(w.t, repo, "rev-parse", tip+"^")
}

// thisHostResumesFromThePersistedToken is the push-never-landed half
// of S14, and S18's own successful resume once the live agent goes
// quiet: the beat's parent is the token the setup step persisted.
func (w *world) thisHostResumesFromThePersistedToken() error {
	parent, err := w.resumedBeatParent()
	if err != nil {
		return err
	}
	cs := section[cliState](w)
	if parent != cs.token {
		return fmt.Errorf(
			"the resume's parent is %s, want the persisted token %s", parent, cs.token)
	}

	return nil
}

// thisHostResumesFromFreshTip is S14's push-did-land half: the beat's
// parent is the raw tip that commit pushed, not the stale token
// (S86).
func (w *world) thisHostResumesFromFreshTip() error {
	cs := section[cliState](w)
	if cs.rawTip == "" {
		return fmt.Errorf("no raw work was pushed; the commits-and-pushes step comes first")
	}
	parent, err := w.resumedBeatParent()
	if err != nil {
		return err
	}
	if parent != cs.rawTip {
		return fmt.Errorf(
			"the resume's parent is %s, want the raw pushed tip %s", parent, cs.rawTip)
	}

	return nil
}

// theHoldsTakeoverWindowHasMatured backdates the observation window
// directly — no clock seam, no sleep — past the repo's configured
// threshold, the state a faithful observer holds over a dead holder's
// unmoving ref.
func (w *world) theHoldsTakeoverWindowHasMatured() error {
	seedWindow(w.t, "atlas", int64(w.planID), w.lease.Tip, 3*time.Hour)

	return nil
}

// thisHostTakesTheLeaseOver is S15's takeover: a matured, unvetoed
// window taken over unaided, the new marker a child of exactly the
// observed stale tip.
func (w *world) thisHostTakesTheLeaseOver(epoch int) error {
	got := section[cliState](w).out.String()
	if !strings.Contains(got, "claimed plan") {
		return fmt.Errorf("expected a takeover, got: %s", got)
	}
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	tip, err := gitCapture(w.t, repo, "rev-parse", "refs/heads/"+claim.Branch(int64(w.planID)))
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	body, err := gitCapture(w.t, repo, "log", "-1", "--format=%B", tip)
	if err != nil {
		return fmt.Errorf("%s: %w", body, err)
	}
	if !strings.Contains(body, fmt.Sprintf("plan %d: takeover", w.planID)) {
		return fmt.Errorf("the new tip is not a takeover marker: %q", body)
	}
	if !strings.Contains(body, fmt.Sprintf("epoch:   %d", epoch)) {
		return fmt.Errorf("the takeover marker does not read epoch %d: %q", epoch, body)
	}
	parent, err := gitCapture(w.t, repo, "rev-parse", tip+"^")
	if err != nil {
		return fmt.Errorf("%s: %w", parent, err)
	}
	if parent != w.lease.Tip {
		return fmt.Errorf(
			"the takeover's parent is %s, want the stale tip %s", parent, w.lease.Tip)
	}

	return nil
}

// originsOrphanReportListsNeither is S31's report-only half: a hold
// neither matured nor herdr-confirmed-dead appears in neither of
// `frit orphans`'s stale or deserted lists. Absent an explicit herdr
// fake already set this scenario, herdr is left genuinely
// unreachable — SessionDeadIn reads any session missing from a
// reachable, empty pane list as confirmed dead, which would falsely
// place this still-sleeping hold in Deserted.
func (w *world) originsOrphanReportListsNeither(planID int) error {
	if planID != w.planID {
		return fmt.Errorf("this scenario set up plan %d, not %d", w.planID, planID)
	}
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	cs := section[cliState](w)
	if !cs.herdrSet {
		withHerdr(w.t, func(...string) ([]byte, error) {
			return nil, fmt.Errorf("herdr unreachable")
		})
		cs.herdrSet = true
	}
	var out, errb bytes.Buffer
	code := run([]string{"orphans", "--root", filepath.Dir(repo), "--json"}, &out, &errb)
	if code != 0 {
		return fmt.Errorf("frit orphans exited %d: %s", code, errb.String())
	}
	var doc report.OrphansDoc
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		return fmt.Errorf("%s: %w", out.String(), err)
	}
	for _, r := range doc.Repos {
		if r.Name != "atlas" {
			continue
		}
		for _, s := range r.StaleHolds {
			if s.PlanID == int64(planID) {
				return fmt.Errorf(
					"plan %d is reported stale before its window matured", planID)
			}
		}
		for _, d := range r.Deserted {
			if d.PlanID == int64(planID) {
				return fmt.Errorf(
					"plan %d is reported deserted though herdr could not confirm it",
					planID)
			}
		}
	}

	return nil
}

// leaseIsRenewedByABeatInsteadOfSeized is S31's veto observable: the
// vetoed holder's own lease was renewed on its behalf, a beat CASed
// from the tip the window matured on, same epoch, its identity
// trailers copied from the holder's own marker rather than this run's.
func (w *world) leaseIsRenewedByABeatInsteadOfSeized(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	tip, err := gitCapture(w.t, repo, "rev-parse", "refs/heads/"+claim.Branch(int64(w.planID)))
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	body, err := gitCapture(w.t, repo, "log", "-1", "--format=%B", tip)
	if err != nil {
		return fmt.Errorf("%s: %w", body, err)
	}
	if !strings.Contains(body, fmt.Sprintf("plan %d: beat", w.planID)) {
		return fmt.Errorf("origin's tip is not a beat: %q", body)
	}
	if !strings.Contains(body, "holder:  "+holder) {
		return fmt.Errorf("the beat does not renew %q's own lease: %q", holder, body)
	}
	parent, err := gitCapture(w.t, repo, "rev-parse", tip+"^")
	if err != nil {
		return fmt.Errorf("%s: %w", parent, err)
	}
	if parent != w.lease.Tip {
		return fmt.Errorf(
			"the beat's parent is %s, want the original tip %s", parent, w.lease.Tip)
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

// TestThisHostCommitsRawWorkAndPushesRefusesWithNoLane: the step
// operates on the lane the bound-lease setup recorded; asked before
// that ran, it refuses rather than committing nowhere.
func TestThisHostCommitsRawWorkAndPushesRefusesWithNoLane(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.thisHostCommitsRawWorkAndPushes())
}

// TestThisHostClaimsPlanRefusesAPlanTheScenarioNeverSetUp: like
// attemptsClaim, a mismatched plan id in the step text is refused
// before the CLI ever runs.
func TestThisHostClaimsPlanRefusesAPlanTheScenarioNeverSetUp(t *testing.T) {
	w := newWorld(t)
	w.planID = 7
	require.Error(t, w.thisHostClaimsPlan(8))
}

// TestALiveAgentSitsOnThatLanesSessionRefusesWithNoBoundSession: the
// fake needs a session id to name live; asked before the bound-lease
// setup recorded one, it refuses rather than faking an empty session.
func TestALiveAgentSitsOnThatLanesSessionRefusesWithNoBoundSession(t *testing.T) {
	w := newWorld(t)
	w.holder = "this host"
	w.clones["this host"] = t.TempDir()
	require.Error(t, w.aLiveAgentSitsOnThatLanesSession())
}

// TestBoundSessionWakesAndAnswersLiveRefusesTheWrongMachineOrNoSession:
// the quoted holder is checked against the scenario's own setup, and
// the fake still needs a recorded session id.
func TestBoundSessionWakesAndAnswersLiveRefusesTheWrongMachineOrNoSession(t *testing.T) {
	w := newWorld(t)
	w.holder = "elsewhere"
	w.clones["elsewhere"] = t.TempDir()

	require.Error(t, w.boundSessionWakesAndAnswersLive("box-a"),
		"box-a never held the lease; elsewhere did")
	require.Error(t, w.boundSessionWakesAndAnswersLive("elsewhere"),
		"no bound session was recorded")
}

// TestTheClaimIsRefusedAlreadyHeldChecksTheCLIOutput: the step reads
// the last claim's captured stdout, and fails on a success or on a
// refusal that never names the lease already held.
func TestTheClaimIsRefusedAlreadyHeldChecksTheCLIOutput(t *testing.T) {
	w := newWorld(t)
	cs := section[cliState](w)

	cs.out.WriteString("claimed plan 7\n")
	require.Error(t, w.theClaimIsRefusedAlreadyHeld(), "a success is not a refusal")

	cs.out.Reset()
	cs.out.WriteString("refused: is held by a live agent session on box-a\n")
	require.Error(t, w.theClaimIsRefusedAlreadyHeld(),
		"a differently-worded refusal is not this one")

	cs.out.Reset()
	cs.out.WriteString("refused: already held (plan/7); seen unchanged for 1m\n")
	assert.NoError(t, w.theClaimIsRefusedAlreadyHeld())
}

// TestTheTakeoverIsRefusedNamingALiveSessionChecksTheCLIOutput: the
// mirror check for S31's own refusal wording.
func TestTheTakeoverIsRefusedNamingALiveSessionChecksTheCLIOutput(t *testing.T) {
	w := newWorld(t)
	cs := section[cliState](w)

	cs.out.WriteString("claimed plan 7\n")
	require.Error(t, w.theTakeoverIsRefusedNamingALiveSession(), "a success is not a refusal")

	cs.out.Reset()
	cs.out.WriteString("refused: already held (plan/7); seen unchanged for 1m\n")
	require.Error(t, w.theTakeoverIsRefusedNamingALiveSession(),
		"S18's wording does not name a live agent session")

	cs.out.Reset()
	cs.out.WriteString("refused: is held by a live agent session on elsewhere\n")
	assert.NoError(t, w.theTakeoverIsRefusedNamingALiveSession())
}

// TestOriginsHoldIsLeftExactlyAsItStoodRefusesAMachineItNeverMet:
// reused from the lease world's own convention — a holder the
// scenario never introduced is refused rather than read as a match.
func TestOriginsHoldIsLeftExactlyAsItStoodRefusesAMachineItNeverMet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.originsHoldIsLeftExactlyAsItStood())
}

// TestResumedBeatParentReadsTheCLIOutputFirst: a refusal or a missing
// "resumed plan" both fail before any git read runs, so a step never
// reads a resume's parent off a claim that did not resume.
func TestResumedBeatParentReadsTheCLIOutputFirst(t *testing.T) {
	w := newWorld(t)
	cs := section[cliState](w)

	cs.out.WriteString("refused: already held (plan/7); seen unchanged for 1m\n")
	_, err := w.resumedBeatParent()
	require.Error(t, err, "a refusal is not a resume")

	cs.out.Reset()
	cs.out.WriteString("claimed plan 7\n")
	_, err = w.resumedBeatParent()
	require.Error(t, err, "a fresh claim is not a resume")
}

// TestThisHostResumesFromFreshTipRefusesWithNoRawPush: the push-did-
// land assertion stands on the raw tip the commits-and-pushes step
// recorded; asked before that ran, it refuses rather than comparing
// against an empty tip.
func TestThisHostResumesFromFreshTipRefusesWithNoRawPush(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.thisHostResumesFromFreshTip())
}

// TestThisHostTakesTheLeaseOverRefusesOnASuccessTextMismatch: the
// step reads the CLI output before touching git, so a refusal (or any
// output missing "claimed plan") fails here rather than later on a
// git read of a ref nothing changed.
func TestThisHostTakesTheLeaseOverRefusesOnASuccessTextMismatch(t *testing.T) {
	w := newWorld(t)
	section[cliState](w).out.WriteString("refused: already held\n")

	require.Error(t, w.thisHostTakesTheLeaseOver(2))
}

// TestOriginsOrphanReportListsNeitherRefusesAPlanTheScenarioNeverSetUp:
// like the other CLI-driven Then steps, a mismatched plan id in the
// step text is refused before `frit orphans` ever runs.
func TestOriginsOrphanReportListsNeitherRefusesAPlanTheScenarioNeverSetUp(t *testing.T) {
	w := newWorld(t)
	w.planID = 7
	require.Error(t, w.originsOrphanReportListsNeither(8))
}

// TestLeaseIsRenewedByABeatInsteadOfSeizedRefusesTheWrongMachine: the
// quoted holder is checked against the scenario's own setup before
// any git read runs.
func TestLeaseIsRenewedByABeatInsteadOfSeizedRefusesTheWrongMachine(t *testing.T) {
	w := newWorld(t)
	w.holder = "elsewhere"

	require.Error(t, w.leaseIsRenewedByABeatInsteadOfSeized("box-a"))
}
