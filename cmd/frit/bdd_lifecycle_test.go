package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The lifecycle vocabulary: a plan file renamed or reused, a work ref
// gone from under a lease, a claim dated against a base that moved,
// origin's own default branch renamed underneath a clone. It registers
// itself, like every section's step file, so a section adds a file and
// never a line to bdd_test.go.
func init() {
	registrars = append(registrars, (*world).registerLifecycle)
}

// lifecycleState is this section's own state beside the shared world:
// the root and repository a claim runs against, the sha a lease was
// dated on before origin moved, the origin and clone S75 renames in
// place, and the last CLI verb's own captured output — kept here, not
// on world, so this section adds a file and never a field to it.
type lifecycleState struct {
	root, repo    string
	staleBase     string
	origin, clone string
	out, errOut   string
	code          int
}

func (w *world) registerLifecycle(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]+)" acquires the lease for plan (\d+)$`, w.acquiresTheLeaseForPlan)
	sc.Step(`^"([^"]+)" loses to the live lease$`, w.losesToTheLiveLease)
	sc.Step(`^the plan file is renamed on main and pushed$`, w.thePlanFileIsRenamedOnMainAndPushed)
	sc.Step(`^origin carries exactly one refs/heads/plan/\* ref, with no slug in its name$`,
		w.originCarriesExactlyOneRefWithNoSlug)

	sc.Step(`^plans (\d+) and (\d+) share a title$`, w.plansShareATitle)
	sc.Step(`^origin holds refs/heads/plan/(\d+) and refs/heads/plan/(\d+), `+
		`two refs, neither naming the shared title$`, w.originHoldsRefsForTwoPlans)

	sc.Step(`^"([^"]+)" deletes its local branch by hand$`, w.deletesItsLocalBranchByHand)
	sc.Step(`^the local branch is restored at the renewed tip$`, w.theLocalBranchIsRestoredAtTheRenewedTip)

	sc.Step(`^a claimable plan (\d+)$`, w.aClaimablePlan)
	sc.Step(`^origin's main moves past the clone's last fetch$`, w.originsMainMovesPastTheClonesLastFetch)
	sc.Step(`^frit claims plan (\d+)$`, w.fritClaimsPlan)
	sc.Step(`^the claim marker's base names origin's current main$`, w.theClaimMarkersBaseNamesOriginsCurrentMain)

	sc.Step(`^a clone of an origin whose default branch is "([^"]+)"$`, w.aCloneOfAnOriginWhoseDefaultBranchIs)
	sc.Step(`^origin renames its default branch to "([^"]+)"$`, w.originRenamesItsDefaultBranchTo)
	sc.Step(`^the clone re-reads origin's HEAD$`, w.theCloneReReadsOriginsHead)
	sc.Step(`^DefaultRef answers "([^"]+)"$`, w.defaultRefAnswers)
}

// acquiresTheLeaseForPlan drives claim.Acquire directly for a named
// machine: its own clone when the scenario already gave it one — the
// same repository, a second plan id acquired alongside the first, as
// S51 needs — else a fresh clone of the holder's origin, the shape a
// second claimant's own machine takes everywhere else in this file.
// The result rides on w.err, the one field every Then step in this
// section reads back.
func (w *world) acquiresTheLeaseForPlan(holder string, planID int) error {
	repo, ok := w.clones[holder]
	if !ok {
		var err error
		repo, err = w.cloneAs(w.holder, holder)
		if err != nil {
			return err
		}
	}
	_, err := claim.Acquire(repo, leaseFor(holder, planID), gitwt.Exec)
	w.err = err

	return nil
}

