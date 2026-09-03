package main

import (
	"fmt"
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
type landedEvidenceState struct {
	tip     string
	runner  gitwt.Runner
	scav    claim.Scavenged
	scavErr error
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
