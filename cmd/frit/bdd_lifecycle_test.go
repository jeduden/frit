package main

import (
	"errors"
	"fmt"
	"os"
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
	releaseTip    string
	unlandedTip   string
	rescue        string
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

	sc.Step(`^plan (\d+) is done and its lease is released$`, w.planIsDoneAndItsLeaseIsReleased)
	sc.Step(`^a different plan's file replaces it under the same id (\d+)$`,
		w.aDifferentPlansFileReplacesItUnderTheSameID)
	sc.Step(`^the plan file is marked done and then re-opened$`, w.thePlanFileIsMarkedDoneAndThenReopened)
	sc.Step(`^the released ref is scavenged by evidence$`, w.theReleasedRefIsScavengedByEvidence)
	sc.Step(`^origin carries no plan/(\d+) ref$`, w.originCarriesNoPlanRef)
	sc.Step(`^frit claims plan (\d+) fresh at epoch (\d+)$`, w.fritClaimsPlanFreshAtEpoch)
	sc.Step(`^plan (\d+) is merged into main with its branch already auto-deleted$`,
		w.planIsMergedIntoMainWithItsBranchAlreadyAutoDeleted)

	sc.Step(`^plan (\d+) is claimed and carries unlanded work$`, w.planIsClaimedAndCarriesUnlandedWork)
	sc.Step(`^the plan file is deleted from main and pushed$`, w.thePlanFileIsDeletedFromMainAndPushed)
	sc.Step(`^the ref is scavenged by evidence$`, w.theRefIsScavengedByEvidence)
	sc.Step(`^the rescue ref carries the unlanded work$`, w.theRescueRefCarriesTheUnlandedWork)

	sc.Step(`^"([^"]+)"'s claim succeeds at epoch (\d+)$`, w.claimSucceedsAtEpoch)
	sc.Step(`^origin's tip is "([^"]+)"'s claim$`, w.originsTipIsTheClaim)
}

// acquiresTheLeaseForPlan drives claim.Acquire directly for a named
// machine: its own clone when the scenario already gave it one — the
// same repository, a second plan id acquired alongside the first, as
// S51 needs — else a fresh clone of the holder's origin, the shape a
// second claimant's own machine takes everywhere else in this file.
// The result rides on w.err and, on success, w.lease — the two fields
// this section's own Then steps read back, w.lease shared with
// bdd_lease_test.go's own steps the same way.
func (w *world) acquiresTheLeaseForPlan(holder string, planID int) error {
	repo, ok := w.clones[holder]
	if !ok {
		var err error
		repo, err = w.cloneAs(w.holder, holder)
		if err != nil {
			return err
		}
	}
	lease, err := claim.Acquire(repo, leaseFor(holder, planID), gitwt.Exec)
	w.err = err
	if err == nil {
		w.lease = lease
	}

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

// planIsDoneAndItsLeaseIsReleased builds a claimable plan, acquires
// its lease and releases it — S53's, S57's and S58's shared Given. The
// released tip is recorded so the scavenge step has exactly what
// claim.Scavenge needs to CAS against, the same shape
// TestClaimReacquiresAReleasedLease builds before reacquiring without
// ever scavenging. The repo is registered under w.clones[w.holder] too,
// so S58's own second machine can clone from it via acquiresTheLeaseForPlan.
func (w *world) planIsDoneAndItsLeaseIsReleased(planID int) error {
	isolate(w.t)
	w.holder = "box-a"
	w.planID = planID
	root := w.t.TempDir()
	repo := claimableRepo(w.t, root, "atlas", planID, "Shader unit")
	w.clones[w.holder] = repo
	st := section[lifecycleState](w)
	st.root, st.repo = root, repo
	opts := leaseFor(w.holder, planID)
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	if err != nil {
		return err
	}
	released, err := claim.Release(repo, opts, lease.Tip, gitwt.Exec)
	if err != nil {
		return err
	}
	st.releaseTip = released.Tip

	return nil
}

// aDifferentPlansFileReplacesItUnderTheSameID drops the plan's own
// file and commits a fresh one under the same id — S53's own When,
// the id-reuse the lease protocol calls forbidden in practice (minute
// ids) but the scavenge mechanics must still answer for if it ever
// happens by hand.
func (w *world) aDifferentPlansFileReplacesItUnderTheSameID(planID int) error {
	st := section[lifecycleState](w)
	if st.repo == "" {
		return fmt.Errorf("no plan set up; the done-and-released step comes first")
	}
	matches, err := planFileMatches(st.repo, planID)
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return fmt.Errorf("expected one plan file for plan %d, found %d", planID, len(matches))
	}
	if err := os.Remove(matches[0]); err != nil {
		return err
	}
	commitPlan(w.t, st.repo, planID, "🔲", "Different shader work", nil, "")
	git(w.t, st.repo, "push", "-q", "origin", "main")

	return nil
}

