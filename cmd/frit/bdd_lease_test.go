package main

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The lease vocabulary: a machine holds, takes over, renews, is fenced,
// yields. It registers itself, like every section's step file, so a
// section adds a file and never a line here.
func init() {
	registrars = append(registrars, (*world).register)
}

// world is the state one scenario threads through its steps: each
// machine's clone keyed by holder, the lease as the first holder took
// it, the takeover that fenced it, the work the first holder never
// pushed, and the error the When step produced.
type world struct {
	t      *testing.T
	planID int
	clones map[string]string
	holder string
	taker  string
	lease  claim.Lease
	taken  claim.Lease
	local  string
	err    error
	// sections holds each section's own state, keyed by type; see
	// section in bdd_test.go.
	sections map[reflect.Type]any
}

func newWorld(t *testing.T) *world {
	return &world{
		t:        t,
		clones:   map[string]string{},
		sections: map[reflect.Type]any{},
	}
}

// register binds the step texts to the world. Every quoted machine
// name is load-bearing: a step naming a machine the scenario never
// introduced, or the wrong machine for its role, fails rather than
// passing on whatever the world happens to hold.
func (w *world) register(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]+)" holds the lease for plan (\d+)$`, w.holdsTheLease)
	sc.Step(`^"([^"]+)" commits work on the lane it never pushes$`, w.commitsUnpushedWork)
	sc.Step(`^"([^"]+)" takes the lease over$`, w.takesTheLeaseOver)
	sc.Step(`^"([^"]+)" comes back and renews its lease$`, w.comesBackAndRenews)
	sc.Step(`^the renewal is fenced, naming "([^"]+)"$`, w.theRenewalIsFencedNaming)
	sc.Step(`^the error suggests yield$`, w.theErrorSuggestsYield)
	sc.Step(`^"([^"]+)"'s sibling history is left where it was$`, w.siblingHistoryIsLeft)
	sc.Step(`^"([^"]+)"'s push of that work is rejected$`, w.pushIsRejected)
	sc.Step(`^yield parks "([^"]+)"'s work and leaves "([^"]+)"'s takeover untouched$`, w.yieldParks)
}

// leaseFor is the lease a machine acquires or takes over with: origin
// as the remote, its default branch as the base, and a lane named for
// the machine.
func leaseFor(holder string, planID int) claim.LeaseOptions {
	return claim.LeaseOptions{
		PlanID: int64(planID),
		Remote: "origin",
		Base:   "origin/main",
		Holder: holder,
		Lane:   "/lanes/" + holder,
	}
}

// cloneOf finds the clone a named machine works in, refusing a machine
// the scenario never introduced.
func (w *world) cloneOf(holder string) (string, error) {
	repo, ok := w.clones[holder]
	if !ok {
		return "", fmt.Errorf("no machine %q in this scenario", holder)
	}

	return repo, nil
}

func (w *world) branch() string {
	return "refs/heads/" + claim.Branch(int64(w.planID))
}

// cloneAs makes a fresh clone off from's own clone and registers it
// under holder — the shape every step that mints a second claimant's
// clone shares, from a takeover to a race attempt to a rename.
func (w *world) cloneAs(from, holder string) (string, error) {
	first, err := w.cloneOf(from)
	if err != nil {
		return "", err
	}
	repo := cloneAgain(w.t, first)
	w.clones[holder] = repo

	return repo, nil
}

// originTipIs checks a holder's clone of origin's remote tip against
// an expected lease tip — the read every Then step confirming an
// origin fact reduces to.
func (w *world) originTipIs(holder, want string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	got := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if got != want {
		return fmt.Errorf("origin holds %s, want %q's tip %s", got, holder, want)
	}

	return nil
}

