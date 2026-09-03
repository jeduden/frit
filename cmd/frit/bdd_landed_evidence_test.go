package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The landed-evidence vocabulary: work pushed on the lane, squash-landed
// on the default branch, and read back through the lease API and
// DefaultRef. It registers itself, like every section's step file, so
// this file adds the section and never edits bdd_test.go.
func init() {
	registrars = append(registrars, (*world).registerLandedEvidence)
}

// landedEvidenceState is this section's own state beside the shared
// world: the tip a row pushed for real (as opposed to the lease
// world's own w.local, left deliberately unpushed), the Runner a row
// swaps in to simulate an unreadable origin, and the outcome of the
// one scavenge a row ran. Kept through section rather than a field on
// world, per bdd_test.go's own convention. Named apart from main.go's
// own landedEvidence type, which is an unrelated fleet-report value.
//
// root, repo, lane and branch carry the verb-level rows' own fixture:
// a `reap --go` run is driven against a fleet root rather than the
// lease-API world's bare clone pair, so those rows record the root a
// Given built and the repo, worktree path and branch its Then steps
// read back. session is the herdr session a Given binds a lease to.
// out and errb capture the last CLI run this section drove.
type landedEvidenceState struct {
	tip     string
	runner  gitwt.Runner
	scav    claim.Scavenged
	scavErr error

	root    string
	repo    string
	lane    string
	branch  string
	session string
	out     bytes.Buffer
	errb    bytes.Buffer
}

// registerLandedEvidence binds the section's step texts to the world. A
// step text bdd_lease_test.go already defines is reused, never
// redefined here — S54, S84 and S85 all lean on "holds the lease for
// plan" from that file.
func (w *world) registerLandedEvidence(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]+)" pushes work on the lane$`, w.pushesWorkOnTheLane)
	sc.Step(`^"([^"]+)" clones the repository$`, w.clonesTheRepository)
	sc.Step(`^"([^"]+)" squash-merges the work onto the default branch$`, w.squashMergesOntoTheDefaultBranch)
	sc.Step(`^"([^"]+)" scavenges at the observed tip$`, w.scavengesAtTheObservedTip)
	sc.Step(`^origin's work ref for the plan is gone$`, w.originsWorkRefForThePlanIsGone)
	sc.Step(`^nothing is parked$`, w.nothingIsParked)
	sc.Step(`^origin becomes unreadable$`, w.originBecomesUnreadable)
	sc.Step(`^the scavenge fails naming the read$`, w.theScavengeFailsNamingTheRead)
	sc.Step(`^"([^"]+)"'s local work ref still resolves at its tip$`, w.localWorkRefStillResolvesAtItsTip)
	sc.Step(`^"([^"]+)"'s local main is behind the default branch$`, w.localMainIsBehindTheDefaultBranch)
	sc.Step(`^the work reads landed for "([^"]+)"$`, w.theWorkReadsLandedFor)
	sc.Step(`^a scavenge by "([^"]+)" parks nothing$`, w.aScavengeByParksNothing)
	sc.Step(`^DefaultRef for "([^"]+)" answers refs/remotes/origin/main, not refs/heads/main$`,
		w.defaultRefAnswersOriginMain)
	sc.Step(`^a worktree stands on "([^"]+)"'s branch$`, w.aWorktreeStandsOnBranch)
	sc.Step(`^the work is parked to a rescue ref$`, w.theWorkIsParkedToARescueRef)
	sc.Step(`^"([^"]+)" holds the lease for plan (\d+) bound to a session$`,
		w.holdsTheLeaseBoundToASession)
	sc.Step(`^a herdr fake confirms "([^"]+)"'s bound session alive$`, w.herdrFakeConfirmsBoundSessionAlive)
	sc.Step(`^a fleet-wide reap --go runs$`, w.aFleetWideReapGoRuns)
	sc.Step(`^the hold is refused naming a live lease$`, w.theHoldIsRefusedNamingALiveLease)
	sc.Step(`^the hold still resolves on origin$`, w.theHoldStillResolvesOnOrigin)
	sc.Step(`^a stranded, landed checkout on plan (\d+)'s branch$`, w.aStrandedLandedCheckoutOnPlansBranch)
	sc.Step(`^the branch is reaped$`, w.theBranchIsReaped)
	sc.Step(`^the checkout's own commit is parked to the plan's rescue ref$`,
		w.theCheckoutsCommitIsParkedToTheRescueRef)
}