// thePlanFileIsMarkedDoneAndThenReopened flips the plan's own file to
// ✅ and back to 🔲 in place — S57's own When, the same file
// throughout, unlike S53's replacement.
func (w *world) thePlanFileIsMarkedDoneAndThenReopened() error {
	st := section[lifecycleState](w)
	if st.repo == "" {
		return fmt.Errorf("no plan set up; the done-and-released step comes first")
	}
	commitPlan(w.t, st.repo, w.planID, "✅", "Shader unit", nil, "")
	commitPlan(w.t, st.repo, w.planID, "🔲", "Shader unit", nil, "")
	git(w.t, st.repo, "push", "-q", "origin", "main")

	return nil
}

// theReleasedRefIsScavengedByEvidence drives claim.Scavenge directly
// against the tip the done-and-released step recorded — the mechanism
// the matrix's own outcome column names for S53 and S57, without the
// evidence-detection wiring that would decide on its own to call it;
// that wiring is out of this plan's scope.
func (w *world) theReleasedRefIsScavengedByEvidence() error {
	st := section[lifecycleState](w)
	if st.repo == "" || st.releaseTip == "" {
		return fmt.Errorf("no released lease to scavenge; the done-and-released step comes first")
	}
	_, err := claim.Scavenge(st.repo, leaseFor("scavenger", w.planID), st.releaseTip, gitwt.Exec)

	return err
}

// originCarriesNoPlanRef checks a plan's own work ref is entirely
// gone from origin — the scavenge's delete, not merely a release
// marker left standing on a ref that still exists.
func (w *world) originCarriesNoPlanRef(planID int) error {
	st := section[lifecycleState](w)
	if st.repo == "" {
		return fmt.Errorf("no repo set up yet")
	}
	branch := "refs/heads/" + claim.Branch(int64(planID))
	out, err := gitCapture(w.t, st.repo, "ls-remote", "origin", branch)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	if out != "" {
		return fmt.Errorf("origin still carries %s: %s", branch, out)
	}

	return nil
}

// fritClaimsPlanFreshAtEpoch runs `frit claim`, reusing S70's own
// driver, and checks the fresh claim marker's epoch — 1 whenever the
// ref it lands on was scavenged away entirely first, never a
// continuation of an epoch a still-standing ref would carry. Holder is
// left blank reading the marker back: fetchedMarker never consults it,
// only the plan id and the tip already sitting locally.
func (w *world) fritClaimsPlanFreshAtEpoch(planID, epoch int) error {
	if err := w.fritClaimsPlan(planID); err != nil {
		return err
	}
	st := section[lifecycleState](w)
	if st.code != 0 {
		return fmt.Errorf("claim exited %d: %s", st.code, st.errOut)
	}
	if !strings.Contains(st.out, fmt.Sprintf("claimed plan %d", planID)) {
		return fmt.Errorf("claim did not report success: %s", st.out)
	}
	tip, err := gitCapture(w.t, st.repo, "rev-parse", "refs/heads/"+claim.Branch(int64(planID)))
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	m, ok := claim.ReadMarker(st.repo, leaseFor("", planID), tip, gitwt.Exec)
	if !ok {
		return fmt.Errorf("no lease marker readable at %s", tip)
	}
	if m.Kind != "claim" {
		return fmt.Errorf("the fresh claim's tip is a %q marker, want claim", m.Kind)
	}
	if m.Epoch != epoch {
		return fmt.Errorf("the fresh claim's epoch is %d, want %d", m.Epoch, epoch)
	}

	return nil
}