// verifyRescue runs a fenced lane's own Yield and confirms the rescue
// ref it wrote on origin actually points at local — the fact every
// yield step checks before asking what else moved.
func (w *world) verifyRescue(repo, holder, local string) error {
	sc, err := claim.Yield(repo, leaseFor(holder, w.planID), local, gitwt.Exec)
	if err != nil {
		return err
	}
	rescue, err := gitCapture(w.t, repo, "ls-remote", "origin", sc.Rescue)
	if err != nil {
		return fmt.Errorf("%s: %w", rescue, err)
	}
	// The rescue ref's name carries the tip too, so it is the object
	// column that says what origin parked, not the line as a whole.
	if fields := strings.Fields(rescue); len(fields) == 0 || fields[0] != local {
		return fmt.Errorf("the rescue ref %s does not point at %s: %q", sc.Rescue, local, rescue)
	}

	return nil
}

func (w *world) holdsTheLease(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	repo := claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")
	w.clones[holder] = repo

	lease, err := claim.Acquire(repo, leaseFor(holder, planID), gitwt.Exec)
	if err != nil {
		return err
	}
	w.holder, w.lease = holder, lease

	return nil
}

func (w *world) commitsUnpushedWork(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	git(w.t, repo, "checkout", "-q", claim.Branch(int64(w.planID)))
	writeFile(w.t, repo, "w.txt", "wip\n")
	git(w.t, repo, "add", "-A")
	git(w.t, repo, "commit", "-q", "-m", "unlanded work")
	tip, err := gitCapture(w.t, repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	git(w.t, repo, "checkout", "-q", "main")
	w.local = tip

	return nil
}

func (w *world) takesTheLeaseOver(holder string) error {
	if holder == w.holder {
		return fmt.Errorf("%q already holds the lease; a takeover comes from another machine", holder)
	}
	second, err := w.cloneAs(w.holder, holder)
	if err != nil {
		return err
	}

	taken, err := claim.Takeover(second, leaseFor(holder, w.planID), w.lease.Tip, gitwt.Exec)
	if err != nil {
		return err
	}
	w.taker, w.taken = holder, taken

	return nil
}

func (w *world) comesBackAndRenews(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	_, w.err = claim.Renew(w.clones[holder], leaseFor(holder, w.planID), w.lease.Tip, gitwt.Exec)

	return nil
}

func (w *world) theRenewalIsFencedNaming(holder string) error {
	var fenced *claim.FenceError
	if !errors.As(w.err, &fenced) {
		return fmt.Errorf("expected a fence, got %v", w.err)
	}
	if fenced.Marker.Holder != holder {
		return fmt.Errorf("the fence names %q, want %q", fenced.Marker.Holder, holder)
	}
	if !strings.Contains(w.err.Error(), holder) {
		return fmt.Errorf("the error %q does not name %q", w.err, holder)
	}

	return nil
}

func (w *world) theErrorSuggestsYield() error {
	if w.err == nil || !strings.Contains(w.err.Error(), "yield") {
		return fmt.Errorf("the error %v does not suggest yield", w.err)
	}

	return nil
}

func (w *world) siblingHistoryIsLeft(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	tip, err := gitCapture(w.t, repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	if tip != w.local {
		return fmt.Errorf("the local work ref moved to %s, want the unpushed %s", tip, w.local)
	}

	return nil
}

// unpushedWork is the tip the commits step left unpushed. The push and
// yield steps stand on it: with no tip, the push would be a delete of
// the ref under test and yield would park nothing, so both refuse
// rather than pass on an empty world.
func (w *world) unpushedWork(holder string) (string, error) {
	if w.local == "" {
		return "", fmt.Errorf("%q has no unpushed work; the commits step comes first", holder)
	}

	return w.local, nil
}

func (w *world) pushIsRejected(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	local, err := w.unpushedWork(holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, repo, "push", "origin", local+":"+w.branch())
	if err == nil {
		return fmt.Errorf("origin accepted the sibling history: %s", out)
	}

	return w.originHoldsTheTakeover()
}

func (w *world) yieldParks(holder, taker string) error {
	if holder != w.holder {
		return fmt.Errorf("%q is not the fenced holder, %q is", holder, w.holder)
	}
	if taker != w.taker {
		return fmt.Errorf("%q did not take the lease over, %q did", taker, w.taker)
	}
	repo := w.clones[holder]
	local, err := w.unpushedWork(holder)
	if err != nil {
		return err
	}
	if err := w.verifyRescue(repo, holder, local); err != nil {
		return err
	}

	return w.originHoldsTheTakeover()
}

// originHoldsTheTakeover checks the work ref on origin still sits at
// the takeover's tip — nothing the fenced holder did moved it.
func (w *world) originHoldsTheTakeover() error {
	return w.originTipIs(w.holder, w.taken.Tip)
}

// cloneAgain makes a second working clone of the origin repo points
// at, so a second machine can move the same plan's work ref.
func cloneAgain(t *testing.T, repo string) string {
	t.Helper()
	origin, err := gitCapture(t, repo, "config", "--get", "remote.origin.url")
	require.NoError(t, err, origin)
	dst := t.TempDir()
	git(t, dst, "clone", "-q", origin, dst)
	git(t, dst, "config", "user.email", "t2@example.com")
	git(t, dst, "config", "user.name", "frit-test-2")

	return dst
}

func TestLeaseForNamesTheMachineAndItsLane(t *testing.T) {
	got := leaseFor("box-a", 7)
	assert.Equal(t, claim.LeaseOptions{
		PlanID: 7, Remote: "origin", Base: "origin/main", Holder: "box-a", Lane: "/lanes/box-a",
	}, got)
}

// TestWorldRefusesAMachineItNeverMet: the quoted names in a step are
// checked against the roles the scenario set up, so a scenario cannot
// pass by naming the wrong machine.
func TestWorldRefusesAMachineItNeverMet(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	w.clones["box-a"] = t.TempDir()

	_, err := w.cloneOf("ghost")
	require.Error(t, err)
	require.Error(t, w.takesTheLeaseOver("box-a"), "the holder cannot take over from itself")
	require.Error(t, w.comesBackAndRenews("box-b"), "only the holder renews")
	require.Error(t, w.yieldParks("box-b", "box-a"), "roles cannot be swapped")
	require.Error(t, w.commitsUnpushedWork("ghost"))
}

// TestWorldRefusesToPushOrYieldWorkItNeverCommitted: the push and
// yield steps stand on the unpushed tip the commits step recorded.
// Without it the push would be `git push origin :<ref>` — a delete of
// the ref under test — and yield would park nothing and pass; both
// refuse before touching git.
func TestWorldRefusesToPushOrYieldWorkItNeverCommitted(t *testing.T) {
	w := newWorld(t)
	w.holder, w.taker = "box-a", "box-b"
	w.clones["box-a"] = t.TempDir()

	err := w.pushIsRejected("box-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no unpushed work")

	err = w.yieldParks("box-a", "box-b")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no unpushed work")
}

// TestThenStepsReadTheRenewalError: the fence step wants a fence that
// names the taker, and the yield hint step wants the word in the
// error; a missing or wrong error fails each rather than passing on
// an empty world.
func TestThenStepsReadTheRenewalError(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theRenewalIsFencedNaming("box-b"), "no error is no fence")
	require.Error(t, w.theErrorSuggestsYield(), "no error suggests nothing")

	w.err = &claim.FenceError{Marker: claim.Marker{Holder: "box-c"}}
	require.Error(t, w.theRenewalIsFencedNaming("box-b"), "the fence names another machine")

	w.err = fmt.Errorf("fenced by box-b: run yield")
	require.Error(t, w.theRenewalIsFencedNaming("box-b"), "a plain error is no fence")
	require.NoError(t, w.theErrorSuggestsYield())
}

// TestCloneAgainSharesTheOrigin: the second clone points at the same
// bare origin the first one pushes to, so the two race for one ref.
func TestCloneAgainSharesTheOrigin(t *testing.T) {
	isolate(t)
	first := claimableRepo(t, t.TempDir(), "atlas", 7, "Shader unit")
	second := cloneAgain(t, first)

	a, err := gitCapture(t, first, "config", "--get", "remote.origin.url")
	require.NoError(t, err)
	b, err := gitCapture(t, second, "config", "--get", "remote.origin.url")
	require.NoError(t, err)
	assert.Equal(t, a, b)
}