// tipObserved is the tip a row's evidence is judged against: this
// section's own pushed tip when a row minted one, else the lease
// world's own acquired tip (S83, which never pushes work of its own).
func (w *world) tipObserved() string {
	if tip := section[landedEvidenceState](w).tip; tip != "" {
		return tip
	}

	return w.lease.Tip
}

// landedEvidenceRunner is the Runner every read in this section goes
// through: the real gitwt.Exec, or the failing one S83 swapped in —
// so a step composed after "origin becomes unreadable" reads through
// the same fault the scavenge step already does, rather than quietly
// bypassing it with a hardcoded gitwt.Exec.
func (w *world) landedEvidenceRunner() gitwt.Runner {
	if run := section[landedEvidenceState](w).runner; run != nil {
		return run
	}

	return gitwt.Exec
}

// squashLandContent is the content both the pushed work commit and its
// later squash-merge carry — the same literal on both sides is what
// makes claim.WorkLanded's merge-tree content check a no-op, and this
// section's own squashMergesOntoTheDefaultBranch shares it rather than
// hardcoding a second copy that could drift from pushesWorkOnTheLane's.
const squashLandContent = "wip\n"

// pushesWorkOnTheLane commits one work file on a holder's lane and
// pushes it for real — the shape S54, S84 and S85 all squash-merge
// against, as opposed to bdd_lease_test.go's own commitsUnpushedWork,
// which deliberately leaves the commit local.
func (w *world) pushesWorkOnTheLane(holder string) error {
	tip, err := w.commitWorkFileOnLane(holder, squashLandContent, "unlanded work")
	if err != nil {
		return err
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	if out, err := gitCapture(w.t, repo, "push", "origin", tip+":"+w.branch()); err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	section[landedEvidenceState](w).tip = tip

	return nil
}

// clonesTheRepository stands a second machine up as a fresh clone of
// the holder's origin, introducing it to the world the way a takeover
// does — a scenario naming this machine anywhere else refuses until
// this step has run.
func (w *world) clonesTheRepository(holder string) error {
	_, err := w.cloneAs(w.holder, holder)

	return err
}

// squashMergesOntoTheDefaultBranch reproduces the four git commands of
// squashLandOnMain in internal/claim/lease_test.go, which cmd/frit
// cannot import: a fresh commit on the default branch carrying the same
// content the lane pushed, with no ancestry to the lane's own commits —
// the shape a squash-merge PR leaves behind. holder's clone is fresh
// off origin (clonesTheRepository), so its local main already matches
// origin/main without a fetch — checkout alone is enough, exactly as
// squashLandOnMain's own fixture does it.
//
// The guard refuses on this section's own pushed tip, not
// tipObserved()'s lease-tip fallback: every acquired lease carries a
// tip, so guarding on the fallback would never actually catch a
// squash-merge run before any push.
func (w *world) squashMergesOntoTheDefaultBranch(holder string) error {
	if section[landedEvidenceState](w).tip == "" {
		return fmt.Errorf("nothing has been pushed on the lane yet; the push step comes first")
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	git(w.t, repo, "checkout", "-q", "main")
	writeFile(w.t, repo, "w.txt", squashLandContent)
	git(w.t, repo, "add", "-A")
	git(w.t, repo, "commit", "-q", "-m", fmt.Sprintf("squash-merge plan %d", w.planID))
	git(w.t, repo, "push", "-q", "origin", "main")

	return nil
}

// failingLsRemote wraps a Runner so every ls-remote call reports a
// read fault while every other git command still runs for real — the
// unreadable-origin shape S83 needs, guarded so a scenario cannot pass
// merely because every git call failed. Deliberately narrower than
// bdd_partitions_and_clocks_test.go's own partitionRunner, which also
// cuts push and fetch: S83 pins that "gone" is only ever a remote's
// answer to the one read a scavenge classifies a ref by, so widening
// the cut to every network call would no longer isolate that read.
func failingLsRemote(run gitwt.Runner) gitwt.Runner {
	return func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ls-remote" {
			return nil, fmt.Errorf("simulated network fault reading %s", strings.Join(args, " "))
		}

		return run(dir, args...)
	}
}

// originBecomesUnreadable swaps in the failing Runner every later git
// call in this scenario, that touches origin through the lease API,
// runs with.
func (w *world) originBecomesUnreadable() error {
	section[landedEvidenceState](w).runner = failingLsRemote(gitwt.Exec)

	return nil
}

