package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The identity and cross-layer vocabulary: a stale hold survived by an
// unreachable herdr, a lane whose own raw commits outrun its persisted
// token, a branch repurposed by hand, a holder string that lies about
// who this machine is. It registers itself, like every section's step
// file, so a section adds a file and never a line to bdd_test.go.
func init() {
	registrars = append(registrars, (*world).registerIdentityAndCrossLayer)
}

// identityAndCrossLayerState is this section's own state beside the
// shared world: the root and repository a scenario built, the lane a
// verb ran from, the tip a Given step left held, the raw or takeover
// tip a later step must be measured against, and the last verb's own
// captured output — kept here, not on world, so this section adds a
// file and never a field to it.
type identityAndCrossLayerState struct {
	root     string
	repo     string
	lane     string
	held     string
	raw      string
	takeover string
	foreign  string
	out      string
	errOut   string
	code     int
	// rec is the herdr calls a run recorded, for a row that must prove
	// what was dispatched — S45 and S73 — rather than reading the
	// verb's own text output alone.
	rec *herdrCalls
}

func (w *world) registerIdentityAndCrossLayer(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]+)" holds plan (\d+) bound to a session$`, w.holdsPlanBoundToASession)
	sc.Step(`^the window has matured for plan (\d+)$`, w.theWindowHasMaturedForPlan)
	sc.Step(`^herdr is unreachable$`, w.herdrIsUnreachable)
	sc.Step(`^"([^"]+)" claims plan (\d+) over the stale hold$`, w.machineClaimsPlan)
	sc.Step(`^the claim is refused: worktree not stood up$`, w.theClaimIsRefusedWorktreeNotStoodUp)
	sc.Step(`^a takeover at epoch 2 sits on the stale tip$`, w.takeoverAtEpoch2SitsOnTheStaleTip)
	sc.Step(`^the veto never fired$`, w.theVetoNeverFired)

	sc.Step(`^this machine holds plan (\d+) in a lane with its token persisted$`,
		w.thisMachineHoldsPlanInALaneWithItsTokenPersisted)
	sc.Step(`^two raw commits are pushed on top of the lane$`, w.twoRawCommitsArePushedOnTopOfTheLane)
	sc.Step(`^the lane runs claim for plan (\d+)$`, w.theLaneRunsClaimForPlan)
	sc.Step(`^it is resumed and the beat's parent is the raw tip$`, w.itIsResumedAndTheBeatsParentIsTheRawTip)
	sc.Step(`^a takeover at a new epoch lands on plan (\d+)$`, w.aTakeoverAtANewEpochLandsOnPlan)
	sc.Step(`^the lane runs release for plan (\d+)$`, w.theLaneRunsReleaseForPlan)
	sc.Step(`^it is refused and the takeover stands$`, w.itIsRefusedAndTheTakeoverStands)

	sc.Step(`^origin's work ref for plan (\d+) is deleted and repurposed by "([^"]+)"$`,
		w.originsWorkRefIsDeletedAndRepurposedBy)
	sc.Step(`^the lane's release is refused and origin's tip is untouched$`,
		w.theLanesReleaseIsRefusedAndOriginsTipIsUntouched)
	sc.Step(`^the lane's claim reports the plan already held, never resumed$`,
		w.theLanesClaimReportsThePlanAlreadyHeldNeverResumed)

	sc.Step(`^a held lane holding plan (\d+) whose marker names "([^"]+)" as holder and whose checkout `+
		`carries the token$`, w.aHeldLaneHoldingPlanWhoseMarkerNamesAsHolder)
	sc.Step(`^herdr shows no agent on the lane$`, w.herdrShowsNoAgentOnTheLane)
	sc.Step(`^this machine runs start --go for plan (\d+)$`, w.thisMachineRunsStartGoForPlan)
	sc.Step(`^the plan is resumed$`, w.thePlanIsResumed)
	sc.Step(`^no takeover marker sits between the held tip and origin's tip$`,
		w.noTakeoverMarkerSitsBetweenTheHeldTipAndOriginsTip)

	sc.Step(`^herdr confirms the session is live$`, w.herdrConfirmsTheSessionIsLive)
	sc.Step(`^start refuses, naming the live agent session$`, w.startRefusesNamingTheLiveAgentSession)
	sc.Step(`^the holder's own lease is renewed, not seized$`, w.theHoldersOwnLeaseIsRenewedNotSeized)

	sc.Step(`^a held lane holding plan (\d+) whose marker names this host as holder but whose checkout `+
		`carries no token$`, w.aHeldLaneNamingThisHostWithNoToken)
	sc.Step(`^start refuses: already held, not takeable until the window matures$`,
		w.startRefusesAlreadyHeldNotTakeable)
	sc.Step(`^the plan is not resumed$`, w.thePlanIsNotResumed)

	sc.Step(`^plan (\d+) is unclaimed$`, w.planIsUnclaimed)
	sc.Step(`^this machine claims plan (\d+)$`, w.thisMachineClaimsPlan)
	sc.Step(`^the lease is released, not left standing$`, w.theLeaseIsReleasedNotLeftStanding)
	sc.Step(`^herdr becomes reachable$`, w.herdrBecomesReachable)
	sc.Step(`^it claims clean at the next epoch$`, w.itClaimsCleanAtTheNextEpoch)

	sc.Step(`^the agent starts but its prompt fails$`, w.theAgentStartsButItsPromptFails)
	sc.Step(`^start fails and a release marker sits on the branch$`,
		w.startFailsAndAReleaseMarkerSitsOnTheBranch)
	sc.Step(`^the agent was started before the failure$`, w.theAgentWasStartedBeforeTheFailure)
	sc.Step(`^the worktree it stood up is torn down$`, w.theWorktreeItStoodUpIsTornDown)
}

