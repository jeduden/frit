package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/observe"
	"github.com/jeduden/frit/internal/report"
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
// origin, the fixed instant an observation reset was driven at, and
// the verb-level state phase 2 adds — the standing worktree a token
// was persisted through, the tip a held lease sits on before any
// takeover, who last ran `claim`, and its decoded document — kept
// here, not on world, so this section adds a file and never a field
// to it.
type deathState struct {
	retryErr     error
	winner       string
	winnerLease  claim.Lease
	pushedTip    string
	resetAt      time.Time
	lane         string
	heldTip      string
	lastClaimant string
	lastClaimDoc report.ClaimDoc
	boardDoc     report.BoardDoc
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
	sc.Step(`^"([^"]+)" holds the lease for plan (\d+) with its token persisted in its own worktree$`,
		w.holdsTheLeaseWithTokenPersisted)
	sc.Step(`^"([^"]+)" runs claim for plan (\d+) from its own worktree$`, w.runsClaimFromItsOwnWorktree)
	sc.Step(`^the claim resumes instead of refusing$`, w.theClaimResumesInsteadOfRefusing)
	sc.Step(`^"([^"]+)" has claimed plan (\d+) but its worktree was never stood up$`,
		w.hasClaimedButWorktreeNeverStoodUp)
	sc.Step(`^"([^"]+)" runs claim for plan (\d+)$`, w.runsClaimForPlan)
	sc.Step(`^the claim is refused, not resumed$`, w.theClaimIsRefusedNotResumed)
	sc.Step(`^the claim takes the lease over$`, w.theClaimTakesTheLeaseOver)
	sc.Step(`^the takeover window has matured for plan (\d+)$`, w.theTakeoverWindowHasMaturedForPlan)
	sc.Step(`^"([^"]+)" runs board$`, w.runsBoard)
	sc.Step(`^board shows plan (\d+) held with no session$`, w.boardShowsPlanHeldWithNoSession)
	sc.Step(`^"([^"]+)" holds the lease for plan (\d+) with a session bound$`, w.holdsTheLeaseWithASessionBound)
	sc.Step(`^herdr confirms no live agent on that session$`, w.herdrConfirmsNoLiveAgentOnThatSession)
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

// holdsTheLeaseWithTokenPersisted mints a lease into a real worktree
// and renews it from inside that worktree — the state a phase's own
// lane is in mid-work: its token, written by that renewal, matches
// exactly the tip origin holds right now.
func (w *world) holdsTheLeaseWithTokenPersisted(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = holder
	repo := claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")
	w.clones[holder] = repo

	lane := filepath.Join(w.t.TempDir(), "atlas-lane")
	opts := leaseFor(holder, planID)
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
	section[deathState](w).lane = lane
	withHerdr(w.t, herdrReturningWithWorktree())

	return nil
}

// runsClaimFromItsOwnWorktree runs `claim` with the calling directory
// standing in the lane a prior step's worktree add stood up — the
// vantage point `ownToken`'s `inOwnLane` check requires, and the one
// a real resumed process runs from.
func (w *world) runsClaimFromItsOwnWorktree(holder string, planID int) error {
	lane := section[deathState](w).lane
	if lane == "" {
		return fmt.Errorf("%q has no worktree to run claim from; the token step comes first", holder)
	}

	return w.runClaimFrom(holder, planID, lane)
}

// hasClaimedButWorktreeNeverStoodUp mints a lease naming a lane that
// is never turned into a real worktree — a claim killed the instant
// after the push landed, before standUpClaimWorktree ever ran. No
// lease transition has run from inside that lane, so no token was
// ever persisted there, on this host or any other.
func (w *world) hasClaimedButWorktreeNeverStoodUp(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = holder
	repo := claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")
	w.clones[holder] = repo

	opts := leaseFor(holder, planID)
	opts.Lane = filepath.Join(w.t.TempDir(), "atlas-lane-never-stood-up")
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	if err != nil {
		return err
	}
	w.lease = lease
	section[deathState](w).heldTip = lease.Tip
	withHerdr(w.t, herdrReturningWithWorktree())

	return nil
}

// holdsTheLeaseWithASessionBound mints a lease whose marker names a
// session, as a renewal from an already-started agent would — the
// state a lane is in once the agent is up, before it has renewed
// again to prove it is still there.
func (w *world) holdsTheLeaseWithASessionBound(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	w.holder = holder
	repo := claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")
	w.clones[holder] = repo

	opts := leaseFor(holder, planID)
	opts.Session = "sess-1"
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	if err != nil {
		return err
	}
	w.lease = lease
	section[deathState](w).heldTip = lease.Tip

	return nil
}

// herdrConfirmsNoLiveAgentOnThatSession installs the herdr fake an
// out-of-band positive read gets: reachable, and its agent list
// names nobody on the session the marker bound — the shape
// `herdr.SessionLive`'s own pane walk needs to answer false through
// its query path, not its empty-session short-circuit.
func (w *world) herdrConfirmsNoLiveAgentOnThatSession() error {
	withHerdr(w.t, herdrReturningWithWorktree())

	return nil
}

// runsClaimForPlan runs `claim` for holder against the plan's own
// repository, cloning one fresh from the origin the Given step
// already introduced when holder is a machine the scenario has not
// met yet — the ordinary case, run from outside any lane.
func (w *world) runsClaimForPlan(holder string, planID int) error {
	if _, ok := w.clones[holder]; !ok {
		first, err := w.cloneOf(w.holder)
		if err != nil {
			return err
		}
		_, dst := cloneRepoIntoRoot(w.t, first)
		w.clones[holder] = dst
	}

	return w.runClaimFrom(holder, planID, "")
}

// cloneRepoIntoRoot clones repo's own origin into a fresh root's
// "atlas" subdirectory — the same shape claimableRepo gives the first
// machine — so a second machine's own `--root` names only its own
// tree. cloneAgain's bare clone, by contrast, sits directly in its
// own temp directory; a `--root` built from its parent would walk
// into the first machine's tree too and find the plan twice.
func cloneRepoIntoRoot(t *testing.T, repo string) (root, dst string) {
	t.Helper()
	origin, err := gitCapture(t, repo, "config", "--get", "remote.origin.url")
	require.NoError(t, err, origin)
	root = t.TempDir()
	dst = filepath.Join(root, "atlas")
	git(t, root, "clone", "-q", origin, dst)
	git(t, dst, "config", "user.email", "t2@example.com")
	git(t, dst, "config", "user.name", "frit-test-2")

	return root, dst
}

// runClaimFrom is runsClaimForPlan's and runsClaimFromItsOwnWorktree's
// shared machinery: invoke `claim` with --json from wd (the
// repository's own root when wd is ""), and decode the document into
// deathState for the Then steps to read back.
func (w *world) runClaimFrom(holder string, planID int, wd string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	root := filepath.Dir(repo)
	if wd != "" {
		w.t.Chdir(wd)
	}
	var out, errb strings.Builder
	code := run([]string{"claim", strconv.Itoa(planID), "--root", root, "--json"}, &out, &errb)
	if code != 0 {
		return fmt.Errorf("claim exited %d: %s", code, errb.String())
	}
	var doc report.ClaimDoc
	if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
		return fmt.Errorf("decode claim's document: %w: %s", err, out.String())
	}
	ds := section[deathState](w)
	ds.lastClaimant, ds.lastClaimDoc = holder, doc

	return nil
}