// scavengesAtTheObservedTip runs claim.Scavenge from holder's own
// clone against the tip this scenario observed, over whatever Runner
// this section currently has in play — the real gitwt.Exec, or the
// failing one S83 swapped in.
func (w *world) scavengesAtTheObservedTip(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	le := section[landedEvidenceState](w)
	run := le.runner
	if run == nil {
		run = gitwt.Exec
	}
	le.scav, le.scavErr = claim.Scavenge(repo, leaseFor(holder, w.planID), w.tipObserved(), run)

	return nil
}

// originsWorkRefForThePlanIsGone confirms the plan's work ref no
// longer resolves on origin — the delete half of a clean scavenge.
func (w *world) originsWorkRefForThePlanIsGone() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, repo, "ls-remote", "origin", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	if out != "" {
		return fmt.Errorf("origin still holds the work ref: %s", out)
	}

	return nil
}

// nothingIsParked confirms the last scavenge run neither failed nor
// parked a rescue ref.
func (w *world) nothingIsParked() error {
	le := section[landedEvidenceState](w)
	if le.scavErr != nil {
		return fmt.Errorf("the scavenge failed: %w", le.scavErr)
	}
	if le.scav.Rescue != "" {
		return fmt.Errorf("expected nothing parked, got rescue ref %s", le.scav.Rescue)
	}

	return nil
}

// theScavengeFailsNamingTheRead confirms the last scavenge run
// returned an error whose text names the specific read it could not
// complete — the plan's own work ref, read from origin — not merely
// any error whose text happens to contain the word "read".
// "gone" is only ever a remote's answer, never a fold of "unreadable".
func (w *world) theScavengeFailsNamingTheRead() error {
	le := section[landedEvidenceState](w)
	if le.scavErr == nil {
		return fmt.Errorf("expected the scavenge to fail naming the read; it reported no error")
	}
	want := fmt.Sprintf("read %s from origin", w.branch())
	if !strings.Contains(le.scavErr.Error(), want) {
		return fmt.Errorf("the error %q does not name the read (%q)", le.scavErr, want)
	}

	return nil
}

// localWorkRefStillResolvesAtItsTip confirms a holder's local copy of
// the plan's work ref was left untouched by a scavenge that could not
// read origin — an unreadable remote must never be folded into "gone"
// and drive a local delete.
func (w *world) localWorkRefStillResolvesAtItsTip(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	tip, err := gitCapture(w.t, repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	if want := w.tipObserved(); tip != want {
		return fmt.Errorf("the local work ref resolved to %s, want %s", tip, want)
	}

	return nil
}

// refreshRemoteTracking fetches origin's default branch into a
// holder's clone without touching its local main — the read every
// landed-evidence assertion needs before it trusts refs/remotes/origin/main,
// and deliberately not a merge or pull, since S84 and S85 both depend
// on local main staying exactly where it was.
func (w *world) refreshRemoteTracking(holder string) (string, error) {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return "", err
	}
	if out, err := gitCapture(w.t, repo, "fetch", "-q", "origin", "main"); err != nil {
		return "", fmt.Errorf("%s: %w", out, err)
	}

	return repo, nil
}

// localMainIsBehindTheDefaultBranch confirms a holder's local main
// genuinely lags origin's — a strict ancestor, not equal — so S84
// cannot pass on an accidental fast-forward.
func (w *world) localMainIsBehindTheDefaultBranch(holder string) error {
	repo, err := w.refreshRemoteTracking(holder)
	if err != nil {
		return err
	}
	local, err := gitCapture(w.t, repo, "rev-parse", "refs/heads/main")
	if err != nil {
		return fmt.Errorf("%s: %w", local, err)
	}
	remote, err := gitCapture(w.t, repo, "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return fmt.Errorf("%s: %w", remote, err)
	}
	if local == remote {
		return fmt.Errorf(
			"%q's local main already matches origin's default branch; the scenario needs it to lag", holder)
	}
	out, err := gitCapture(w.t, repo, "merge-base",
		"--is-ancestor", "refs/heads/main", "refs/remotes/origin/main")
	if err != nil {
		return fmt.Errorf("%q's local main is not behind origin's default branch: %s", holder, out)
	}

	return nil
}

// theWorkReadsLandedFor confirms claim.WorkLanded reads the observed
// tip as landed against gitobj.DefaultRef's own answer for holder's
// clone — the composed read a scavenge and a report both rely on.
func (w *world) theWorkReadsLandedFor(holder string) error {
	repo, err := w.refreshRemoteTracking(holder)
	if err != nil {
		return err
	}
	run := w.landedEvidenceRunner()
	base := gitobj.DefaultRef(repo, run)
	if base == "" {
		return fmt.Errorf("DefaultRef found no default branch for %q", holder)
	}
	tip := w.tipObserved()
	if !claim.WorkLanded(repo, int64(w.planID), base, tip, run) {
		return fmt.Errorf("WorkLanded(%s, %s) reported unlanded, want landed", base, tip)
	}

	return nil
}