// holdsPlanBoundToASession mints a lease bound to a session that no
// herdr fake will ever answer for — S61's Given: a hold whose window
// can mature while its own liveness question goes unanswered.
func (w *world) holdsPlanBoundToASession(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = holder
	root := w.t.TempDir()
	repo := claimableRepo(w.t, root, "atlas", planID, "Shader unit")
	w.clones[holder] = repo

	opts := leaseFor(holder, planID)
	opts.Session = "wS:p9"
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	if err != nil {
		return err
	}
	w.lease = lease
	st := section[identityAndCrossLayerState](w)
	st.root, st.held = root, lease.Tip

	return nil
}

// theWindowHasMaturedForPlan seeds an observation that has watched the
// Given step's own held tip unchanged for hours, the fixture claim's
// own stale-lease tests share.
func (w *world) theWindowHasMaturedForPlan(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.held == "" {
		return fmt.Errorf("plan %d carries no held tip to mature a window on", planID)
	}
	seedWindow(w.t, "atlas", int64(planID), st.held, 3*time.Hour)

	return nil
}

// herdrIsUnreachable installs a runner that answers every call with a
// dial error, the shape TestClaimTakesOverWhenHerdrCannotAnswer drives
// claim's live-session veto with — never a missing socket left to the
// build box's own luck.
func (w *world) herdrIsUnreachable() error {
	withHerdr(w.t, func(...string) ([]byte, error) {
		return nil, errors.New("dial unix .herdr.sock: no such file")
	})

	return nil
}

// machineClaimsPlan runs `claim` for holder against the repository the
// stale hold lives in. holder must be a machine other than the one
// already holding the plan — the same role guard bdd_lease_test.go's
// own takeover step carries — since this row is about a second machine
// racing a stale hold, not the holder claiming its own.
func (w *world) machineClaimsPlan(holder string, planID int) error {
	if holder == w.holder {
		return fmt.Errorf("%q already holds plan %d; a claim over a stale hold comes from another machine",
			holder, planID)
	}
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	root := filepath.Dir(repo)
	var out, errb strings.Builder
	code := run([]string{"claim", strconv.Itoa(planID), "--root", root}, &out, &errb)
	st := section[identityAndCrossLayerState](w)
	st.out, st.errOut, st.code = out.String(), errb.String(), code

	return nil
}

// theClaimIsRefusedWorktreeNotStoodUp checks the last claim's own
// output: an unreachable herdr cannot stand a worktree up either, so
// the takeover it minted unwinds and the run reports the failed
// stand-up, not a plain success.
func (w *world) theClaimIsRefusedWorktreeNotStoodUp() error {
	out := section[identityAndCrossLayerState](w).out
	if !strings.Contains(out, "refused: plan") {
		return fmt.Errorf("claim did not refuse: %s", out)
	}
	if !strings.Contains(out, "worktree not stood up") {
		return fmt.Errorf("claim's refusal does not name a failed stand-up: %s", out)
	}

	return nil
}

// takeoverAtEpoch2SitsOnTheStaleTip reads the branch left on origin:
// a release marker for the failed stand-up, child of a takeover minted
// at epoch 2, itself a child of exactly the stale tip the Given step
// observed — never a tip read fresh after the fact.
func (w *world) takeoverAtEpoch2SitsOnTheStaleTip() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	tip, err := gitCapture(w.t, repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	tipBody, err := gitCapture(w.t, repo, "log", "-1", "--format=%B", tip)
	if err != nil {
		return fmt.Errorf("%s: %w", tipBody, err)
	}
	if !strings.Contains(tipBody, fmt.Sprintf("plan %d: release", w.planID)) {
		return fmt.Errorf("the branch tip is not the failed stand-up's release marker: %q", tipBody)
	}
	takeover, err := gitCapture(w.t, repo, "rev-parse", tip+"^")
	if err != nil {
		return fmt.Errorf("%s: %w", takeover, err)
	}
	takeoverBody, err := gitCapture(w.t, repo, "log", "-1", "--format=%B", takeover)
	if err != nil {
		return fmt.Errorf("%s: %w", takeoverBody, err)
	}
	if !strings.Contains(takeoverBody, fmt.Sprintf("plan %d: takeover", w.planID)) ||
		!strings.Contains(takeoverBody, "epoch:   2") {
		return fmt.Errorf("the release does not sit on an epoch-2 takeover: %q", takeoverBody)
	}
	parent, err := gitCapture(w.t, repo, "rev-parse", takeover+"^")
	if err != nil {
		return fmt.Errorf("%s: %w", parent, err)
	}
	if parent != w.lease.Tip {
		return fmt.Errorf("the takeover's parent is %s, want the observed stale tip %s", parent, w.lease.Tip)
	}

	return nil
}