// losesToTheLiveLease checks the last acquire this section drove:
// a HeldError naming the scenario's own live holder, not merely any
// error, so a scenario cannot pass on the wrong failure.
func (w *world) losesToTheLiveLease(holder string) error {
	if holder == w.holder {
		return fmt.Errorf("%q holds the lease; it cannot lose to itself", holder)
	}
	var held *claim.HeldError
	if !errors.As(w.err, &held) {
		return fmt.Errorf("%q's acquire was expected to lose, got %v", holder, w.err)
	}
	if held.Known && held.Marker.Holder != w.holder {
		return fmt.Errorf("%q's acquire lost to %q, want %q", holder, held.Marker.Holder, w.holder)
	}

	return nil
}

// thePlanFileIsRenamedOnMainAndPushed renames the plan's own markdown
// file on the holder's checkout of main and pushes the rename to
// origin — S50's own When. claim.Branch derives the work ref from the
// plan id alone, so nothing this rename does can reach it; that is
// the row's whole point, proven by the ref count Then step reads back.
func (w *world) thePlanFileIsRenamedOnMainAndPushed() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	matches, err := planFileMatches(repo, w.planID)
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return fmt.Errorf("expected one plan file for plan %d, found %d", w.planID, len(matches))
	}
	renamed := planFileRenamed(matches[0], w.planID)
	git(w.t, repo, "mv", matches[0], renamed)
	git(w.t, repo, "commit", "-q", "-m", "rename plan file")
	git(w.t, repo, "push", "-q", "origin", "main")

	return nil
}

// originCarriesExactlyOneRefWithNoSlug reads origin's own refs/heads/
// plan/* namespace back: exactly one ref, and its name is the id-only
// branch claim.Branch mints — never a name carrying the file's own
// slug, whatever that file was last renamed to.
func (w *world) originCarriesExactlyOneRefWithNoSlug() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	refs, err := gitCapture(w.t, repo, "ls-remote", "--heads", "origin", "refs/heads/plan/*")
	if err != nil {
		return fmt.Errorf("%s: %w", refs, err)
	}
	names := refNames(refs)
	if len(names) != 1 {
		return fmt.Errorf("origin carries %d plan/* refs, want 1: %v", len(names), names)
	}
	if names[0] != w.branch() {
		return fmt.Errorf("origin's one plan/* ref is %q, want %q", names[0], w.branch())
	}

	return nil
}

// plansShareATitle builds one repository holding two plans whose
// titles collide, so their files share a slug — S51's own Given.
// claim.Branch never reads a file name, which is the whole answer:
// two ids can share a slug and still mint two distinct refs.
func (w *world) plansShareATitle(idA, idB int) error {
	isolate(w.t)
	w.holder = "box-a"
	root := w.t.TempDir()
	repo := claimableRepo(w.t, root, "atlas", idA, "Shader unit")
	commitPlan(w.t, repo, idB, "🔲", "Shader unit", nil, "")
	git(w.t, repo, "push", "-q", "origin", "main")
	w.clones[w.holder] = repo

	return nil
}

// originHoldsRefsForTwoPlans checks S51's own Then: origin's plan/*
// namespace holds exactly the two id-only refs the two acquires
// minted, and nothing named for the title the two plans share.
func (w *world) originHoldsRefsForTwoPlans(idA, idB int) error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	refs, err := gitCapture(w.t, repo, "ls-remote", "--heads", "origin", "refs/heads/plan/*")
	if err != nil {
		return fmt.Errorf("%s: %w", refs, err)
	}
	names := refNames(refs)
	want := map[string]bool{
		"refs/heads/" + claim.Branch(int64(idA)): true,
		"refs/heads/" + claim.Branch(int64(idB)): true,
	}
	if len(names) != 2 {
		return fmt.Errorf("origin carries %d plan/* refs, want 2: %v", len(names), names)
	}
	for _, n := range names {
		if !want[n] {
			return fmt.Errorf("origin's plan/* refs are %v, want exactly %v", names, want)
		}
	}

	return nil
}

// deletesItsLocalBranchByHand drops a holder's own local copy of the
// lease branch, the way a person cleaning up a checkout would, without
// touching origin — S56's own When.
func (w *world) deletesItsLocalBranchByHand(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	git(w.t, repo, "branch", "-D", claim.Branch(int64(w.planID)))

	return nil
}