// aScavengeByParksNothing runs a scavenge from holder's own clone and
// confirms it parked no rescue ref — composed from the same two steps
// S54 chains explicitly, so the two rows can never silently diverge on
// what "a clean scavenge" actually checks.
func (w *world) aScavengeByParksNothing(holder string) error {
	if err := w.scavengesAtTheObservedTip(holder); err != nil {
		return err
	}

	return w.nothingIsParked()
}

// defaultRefAnswersOriginMain confirms gitobj.DefaultRef reaches
// refs/remotes/origin/main for holder's clone — never the lagging
// refs/heads/main — and that origin/HEAD is genuinely unset there, so
// the row cannot pass by accident on a symref it never exercised.
func (w *world) defaultRefAnswersOriginMain(holder string) error {
	repo, err := w.refreshRemoteTracking(holder)
	if err != nil {
		return err
	}
	if out, err := gitCapture(w.t, repo, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		return fmt.Errorf("%q has refs/remotes/origin/HEAD set to %s; the scenario needs it unset", holder, out)
	}
	if got := gitobj.DefaultRef(repo, w.landedEvidenceRunner()); got != "refs/remotes/origin/main" {
		return fmt.Errorf("DefaultRef answered %q, want refs/remotes/origin/main", got)
	}

	return nil
}

// aWorktreeStandsOnBranch links a second worktree of holder's own
// clone onto the plan's own branch — S79's own hazard: a scavenge
// that deletes origin's copy must not also clobber the local ref out
// from under it, the way `update-ref -d` would with no guard.
func (w *world) aWorktreeStandsOnBranch(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	linked := filepath.Join(w.t.TempDir(), "linked")
	if out, err := gitCapture(w.t, repo, "worktree", "add", "-q", linked,
		claim.Branch(int64(w.planID))); err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}

	return nil
}

// theWorkIsParkedToARescueRef confirms the last scavenge run parked a
// rescue ref — the opposite of nothingIsParked, for a row whose tip is
// genuinely unlanded content rather than a squash-merge no-op.
func (w *world) theWorkIsParkedToARescueRef() error {
	le := section[landedEvidenceState](w)
	if le.scavErr != nil {
		return fmt.Errorf("the scavenge failed: %w", le.scavErr)
	}
	if le.scav.Rescue == "" {
		return fmt.Errorf("expected the work parked to a rescue ref; nothing was parked")
	}

	return nil
}

// holdsTheLeaseBoundToASession is S81's own Given: a lease minted the
// way claimableRepo's fleet-root shape needs — repo one level under a
// fresh root, so a later `reap --root` sees exactly this one lane —
// bound to a session a herdr fake can later answer for.
func (w *world) holdsTheLeaseBoundToASession(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	root := w.t.TempDir()
	repo := claimableRepo(w.t, root, "atlas", planID, "Shader unit")
	w.clones[holder] = repo

	opts := leaseFor(holder, planID)
	opts.Session = holder + "-session"
	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	if err != nil {
		return err
	}
	w.holder, w.lease = holder, lease
	le := section[landedEvidenceState](w)
	le.root, le.repo, le.session = root, repo, opts.Session

	return nil
}

// herdrFakeConfirmsBoundSessionAlive installs a herdr fake reporting
// the session the Given step bound as a live, working agent — the
// positive-liveness shape S81 needs, not merely an empty herdr
// response, which a fresh, never-checked claim would also produce.
func (w *world) herdrFakeConfirmsBoundSessionAlive(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	le := section[landedEvidenceState](w)
	if le.session == "" {
		return fmt.Errorf("no bound session recorded for %q", holder)
	}
	withHerdr(w.t, herdrReturning(map[string]any{
		"agent": "claude", "agent_status": "working",
		"pane_id":       le.session,
		"agent_session": map[string]any{"value": le.session},
	}))

	return nil
}

// aFleetWideReapGoRuns drives `reap --go` over the fleet root a Given
// built, the shared When for every CLI-level row in this section — the
// only fact that differs between S81 and S82 is what the fleet root
// holds, never how reap is invoked.
func (w *world) aFleetWideReapGoRuns() error {
	le := section[landedEvidenceState](w)
	if le.root == "" {
		return fmt.Errorf("no fleet root recorded for this scenario")
	}
	le.out.Reset()
	le.errb.Reset()
	run([]string{"reap", "--go", "--root", le.root}, &le.out, &le.errb)

	return nil
}