// planIsMergedIntoMainWithItsBranchAlreadyAutoDeleted builds a plan
// merged into main whose own lease branch was never even minted on
// origin — S55's own Given, a GitHub-style merge-and-auto-delete leaves
// exactly the shape resumableRepo already builds: 🔳 on main, no
// plan/<id> ref anywhere, nothing live to lose to and nothing to
// scavenge.
func (w *world) planIsMergedIntoMainWithItsBranchAlreadyAutoDeleted(planID int) error {
	isolate(w.t)
	w.planID = planID
	root := w.t.TempDir()
	repo := resumableRepo(w.t, root, "atlas", planID, "Shader unit")
	st := section[lifecycleState](w)
	st.root, st.repo = root, repo

	return nil
}

// planIsClaimedAndCarriesUnlandedWork builds a claimable plan, acquires
// its lease and pushes one unlanded commit onto the work ref — S52's
// own Given, the same shape TestScavengeParksUnlandedWorkThenDeletes
// builds in internal/claim's own tests, written locally since that
// package's own workOn helper is unexported. Unlike
// planIsDoneAndItsLeaseIsReleased, the lease is never released: S52's
// point is a plan abandoned mid-claim, not one closed out cleanly.
func (w *world) planIsClaimedAndCarriesUnlandedWork(planID int) error {
	isolate(w.t)
	w.holder = "box-a"
	w.planID = planID
	root := w.t.TempDir()
	repo := claimableRepo(w.t, root, "atlas", planID, "Shader unit")
	st := section[lifecycleState](w)
	st.root, st.repo = root, repo
	opts := leaseFor(w.holder, planID)
	if _, err := claim.Acquire(repo, opts, gitwt.Exec); err != nil {
		return err
	}

	branch := claim.Branch(int64(planID))
	git(w.t, repo, "checkout", "-q", branch)
	writeFile(w.t, repo, "w.txt", "wip\n")
	git(w.t, repo, "add", "-A")
	git(w.t, repo, "commit", "-q", "-m", "unlanded work")
	git(w.t, repo, "push", "-q", "origin", branch)
	git(w.t, repo, "checkout", "-q", "main")
	tip, err := gitCapture(w.t, repo, "rev-parse", "refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	st.unlandedTip = tip

	return nil
}

// thePlanFileIsDeletedFromMainAndPushed removes the plan's own
// markdown file from main and pushes the delete — S52's own When, the
// "plan-gone" fact the matrix names as this row's evidence. No code in
// this repository reads that fact today; this step only makes it true
// on disk, so the next step can drive the scavenge it would eventually
// justify directly, without the missing detection wiring.
func (w *world) thePlanFileIsDeletedFromMainAndPushed() error {
	st := section[lifecycleState](w)
	if st.repo == "" {
		return fmt.Errorf("no plan set up; the claimed-and-carries-work step comes first")
	}
	matches, err := planFileMatches(st.repo, w.planID)
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return fmt.Errorf("expected one plan file for plan %d, found %d", w.planID, len(matches))
	}
	if err := os.Remove(matches[0]); err != nil {
		return err
	}
	git(w.t, st.repo, "add", "-A")
	git(w.t, st.repo, "commit", "-q", "-m", "delete plan file")
	git(w.t, st.repo, "push", "-q", "origin", "main")

	return nil
}

// theRefIsScavengedByEvidence drives claim.Scavenge directly against
// the unlanded tip S52's own Given recorded — the mechanism the
// matrix's own outcome column names for a plan gone while claimed,
// distinct step text from theReleasedRefIsScavengedByEvidence since
// this tip was never released, only abandoned; the underlying call is
// the same one phase 2 already proved.
func (w *world) theRefIsScavengedByEvidence() error {
	st := section[lifecycleState](w)
	if st.repo == "" || st.unlandedTip == "" {
		return fmt.Errorf("no claimed lease to scavenge; the claimed-and-carries-work step comes first")
	}
	sc, err := claim.Scavenge(st.repo, leaseFor("scavenger", w.planID), st.unlandedTip, gitwt.Exec)
	if err != nil {
		return err
	}
	st.rescue = sc.Rescue

	return nil
}

