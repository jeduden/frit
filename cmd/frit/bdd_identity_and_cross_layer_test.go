package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	registrars = append(registrars, (*world).registerVerbLevelIdentityAndCrossLayer)
	registrars = append(registrars, (*world).registerObservationAndBoundaryIdentityAndCrossLayer)
	registrars = append(registrars, (*world).registerRaceAndMultiRepoIdentityAndCrossLayer)
	registrars = append(registrars, (*world).registerPickWalkIdentityAndCrossLayer)
}

// raceResult is one contender's own captured run — a claim or a start
// invocation's exit code and output — kept in a pair on
// identityAndCrossLayerState rather than its single out/errOut/code,
// since S72's two racing verbs and S74's two repositories both need
// their results read back side by side.
type raceResult struct {
	out  string
	code int
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
	// repoA and repoB are S74's own two repositories sharing one plan
	// id, kept as a pair since the section's single repo field cannot
	// hold both.
	repoA, repoB string
	// raceA and raceB are S72's two contending verbs' own captured
	// results, and S74's two repositories' own claims.
	raceA, raceB raceResult
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
}

// registerVerbLevelIdentityAndCrossLayer registers the rows that run
// over start, claim, release and yield rather than the lease API
// directly — Phase 2 and Phase 3's own steps, split from
// registerIdentityAndCrossLayer's Phase 1 rows so neither function
// trips golangci-lint's funlen.
func (w *world) registerVerbLevelIdentityAndCrossLayer(sc *godog.ScenarioContext) {
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

	sc.Step(`^"([^"]+)" holds plan (\d+) with its lane's token persisted$`,
		w.holdsPlanWithItsLanesTokenPersisted)
	sc.Step(`^this machine runs claim for plan (\d+) from an unrelated directory$`,
		w.thisMachineRunsClaimForPlanFromAnUnrelatedDirectory)
	sc.Step(`^claim refuses: already held$`, w.claimRefusesAlreadyHeld)
	sc.Step(`^the plan (\d+) ref is unchanged$`, w.thePlanRefIsUnchanged)

	sc.Step(`^the agent fails to start and its own teardown leaves debris behind$`,
		w.theAgentFailsToStartAndItsOwnTeardownLeavesDebrisBehind)
	sc.Step(`^the error names the worktree and pane left behind$`,
		w.theErrorNamesTheWorktreeAndPaneLeftBehind)

	sc.Step(`^a held lane holding plan (\d+) whose marker names "([^"]+)" as holder and names no session$`,
		w.aHeldLaneHoldingPlanWhoseMarkerNamesAsHolderAndNamesNoSession)

	sc.Step(`^this machine holds plan (\d+) in a lane bound to a session, with its token persisted$`,
		w.thisMachineHoldsPlanInALaneBoundToASessionWithItsTokenPersisted)
	sc.Step(`^a takeover bound to a session at a new epoch lands on plan (\d+)$`,
		w.aTakeoverBoundToASessionAtANewEpochLandsOnPlan)
	sc.Step(`^the lane runs start --go for plan (\d+)$`, w.theLaneRunsStartGoForPlan)
	sc.Step(`^start refuses and names yield$`, w.startRefusesAndNamesYield)
}

// registerObservationAndBoundaryIdentityAndCrossLayer registers
// Phase 4's own six steps: the window-reset row (S62), the
// fenced-release row that proves liveness never rescues a lane a CAS
// has already lost (S63), the reachable-but-empty herdr row (S65),
// and the doc-boundary row that asserts a lane's marker and token
// carry no host at all (S66) — split from
// registerVerbLevelIdentityAndCrossLayer so neither trips
// golangci-lint's funlen.
func (w *world) registerObservationAndBoundaryIdentityAndCrossLayer(sc *godog.ScenarioContext) {
	sc.Step(`^the holder pushes a raw commit on top of the held tip$`,
		w.theHolderPushesARawCommitOnTopOfTheHeldTip)
	sc.Step(`^the refusal names the window not yet matured$`, w.theRefusalNamesTheWindowNotYetMatured)

	sc.Step(`^herdr confirms the lane's own session is live$`, w.herdrConfirmsTheLanesOwnSessionIsLive)

	sc.Step(`^it takes over cleanly at the next epoch$`, w.itTakesOverCleanlyAtTheNextEpoch)

	sc.Step(`^the marker's lane trailer is a bare path naming no host$`,
		w.theMarkersLaneTrailerIsABarePathNamingNoHost)
	sc.Step(`^the lane's token lives inside that path's git directory$`,
		w.theLanesTokenLivesInsideThatPathsGitDirectory)
}

// registerRaceAndMultiRepoIdentityAndCrossLayer registers Phase 5's
// own five steps: a genuine two-goroutine contention for one plan's
// lease (S72) and two repositories sharing one plan id (S74) — split
// from registerObservationAndBoundaryIdentityAndCrossLayer so neither
// trips golangci-lint's funlen.
func (w *world) registerRaceAndMultiRepoIdentityAndCrossLayer(sc *godog.ScenarioContext) {
	sc.Step(`^claim and start both race to mint plan (\d+)$`, w.claimAndStartBothRaceToMintPlan)
	sc.Step(`^one wins and the loser's refusal names the winning lane$`,
		w.oneWinsAndTheLosersRefusalNamesTheWinningLane)

	sc.Step(`^plan (\d+) is unclaimed in "([^"]+)" and in "([^"]+)"$`, w.planIsUnclaimedInTwoRepos)
	sc.Step(`^this machine claims plan (\d+) in "([^"]+)" and in "([^"]+)"$`,
		w.thisMachineClaimsPlanInTwoRepos)
	sc.Step(`^both are claimed with no collision, and the lanes and panes carry the repo$`,
		w.bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo)
}