// theHoldIsRefusedNamingALiveLease confirms reap's own report refused
// the hold, naming a live lease as the reason — holdRefusal's own
// wording, not just any refusal.
func (w *world) theHoldIsRefusedNamingALiveLease() error {
	got := section[landedEvidenceState](w).out.String()
	if !strings.Contains(got, "refused") {
		return fmt.Errorf("expected a refusal, got: %s", got)
	}
	if !strings.Contains(got, "live lease") {
		return fmt.Errorf("the refusal does not name a live lease: %s", got)
	}

	return nil
}

// theHoldStillResolvesOnOrigin confirms the canonical hold ref reap
// refused to touch is still exactly where it was.
func (w *world) theHoldStillResolvesOnOrigin() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, repo, "ls-remote", "origin", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	if out == "" {
		return fmt.Errorf("origin no longer carries the hold ref")
	}

	return nil
}

// aStrandedLandedCheckoutOnPlansBranch is S82's own Given: a worktree
// carrying a real, unmerged commit on the plan's branch, with the plan
// file itself the only evidence the work landed — strandedCheckout,
// landPlan and addOrigin, the same fixtures
// TestReapSquashMergedBranchIsReapedEvenNotAnAncestor already proves
// this shape with, reused rather than reinvented.
func (w *world) aStrandedLandedCheckoutOnPlansBranch(planID int) error {
	isolate(w.t)
	w.planID = planID
	root := w.t.TempDir()
	repo := initRepo(w.t, root, "atlas")
	branch := claim.Branch(int64(planID))
	lane := strandedCheckout(w.t, root, repo, "atlas-squashed", branch)
	tip, err := gitCapture(w.t, repo, "rev-parse", branch)
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	landPlan(w.t, repo, int64(planID), "fleet-index", "✅")
	addOrigin(w.t, repo)

	le := section[landedEvidenceState](w)
	le.root, le.repo, le.lane, le.branch, le.tip = root, repo, lane, branch, tip

	return nil
}

// theBranchIsReaped confirms reap's own report reaped the lane and
// that its branch no longer resolves — the CLI-level echo of S54's own
// "origin's work ref for the plan is gone", but of the local branch
// reap's stranded pass deletes directly rather than a remote ref a
// scavenge CASes on.
func (w *world) theBranchIsReaped() error {
	le := section[landedEvidenceState](w)
	got := le.out.String()
	if !strings.Contains(got, "reaped") {
		return fmt.Errorf("expected the branch reaped, got: %s", got)
	}
	if branchExists(w.t, le.repo, le.branch) {
		return fmt.Errorf("branch %s still resolves after reap", le.branch)
	}

	return nil
}

// theCheckoutsCommitIsParkedToTheRescueRef confirms the stranded
// checkout's own tip — the follow-up commit the plan's ✅ glyph never
// carried — is the exact object origin's rescue ref for this plan now
// holds, not merely that some rescue ref exists.
func (w *world) theCheckoutsCommitIsParkedToTheRescueRef() error {
	le := section[landedEvidenceState](w)
	rescue, err := gitCapture(w.t, le.repo, "ls-remote", "origin",
		fmt.Sprintf("refs/frit/rescue/%d/*", w.planID))
	if err != nil {
		return fmt.Errorf("%s: %w", rescue, err)
	}
	if !strings.Contains(rescue, le.tip) {
		return fmt.Errorf("the rescue ref does not carry the checkout's own tip %s: %s", le.tip, rescue)
	}

	return nil
}

// TestTipObservedPrefersItsOwnPushOverTheLeaseTip: a row that pushed
// real work is judged against that tip, not the claim marker Acquire
// left behind; a row that never pushed (S83) falls back to it.
func TestTipObservedPrefersItsOwnPushOverTheLeaseTip(t *testing.T) {
	w := newWorld(t)
	w.lease.Tip = "claim-tip"
	assert.Equal(t, "claim-tip", w.tipObserved())

	section[landedEvidenceState](w).tip = "pushed-tip"
	assert.Equal(t, "pushed-tip", w.tipObserved())
}