// theVetoNeverFired checks the last claim's output never carries the
// live-session veto's own wording: an unreachable herdr proves no
// liveness, so a takeover proceeds on the window alone rather than
// being blocked by a check that could not run.
func (w *world) theVetoNeverFired() error {
	out := section[identityAndCrossLayerState](w).out
	if strings.Contains(out, "is held by a live agent session") {
		return fmt.Errorf("the veto fired even though herdr could not answer: %s", out)
	}

	return nil
}

// thisMachineHoldsPlanInALaneWithItsTokenPersisted mints a lease into
// a real worktree under this machine's own hostname and renews it from
// inside, the state a phase's own lane is in mid-work: its token,
// written by that renewal, matches exactly the tip origin holds. S64
// and S86 share it, since both start from the same live lane.
func (w *world) thisMachineHoldsPlanInALaneWithItsTokenPersisted(planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = hostname()
	root := w.t.TempDir()
	repo := claimableRepo(w.t, root, "atlas", planID, "Shader unit")
	w.clones[w.holder] = repo

	lane := filepath.Join(w.t.TempDir(), "atlas-lane")
	opts := leaseFor(w.holder, planID)
	opts.Lane = lane
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	if err != nil {
		return err
	}
	git(w.t, repo, "worktree", "add", "-q", lane, claim.Branch(int64(planID)))
	renewed, err := claim.Renew(repo, opts, lease.Tip, gitwt.Exec)
	if err != nil {
		return err
	}
	w.lease = renewed
	st := section[identityAndCrossLayerState](w)
	st.root, st.repo, st.lane = root, repo, lane

	return nil
}

// twoRawCommitsArePushedOnTopOfTheLane pushes the prescribed TDD
// workflow's own shape — plain git commit and push, no frit transition
// between — leaving origin's tip a descendant of the lane's persisted
// token that no verb ever CASed from.
func (w *world) twoRawCommitsArePushedOnTopOfTheLane() error {
	st := section[identityAndCrossLayerState](w)
	if st.lane == "" {
		return fmt.Errorf("no lane to push raw commits on; the token step comes first")
	}
	git(w.t, st.lane, "commit", "--allow-empty", "-q", "-m", "red: add failing test")
	git(w.t, st.lane, "commit", "--allow-empty", "-q", "-m", "green: make it pass")
	if out, err := gitCapture(w.t, st.lane, "push", "-q", "origin", claim.Branch(int64(w.planID))); err != nil {
		return fmt.Errorf("push raw commits: %s: %w", out, err)
	}
	raw, err := gitCapture(w.t, st.lane, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("%s: %w", raw, err)
	}
	st.raw = raw

	return nil
}

// theLaneRunsClaimForPlan runs `claim` with the calling directory
// standing in the lane a prior step stood up — the vantage point
// `ownToken`'s `inOwnLane` check requires.
func (w *world) theLaneRunsClaimForPlan(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.lane == "" {
		return fmt.Errorf("no lane to run claim from; the token step comes first")
	}
	w.t.Chdir(st.lane)
	var out, errb strings.Builder
	code := run([]string{"claim", strconv.Itoa(planID), "--root", st.root}, &out, &errb)
	st.out, st.errOut, st.code = out.String(), errb.String(), code

	return nil
}

