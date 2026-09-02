package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// featuresDir holds the executable lease-protocol scenarios: one
// tagged Gherkin scenario per matrix row.
const featuresDir = "../../features"

// TestFeatures runs every scenario declared under features/ as its own
// subtest named by its matrix id, so `-run TestFeatures/S16` picks one
// out. A scenario tagged @pending is declared but unwritten: it is
// skipped, and reported as such, rather than run as a pass that proves
// nothing. Every other scenario runs through godog in strict mode, so a
// step whose text matches no definition fails the build instead of
// passing as undefined. The suite lives in cmd/frit because this is the
// one package where everything a matrix row can name meets: the lease
// API, the repository fixtures, the herdr fake and the verbs.
func TestFeatures(t *testing.T) {
	features, err := godog.TestSuite{
		Options: &godog.Options{Paths: []string{featuresDir}},
	}.RetrieveFeatures()
	require.NoError(t, err)

	for _, f := range features {
		for _, p := range f.Pickles {
			names := make([]string, len(p.Tags))
			for i, tag := range p.Tags {
				names[i] = tag.Name
			}
			id, pending := scenarioTags(names)
			t.Run(id+" "+p.Name, func(t *testing.T) {
				if pending {
					t.Skip("pending: declared in the matrix, its steps not yet written")
				}
				runScenario(t, id)
			})
		}
	}
}

// scenarioTags reads a scenario's matrix id, and whether it is still
// pending, off its tags.
func scenarioTags(tags []string) (id string, pending bool) {
	for _, tag := range tags {
		switch {
		case tag == "@pending":
			pending = true
		case strings.HasPrefix(tag, "@S"):
			id = strings.TrimPrefix(tag, "@")
		}
	}

	return id, pending
}

// runScenario drives the one scenario tagged id through godog on this
// subtest's own goroutine: godog's per-scenario subtests are left off,
// so the world holds the real *testing.T the fixtures need and a
// failing step fails exactly this subtest. godog's report is kept and
// shown only when the scenario fails.
func runScenario(t *testing.T, id string) {
	t.Helper()
	var report bytes.Buffer
	suite := godog.TestSuite{
		ScenarioInitializer: newWorld(t).register,
		Options: &godog.Options{
			Paths:    []string{featuresDir},
			Tags:     "@" + id,
			Strict:   true,
			Format:   "progress",
			NoColors: true,
			Output:   &report,
		},
	}
	if status := suite.Run(); status != 0 {
		t.Errorf("scenario %s: godog exit status %d\n%s", id, status, report.String())
	}
}

// world is the state one scenario threads through its steps: each
// machine's clone keyed by holder, the lease as the first holder took
// it, the takeover that fenced it, the work the first holder never
// pushed, and the error the When step produced.
type world struct {
	t      *testing.T
	planID int
	clones map[string]string
	opts   map[string]claim.LeaseOptions
	holder string
	taker  string
	lease  claim.Lease
	taken  claim.Lease
	local  string
	err    error
}

func newWorld(t *testing.T) *world {
	return &world{
		t:      t,
		clones: map[string]string{},
		opts:   map[string]claim.LeaseOptions{},
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

func (w *world) holdsTheLease(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	repo := claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")
	w.clones[holder] = repo
	w.opts[holder] = leaseFor(holder, planID)

	lease, err := claim.Acquire(repo, w.opts[holder], gitwt.Exec)
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
	require.NoError(w.t, os.WriteFile(filepath.Join(repo, "w.txt"), []byte("wip\n"), 0o600))
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
	first, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	second := cloneAgain(w.t, first)
	w.clones[holder] = second
	w.opts[holder] = leaseFor(holder, w.planID)

	taken, err := claim.Takeover(second, w.opts[holder], w.lease.Tip, gitwt.Exec)
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
	_, w.err = claim.Renew(w.clones[holder], w.opts[holder], w.lease.Tip, gitwt.Exec)

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

func (w *world) pushIsRejected(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, repo, "push", "origin", w.local+":"+w.branch())
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

	sc, err := claim.Yield(repo, w.opts[holder], w.local, gitwt.Exec)
	if err != nil {
		return err
	}
	rescue, err := gitCapture(w.t, repo, "ls-remote", "origin", sc.Rescue)
	if err != nil {
		return fmt.Errorf("%s: %w", rescue, err)
	}
	if !strings.Contains(rescue, w.local) {
		return fmt.Errorf("the rescue ref %s does not carry %s: %q", sc.Rescue, w.local, rescue)
	}

	return w.originHoldsTheTakeover()
}

// originHoldsTheTakeover checks the work ref on origin still sits at
// the takeover's tip — nothing the fenced holder did moved it.
func (w *world) originHoldsTheTakeover() error {
	repo := w.clones[w.holder]
	if got := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec); got != w.taken.Tip {
		return fmt.Errorf("origin holds %s, want the takeover %s", got, w.taken.Tip)
	}

	return nil
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

func TestScenarioTagsReadsTheIDAndWhetherPending(t *testing.T) {
	id, pending := scenarioTags([]string{"@S16"})
	assert.Equal(t, "S16", id)
	assert.False(t, pending)

	id, pending = scenarioTags([]string{"@S17", "@pending"})
	assert.Equal(t, "S17", id)
	assert.True(t, pending)
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

// TestBindAllCallsEveryRegistrar: a section's step file appends its
// binder to the registry from init, and a scenario run binds every one
// of them on the subtest's own *testing.T — so a new section adds a
// file, never a line to this one.
func TestBindAllCallsEveryRegistrar(t *testing.T) {
	var seen []*testing.T
	probe := func(pt *testing.T, _ *godog.ScenarioContext) { seen = append(seen, pt) }

	bindAll(t, nil, []registrar{probe, probe})

	assert.Equal(t, []*testing.T{t, t}, seen)
	assert.NotEmpty(t, registrars, "the lease steps register themselves")
}