// TestFailingLsRemoteRefusesOnlyLsRemote: the Runner S83 swaps in must
// fail exactly the read a scavenge classifies a ref by, and nothing
// else — a scenario that passed because every git call failed would
// prove nothing about the unreadable-origin path.
func TestFailingLsRemoteRefusesOnlyLsRemote(t *testing.T) {
	var seen []string
	inner := func(dir string, args ...string) ([]byte, error) {
		seen = append(seen, strings.Join(args, " "))

		return []byte("ok"), nil
	}
	run := failingLsRemote(inner)

	_, err := run("/r", "ls-remote", "origin", "refs/heads/plan/7")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ls-remote")

	out, err := run("/r", "rev-parse", "HEAD")
	require.NoError(t, err)
	assert.Equal(t, "ok", string(out))
	assert.Equal(t, []string{"rev-parse HEAD"}, seen, "the failing call never reached the wrapped Runner")
}

// TestLandedEvidenceStepsRefuseAMachineTheyNeverMet: every step this
// section adds resolves its quoted machine through cloneOf, the same
// guard bdd_lease_test.go's own steps stand on, so none of them can
// pass by falling back to whatever the world happens to hold.
func TestLandedEvidenceStepsRefuseAMachineTheyNeverMet(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	w.clones["box-a"] = t.TempDir()

	require.Error(t, w.pushesWorkOnTheLane("ghost"))
	require.Error(t, w.squashMergesOntoTheDefaultBranch("ghost"))
	require.Error(t, w.scavengesAtTheObservedTip("ghost"))
	require.Error(t, w.localWorkRefStillResolvesAtItsTip("ghost"))
	require.Error(t, w.localMainIsBehindTheDefaultBranch("ghost"))
	require.Error(t, w.theWorkReadsLandedFor("ghost"))
	require.Error(t, w.aScavengeByParksNothing("ghost"))
	require.Error(t, w.defaultRefAnswersOriginMain("ghost"))
	require.Error(t, w.aWorktreeStandsOnBranch("ghost"))
	require.Error(t, w.herdrFakeConfirmsBoundSessionAlive("ghost"))
}

// TestSquashMergesRefusesBeforeAnyPush: the squash step reproduces
// what landed on the default branch on top of work a row actually
// pushed; run before any push, it would squash-merge content that was
// never on the lane, so it refuses instead.
func TestSquashMergesRefusesBeforeAnyPush(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	w.clones["box-a"] = t.TempDir()

	err := w.squashMergesOntoTheDefaultBranch("box-a")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nothing has been pushed")
}

// TestRefreshRemoteTrackingFetchesOriginMainWithoutTouchingLocalMain:
// S84 and S85 both depend on a holder's local main staying exactly
// where it was while origin's own advances — refreshRemoteTracking
// exists to fetch that fresh view without ever merging or pulling it
// in, so this pins that a fetch alone moves refs/remotes/origin/main
// and never refs/heads/main.
func TestRefreshRemoteTrackingFetchesOriginMainWithoutTouchingLocalMain(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.holder = "box-a"
	repo := claimableRepo(t, t.TempDir(), "atlas", 7, "Shader unit")
	w.clones["box-a"] = repo

	before, err := gitCapture(t, repo, "rev-parse", "refs/heads/main")
	require.NoError(t, err)

	// A second clone advances origin's main behind box-a's back.
	second := cloneAgain(t, repo)
	writeFile(t, second, "w.txt", "wip\n")
	git(t, second, "add", "-A")
	git(t, second, "commit", "-q", "-m", "advance main")
	git(t, second, "push", "-q", "origin", "main")

	got, err := w.refreshRemoteTracking("box-a")
	require.NoError(t, err)
	assert.Equal(t, repo, got)

	local, err := gitCapture(t, repo, "rev-parse", "refs/heads/main")
	require.NoError(t, err)
	assert.Equal(t, before, local, "refreshRemoteTracking must never move local main")

	remote, err := gitCapture(t, repo, "rev-parse", "refs/remotes/origin/main")
	require.NoError(t, err)
	assert.NotEqual(t, before, remote, "the fetch must bring origin's advance in")
}