// itIsResumedAndTheBeatsParentIsTheRawTip checks the claim resumed —
// never refused — and that the beat it minted is a direct child of the
// raw tip the lane's own commits reached, not the stale persisted
// token: an ordinary run of raw commits is this lane's own advance.
func (w *world) itIsResumedAndTheBeatsParentIsTheRawTip() error {
	st := section[identityAndCrossLayerState](w)
	if strings.Contains(st.out, "refused") || !strings.Contains(st.out, "resumed plan") {
		return fmt.Errorf("claim did not resume: %s", st.out)
	}
	tip, err := gitCapture(w.t, st.repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	body, err := gitCapture(w.t, st.repo, "log", "-1", "--format=%B", tip)
	if err != nil {
		return fmt.Errorf("%s: %w", body, err)
	}
	if !strings.Contains(body, fmt.Sprintf("plan %d: beat", w.planID)) {
		return fmt.Errorf("the resumed tip is not a beat: %q", body)
	}
	parent, err := gitCapture(w.t, st.repo, "rev-parse", tip+"^")
	if err != nil {
		return fmt.Errorf("%s: %w", parent, err)
	}
	if parent != st.raw {
		return fmt.Errorf("the beat's parent is %s, want the raw tip %s", parent, st.raw)
	}

	return nil
}

// aTakeoverAtANewEpochLandsOnPlan takes the lease over from whatever
// tip origin holds right now — the resumed lane's own beat — as a
// foreign move, exactly as a genuine takeover after a renewal would.
func (w *world) aTakeoverAtANewEpochLandsOnPlan(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.repo == "" {
		return fmt.Errorf("no repo to take the lease over on; the token step comes first")
	}
	from := claim.RemoteTip(st.repo, "origin", int64(planID), gitwt.Exec)
	if from == "" {
		return fmt.Errorf("origin carries no work ref for plan %d to take over", planID)
	}
	taken, err := claim.Takeover(st.repo, leaseFor("elsewhere", planID), from, gitwt.Exec)
	if err != nil {
		return err
	}
	st.takeover = taken.Tip

	return nil
}

// theLaneRunsReleaseForPlan runs `release` with the calling directory
// still standing in the lane, leaving the result for the following Then
// to read back.
func (w *world) theLaneRunsReleaseForPlan(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.lane == "" {
		return fmt.Errorf("no lane to run release from; the token step comes first")
	}
	w.t.Chdir(st.lane)
	var out, errb strings.Builder
	code := run([]string{"release", strconv.Itoa(planID), "--root", st.root}, &out, &errb)
	st.out, st.errOut, st.code = out.String(), errb.String(), code

	return nil
}

// itIsRefusedAndTheTakeoverStands checks the release refused and that
// origin's tip is still exactly the takeover's own — the Phase 1
// relaxation that recognizes a lane's raw commits as its own advance
// must never widen into recognizing a genuine foreign takeover too.
func (w *world) itIsRefusedAndTheTakeoverStands() error {
	st := section[identityAndCrossLayerState](w)
	if !strings.Contains(st.out, "refused") {
		return fmt.Errorf("release did not refuse: %s", st.out)
	}
	if st.takeover == "" {
		return fmt.Errorf("no takeover landed to check origin's tip against")
	}
	tip, err := gitCapture(w.t, st.repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	if tip != st.takeover {
		return fmt.Errorf("origin holds %s, want the takeover %s left untouched", tip, st.takeover)
	}

	return nil
}

// originsWorkRefIsDeletedAndRepurposedBy deletes the plan's work ref
// on origin and mints a wholly fresh lease over the gap, from a second
// clone, as holder — a branch moved by hand no longer descends from
// the lane's own token at all. holder must be a machine other than the
// one already holding the plan, the same role guard machineClaimsPlan
// carries.
func (w *world) originsWorkRefIsDeletedAndRepurposedBy(planID int, holder string) error {
	st := section[identityAndCrossLayerState](w)
	if st.repo == "" {
		return fmt.Errorf("no repo whose work ref to repurpose; the token step comes first")
	}
	if holder == w.holder {
		return fmt.Errorf("%q already holds plan %d; a repurpose comes from another machine", holder, planID)
	}
	if out, err := gitCapture(w.t, st.repo, "push", "origin", "--delete", claim.Branch(int64(planID))); err != nil {
		return fmt.Errorf("delete origin's work ref: %s: %w", out, err)
	}
	second := cloneAgain(w.t, st.repo)
	lease, err := claim.Acquire(second, leaseFor(holder, planID), gitwt.Exec)
	if err != nil {
		return err
	}
	st.foreign = lease.Tip

	return nil
}

// theLanesReleaseIsRefusedAndOriginsTipIsUntouched runs `release` from
// the lane whose token no longer proves anything against the
// repurposed branch, and checks it changed nothing: the fresh lease
// minted over the gap is not this lane's own advance, so release must
// leave it exactly where the repurpose left it.
func (w *world) theLanesReleaseIsRefusedAndOriginsTipIsUntouched() error {
	st := section[identityAndCrossLayerState](w)
	if st.lane == "" {
		return fmt.Errorf("no lane to run release from; the token step comes first")
	}
	w.t.Chdir(st.lane)
	var out, errb strings.Builder
	code := run([]string{"release", strconv.Itoa(w.planID), "--root", st.root}, &out, &errb)
	if code != 0 {
		return fmt.Errorf("release exited %d: %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "refused") {
		return fmt.Errorf("release did not refuse a repurposed branch: %s", got)
	}
	// origin's ref is read fresh, via ls-remote, rather than st.repo's own
	// local branch: the repurpose minted its fresh lease from a second
	// clone, so st.repo's local ref never moved and would misreport a
	// touch that never happened.
	if got := claim.RemoteTip(st.repo, "origin", int64(w.planID), gitwt.Exec); got != st.foreign {
		return fmt.Errorf("origin's tip is %s, want the repurposed %s left untouched", got, st.foreign)
	}

	return nil
}

// theLanesClaimReportsThePlanAlreadyHeldNeverResumed runs `claim` from
// the same lane and checks it takes the ordinary "already held" door,
// never the resume door a matching token would open: the token this
// lane persisted proves nothing about the branch as it stands now.
func (w *world) theLanesClaimReportsThePlanAlreadyHeldNeverResumed() error {
	st := section[identityAndCrossLayerState](w)
	if st.lane == "" {
		return fmt.Errorf("no lane to run claim from; the token step comes first")
	}
	w.t.Chdir(st.lane)
	var out, errb strings.Builder
	code := run([]string{"claim", strconv.Itoa(w.planID), "--root", st.root}, &out, &errb)
	if code != 0 {
		return fmt.Errorf("claim exited %d: %s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "already held") {
		return fmt.Errorf("claim did not report the plan already held: %s", got)
	}
	if strings.Contains(got, "resumed") {
		return fmt.Errorf("claim reported a resume over a repurposed branch: %s", got)
	}

	return nil
}

// aHeldLaneHoldingPlanWhoseMarkerNamesAsHolder builds a held lane whose
// marker names holder — never this host's own hostname — and whose
// checkout carries the token that holder's own renewal persisted: S48's
// identity convention, that the token proves the lease and the holder
// trailer is only ever reporting.
func (w *world) aHeldLaneHoldingPlanWhoseMarkerNamesAsHolder(planID int, holder string) error {
	isolate(w.t)
	w.planID = planID
	w.holder = holder
	root := w.t.TempDir()
	repo, lane, held := heldLaneOwnedBy(w.t, root, holder, "wOld:p1")
	w.clones[holder] = repo
	st := section[identityAndCrossLayerState](w)
	st.root, st.repo, st.lane, st.held = root, repo, lane, held

	return nil
}

// herdrShowsNoAgentOnTheLane installs the herdr fake `start`'s own
// handshake needs: reachable, but naming nobody on the session the
// held lane's marker bound, so no live agent stands between a resume
// and the plan.
func (w *world) herdrShowsNoAgentOnTheLane() error {
	runner, _ := startHerdr()
	withHerdr(w.t, runner)

	return nil
}

// thisMachineRunsStartGoForPlan runs `start --go` outside the lane —
// this host, resolving whatever the held lane's own marker says,
// whoever it names as holder.
func (w *world) thisMachineRunsStartGoForPlan(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.root == "" {
		return fmt.Errorf("no root to run start from; the held-lane step comes first")
	}
	var out, errb strings.Builder
	code := run([]string{"start", strconv.Itoa(planID), "--phase", "3", "--go", "--root", st.root}, &out, &errb)
	st.out, st.errOut, st.code = out.String(), errb.String(), code

	return nil
}

// thePlanIsResumed checks the last verb's own output claims a resume —
// the one shape a proof through the token produces, whatever the
// holder trailer says.
func (w *world) thePlanIsResumed() error {
	if !strings.Contains(section[identityAndCrossLayerState](w).out, "resumed plan") {
		return fmt.Errorf("the run did not resume: %s", section[identityAndCrossLayerState](w).out)
	}

	return nil
}

// noTakeoverMarkerSitsBetweenTheHeldTipAndOriginsTip reads origin's
// commit log between the Given step's own held tip and the current
// tip, and refuses a "takeover" subject line anywhere in it: a lease
// this machine can prove through its token is resumed, never seized.
func (w *world) noTakeoverMarkerSitsBetweenTheHeldTipAndOriginsTip() error {
	st := section[identityAndCrossLayerState](w)
	if st.held == "" {
		return fmt.Errorf("no held tip recorded; the held-lane step comes first")
	}
	tip := remoteWorkTip(w.t, st.repo)
	body, err := gitCapture(w.t, st.repo, "log", "--format=%s", tip, "^"+st.held)
	if err != nil {
		return fmt.Errorf("%s: %w", body, err)
	}
	if strings.Contains(body, "takeover") {
		return fmt.Errorf("a takeover marker sits between %s and %s: %q", st.held, tip, body)
	}

	return nil
}

// herdrConfirmsTheSessionIsLive installs a herdr fake naming the Given
// step's own session as a live agent — the positive answer
// herdr.SessionLive needs to veto a takeover, whatever the caller's own
// cwd happens to be.
func (w *world) herdrConfirmsTheSessionIsLive() error {
	withHerdr(w.t, herdrReturning(map[string]any{
		"agent":        "claude",
		"agent_status": "working",
		"pane_id":      "wS:p9",
		"agent_session": map[string]any{
			"value": "wS:p9",
		},
	}))

	return nil
}

// startRefusesNamingTheLiveAgentSession checks the last run's own
// output refuses and names the live-session veto's own wording, so a
// refusal for an unrelated reason does not pass this Then by accident.
func (w *world) startRefusesNamingTheLiveAgentSession() error {
	out := section[identityAndCrossLayerState](w).out
	if !strings.Contains(out, "refused") {
		return fmt.Errorf("start did not refuse: %s", out)
	}
	if !strings.Contains(out, "live agent session") {
		return fmt.Errorf("the refusal does not name the live agent session: %s", out)
	}

	return nil
}

// theHoldersOwnLeaseIsRenewedNotSeized reads origin's tip after a
// vetoed takeover and checks it is a beat CASed straight from the
// holder's own held tip, naming the holder — a renewal, never a
// takeover marker bumping the epoch.
func (w *world) theHoldersOwnLeaseIsRenewedNotSeized() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	tip, err := gitCapture(w.t, repo, "rev-parse", w.branch())
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
	if !strings.Contains(body, "holder:  "+w.holder) {
		return fmt.Errorf("the beat does not renew %q's own lease: %q", w.holder, body)
	}
	parent, err := gitCapture(w.t, repo, "rev-parse", tip+"^")
	if err != nil {
		return fmt.Errorf("%s: %w", parent, err)
	}
	if parent != w.lease.Tip {
		return fmt.Errorf("the beat's parent is %s, want the held tip %s", parent, w.lease.Tip)
	}

	return nil
}

// aHeldLaneNamingThisHostWithNoToken builds a held lane whose marker
// names this very host as holder, exactly as a hostname change would,
// but strips the token its own renewal persisted — the shape a cloned
// machine-id or a reused path leaves: an equal holder string with
// nothing behind it. S49's own fixture, S48's photographic negative.
func (w *world) aHeldLaneNamingThisHostWithNoToken(planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = hostname()
	root := w.t.TempDir()
	repo, lane, held := heldLaneOwnedBy(w.t, root, w.holder, "")
	dropToken(w.t, lane)
	w.clones[w.holder] = repo
	st := section[identityAndCrossLayerState](w)
	st.root, st.repo, st.lane, st.held = root, repo, lane, held

	return nil
}

// startRefusesAlreadyHeldNotTakeable checks the last run's own output
// refuses on the ordinary already-held door — never a resume, since an
// equal holder string with no token proves nothing — and that origin's
// tip sits exactly where the Given step left it.
func (w *world) startRefusesAlreadyHeldNotTakeable() error {
	st := section[identityAndCrossLayerState](w)
	if !strings.Contains(st.out, "refused") {
		return fmt.Errorf("start did not refuse: %s", st.out)
	}
	if !strings.Contains(st.out, "already held") {
		return fmt.Errorf("the refusal does not say already held: %s", st.out)
	}
	if !strings.Contains(st.out, "not takeable until the window matures") {
		return fmt.Errorf("the refusal does not name the window: %s", st.out)
	}
	if got := remoteWorkTip(w.t, st.repo); got != st.held {
		return fmt.Errorf("origin's tip is %s, want the untouched hold %s", got, st.held)
	}

	return nil
}

// thePlanIsNotResumed checks the last run's own output never claims
// the one shape a proven token produces — the mirror of
// thePlanIsResumed, for the row where the token is exactly what is
// missing.
func (w *world) thePlanIsNotResumed() error {
	if strings.Contains(section[identityAndCrossLayerState](w).out, "resumed plan") {
		return fmt.Errorf("the run resumed a lease it could not prove: %s",
			section[identityAndCrossLayerState](w).out)
	}

	return nil
}

// planIsUnclaimed builds a fresh, wholly unheld plan — S60 and S73's
// shared Given, since both rows turn on herdr trouble during a lane's
// very first stand-up, never on a prior hold.
func (w *world) planIsUnclaimed(planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = hostname()
	root := w.t.TempDir()
	repo := claimableRepo(w.t, root, "atlas", planID, "Shader unit")
	w.clones[w.holder] = repo
	st := section[identityAndCrossLayerState](w)
	st.root, st.repo = root, repo

	return nil
}

// thisMachineClaimsPlan runs `claim` against the unclaimed plan's own
// repository — S60's own When, run twice across the scenario as herdr
// goes from unreachable to answering.
func (w *world) thisMachineClaimsPlan(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.root == "" {
		return fmt.Errorf("no root to claim from; the unclaimed-plan step comes first")
	}
	var out, errb strings.Builder
	code := run([]string{"claim", strconv.Itoa(planID), "--root", st.root}, &out, &errb)
	st.out, st.errOut, st.code = out.String(), errb.String(), code

	return nil
}

// theLeaseIsReleasedNotLeftStanding checks the branch a failed
// stand-up leaves behind: the ref still exists, its tip a release
// marker, never a delete and never a claim left standing with no lane
// behind it.
func (w *world) theLeaseIsReleasedNotLeftStanding() error {
	st := section[identityAndCrossLayerState](w)
	tip, err := gitCapture(w.t, st.repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	body, err := gitCapture(w.t, st.repo, "log", "-1", "--format=%B", tip)
	if err != nil {
		return fmt.Errorf("%s: %w", body, err)
	}
	if !strings.Contains(body, fmt.Sprintf("plan %d: release", w.planID)) {
		return fmt.Errorf("the branch tip is not a release marker: %q", body)
	}

	return nil
}

// herdrBecomesReachable installs a fully working herdr handshake in
// place of whatever fake an earlier step left, the same fixture a
// fresh stand-up uses everywhere else in this file.
func (w *world) herdrBecomesReachable() error {
	runner, _ := startHerdr()
	withHerdr(w.t, runner)

	return nil
}

// itClaimsCleanAtTheNextEpoch checks the second claim in S60's own
// scenario actually claimed, at the epoch right after the released
// attempt — no takeover window waited, since a release ends a hold
// rather than merely abandoning it.
func (w *world) itClaimsCleanAtTheNextEpoch() error {
	st := section[identityAndCrossLayerState](w)
	if !strings.Contains(st.out, fmt.Sprintf("claimed plan %d", w.planID)) {
		return fmt.Errorf("claim did not succeed cleanly: %s", st.out)
	}
	tip, err := gitCapture(w.t, st.repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	body, err := gitCapture(w.t, st.repo, "log", "-1", "--format=%B", tip)
	if err != nil {
		return fmt.Errorf("%s: %w", body, err)
	}
	if !strings.Contains(body, "epoch:   2") {
		return fmt.Errorf("the claim is not at the next epoch: %q", body)
	}

	return nil
}

// theAgentStartsButItsPromptFails installs a herdr fake that answers
// worktree.create and agent.start normally — the agent really starts —
// but fails the prompt call after it, recording every call so the
// Then steps can prove the agent was dispatched before the failure.
func (w *world) theAgentStartsButItsPromptFails() error {
	rec := &herdrCalls{}
	withHerdr(w.t, func(args ...string) ([]byte, error) {
		rec.mu.Lock()
		rec.calls = append(rec.calls, append([]string(nil), args...))
		rec.mu.Unlock()
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
		}
		if len(args) >= 2 && args[0] == "agent" && args[1] == "prompt" {
			return nil, errors.New("agent target pane wZ:p1 refused the prompt")
		}

		return nil, nil
	})
	section[identityAndCrossLayerState](w).rec = rec

	return nil
}

// startFailsAndAReleaseMarkerSitsOnTheBranch checks the run exited
// non-zero and that the branch it minted a claim on carries a release
// marker at its tip — the failed handoff's own unwind, never a claim
// left standing over a dead pane.
func (w *world) startFailsAndAReleaseMarkerSitsOnTheBranch() error {
	st := section[identityAndCrossLayerState](w)
	if st.code == 0 {
		return fmt.Errorf("start did not fail: %s", st.out)
	}
	body, err := gitCapture(w.t, st.repo, "log", "-1", "--format=%B", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", body, err)
	}
	if !strings.Contains(body, fmt.Sprintf("plan %d: release", w.planID)) {
		return fmt.Errorf("the branch tip is not a release marker: %q", body)
	}

	return nil
}

// theAgentWasStartedBeforeTheFailure reads the recorded herdr calls
// back and refuses unless agent.start actually ran — the row's own
// point, that the agent was dispatched before its prompt failed, not
// merely that the run exited non-zero.
func (w *world) theAgentWasStartedBeforeTheFailure() error {
	st := section[identityAndCrossLayerState](w)
	if st.rec == nil {
		return fmt.Errorf("no herdr calls recorded; the prompt-fails step comes first")
	}
	if !st.rec.verb("agent", "start") {
		return fmt.Errorf("the agent was never started")
	}

	return nil
}

// theWorktreeItStoodUpIsTornDown reads the recorded herdr calls back
// and refuses unless the failed handoff's own unwind removed the
// worktree it stood up — the abort is atomic, not a freed claim left
// over a live checkout.
func (w *world) theWorktreeItStoodUpIsTornDown() error {
	st := section[identityAndCrossLayerState](w)
	if st.rec == nil {
		return fmt.Errorf("no herdr calls recorded; the prompt-fails step comes first")
	}
	if !st.rec.verb("worktree", "remove") {
		return fmt.Errorf("the worktree stood up for the lane was never torn down")
	}

	return nil
}

// TestIdentityAndCrossLayerStepsRefuseAMachineTheyNeverMet: the two
// steps that name a second machine by role — the claimant racing a
// stale hold, the machine repurposing a branch by hand — refuse when
// the name given is the very machine already holding the plan, the
// same guard bdd_lease_test.go's own takeover step carries.
func TestIdentityAndCrossLayerStepsRefuseAMachineTheyNeverMet(t *testing.T) {
	w := newWorld(t)
	w.holder = "elsewhere"
	w.clones["elsewhere"] = t.TempDir()

	require.Error(t, w.machineClaimsPlan("elsewhere", 7), "the stale holder cannot claim its own hold")
	require.Error(t, w.originsWorkRefIsDeletedAndRepurposedBy(7, "elsewhere"),
		"the current holder cannot repurpose its own branch")
}

// TestTheHoldersOwnLeaseIsRenewedNotSeizedRefusesAMachineTheScenarioNeverMet:
// the renewal read-back names the holder the Given step introduced, so a
// scenario that never set one up finds no clone to read from.
func TestTheHoldersOwnLeaseIsRenewedNotSeizedRefusesAMachineTheScenarioNeverMet(t *testing.T) {
	w := newWorld(t)
	w.holder = "elsewhere"

	err := w.theHoldersOwnLeaseIsRenewedNotSeized()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no machine")
}

// TestIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition: every
// step that reads state an earlier step recorded refuses when that
// step never ran, rather than reading a zero value as if it were real.
func TestIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition(t *testing.T) {
	w := newWorld(t)

	err := w.theWindowHasMaturedForPlan(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no held tip")

	err = w.twoRawCommitsArePushedOnTopOfTheLane()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no lane")

	err = w.theLaneRunsClaimForPlan(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no lane")

	err = w.aTakeoverAtANewEpochLandsOnPlan(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no repo")

	err = w.theLaneRunsReleaseForPlan(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no lane")

	err = w.theLanesReleaseIsRefusedAndOriginsTipIsUntouched()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no lane")

	err = w.theLanesClaimReportsThePlanAlreadyHeldNeverResumed()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no lane")

	err = w.thisMachineRunsStartGoForPlan(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no root")

	err = w.noTakeoverMarkerSitsBetweenTheHeldTipAndOriginsTip()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no held tip")

	err = w.itIsRefusedAndTheTakeoverStands()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not refuse")

	err = w.thisMachineClaimsPlan(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no root")

	err = w.theAgentWasStartedBeforeTheFailure()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no herdr calls")

	err = w.theWorktreeItStoodUpIsTornDown()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no herdr calls")
}

// TestIdentityAndCrossLayerReadBacksWantTheirExactShape: each
// assertion step that reads the last verb's own captured output
// refuses on the wrong shape rather than passing on an unrelated
// success or a refusal for the wrong reason.
func TestIdentityAndCrossLayerReadBacksWantTheirExactShape(t *testing.T) {
	w := newWorld(t)
	st := section[identityAndCrossLayerState](w)

	st.out = "claimed plan 7"
	require.Error(t, w.theClaimIsRefusedWorktreeNotStoodUp(), "no refusal at all")
	st.out = "refused: plan 7 is held by a live agent session on box-a"
	require.Error(t, w.theClaimIsRefusedWorktreeNotStoodUp(), "refused, but not for a failed stand-up")
	st.out = "refused: plan 7 could not be resumed: worktree not stood up: dial error"
	assert.NoError(t, w.theClaimIsRefusedWorktreeNotStoodUp())

	st.out = "refused: plan 7 is held by a live agent session on box-a"
	require.Error(t, w.theVetoNeverFired(), "the veto's own wording is right there in the output")
	st.out = "refused: plan 7 could not be resumed: worktree not stood up"
	assert.NoError(t, w.theVetoNeverFired())

	st.out = "refused: plan 7 already held"
	require.Error(t, w.itIsResumedAndTheBeatsParentIsTheRawTip(), "a refusal is not a resume")

	st.out = "refused: plan 7 already held"
	require.Error(t, w.thePlanIsResumed())
	st.out = "resumed plan 7 — Shader unit"
	assert.NoError(t, w.thePlanIsResumed())

	st.out = "started plan 7"
	require.Error(t, w.startRefusesNamingTheLiveAgentSession(), "no refusal at all")
	st.out = "refused: plan 7 already held"
	require.Error(t, w.startRefusesNamingTheLiveAgentSession(), "refused, but not for a live session")
	st.out = "refused: plan 7 is held by a live agent session on box-a"
	assert.NoError(t, w.startRefusesNamingTheLiveAgentSession())

	st.out = "resumed plan 7 — Shader unit"
	require.Error(t, w.startRefusesAlreadyHeldNotTakeable(), "no refusal at all")
	st.out = "refused: plan 7 is held by a live agent session on box-a"
	require.Error(t, w.startRefusesAlreadyHeldNotTakeable(), "refused, but not on the already-held door")

	st.out = "refused: plan 7 already held"
	assert.NoError(t, w.thePlanIsNotResumed(), "a bare refusal never claims a resume")
	st.out = "resumed plan 7 — Shader unit"
	require.Error(t, w.thePlanIsNotResumed(), "a resume is exactly what this row refuses")

	st.out = "refused: plan 7 already held"
	require.Error(t, w.itClaimsCleanAtTheNextEpoch(), "a refusal is not a clean claim")

	st.code = 0
	require.Error(t, w.startFailsAndAReleaseMarkerSitsOnTheBranch(), "an exit code of 0 is not a failure")
}