// theRescueRefCarriesTheUnlandedWork checks the matrix's own "PARK
// first" directly: the rescue ref the scavenge step recorded is not
// merely non-empty, but origin's own copy of it actually carries the
// tip that was parked.
func (w *world) theRescueRefCarriesTheUnlandedWork() error {
	st := section[lifecycleState](w)
	if st.repo == "" {
		return fmt.Errorf("no repo set up yet")
	}
	if st.rescue == "" {
		return fmt.Errorf("no rescue ref was recorded; the scavenge step comes first")
	}
	out, err := gitCapture(w.t, st.repo, "ls-remote", "origin", st.rescue)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	if !strings.Contains(out, st.unlandedTip) {
		return fmt.Errorf("rescue ref %s is %q, want it to carry the parked tip %s",
			st.rescue, out, st.unlandedTip)
	}

	return nil
}

// claimSucceedsAtEpoch checks S58's own doc-by-argument observable: a
// named holder's acquire — driven by the shared acquiresTheLeaseForPlan
// step — succeeded, landed at the expected epoch, and minted a claim
// marker naming that same holder, never the released lease's old one.
func (w *world) claimSucceedsAtEpoch(holder string, epoch int) error {
	if w.err != nil {
		return fmt.Errorf("%q's claim failed: %w", holder, w.err)
	}
	if w.lease.Epoch != epoch {
		return fmt.Errorf("%q's claim landed at epoch %d, want %d", holder, w.lease.Epoch, epoch)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	m, ok := claim.ReadMarker(repo, leaseFor(holder, w.planID), w.lease.Tip, gitwt.Exec)
	if !ok {
		return fmt.Errorf("no lease marker readable at %s", w.lease.Tip)
	}
	if m.Kind != "claim" {
		return fmt.Errorf("%q's tip is a %q marker, want claim", holder, m.Kind)
	}
	if m.Holder != holder {
		return fmt.Errorf("the marker names %q as holder, want %q", m.Holder, holder)
	}

	return nil
}

// originsTipIsTheClaim is a thin wrapper over originTipIs,
// bdd_lease_test.go's own shared helper: origin's remote tip for the
// plan matches the named holder's own claim, the fact S58's outcome
// column calls "origin's tip is that claim".
func (w *world) originsTipIsTheClaim(holder string) error {
	return w.originTipIs(holder, w.lease.Tip)
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

// TestADifferentPlansFileReplacesItUnderTheSameIDRefusesWithNoPlanYet:
// the id-reuse step needs the plan the done-and-released step already
// built.
func TestADifferentPlansFileReplacesItUnderTheSameIDRefusesWithNoPlanYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.aDifferentPlansFileReplacesItUnderTheSameID(7))
}

// TestThePlanFileIsMarkedDoneAndThenReopenedRefusesWithNoPlanYet: the
// re-open step needs the plan the done-and-released step already
// built.
func TestThePlanFileIsMarkedDoneAndThenReopenedRefusesWithNoPlanYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.thePlanFileIsMarkedDoneAndThenReopened())
}

// TestTheReleasedRefIsScavengedByEvidenceRefusesWithNoReleaseYet: the
// scavenge step needs the released tip the done-and-released step
// already recorded.
func TestTheReleasedRefIsScavengedByEvidenceRefusesWithNoReleaseYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theReleasedRefIsScavengedByEvidence())
}

// TestOriginCarriesNoPlanRefRefusesWithNoRepoYet: the ref-gone check
// needs a repo some earlier step already built.
func TestOriginCarriesNoPlanRefRefusesWithNoRepoYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.originCarriesNoPlanRef(7))
}

// TestFritClaimsPlanFreshAtEpochRefusesWithNoRootYet: the fresh-claim
// check needs the root some earlier step already built, the same
// guard fritClaimsPlan itself carries.
func TestFritClaimsPlanFreshAtEpochRefusesWithNoRootYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.fritClaimsPlanFreshAtEpoch(7, 1))
}

// TestThePlanFileIsDeletedFromMainAndPushedRefusesWithNoPlanYet: the
// delete step needs the plan the claimed-and-carries-work step already
// built.
func TestThePlanFileIsDeletedFromMainAndPushedRefusesWithNoPlanYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.thePlanFileIsDeletedFromMainAndPushed())
}