// theLocalBranchIsRestoredAtTheRenewedTip checks S56's own second
// Then: the renewal this section's comesBackAndRenews step drove
// succeeded, and the local branch it deleted by hand exists again,
// matching origin's own tip exactly — casPush's syncLocalRef, proven
// from outside the package.
func (w *world) theLocalBranchIsRestoredAtTheRenewedTip() error {
	if w.err != nil {
		return fmt.Errorf("the renewal failed: %w", w.err)
	}
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	local, err := gitCapture(w.t, repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("the local branch was not restored: %s: %w", local, err)
	}
	remote, err := gitCapture(w.t, repo, "ls-remote", "origin", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", remote, err)
	}
	if !strings.Contains(remote, local) {
		return fmt.Errorf("the restored local branch is %s, origin's tip is %q", local, remote)
	}

	return nil
}

// aClaimablePlan builds a claimable plan and records the base sha the
// clone would date a claim against without ever fetching again — the
// stale copy S70's own Then step proves the claim marker does not
// carry.
func (w *world) aClaimablePlan(planID int) error {
	isolate(w.t)
	w.planID = planID
	root := w.t.TempDir()
	repo := claimableRepo(w.t, root, "atlas", planID, "Shader unit")
	st := section[lifecycleState](w)
	st.root, st.repo = root, repo
	stale, err := gitCapture(w.t, repo, "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return fmt.Errorf("%s: %w", stale, err)
	}
	st.staleBase = stale

	return nil
}

// originsMainMovesPastTheClonesLastFetch pushes a fresh commit onto
// origin's main from a second clone, moving origin ahead of what the
// claimable plan's own clone last fetched — S70's own Given, the seam
// the gather's own fetch is meant to close before a claim dates
// itself against the base.
func (w *world) originsMainMovesPastTheClonesLastFetch() error {
	st := section[lifecycleState](w)
	if st.repo == "" {
		return fmt.Errorf("no claimable plan set up; the claimable-plan step comes first")
	}
	second := cloneAgain(w.t, st.repo)
	writeFile(w.t, second, "race.txt", "moved\n")
	git(w.t, second, "add", "-A")
	git(w.t, second, "commit", "-q", "-m", "move main on")
	git(w.t, second, "push", "-q", "origin", "main")

	return nil
}

// fritClaimsPlan runs `frit claim` against the claimable plan's own
// repository, with herdr faked so a winning claim can stand its lane
// up — S70's own When, driving the verb rather than the lease API
// alone, since the verb's own gather is the fetch under test.
func (w *world) fritClaimsPlan(planID int) error {
	st := section[lifecycleState](w)
	if st.root == "" {
		return fmt.Errorf("no root to claim from; the claimable-plan step comes first")
	}
	runner, _ := startHerdr()
	withHerdr(w.t, runner)
	var out, errb strings.Builder
	code := run([]string{"claim", strconv.Itoa(planID), "--root", st.root}, &out, &errb)
	st.out, st.errOut, st.code = out.String(), errb.String(), code

	return nil
}

// theClaimMarkersBaseNamesOriginsCurrentMain reads the claim marker
// the last `frit claim` minted and checks its base trailer names
// origin's current main — the fresh sha the verb's own fetch read,
// never the stale copy the clone would have dated against without it.
func (w *world) theClaimMarkersBaseNamesOriginsCurrentMain() error {
	st := section[lifecycleState](w)
	tip, err := gitCapture(w.t, st.repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("the claim minted no work ref: %s: %w", tip, err)
	}
	m, ok := claim.ReadMarker(st.repo, leaseFor(w.holder, w.planID), tip, gitwt.Exec)
	if !ok {
		return fmt.Errorf("no lease marker readable at %s", tip)
	}
	fresh, err := gitCapture(w.t, st.repo, "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return fmt.Errorf("%s: %w", fresh, err)
	}
	if m.Base != fresh {
		return fmt.Errorf("the claim marker's base is %s, want origin's current main %s", m.Base, fresh)
	}
	if m.Base == st.staleBase {
		return fmt.Errorf(
			"the claim marker's base %s is the clone's stale copy, not origin's current main", m.Base)
	}

	return nil
}