// theClaimResumesInsteadOfRefusing checks the last claim document
// took the resume door: claimed, resumed, and refused nothing — the
// one shape `MarkResumed` produces and no other path does.
func (w *world) theClaimResumesInsteadOfRefusing() error {
	doc := section[deathState](w).lastClaimDoc
	if !doc.Claimed || !doc.Resumed || doc.Refused != "" {
		return fmt.Errorf("claim did not resume: claimed=%v resumed=%v refused=%q",
			doc.Claimed, doc.Resumed, doc.Refused)
	}

	return nil
}

// theClaimIsRefusedNotResumed checks the last claim document was
// refused, and specifically not through the resume door — a takeover
// that failed for some other reason must not read as this row's
// promise.
func (w *world) theClaimIsRefusedNotResumed() error {
	doc := section[deathState](w).lastClaimDoc
	if doc.Refused == "" || doc.Claimed || doc.Resumed {
		return fmt.Errorf("claim was not a plain refusal: claimed=%v resumed=%v refused=%q",
			doc.Claimed, doc.Resumed, doc.Refused)
	}

	return nil
}

// theClaimTakesTheLeaseOver checks the last claim document claimed
// without resuming, and that the tip it minted really is a takeover
// marker, child of the tip the window matured on — not merely that
// the run reported success.
func (w *world) theClaimTakesTheLeaseOver() error {
	ds := section[deathState](w)
	doc := ds.lastClaimDoc
	if !doc.Claimed || doc.Resumed || doc.Refused != "" {
		return fmt.Errorf("claim did not take the lease over: claimed=%v resumed=%v refused=%q",
			doc.Claimed, doc.Resumed, doc.Refused)
	}
	repo, err := w.cloneOf(ds.lastClaimant)
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
	want := fmt.Sprintf("plan %d: takeover", w.planID)
	if !strings.Contains(body, want) {
		return fmt.Errorf("the minted tip's marker is %q, want it to carry %q", body, want)
	}

	return nil
}