// TestTheRefIsScavengedByEvidenceRefusesWithNoClaimYet: the scavenge
// step needs the unlanded tip the claimed-and-carries-work step
// already recorded.
func TestTheRefIsScavengedByEvidenceRefusesWithNoClaimYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theRefIsScavengedByEvidence())
}

// TestTheRescueRefCarriesTheUnlandedWorkRefusesWithNoRescueYet: the
// rescue-ref check needs the ref the scavenge step already recorded,
// not merely a repo.
func TestTheRescueRefCarriesTheUnlandedWorkRefusesWithNoRescueYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theRescueRefCarriesTheUnlandedWork())

	st := section[lifecycleState](w)
	st.repo = t.TempDir()
	require.Error(t, w.theRescueRefCarriesTheUnlandedWork(),
		"a repo alone, with no rescue ref recorded, still refuses")
}

// TestPlanIsClaimedAndCarriesUnlandedWorkBuildsAnUnlandedTip: the
// Given actually pushes real work onto the lease, so the scavenge step
// has something to park, and the lease it acquires is never released —
// S52's own point, a plan abandoned mid-claim rather than closed out.
func TestPlanIsClaimedAndCarriesUnlandedWorkBuildsAnUnlandedTip(t *testing.T) {
	w := newWorld(t)

	require.NoError(t, w.planIsClaimedAndCarriesUnlandedWork(7))

	st := section[lifecycleState](w)
	require.NotEmpty(t, st.unlandedTip)
	assert.False(t, claim.Released(st.repo, st.unlandedTip, 7, gitwt.Exec),
		"the lease this Given builds is live, never released")
}

// TestClaimSucceedsAtEpochRefusesAFailedClaim: a prior acquire error
// is read back and reported, not papered over by checking the epoch
// anyway.
func TestClaimSucceedsAtEpochRefusesAFailedClaim(t *testing.T) {
	w := newWorld(t)
	w.err = fmt.Errorf("held elsewhere")
	require.Error(t, w.claimSucceedsAtEpoch("box-b", 2))
}

// TestClaimSucceedsAtEpochRefusesTheWrongEpoch: a claim that landed on
// an epoch other than the one the scenario named fails this check,
// even though the acquire itself reported no error.
func TestClaimSucceedsAtEpochRefusesTheWrongEpoch(t *testing.T) {
	w := newWorld(t)
	w.lease = claim.Lease{Epoch: 1}
	require.Error(t, w.claimSucceedsAtEpoch("box-b", 2))
}

// TestOriginsTipIsTheClaimRefusesAnUnknownHolder: the wrapper over
// originTipIs still refuses a holder the scenario never introduced,
// the same way originTipIs itself does.
func TestOriginsTipIsTheClaimRefusesAnUnknownHolder(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.originsTipIsTheClaim("ghost"))
}

// TestClaimSucceedsAtEpochAndOriginsTipIsTheClaimPassTogether: S58's
// own chain end to end — a released lease, a second named machine's
// acquire, and both new Then steps — without godog's own layer, so a
// failure here points straight at the step functions rather than at
// step-text matching.
func TestClaimSucceedsAtEpochAndOriginsTipIsTheClaimPassTogether(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, w.planIsDoneAndItsLeaseIsReleased(7))

	require.NoError(t, w.acquiresTheLeaseForPlan("box-b", 7))

	require.NoError(t, w.claimSucceedsAtEpoch("box-b", 2))
	require.NoError(t, w.originsTipIsTheClaim("box-b"))
}

// TestPlanIsDoneAndItsLeaseIsReleasedBuildsAReleasedTip: the shared
// Given actually releases the lease it acquires, so the scavenge step
// has a release marker's tip to CAS against, not a live hold's.
func TestPlanIsDoneAndItsLeaseIsReleasedBuildsAReleasedTip(t *testing.T) {
	w := newWorld(t)

	require.NoError(t, w.planIsDoneAndItsLeaseIsReleased(7))

	st := section[lifecycleState](w)
	require.NotEmpty(t, st.releaseTip)
	assert.True(t, claim.Released(st.repo, st.releaseTip, 7, gitwt.Exec))
}