// aCloneOfAnOriginWhoseDefaultBranchIs builds a bare origin, seeds it
// on the named branch, and clones it — S75's own Given, a real clone
// so `git remote set-head` has a remote to ask.
func (w *world) aCloneOfAnOriginWhoseDefaultBranchIs(branch string) error {
	isolate(w.t)
	st := section[lifecycleState](w)
	root := w.t.TempDir()
	origin := filepath.Join(root, "origin.git")
	git(w.t, root, "init", "-q", "--bare", "-b", branch, origin)

	seed := filepath.Join(root, "seed")
	git(w.t, root, "init", "-q", "-b", branch, seed)
	git(w.t, seed, "config", "user.email", "t@example.com")
	git(w.t, seed, "config", "user.name", "frit-test")
	writeFile(w.t, seed, "f.txt", "seed\n")
	git(w.t, seed, "add", "-A")
	git(w.t, seed, "commit", "-q", "-m", "seed")
	git(w.t, seed, "remote", "add", "origin", origin)
	git(w.t, seed, "push", "-q", "origin", branch)

	clone := filepath.Join(root, "clone")
	git(w.t, root, "clone", "-q", origin, clone)
	st.origin, st.clone = origin, clone

	return nil
}

// originRenamesItsDefaultBranchTo renames origin's own default branch
// in place and repoints its HEAD there, leaving the clone untouched
// until it re-reads origin's own HEAD — S75's own When, run twice to
// prove the second rename is read fresh too.
func (w *world) originRenamesItsDefaultBranchTo(name string) error {
	st := section[lifecycleState](w)
	if st.origin == "" {
		return fmt.Errorf("no origin to rename; the clone-of-an-origin step comes first")
	}
	current, err := gitCapture(w.t, st.origin, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return fmt.Errorf("%s: %w", current, err)
	}
	git(w.t, st.origin, "branch", "-m", current, name)
	git(w.t, st.origin, "symbolic-ref", "HEAD", "refs/heads/"+name)

	return nil
}

// theCloneReReadsOriginsHead fetches origin's own moved branch and
// re-derives the clone's refs/remotes/origin/HEAD symref from it —
// the `git remote set-head -a` a clone runs to catch a renamed
// default branch, gitobj.DefaultRef's own seam.
func (w *world) theCloneReReadsOriginsHead() error {
	st := section[lifecycleState](w)
	if st.clone == "" {
		return fmt.Errorf("no clone to re-read; the clone-of-an-origin step comes first")
	}
	git(w.t, st.clone, "fetch", "-q", "origin")
	git(w.t, st.clone, "remote", "set-head", "origin", "-a")

	return nil
}

// defaultRefAnswers checks gitobj.DefaultRef reads the clone's own,
// freshly re-read HEAD symref uncached — S75's own Then, run twice
// against two different names so a cached first read cannot pass.
func (w *world) defaultRefAnswers(want string) error {
	st := section[lifecycleState](w)
	got := gitobj.DefaultRef(st.clone, gitwt.Exec)
	if got != want {
		return fmt.Errorf("DefaultRef answered %q, want %q", got, want)
	}

	return nil
}

// planFileMatches globs a plan's own markdown file by id, the shape
// S27's fixture and S50's own rename step both need.
func planFileMatches(repo string, planID int) ([]string, error) {
	return filepath.Glob(filepath.Join(repo, "plan", fmt.Sprintf("%d_*.md", planID)))
}