// registerPickWalkIdentityAndCrossLayer is S88's own five steps, split
// out so neither this file's other registrars nor this one trips
// golangci-lint's funlen.
func (w *world) registerPickWalkIdentityAndCrossLayer(sc *godog.ScenarioContext) {
	sc.Step(`^plan (\d+)'s hold branch already carries a live herdr pane$`,
		w.plansHoldBranchAlreadyCarriesALiveHerdrPane)
	sc.Step(`^plan (\d+) is ready and held by nobody$`, w.planIsReadyAndHeldByNobody)
	sc.Step(`^pick --go runs$`, w.pickGoRuns)
	sc.Step(`^plan (\d+) is the one started$`, w.planIsTheOneStarted)
	sc.Step(`^plan (\d+) is not refused on$`, w.planIsNotRefusedOn)
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

// refuseOwnMachine guards a step that names a second machine by
// role — the claimant racing a stale hold, the machine repurposing a
// branch by hand — against naming the very machine already holding
// the plan: bdd_lease_test.go's own takeover step carries the same
// guard, since a scenario's stale holder can never play the second
// machine's role against its own hold.
func (w *world) refuseOwnMachine(holder string, planID int, action string) error {
	if holder == w.holder {
		return fmt.Errorf("%q already holds plan %d; %s", holder, planID, action)
	}

	return nil
}

// machineClaimsPlan runs `claim` for holder against the repository the
// stale hold lives in. holder must be a machine other than the one
// already holding the plan — the same role guard bdd_lease_test.go's
// own takeover step carries — since this row is about a second machine
// racing a stale hold, not the holder claiming its own.
func (w *world) machineClaimsPlan(holder string, planID int) error {
	if err := w.refuseOwnMachine(holder, planID, "a claim over a stale hold comes from another machine"); err != nil {
		return err
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
	return w.buildLiveLane(planID, "")
}

// buildLiveLane is thisMachineHoldsPlanInALaneWithItsTokenPersisted
// and thisMachineHoldsPlanInALaneBoundToASessionWithItsTokenPersisted's
// shared fixture: mint a lease into a real worktree under this
// machine's own hostname, bound to session (empty for S64 and S86's
// own unbound fixture), and renew it from inside.
func (w *world) buildLiveLane(planID int, session string) error {
	isolate(w.t)
	w.planID = planID
	w.holder = hostname()
	root := w.t.TempDir()
	repo := claimableRepo(w.t, root, "atlas", planID, "Shader unit")
	w.clones[w.holder] = repo

	lane := filepath.Join(w.t.TempDir(), "atlas-lane")
	opts := leaseFor(w.holder, planID)
	opts.Lane = lane
	opts.Session = session
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
	return w.landTakeover(planID, "")
}

// landTakeover is aTakeoverAtANewEpochLandsOnPlan and
// aTakeoverBoundToASessionAtANewEpochLandsOnPlan's shared move: take
// the lease over from whatever tip origin holds right now — the
// resumed lane's own beat — as a foreign move, exactly as a genuine
// takeover after a renewal would, bound to session (empty for S86's
// own unbound fixture).
func (w *world) landTakeover(planID int, session string) error {
	st := section[identityAndCrossLayerState](w)
	if st.repo == "" {
		return fmt.Errorf("no repo to take the lease over on; the token step comes first")
	}
	from := claim.RemoteTip(st.repo, "origin", int64(planID), gitwt.Exec)
	if from == "" {
		return fmt.Errorf("origin carries no work ref for plan %d to take over", planID)
	}
	opts := leaseFor("elsewhere", planID)
	opts.Session = session
	taken, err := claim.Takeover(st.repo, opts, from, gitwt.Exec)
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
	if err := w.refuseOwnMachine(holder, planID, "a repurpose comes from another machine"); err != nil {
		return err
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
	return w.buildHeldLane(planID, holder, "wOld:p1")
}

// buildHeldLane is aHeldLaneHoldingPlanWhoseMarkerNamesAsHolder and
// aHeldLaneHoldingPlanWhoseMarkerNamesAsHolderAndNamesNoSession's
// shared fixture: a held lane for holder, bound to session (empty for
// S76's own "no session at all"), with its token persisted.
//
// heldLaneOwnedBy (start_test.go), the fixture underneath it, mints
// its lease for plan 7 outright rather than taking a plan id of its
// own — every row in this file's own two features happens to say
// "plan 7", so planID is accepted here only to drive w.planID and the
// Then steps that read it back. A row for any other plan id would
// silently get plan 7's lease instead, so that mismatch is refused
// here rather than surfacing later as an opaque git error against a
// ref that heldLaneOwnedBy never created.
func (w *world) buildHeldLane(planID int, holder, session string) error {
	if planID != 7 {
		return fmt.Errorf(
			"buildHeldLane's own fixture, heldLaneOwnedBy, only ever mints plan 7's lease; got plan %d",
			planID)
	}
	isolate(w.t)
	w.planID = planID
	w.holder = holder
	root := w.t.TempDir()
	repo, lane, held := heldLaneOwnedBy(w.t, root, holder, session)
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
	if err := w.buildHeldLane(planID, hostname(), ""); err != nil {
		return err
	}
	dropToken(w.t, section[identityAndCrossLayerState](w).lane)

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
	if st.repo == "" {
		return fmt.Errorf("no repo to check for a release marker; the unclaimed-plan step comes first")
	}
	tip, err := gitCapture(w.t, st.repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	if !claim.Released(st.repo, tip, int64(w.planID), gitwt.Exec) {
		return fmt.Errorf("the branch tip %s is not a release marker for plan %d", tip, w.planID)
	}

	return nil
}

// herdrBecomesReachable installs a fully working herdr handshake in
// place of whatever fake an earlier step left, the same fixture a
// fresh stand-up uses everywhere else in this file — exactly
// herdrShowsNoAgentOnTheLane's own fixture, reused rather than
// reinstalled, since a working handshake naming no agent on any given
// lane is the same runner either step needs.
func (w *world) herdrBecomesReachable() error {
	return w.herdrShowsNoAgentOnTheLane()
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
	tip, err := gitCapture(w.t, st.repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	if !claim.Released(st.repo, tip, int64(w.planID), gitwt.Exec) {
		return fmt.Errorf("the branch tip %s is not a release marker for plan %d", tip, w.planID)
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

// holdsPlanWithItsLanesTokenPersisted builds a real, held lane for
// holder with its token persisted — S46's Given — but never chdirs
// into it: the row's own point is that a directory this run happens
// to be in gets no shortcut from a lease it never carried.
//
// heldLaneOwnedBy mints for plan 7 outright regardless of planID —
// see buildHeldLane's own note — so a planID other than 7 is refused
// here rather than silently building the wrong plan's lease.
func (w *world) holdsPlanWithItsLanesTokenPersisted(holder string, planID int) error {
	if planID != 7 {
		return fmt.Errorf(
			"holdsPlanWithItsLanesTokenPersisted's own fixture, heldLaneOwnedBy, "+
				"only ever mints plan 7's lease; got plan %d", planID)
	}
	isolate(w.t)
	w.planID = planID
	w.holder = holder
	root := w.t.TempDir()
	repo, _, held := heldLaneOwnedBy(w.t, root, holder, "")
	w.clones[holder] = repo
	st := section[identityAndCrossLayerState](w)
	st.root, st.repo, st.held = root, repo, held

	return nil
}

// thisMachineRunsClaimForPlanFromAnUnrelatedDirectory runs `claim`
// with the calling directory deliberately not the plan's own lane —
// S46's When, mirroring TestResumeIgnoresATokenFromAnotherLane: a
// fresh temp directory carries no token for any plan, the shape a
// reused worktree path or a cloned lane leaves once its own token is
// gone.
func (w *world) thisMachineRunsClaimForPlanFromAnUnrelatedDirectory(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.root == "" {
		return fmt.Errorf("no root to claim from; the held-lane step comes first")
	}
	w.t.Chdir(w.t.TempDir())
	var out, errb strings.Builder
	code := run([]string{"claim", strconv.Itoa(planID), "--root", st.root}, &out, &errb)
	st.out, st.errOut, st.code = out.String(), errb.String(), code

	return nil
}

// claimRefusesAlreadyHeld checks the last claim's own output took the
// ordinary already-held door, never a resume it had no token to earn.
func (w *world) claimRefusesAlreadyHeld() error {
	out := section[identityAndCrossLayerState](w).out
	if !strings.Contains(out, "refused") {
		return fmt.Errorf("claim did not refuse: %s", out)
	}
	if !strings.Contains(out, "already held") {
		return fmt.Errorf("claim's refusal does not name an already-held plan: %s", out)
	}

	return nil
}

// thePlanRefIsUnchanged reads origin's tip fresh and checks it still
// sits exactly where the Given step's own held lane left it: a claim
// with no token to prove pushes nothing.
func (w *world) thePlanRefIsUnchanged(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.held == "" {
		return fmt.Errorf("plan %d carries no held tip to compare against; the held-lane step comes first", planID)
	}
	tip := claim.RemoteTip(st.repo, "origin", int64(planID), gitwt.Exec)
	if tip != st.held {
		return fmt.Errorf("origin's ref moved to %s, want it left at %s", tip, st.held)
	}

	return nil
}

// theAgentFailsToStartAndItsOwnTeardownLeavesDebrisBehind installs a
// herdr fake whose worktree stands up, whose agent start then fails —
// S47's own failed handoff — and whose own unwind's worktree.remove
// fails too, so the abort cannot clean up after itself and must name
// what it left behind instead.
func (w *world) theAgentFailsToStartAndItsOwnTeardownLeavesDebrisBehind() error {
	withHerdr(w.t, func(args ...string) ([]byte, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "create" {
			return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
		}
		if len(args) >= 2 && args[0] == "agent" && args[1] == "start" {
			return nil, errors.New(
				"agent target pane wZ:p1 is not an available shell")
		}
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "remove" {
			return nil, errors.New("workspace busy")
		}

		return nil, nil
	})

	return nil
}

// theErrorNamesTheWorktreeAndPaneLeftBehind checks the last run's own
// stderr names both the worktree and the pane a failed teardown could
// not clean up, so `frit orphans` has something to find.
func (w *world) theErrorNamesTheWorktreeAndPaneLeftBehind() error {
	errOut := section[identityAndCrossLayerState](w).errOut
	if !strings.Contains(errOut, "atlas-shader-unit") {
		return fmt.Errorf("the error does not name the worktree left behind: %s", errOut)
	}
	if !strings.Contains(errOut, "wZ:p1") {
		return fmt.Errorf("the error does not name the pane left behind: %s", errOut)
	}

	return nil
}

// aHeldLaneHoldingPlanWhoseMarkerNamesAsHolderAndNamesNoSession builds
// a held lane whose marker never bound a session at all — S76's
// Given, the sharpest reading of "pane gone": there is no session for
// herdr to confirm dead, only a checkout for it to confirm unattended.
func (w *world) aHeldLaneHoldingPlanWhoseMarkerNamesAsHolderAndNamesNoSession(planID int, holder string) error {
	return w.buildHeldLane(planID, holder, "")
}

// thisMachineHoldsPlanInALaneBoundToASessionWithItsTokenPersisted
// mints and renews a lease exactly as
// thisMachineHoldsPlanInALaneWithItsTokenPersisted does, but bound to
// a session — S77's own Given, distinct from S64 and S86's unbound
// fixture: deadSession needs a session herdr can positively confirm
// gone before desertedRefusal will ever fire, and an unbound marker
// gives it nothing to confirm.
func (w *world) thisMachineHoldsPlanInALaneBoundToASessionWithItsTokenPersisted(planID int) error {
	return w.buildLiveLane(planID, "wOld:p1")
}

// aTakeoverBoundToASessionAtANewEpochLandsOnPlan takes the lease over
// exactly as aTakeoverAtANewEpochLandsOnPlan does, but binds the
// takeover to a session — S77's own Given: deadSession reads the
// marker at the *current* tip, so a takeover minted with no session at
// all would give herdr nothing to confirm gone, and desertedRefusal
// would never fire.
func (w *world) aTakeoverBoundToASessionAtANewEpochLandsOnPlan(planID int) error {
	return w.landTakeover(planID, "wGhost:p1")
}

// theLaneRunsStartGoForPlan runs `start --go` with the calling
// directory standing in the lane a prior step stood up — S77's own
// vantage point, the same one a dead host's own dead pane would find
// itself starting from.
func (w *world) theLaneRunsStartGoForPlan(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.lane == "" {
		return fmt.Errorf("no lane to run start from; the token step comes first")
	}
	w.t.Chdir(st.lane)
	var out, errb strings.Builder
	code := run([]string{"start", strconv.Itoa(planID), "--phase", "3", "--go",
		"--root", st.root}, &out, &errb)
	st.out, st.errOut, st.code = out.String(), errb.String(), code

	return nil
}

// startRefusesAndNamesYield checks the last start's own output refused
// and named `yield`, never reporting a started plan: self-resume
// cannot recover a lane whose token a foreign takeover has already
// superseded.
func (w *world) startRefusesAndNamesYield() error {
	st := section[identityAndCrossLayerState](w)
	if !strings.Contains(st.out, "refused") {
		return fmt.Errorf("start did not refuse: %s", st.out)
	}
	if !strings.Contains(st.out, fmt.Sprintf("yield %d", w.planID)) {
		return fmt.Errorf("the refusal does not name yield: %s", st.out)
	}
	if strings.Contains(st.out, "started plan") {
		return fmt.Errorf("start reported success over a deserted lane: %s", st.out)
	}

	return nil
}

// theHolderPushesARawCommitOnTopOfTheHeldTip advances origin's own
// tip past the window's own held observation — S62's own Given: the
// holder is still working, herdr just cannot be reached to say so,
// and the tip moving is what a claimant's own gather must read as
// progress, never silence. The push runs through a fresh worktree on
// the stale holder's own clone, never a chdir into it, since that
// directory already plays "another machine's own checkout" for
// machineClaimsPlan.
func (w *world) theHolderPushesARawCommitOnTopOfTheHeldTip() error {
	st := section[identityAndCrossLayerState](w)
	if st.held == "" {
		return fmt.Errorf("no held tip to push past; the held-plan step comes first")
	}
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	lane := filepath.Join(w.t.TempDir(), "atlas-push")
	git(w.t, repo, "worktree", "add", "-q", lane, claim.Branch(int64(w.planID)))
	git(w.t, lane, "commit", "--allow-empty", "-q", "-m", "work: keep going")
	if out, err := gitCapture(w.t, lane, "push", "-q", "origin", claim.Branch(int64(w.planID))); err != nil {
		return fmt.Errorf("push a raw commit past the held tip: %s: %w", out, err)
	}

	return nil
}

// theRefusalNamesTheWindowNotYetMatured checks the last claim's own
// output names notMaturedReason's own wording — the shape a claim
// takes once Observe has reset the window on an advanced tip, never a
// takeover or a live-session veto.
func (w *world) theRefusalNamesTheWindowNotYetMatured() error {
	out := section[identityAndCrossLayerState](w).out
	if !strings.Contains(out, "not takeable until the window matures") {
		return fmt.Errorf("the refusal does not name the window: %s", out)
	}

	return nil
}

// herdrConfirmsTheLanesOwnSessionIsLive installs a herdr fake naming
// the exact session buildLiveLane's session-bound Given already
// binds — "wOld:p1", the same session S77 already uses — as a live
// agent. S63's own point is that fencing ignores this positive
// answer once a takeover has moved the ref out from under the lane,
// not that liveness goes unchecked.
func (w *world) herdrConfirmsTheLanesOwnSessionIsLive() error {
	withHerdr(w.t, herdrReturning(map[string]any{
		"agent":        "claude",
		"agent_status": "working",
		"pane_id":      "wOld:p1",
		"agent_session": map[string]any{
			"value": "wOld:p1",
		},
	}))

	return nil
}

// itTakesOverCleanlyAtTheNextEpoch checks the last claim's own output
// claimed cleanly — S65's own point, that a herdr which answers but
// names nobody lets the takeover's own worktree stand-up succeed,
// unlike S61's unreachable fake — and that the branch it minted is a
// plain takeover marker at the next epoch, a direct child of exactly
// the stale tip the Given step observed.
func (w *world) itTakesOverCleanlyAtTheNextEpoch() error {
	st := section[identityAndCrossLayerState](w)
	if !strings.Contains(st.out, fmt.Sprintf("claimed plan %d", w.planID)) {
		return fmt.Errorf("claim did not succeed cleanly: %s", st.out)
	}
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
	if !strings.Contains(body, fmt.Sprintf("plan %d: takeover", w.planID)) ||
		!strings.Contains(body, "epoch:   2") {
		return fmt.Errorf("the tip is not a clean epoch-2 takeover: %q", body)
	}
	parent, err := gitCapture(w.t, repo, "rev-parse", tip+"^")
	if err != nil {
		return fmt.Errorf("%s: %w", parent, err)
	}
	if parent != w.lease.Tip {
		return fmt.Errorf("the takeover's parent is %s, want the observed stale tip %s", parent, w.lease.Tip)
	}

	return nil
}

// theMarkersLaneTrailerIsABarePathNamingNoHost reads the current
// marker back and checks its lane: trailer is exactly the fixture's
// own filesystem path — leaseMessage writes opts.Lane verbatim — and
// that the path is a bare absolute path, never a host:path pair:
// S66's whole boundary is that there is nowhere in the trailer a host
// could be recorded.
func (w *world) theMarkersLaneTrailerIsABarePathNamingNoHost() error {
	st := section[identityAndCrossLayerState](w)
	if st.lane == "" {
		return fmt.Errorf("no lane to read the marker's trailer from; the token step comes first")
	}
	tip, err := gitCapture(w.t, st.repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	body, err := gitCapture(w.t, st.repo, "log", "-1", "--format=%B", tip)
	if err != nil {
		return fmt.Errorf("%s: %w", body, err)
	}
	if !strings.Contains(body, "lane:    "+st.lane) {
		return fmt.Errorf("the marker's lane trailer does not read %q: %q", st.lane, body)
	}
	if !filepath.IsAbs(st.lane) {
		return fmt.Errorf("the lane %q is not a bare filesystem path", st.lane)
	}

	return nil
}

// theLanesTokenLivesInsideThatPathsGitDirectory checks claim.TokenPath
// resolves under the lane's own git directory, and that
// claim.ReadToken there matches the lease's own persisted tip — S66's
// other half: nothing about the token's location or its own file
// needs a host either.
func (w *world) theLanesTokenLivesInsideThatPathsGitDirectory() error {
	st := section[identityAndCrossLayerState](w)
	if st.lane == "" {
		return fmt.Errorf("no lane to read the token from; the token step comes first")
	}
	path, err := claim.TokenPath(st.lane, int64(w.planID), gitwt.Exec)
	if err != nil {
		return fmt.Errorf("token path: %w", err)
	}
	gitDir, err := gitwt.GitDir(st.lane, gitwt.Exec)
	if err != nil {
		return fmt.Errorf("git dir: %w", err)
	}
	if !strings.HasPrefix(path, gitDir) {
		return fmt.Errorf("the token path %q does not sit inside the lane's own git directory %q", path, gitDir)
	}
	if got := claim.ReadToken(st.lane, int64(w.planID), gitwt.Exec); got != w.lease.Tip {
		return fmt.Errorf("the persisted token is %q, want the lease's own tip %q", got, w.lease.Tip)
	}

	return nil
}

// claimAndStartBothRaceToMintPlan runs `claim` and `start --go`
// concurrently against the Given step's own unclaimed plan, each from
// its own clone of the same origin — a genuine two-goroutine
// contention over one push, not two runs in sequence, so the
// git-level force-with-lease CAS is what actually decides the winner.
// Neither goroutine calls t.Chdir: both would race the same
// process-wide directory, so each passes --root instead, exactly as
// every other step in this file already does. The herdr fake both
// share must actually answer, since standUpClaimWorktree means the
// winner — whichever verb it is — still stands its own worktree up
// behind the lease; a fake that errors would make that failure read
// as "refused" too, and the row could not tell the two apart.
func (w *world) claimAndStartBothRaceToMintPlan(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.root == "" {
		return fmt.Errorf("no root to race from; the unclaimed-plan step comes first")
	}
	runner, _ := startHerdr()
	withHerdr(w.t, runner)

	root2, _ := cloneRepoIntoRoot(w.t, st.repo)

	var wg sync.WaitGroup
	ready := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-ready
		var out, errb strings.Builder
		code := run([]string{"claim", strconv.Itoa(planID), "--root", st.root}, &out, &errb)
		st.raceA = raceResult{out: out.String(), code: code}
	}()
	go func() {
		defer wg.Done()
		<-ready
		var out, errb strings.Builder
		code := run([]string{
			"start", strconv.Itoa(planID), "--phase", "3", "--go", "--root", root2,
		}, &out, &errb)
		st.raceB = raceResult{out: out.String(), code: code}
	}()
	close(ready)
	wg.Wait()

	return nil
}

// oneWinsAndTheLosersRefusalNamesTheWinningLane checks the race's own
// two results: exactly one of claim and start actually minted the
// lease, and the other's refusal names this host as the plan's own
// holder — the only lane a race confined to one host could have
// produced, read off the marker the loser's own lost-race error
// carries rather than guessed from timing.
func (w *world) oneWinsAndTheLosersRefusalNamesTheWinningLane() error {
	st := section[identityAndCrossLayerState](w)
	if st.raceA.out == "" && st.raceB.out == "" {
		return fmt.Errorf("no race results to read; the race step comes first")
	}
	claimWon := !strings.Contains(st.raceA.out, "refused:")
	startWon := !strings.Contains(st.raceB.out, "refused:")
	if claimWon == startWon {
		return fmt.Errorf(
			"want exactly one winner; claim: %q, start: %q", st.raceA.out, st.raceB.out)
	}
	loser := st.raceA.out
	if claimWon {
		loser = st.raceB.out
	}
	if !strings.Contains(loser, "refused: plan") {
		return fmt.Errorf("the loser's own output does not refuse: %s", loser)
	}
	if !strings.Contains(loser, hostname()) {
		return fmt.Errorf(
			"the loser's refusal does not name this host as the winning lane's holder: %s", loser)
	}

	return nil
}

// planIsUnclaimedInTwoRepos is planIsUnclaimed's sibling for S74:
// two repositories under one root, each carrying plan 7 unclaimed,
// each titled with its own repository's name so a later claim can
// select by that name rather than the bare id — a bare numeric
// selector two repositories in one root both answer to is refused as
// ambiguous by discovery.Resolve, a different, correct refusal this
// row is not about.
func (w *world) planIsUnclaimedInTwoRepos(planID int, repoA, repoB string) error {
	isolate(w.t)
	w.planID = planID
	w.holder = hostname()
	root := w.t.TempDir()
	a := claimableRepo(w.t, root, repoA, planID, repoA+" plan")
	b := claimableRepo(w.t, root, repoB, planID, repoB+" plan")
	w.clones[w.holder] = a
	st := section[identityAndCrossLayerState](w)
	st.root, st.repoA, st.repoB = root, a, b

	return nil
}

// thisMachineClaimsPlanInTwoRepos claims plan 7 in each of S74's own
// two repositories, by each one's own name rather than the bare id,
// installing the reachable herdr fake first so both worktrees stand
// up and its recorder can be read back for each pane's own label.
func (w *world) thisMachineClaimsPlanInTwoRepos(planID int, repoA, repoB string) error {
	st := section[identityAndCrossLayerState](w)
	if st.root == "" {
		return fmt.Errorf("no root to claim from; the two-repo unclaimed-plan step comes first")
	}
	runner, rec := startHerdr()
	withHerdr(w.t, runner)
	st.rec = rec

	var outA, errA strings.Builder
	codeA := run([]string{"claim", repoA, "--root", st.root}, &outA, &errA)
	st.raceA = raceResult{out: outA.String(), code: codeA}

	var outB, errB strings.Builder
	codeB := run([]string{"claim", repoB, "--root", st.root}, &outB, &errB)
	st.raceB = raceResult{out: outB.String(), code: codeB}

	return nil
}

// bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo checks
// S74's own two claims: neither refused despite sharing one plan id,
// each one's own worktree line names its own repository, and herdr's
// own recorder shows a distinct worktree-create label per repository
// — the observable behind "lanes key host:repo:id, not host:id
// alone" and "pane names carry the repo".
func (w *world) bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo() error {
	st := section[identityAndCrossLayerState](w)
	if st.repoA == "" || st.repoB == "" {
		return fmt.Errorf("no two-repo fixture; the two-repo claim step comes first")
	}
	if strings.Contains(st.raceA.out, "refused") {
		return fmt.Errorf("the first repo's claim was refused: %s", st.raceA.out)
	}
	if strings.Contains(st.raceB.out, "refused") {
		return fmt.Errorf("the second repo's claim was refused: %s", st.raceB.out)
	}

	nameA, nameB := filepath.Base(st.repoA), filepath.Base(st.repoB)
	if !strings.Contains(st.raceA.out, nameA+"-") {
		return fmt.Errorf("the first repo's own worktree does not carry its own repo name: %s", st.raceA.out)
	}
	if !strings.Contains(st.raceB.out, nameB+"-") {
		return fmt.Errorf("the second repo's own worktree does not carry its own repo name: %s", st.raceB.out)
	}

	if st.rec == nil || st.rec.count("worktree", "create") != 2 {
		return fmt.Errorf("want 2 worktree-create calls to herdr")
	}
	labelA := nameA + " plan " + strconv.Itoa(w.planID)
	if !st.rec.hasArg(labelA) {
		return fmt.Errorf("no pane names the first repo: want a label %q", labelA)
	}
	labelB := nameB + " plan " + strconv.Itoa(w.planID)
	if !st.rec.hasArg(labelB) {
		return fmt.Errorf("no pane names the second repo: want a label %q", labelB)
	}

	return nil
}

// plansHoldBranchAlreadyCarriesALiveHerdrPane is S88's own Given: a
// matured, session-less hold — fair game for every earlier takeover
// guard — with a herdr pane already sitting live on that exact
// branch in a clone outside root, the live-but-unbound lane the
// takeover veto cannot see (issue #126). It reuses liveLeaseFixture,
// the same fixture the unit-level pin in cmd/frit/pick_test.go
// builds, and layers freshDispatchAfterLiveLaneQuery's worktree
// create / pane current answers on top so a real fresh dispatch onto
// the plan the walk advances to can still run. liveLeaseFixture only
// ever mints plan 7's lease, so any other id is refused here rather
// than silently answered with plan 7's lease instead.
func (w *world) plansHoldBranchAlreadyCarriesALiveHerdrPane(planID int) error {
	if planID != 7 {
		return fmt.Errorf(
			"liveLeaseFixture only ever mints plan 7's live lane; got plan %d",
			planID)
	}
	isolate(w.t)
	w.planID = planID
	root := w.t.TempDir()
	repo, lease, runner, rec := liveLeaseFixture(w.t, root)
	st := section[identityAndCrossLayerState](w)
	st.root, st.repo, st.held, st.rec = root, repo, lease.Tip, rec
	withHerdr(w.t, freshDispatchAfterLiveLaneQuery(runner))

	return nil
}

// planIsReadyAndHeldByNobody adds S88's own second candidate to the
// fleet the live-lane step built: a plain ready plan, ranked below
// the live one by id, that pick --go's walk should reach once it
// skips the busy top pick.
func (w *world) planIsReadyAndHeldByNobody(planID int) error {
	st := section[identityAndCrossLayerState](w)
	if st.repo == "" {
		return fmt.Errorf(
			"no repo to add plan %d to; the live-lane step comes first", planID)
	}
	commitPlan(w.t, st.repo, planID, "🔲", fmt.Sprintf("Plan %d unit", planID), nil, "")

	return nil
}

// pickGoRuns is S88's own When: `pick --go`, run for real against the
// fleet the Given steps built, capturing its report the way every
// other driver step in this section does.
func (w *world) pickGoRuns() error {
	st := section[identityAndCrossLayerState](w)
	if st.root == "" {
		return fmt.Errorf("no root to run pick --go from; the fleet-building steps come first")
	}
	var out, errb strings.Builder
	code := run([]string{"pick", "--go", "--root", st.root}, &out, &errb)
	st.out, st.errOut, st.code = out.String(), errb.String(), code

	return nil
}

// planIsTheOneStarted checks pick --go's own output names planID as
// started — the scenario's own identity assertion, so a walk that
// started some other plan, or nothing at all, cannot pass it.
func (w *world) planIsTheOneStarted(planID int) error {
	out := section[identityAndCrossLayerState](w).out
	want := fmt.Sprintf("started plan %d", planID)
	if !strings.Contains(out, want) {
		return fmt.Errorf("expected %q in pick --go's output, got: %s", want, out)
	}

	return nil
}

// planIsNotRefusedOn checks pick --go's own output carries no refusal
// at all: the live top lane this scenario names is a candidate the
// walk skips, never one it stalls and reports a refusal on. planID
// names which lane the scenario means, read for its place in the
// error only — a refusal anywhere in the output fails this Then, the
// same "no refused at all" shape the unit-level pin asserts.
func (w *world) planIsNotRefusedOn(planID int) error {
	out := section[identityAndCrossLayerState](w).out
	if strings.Contains(out, "refused") {
		return fmt.Errorf("pick --go refused rather than skipping plan %d: %s", planID, out)
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

// TestMoreIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition: two
// more assertion steps that read state an earlier step recorded, split
// from TestIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition so
// neither function trips golangci-lint's funlen.
func TestMoreIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition(t *testing.T) {
	w := newWorld(t)

	err := w.theLeaseIsReleasedNotLeftStanding()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no repo")

	err = w.takeoverAtEpoch2SitsOnTheStaleTip()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no machine")
}

// TestIdentityAndCrossLayerHeldLaneFixturesRefuseAnUnsupportedPlanID:
// buildHeldLane and holdsPlanWithItsLanesTokenPersisted both sit on
// heldLaneOwnedBy (start_test.go), which mints its lease for plan 7
// outright regardless of the planID it is given — a row for any other
// plan id must be refused here, not silently answered with plan 7's
// lease instead.
func TestIdentityAndCrossLayerHeldLaneFixturesRefuseAnUnsupportedPlanID(t *testing.T) {
	w := newWorld(t)

	err := w.buildHeldLane(12, "elsewhere", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan 7")

	err = w.holdsPlanWithItsLanesTokenPersisted("elsewhere", 12)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan 7")
}

// TestPhase3IdentityAndCrossLayerStepsRefuseTheirMissingPrecondition:
// Phase 3's own four new steps, split from
// TestIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition so
// neither function trips golangci-lint's funlen.
func TestPhase3IdentityAndCrossLayerStepsRefuseTheirMissingPrecondition(t *testing.T) {
	w := newWorld(t)

	err := w.thisMachineRunsClaimForPlanFromAnUnrelatedDirectory(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no root")

	err = w.thePlanRefIsUnchanged(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no held tip")

	err = w.theLaneRunsStartGoForPlan(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no lane")

	err = w.aTakeoverBoundToASessionAtANewEpochLandsOnPlan(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no repo")
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

// TestPhase3IdentityAndCrossLayerReadBacksWantTheirExactShape: Phase
// 3's own three new read-back steps, split from
// TestIdentityAndCrossLayerReadBacksWantTheirExactShape so neither
// function trips golangci-lint's funlen.
func TestPhase3IdentityAndCrossLayerReadBacksWantTheirExactShape(t *testing.T) {
	w := newWorld(t)
	st := section[identityAndCrossLayerState](w)

	st.out = "claimed plan 7"
	require.Error(t, w.claimRefusesAlreadyHeld(), "no refusal at all")
	st.out = "refused: plan 7 is held by a live agent session on box-a"
	require.Error(t, w.claimRefusesAlreadyHeld(), "refused, but not on the already-held door")
	st.out = "refused: plan 7 already held (plan/7); not takeable until the window matures"
	assert.NoError(t, w.claimRefusesAlreadyHeld())

	st.errOut = "workspace busy"
	require.Error(t, w.theErrorNamesTheWorktreeAndPaneLeftBehind(), "names neither the worktree nor the pane")
	st.errOut = "left behind: atlas-shader-unit"
	require.Error(t, w.theErrorNamesTheWorktreeAndPaneLeftBehind(), "names the worktree but not the pane")
	st.errOut = "left behind: atlas-shader-unit, wZ:p1"
	assert.NoError(t, w.theErrorNamesTheWorktreeAndPaneLeftBehind())

	w.planID = 7
	st.out = "started plan 7"
	require.Error(t, w.startRefusesAndNamesYield(), "no refusal at all")
	st.out = "refused: plan 7 already held"
	require.Error(t, w.startRefusesAndNamesYield(), "refused, but does not name yield")
	st.out = "refused: plan 7 could not be resumed: deserted hold: its token cannot " +
		"self-resume; run `frit yield 7` to retire this lane"
	assert.NoError(t, w.startRefusesAndNamesYield())
}

// TestPhase4IdentityAndCrossLayerStepsRefuseTheirMissingPrecondition:
// Phase 4's own new steps refuse when the state an earlier step
// records was never built, rather than reading a zero value as real.
func TestPhase4IdentityAndCrossLayerStepsRefuseTheirMissingPrecondition(t *testing.T) {
	w := newWorld(t)

	err := w.theHolderPushesARawCommitOnTopOfTheHeldTip()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no held tip")

	err = w.theMarkersLaneTrailerIsABarePathNamingNoHost()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no lane")

	err = w.theLanesTokenLivesInsideThatPathsGitDirectory()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no lane")
}

// TestPhase4IdentityAndCrossLayerReadBacksWantTheirExactShape: Phase
// 4's own read-back steps refuse on the wrong shape rather than
// passing on an unrelated success or a refusal for the wrong reason.
func TestPhase4IdentityAndCrossLayerReadBacksWantTheirExactShape(t *testing.T) {
	w := newWorld(t)
	st := section[identityAndCrossLayerState](w)

	st.out = "refused: plan 7 is held by a live agent session on box-a"
	require.Error(t, w.theRefusalNamesTheWindowNotYetMatured(), "refused, but not for the window")
	st.out = "refused: plan 7 already held (plan/7); not takeable until the window matures"
	assert.NoError(t, w.theRefusalNamesTheWindowNotYetMatured())

	w.planID = 7
	st.out = "refused: plan 7 already held"
	require.Error(t, w.itTakesOverCleanlyAtTheNextEpoch(), "a refusal is not a clean takeover")
	st.out = "claimed plan 7"
	err := w.itTakesOverCleanlyAtTheNextEpoch()
	require.Error(t, err, "a claimed output with no machine introduced has nothing to read back")
	assert.Contains(t, err.Error(), "no machine")
}

// TestPhase5IdentityAndCrossLayerStepsRefuseTheirMissingPrecondition:
// Phase 5's own new steps refuse when the state an earlier step
// records was never built, rather than reading a zero value as real.
func TestPhase5IdentityAndCrossLayerStepsRefuseTheirMissingPrecondition(t *testing.T) {
	w := newWorld(t)

	err := w.claimAndStartBothRaceToMintPlan(7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no root")

	err = w.oneWinsAndTheLosersRefusalNamesTheWinningLane()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no race results")

	err = w.thisMachineClaimsPlanInTwoRepos(7, "atlas", "forge")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no root")

	err = w.bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no two-repo fixture")
}

// TestPhase5RaceReadBackWantsItsExactShape:
// oneWinsAndTheLosersRefusalNamesTheWinningLane refuses unless exactly
// one of the race's two results actually refused, and that refusal
// names this host, rather than passing on two winners, two losers, or
// a loser whose refusal names some other machine.
func TestPhase5RaceReadBackWantsItsExactShape(t *testing.T) {
	w := newWorld(t)
	st := section[identityAndCrossLayerState](w)

	st.raceA = raceResult{out: "claimed plan 7"}
	st.raceB = raceResult{out: "started plan 7"}
	require.Error(t, w.oneWinsAndTheLosersRefusalNamesTheWinningLane(), "both won")

	st.raceA = raceResult{out: "refused: plan 7 already held on this host"}
	st.raceB = raceResult{out: "refused: plan 7 already held on this host"}
	require.Error(t, w.oneWinsAndTheLosersRefusalNamesTheWinningLane(), "both lost")

	st.raceA = raceResult{out: "claimed plan 7"}
	st.raceB = raceResult{out: "internal error: refused: access denied"}
	err := w.oneWinsAndTheLosersRefusalNamesTheWinningLane()
	require.Error(t, err, "the loser's refusal is not shaped like the ordinary one")
	assert.Contains(t, err.Error(), "does not refuse")

	st.raceA = raceResult{out: "claimed plan 7"}
	st.raceB = raceResult{out: "refused: plan 7 lost the race to another machine (elsewhere)"}
	err = w.oneWinsAndTheLosersRefusalNamesTheWinningLane()
	require.Error(t, err, "the refusal names a different machine, not this host")
	assert.Contains(t, err.Error(), "does not name this host")

	st.raceA = raceResult{out: "claimed plan 7\n  branch: plan/7\n"}
	st.raceB = raceResult{out: "refused: plan 7 already held on this host (" + hostname() + ")"}
	assert.NoError(t, w.oneWinsAndTheLosersRefusalNamesTheWinningLane())
}

// TestPhase5MultiRepoReadBackWantsItsExactShape:
// bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo refuses
// on either claim's own refusal, a worktree line missing its own
// repository's name, or a herdr recorder whose panes do not tell the
// two repositories apart, rather than passing on a coincidence.
func TestPhase5MultiRepoReadBackWantsItsExactShape(t *testing.T) {
	w := newWorld(t)
	w.planID = 7
	st := section[identityAndCrossLayerState](w)
	st.repoA, st.repoB = "/root/atlas", "/root/forge"

	st.raceA = raceResult{out: "refused: plan 7 already held"}
	st.raceB = raceResult{out: "claimed plan 7\n  worktree: /root/forge-plan\n"}
	require.Error(t, w.bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo(),
		"the first repo's claim was refused")

	st.raceA = raceResult{out: "claimed plan 7\n  worktree: /root/atlas-plan\n"}
	st.raceB = raceResult{out: "refused: plan 7 already held"}
	require.Error(t, w.bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo(),
		"the second repo's claim was refused")

	st.raceB = raceResult{out: "claimed plan 7\n  worktree: /root/no-repo-name\n"}
	err := w.bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo()
	require.Error(t, err, "the second repo's own worktree names nothing")
	assert.Contains(t, err.Error(), "does not carry")

	st.raceB = raceResult{out: "claimed plan 7\n  worktree: /root/forge-plan\n"}
	err = w.bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo()
	require.Error(t, err, "no herdr recorder at all")
	assert.Contains(t, err.Error(), "worktree-create")

	st.rec = &herdrCalls{}
	require.Error(t, w.bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo(),
		"no worktree-create calls recorded")

	st.rec.calls = [][]string{
		{"worktree", "create", "--label", "atlas plan 7"},
		{"worktree", "create", "--label", "atlas plan 7"},
	}
	err = w.bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo()
	require.Error(t, err, "neither pane names the second repo")
	assert.Contains(t, err.Error(), "no pane names")

	st.rec.calls = [][]string{
		{"worktree", "create", "--label", "atlas plan 7"},
		{"worktree", "create", "--label", "forge plan 7"},
	}
	assert.NoError(t, w.bothAreClaimedWithNoCollisionAndTheLanesAndPanesCarryTheRepo())
}

// TestPickWalkIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition:
// S88's own new steps refuse when the state an earlier step records
// was never built, or when given a plan id its fixture cannot mint,
// rather than reading a zero value as real.
func TestPickWalkIdentityAndCrossLayerStepsRefuseTheirMissingPrecondition(t *testing.T) {
	w := newWorld(t)

	err := w.plansHoldBranchAlreadyCarriesALiveHerdrPane(12)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan 7")

	err = w.planIsReadyAndHeldByNobody(8)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no repo")

	err = w.pickGoRuns()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no root")
}

// TestPickWalkIdentityAndCrossLayerReadBacksWantTheirExactShape:
// planIsTheOneStarted and planIsNotRefusedOn read pick --go's own
// captured output, so a walk that started the wrong plan, or stalled
// and refused, must not pass either Then.
func TestPickWalkIdentityAndCrossLayerReadBacksWantTheirExactShape(t *testing.T) {
	w := newWorld(t)
	st := section[identityAndCrossLayerState](w)

	st.out = "started plan 7"
	require.Error(t, w.planIsTheOneStarted(8), "the wrong plan started")
	st.out = "refused: plan 7 already sits on a live lane"
	require.Error(t, w.planIsTheOneStarted(8), "a refusal is not a start")
	st.out = "started plan 8"
	assert.NoError(t, w.planIsTheOneStarted(8))

	st.out = "refused: plan 7 already sits on a live lane"
	require.Error(t, w.planIsNotRefusedOn(7))
	st.out = "started plan 8"
	assert.NoError(t, w.planIsNotRefusedOn(7))
}