// TestNothingIsParkedReportsAFailedScavenge: the Then step that checks
// no rescue ref was parked must not silently pass over a scavenge that
// itself failed — a nil Scavenged and a nil error look the same as "no
// rescue", so the error is checked first.
func TestNothingIsParkedReportsAFailedScavenge(t *testing.T) {
	w := newWorld(t)
	section[landedEvidenceState](w).scavErr = fmt.Errorf("boom")

	err := w.nothingIsParked()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

// TestTheScavengeFailsNamingTheReadWantsAnActualFailure: the Then step
// pinning S83's fault must not pass on a scavenge that quietly
// succeeded, must reject an error that never names the read at all,
// and must reject an error that merely contains the word "read" for
// some unrelated reason — none of the three would prove the fault
// this row exists to pin.
func TestTheScavengeFailsNamingTheReadWantsAnActualFailure(t *testing.T) {
	w := newWorld(t)
	w.planID = 7
	require.Error(t, w.theScavengeFailsNamingTheRead(), "no error at all is not the fault")

	section[landedEvidenceState](w).scavErr = fmt.Errorf("plan 7 is held: the work ref carries a live lease")
	require.Error(t, w.theScavengeFailsNamingTheRead(), "an error that never names the read is not this fault")

	section[landedEvidenceState](w).scavErr = fmt.Errorf("could not read the plan's status file")
	require.Error(t, w.theScavengeFailsNamingTheRead(), "an unrelated read is not this fault")

	section[landedEvidenceState](w).scavErr = fmt.Errorf("read refs/heads/plan/7 from origin for plan 7: boom")
	require.NoError(t, w.theScavengeFailsNamingTheRead())
}

// TestTheWorkIsParkedToARescueRefWantsAnActualRescue: the Then step
// pinning S79's own rescue must reject a scavenge that failed outright
// and a scavenge that quietly parked nothing — a nil Scavenged and a
// nil error look the same as "no rescue" as nothingIsParked's own test
// already pins for the opposite claim.
func TestTheWorkIsParkedToARescueRefWantsAnActualRescue(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theWorkIsParkedToARescueRef(), "no scavenge run at all is not a parked rescue")

	section[landedEvidenceState](w).scavErr = fmt.Errorf("boom")
	require.Error(t, w.theWorkIsParkedToARescueRef(), "a failed scavenge parked nothing")

	section[landedEvidenceState](w).scavErr = nil
	section[landedEvidenceState](w).scav = claim.Scavenged{}
	require.Error(t, w.theWorkIsParkedToARescueRef(), "an empty rescue is not a parked rescue")

	section[landedEvidenceState](w).scav = claim.Scavenged{Rescue: "refs/frit/rescue/79/box-a-abc"}
	require.NoError(t, w.theWorkIsParkedToARescueRef())
}

// TestAFleetWideReapGoRunsRefusesWithNoRoot: the shared CLI When must
// not silently run reap against whatever directory happens to be
// current — a scenario whose Given never recorded a fleet root fails
// loudly rather than reaping the test binary's own working directory.
func TestAFleetWideReapGoRunsRefusesWithNoRoot(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.aFleetWideReapGoRuns())
}

// TestTheHoldIsRefusedNamingALiveLeaseWantsTheWording: the Then step
// must reject a report that never refused at all and one that refused
// for an unrelated reason — a scenario passing on either would prove
// nothing about S81's own claim.
func TestTheHoldIsRefusedNamingALiveLeaseWantsTheWording(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theHoldIsRefusedNamingALiveLease(), "an empty report refused nothing")

	section[landedEvidenceState](w).out.WriteString("dropped\tplan 7\tplan/7\n")
	require.Error(t, w.theHoldIsRefusedNamingALiveLease(), "a drop is not a refusal")

	le := section[landedEvidenceState](w)
	le.out.Reset()
	le.out.WriteString("refused\tplan 7\tplan/7 (decorated hold; migrate to plan/7 first)\n")
	require.Error(t, w.theHoldIsRefusedNamingALiveLease(),
		"a refusal for an unrelated reason does not name a live lease")

	le.out.Reset()
	le.out.WriteString("refused\tplan 7\tplan/7 (held by a live lease, unchanged for 3m)\n")
	require.NoError(t, w.theHoldIsRefusedNamingALiveLease())
}

// TestTheHoldStillResolvesOnOriginRequiresAHolder pins the same
// machine-refusal guard the section's other steps carry: with no
// holder ever introduced, the check has nothing to read origin
// through and fails rather than passing on an empty world.
func TestTheHoldStillResolvesOnOriginRequiresAHolder(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theHoldStillResolvesOnOrigin())
}

// TestTheBranchIsReapedChecksBothTheReportAndTheRef: a report that
// says "reaped" while the branch still resolves — or the reverse — is
// not this row's own claim; both facts must hold.
func TestTheBranchIsReapedChecksBothTheReportAndTheRef(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	git(t, repo, "branch", "still-here")

	w := newWorld(t)
	le := section[landedEvidenceState](w)
	le.repo, le.branch = repo, "still-here"

	require.Error(t, w.theBranchIsReaped(), "an empty report never claimed anything reaped")

	le.out.WriteString("reaped\tatlas\tstill-here\n")
	require.Error(t, w.theBranchIsReaped(), "the branch still resolves; the report alone is not enough")

	git(t, repo, "branch", "-D", "still-here")
	require.NoError(t, w.theBranchIsReaped())
}