// planFileRenamed builds a new path for a plan file that keeps its id
// but drops its old slug entirely, proving the rename carries no trace
// of the title a slug is derived from.
func planFileRenamed(path string, planID int) string {
	return filepath.Join(filepath.Dir(path), fmt.Sprintf("%d_renamed.md", planID))
}

// refNames extracts the ref column from `ls-remote`'s
// "<sha>\t<ref>" lines, skipping the blank a no-match run answers.
func refNames(lsRemote string) []string {
	var names []string
	for _, line := range strings.Split(lsRemote, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 2 {
			names = append(names, fields[1])
		}
	}

	return names
}

// TestRefNamesSkipsBlankAndReadsTheRefColumn: ls-remote's own tab
// layout is <sha>\t<ref>; a Then step reads the ref, never the sha,
// and a no-match run's blank line yields no names at all.
func TestRefNamesSkipsBlankAndReadsTheRefColumn(t *testing.T) {
	assert.Empty(t, refNames(""))
	got := refNames("abc123\trefs/heads/plan/7\ndef456\trefs/heads/plan/8\n")
	assert.Equal(t, []string{"refs/heads/plan/7", "refs/heads/plan/8"}, got)
}

// TestPlanFileRenamedDropsTheSlug: the renamed path keeps the plan's
// id and the file's own directory, but carries no trace of the old
// slug — the point S50 and S27 both prove at the ref level.
func TestPlanFileRenamedDropsTheSlug(t *testing.T) {
	got := planFileRenamed("/tmp/x/plan/7_shader-unit.md", 7)
	assert.Equal(t, "/tmp/x/plan/7_renamed.md", got)
	assert.NotContains(t, got, "shader-unit")
}

// TestAcquiresTheLeaseForPlanRefusesAnUnknownHolder: a holder the
// scenario never introduced, and no lease already held to clone from,
// is refused rather than silently minting a clone from nothing.
func TestAcquiresTheLeaseForPlanRefusesAnUnknownHolder(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.acquiresTheLeaseForPlan("ghost", 7))
}

// TestLosesToTheLiveLeaseRefusesTheLiveHolderItself: a scenario cannot
// claim the live holder lost to itself, and a nil error is not a loss.
func TestLosesToTheLiveLeaseRefusesTheLiveHolderItself(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	require.Error(t, w.losesToTheLiveLease("box-a"), "the holder cannot lose to itself")
	require.Error(t, w.losesToTheLiveLease("box-b"), "no error is no loss")
}

// TestTheLocalBranchIsRestoredAtTheRenewedTipRefusesAFailedRenewal: a
// prior renewal error is read back and reported, not papered over by
// checking the local branch anyway.
func TestTheLocalBranchIsRestoredAtTheRenewedTipRefusesAFailedRenewal(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	w.err = fmt.Errorf("fenced")
	require.Error(t, w.theLocalBranchIsRestoredAtTheRenewedTip())
}

// TestOriginsMainMovesPastTheClonesLastFetchRefusesWithNoPlanYet: the
// staleness step needs a repository the claimable-plan step already
// built.
func TestOriginsMainMovesPastTheClonesLastFetchRefusesWithNoPlanYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.originsMainMovesPastTheClonesLastFetch())
}

// TestFritClaimsPlanRefusesWithNoRootYet: the claim step needs a root
// the claimable-plan step already built.
func TestFritClaimsPlanRefusesWithNoRootYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.fritClaimsPlan(7))
}

// TestOriginRenamesItsDefaultBranchToRefusesWithNoOriginYet: the
// rename step needs the origin the clone-of-an-origin step already
// built.
func TestOriginRenamesItsDefaultBranchToRefusesWithNoOriginYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.originRenamesItsDefaultBranchTo("trunk"))
}

// TestTheCloneReReadsOriginsHeadRefusesWithNoCloneYet: the re-read
// step needs the clone the clone-of-an-origin step already built.
func TestTheCloneReReadsOriginsHeadRefusesWithNoCloneYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theCloneReReadsOriginsHead())
}