// theTakeoverWindowHasMaturedForPlan seeds a window over the held
// tip a Given step recorded, matured well past the default takeover
// window, the same fixture claim_test.go's own stale-lease tests use.
func (w *world) theTakeoverWindowHasMaturedForPlan(planID int) error {
	heldTip := section[deathState](w).heldTip
	if heldTip == "" {
		return fmt.Errorf("plan %d carries no held tip to mature a window on", planID)
	}
	seedWindow(w.t, "atlas", int64(planID), heldTip, 3*time.Hour)

	return nil
}

// runsBoard runs `board --json` and decodes it into deathState, with
// herdr answering reachable and empty — the "no agent anywhere" read
// board's own no-session row needs.
func (w *world) runsBoard(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	withHerdr(w.t, herdrReturning())
	var out, errb strings.Builder
	code := run([]string{"board", "--root", filepath.Dir(repo), "--json"}, &out, &errb)
	if code != 0 {
		return fmt.Errorf("board exited %d: %s", code, errb.String())
	}
	var doc report.BoardDoc
	if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
		return fmt.Errorf("decode board's document: %w: %s", err, out.String())
	}
	section[deathState](w).boardDoc = doc

	return nil
}

// boardShowsPlanHeldWithNoSession finds the plan's own row on the
// board just read and checks it reads held, with no agent — the
// shape a live session's absence leaves, as opposed to an unreachable
// herdr, which would leave the field unset for a different reason.
func (w *world) boardShowsPlanHeldWithNoSession(planID int) error {
	doc := section[deathState](w).boardDoc
	for _, p := range doc.Plans {
		if p.ID != int64(planID) {
			continue
		}
		if !p.Held {
			return fmt.Errorf("plan %d's board row is not held", planID)
		}
		if p.Agent != "" {
			return fmt.Errorf("plan %d's board row names agent %q, want none", planID, p.Agent)
		}

		return nil
	}

	return fmt.Errorf("plan %d has no row on the board", planID)
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

// TestRunsClaimFromItsOwnWorktreeRefusesWithNoLane: the step stands
// on the worktree "holds the lease ... token persisted" stood up —
// without it there is no directory to run claim from at all.
func TestRunsClaimFromItsOwnWorktreeRefusesWithNoLane(t *testing.T) {
	w := newWorld(t)

	err := w.runsClaimFromItsOwnWorktree("box-a", 7)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no worktree")
}

// TestClaimDocReadBacksWantTheirExactShape: each Then step reads a
// specific combination of Claimed/Resumed/Refused off the last claim
// document, and none of the three is happy with a claim that
// succeeded, failed or resumed for some other, unrelated reason.
func TestClaimDocReadBacksWantTheirExactShape(t *testing.T) {
	w := newWorld(t)
	ds := section[deathState](w)

	ds.lastClaimDoc = report.ClaimDoc{Claimed: true, Refused: "already held"}
	require.Error(t, w.theClaimResumesInsteadOfRefusing(), "claimed and refused never coexist as a resume")
	require.Error(t, w.theClaimIsRefusedNotResumed(), "a refusal never carries Claimed true")
	require.Error(t, w.theClaimTakesTheLeaseOver(), "Refused set is not a takeover")

	ds.lastClaimDoc = report.ClaimDoc{Refused: "already held"}
	assert.NoError(t, w.theClaimIsRefusedNotResumed())
	require.Error(t, w.theClaimResumesInsteadOfRefusing())

	ds.lastClaimDoc = report.ClaimDoc{Claimed: true, Resumed: true}
	assert.NoError(t, w.theClaimResumesInsteadOfRefusing())
	require.Error(t, w.theClaimIsRefusedNotResumed(), "a resume is not a plain refusal")
	require.Error(t, w.theClaimTakesTheLeaseOver(), "a resume is not a takeover")

	ds.lastClaimDoc = report.ClaimDoc{Claimed: true}
	ds.lastClaimant = "ghost"
	err := w.theClaimTakesTheLeaseOver()
	require.Error(t, err, "claimed, not resumed, still needs the marker check, which fails with no such clone")
}

// TestTakeoverWindowRefusesWithNoHeldTip: without a Given step
// recording what tip is actually held, seeding a window would mature
// staleness over nothing.
func TestTakeoverWindowRefusesWithNoHeldTip(t *testing.T) {
	w := newWorld(t)

	err := w.theTakeoverWindowHasMaturedForPlan(7)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no held tip")
}

// TestBoardShowsPlanHeldWithNoSessionReadsTheRightRow: the step finds
// the plan's own row by id, and is exacting about both Held and
// Agent rather than passing on either alone.
func TestBoardShowsPlanHeldWithNoSessionReadsTheRightRow(t *testing.T) {
	w := newWorld(t)
	ds := section[deathState](w)

	err := w.boardShowsPlanHeldWithNoSession(7)
	require.Error(t, err, "no board was ever run")
	assert.Contains(t, err.Error(), "no row")

	ds.boardDoc = report.BoardDoc{Plans: []report.BoardPlan{{ID: 7, Held: false}}}
	require.Error(t, w.boardShowsPlanHeldWithNoSession(7), "not held")

	ds.boardDoc = report.BoardDoc{Plans: []report.BoardPlan{{ID: 7, Held: true, Agent: "claude"}}}
	require.Error(t, w.boardShowsPlanHeldWithNoSession(7), "an agent is on it")

	ds.boardDoc = report.BoardDoc{Plans: []report.BoardPlan{{ID: 7, Held: true}}}
	assert.NoError(t, w.boardShowsPlanHeldWithNoSession(7))
}

// TestCloneRepoIntoRootIsolatesASecondMachinesRoot: a second
// machine's clone must sit under its own root, one level below it —
// unlike cloneAgain's bare clone, whose parent directory a fleet walk
// would otherwise share with the first machine's own tree, finding
// the same plan twice.
func TestCloneRepoIntoRootIsolatesASecondMachinesRoot(t *testing.T) {
	isolate(t)
	first := claimableRepo(t, t.TempDir(), "atlas", 7, "Shader unit")

	root, dst := cloneRepoIntoRoot(t, first)

	assert.Equal(t, root, filepath.Dir(dst))
	assert.NotEqual(t, filepath.Dir(first), root, "the second machine's root is its own, not shared")
	a, err := gitCapture(t, first, "config", "--get", "remote.origin.url")
	require.NoError(t, err)
	b, err := gitCapture(t, dst, "config", "--get", "remote.origin.url")
	require.NoError(t, err)
	assert.Equal(t, a, b, "both clones share the same origin")
}