// TestTheCheckoutsCommitIsParkedToTheRescueRefWantsTheExactTip pins
// that a rescue ref existing at all is not enough — it must carry the
// checkout's own tip, not some other object a foreign push could have
// left at the same address.
func TestTheCheckoutsCommitIsParkedToTheRescueRefWantsTheExactTip(t *testing.T) {
	isolate(t)
	root := t.TempDir()
	repo := initRepo(t, root, "atlas")
	addOrigin(t, repo)
	foreign, err := gitCapture(t, repo, "rev-parse", "main")
	require.NoError(t, err)
	_, err = gitCapture(t, repo, "push", "-q", "origin",
		foreign+":refs/frit/rescue/82/box-a-notthetip")
	require.NoError(t, err)

	w := newWorld(t)
	w.planID = 82
	le := section[landedEvidenceState](w)
	le.repo, le.tip = repo, "0000000000000000000000000000000000000tip"

	require.Error(t, w.theCheckoutsCommitIsParkedToTheRescueRef(),
		"a rescue ref exists, but not for this tip")
}

// TestHoldsTheLeaseBoundToASessionBindsTheSessionAndTheFleetRoot: the
// Given S81 stands on must mint a real lease bound to the session its
// later herdr fake answers for, and record a root a `reap --root` run
// can find exactly this one lane under.
func TestHoldsTheLeaseBoundToASessionBindsTheSessionAndTheFleetRoot(t *testing.T) {
	w := newWorld(t)

	require.NoError(t, w.holdsTheLeaseBoundToASession("box-a", 81))

	le := section[landedEvidenceState](w)
	assert.Equal(t, "box-a-session", le.session)
	assert.Equal(t, w.clones["box-a"], le.repo)
	assert.Equal(t, filepath.Dir(le.repo), le.root)
	assert.Equal(t, "box-a", w.holder)
	assert.NotEmpty(t, w.lease.Tip)
}

// TestAStrandedLandedCheckoutOnPlansBranchBuildsTheSquashShape: the
// Given S82 stands on must leave a real, unmerged commit on the plan's
// own branch, a plan file already reading ✅, and an origin the park
// can push a rescue ref to — the same shape
// TestReapSquashMergedBranchIsReapedEvenNotAnAncestor already proves
// reap reaps.
func TestAStrandedLandedCheckoutOnPlansBranchBuildsTheSquashShape(t *testing.T) {
	w := newWorld(t)

	require.NoError(t, w.aStrandedLandedCheckoutOnPlansBranch(82))

	le := section[landedEvidenceState](w)
	assert.Equal(t, "plan/82", le.branch)
	assert.NotEmpty(t, le.lane)
	assert.NotEmpty(t, le.tip)
	assert.True(t, branchExists(t, le.repo, le.branch))
	remote, err := gitCapture(t, le.repo, "config", "--get", "remote.origin.url")
	require.NoError(t, err)
	assert.NotEmpty(t, remote)
}

// TestHerdrFakeConfirmsBoundSessionAliveIsLoadBearing: the same fleet
// root, the same session-bound hold — an empty herdr answer reads the
// bound session as confirmed gone and reap drops the hold, while a
// herdr answer naming that exact session alive is what turns the same
// drop into S81's own refusal. Without this the scenario could pass
// for the wrong reason: a session-bound hold reading as abandoned by
// default, never actually exercising the live-holder path S81 names.
func TestHerdrFakeConfirmsBoundSessionAliveIsLoadBearing(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, w.holdsTheLeaseBoundToASession("box-a", 81))
	withHerdr(t, herdrReturning())
	require.NoError(t, w.aFleetWideReapGoRuns())
	assert.Contains(t, section[landedEvidenceState](w).out.String(), "dropped",
		"a session bound to nobody herdr can find reads as abandoned, not merely unstaffed")

	w2 := newWorld(t)
	require.NoError(t, w2.holdsTheLeaseBoundToASession("box-a", 81))
	require.NoError(t, w2.herdrFakeConfirmsBoundSessionAlive("box-a"))
	require.NoError(t, w2.aFleetWideReapGoRuns())
	require.NoError(t, w2.theHoldIsRefusedNamingALiveLease())
}
