package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The partitions-and-clocks vocabulary: a network cut, a heal, a
// repeated renewal, an observer sampling on a clock it chooses. It
// registers itself, like every section's step file, so this section
// adds a file and never edits bdd_lease_test.go or bdd_test.go. Every
// step here that the lease world already defines — "holds the lease",
// "commits work", "takes the lease over", "comes back and renews",
// "the renewal is fenced", "the error suggests yield", "sibling
// history", "push is rejected", "yield parks" — is reused as-is.
func init() {
	registrars = append(registrars, (*world).registerPartitionsAndClocks)
}

// pcState is this section's own state, kept beside the shared world
// via section — never a field on world itself. It carries a Runner per
// machine (gitwt.Exec until a partition step swaps one in and a heal
// step swaps it back), which machines are currently cut off, each
// machine's real worktree lane where one was built, the chain of
// beats a scenario's own renewals produced, and the observation window
// a step advances on a clock it — never time.Now — chooses.
type pcState struct {
	runners map[string]gitwt.Runner
	cut     map[string]bool
	lanes   map[string]string
	tips    map[string]string
	beats   []string
	window  discovery.Window
	clock   time.Time
}

// pc fetches this scenario's section state, lazily initialized on
// first use: a fresh set of maps and a clock pinned to a fixed instant
// rather than wherever time.Now happens to sit when the test runs.
func (w *world) pc() *pcState {
	s := section[pcState](w)
	if s.runners == nil {
		s.runners = map[string]gitwt.Runner{}
		s.cut = map[string]bool{}
		s.lanes = map[string]string{}
		s.tips = map[string]string{}
		s.clock = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}

	return s
}

// runnerFor is the Runner a machine's own transitions go through right
// now: gitwt.Exec by default, or whatever a partition step armed for
// it until a heal step restores it.
func (w *world) runnerFor(holder string) gitwt.Runner {
	if r, ok := w.pc().runners[holder]; ok {
		return r
	}

	return gitwt.Exec
}

// partitionRunner wraps base so push, fetch and ls-remote — the three
// verbs every lease transition arbitrates or reconciles through — fail
// as a cut network would; every other call, the local plumbing a
// transition mints its marker with, passes straight through.
func partitionRunner(base gitwt.Runner) gitwt.Runner {
	return func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 {
			switch args[0] {
			case "push", "fetch", "ls-remote":
				return nil, fmt.Errorf("network unreachable: git %s", args[0])
			}
		}

		return base(dir, args...)
	}
}

// landedButUnconfirmedRunner wraps base to shape S21's own partition:
// the push actually runs, and lands, but the client is told it failed
// — a connection dropped after the remote committed the ref
// transaction, not before. The reconciliation read casPush would use
// to notice the push in fact landed is cut too, so the renewal cannot
// tell landed from lost and reports unconfirmed rather than guessing.
func landedButUnconfirmedRunner(base gitwt.Runner) gitwt.Runner {
	return func(dir string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "push" {
			if _, err := base(dir, args...); err != nil {
				return nil, err
			}

			return nil, errors.New("connection dropped after the push committed")
		}
		if len(args) > 0 && args[0] == "ls-remote" {
			return nil, errors.New("connection dropped: cannot confirm")
		}

		return base(dir, args...)
	}
}

// registerPartitionsAndClocks binds the step texts S20, S21, S25, S33
// and S34 add. A quoted machine name is checked against the role the
// scenario set it up in, the same discipline bdd_lease_test.go's own
// steps hold to.
func (w *world) registerPartitionsAndClocks(sc *godog.ScenarioContext) {
	sc.Step(`^"([^"]+)" holds the lease for plan (\d+) in a real lane$`, w.holdsTheLeaseInARealLane)
	sc.Step(`^the network cuts "([^"]+)" off from origin$`, w.theNetworkCutsOff)
	sc.Step(`^"([^"]+)"'s next push lands on origin but its confirmation is lost$`,
		w.nextPushLandsButConfirmationIsLost)
	sc.Step(`^the partition heals for "([^"]+)"$`, w.thePartitionHealsFor)
	sc.Step(`^"([^"]+)" renews its lease$`, w.renewsItsLease)
	sc.Step(`^the renewal reports the push unconfirmed$`, w.theRenewalReportsUnconfirmed)
	sc.Step(`^origin's tip has not moved$`, w.originsTipHasNotMoved)
	sc.Step(`^origin's tip has moved past "([^"]+)"'s last confirmed beat$`, w.originsTipHasMovedPast)
	sc.Step(`^an observer watches "([^"]+)"'s tip go stale$`, w.observerWatchesTipGoStale)
	sc.Step(`^the window reads the hold stale$`, w.theWindowReadsTheHoldStale)
	sc.Step(`^"([^"]+)"'s persisted token still matches its last confirmed beat$`, w.tokenStillMatchesClaim)
	sc.Step(`^"([^"]+)" is recognized as the owner of origin's tip$`, w.recognizedAsOwnerOfOriginsTip)
	sc.Step(`^"([^"]+)" resumes at the same epoch$`, w.resumesAtTheSameEpoch)
	sc.Step(`^"([^"]+)" releases from its recorded tip$`, w.releasesFromItsRecordedTip)
	sc.Step(`^the release is fenced naming "([^"]+)"$`, w.theReleaseIsFencedNaming)
	sc.Step(`^origin still holds the takeover$`, w.originStillHoldsTheTakeover)
	sc.Step(`^the work ref still exists$`, w.theWorkRefStillExists)
	sc.Step(`^"([^"]+)"'s commit clock is pinned to one instant$`, w.commitClockIsPinned)
	sc.Step(`^"([^"]+)"'s commit clock steps years backward$`, w.commitClockStepsBackward)
	sc.Step(`^an observer samples the current tip$`, w.observerSamplesTheCurrentTip)
	sc.Step(`^the two beats carry the same commit date and different SHAs$`, w.theTwoBeatsShareADateNotASHA)
	sc.Step(`^the window resets to one sample with no void$`, w.theWindowResetsToOneSample)
	sc.Step(`^the window reads the hold live$`, w.theWindowReadsTheHoldLive)
	sc.Step(`^the tip still moved$`, w.theTipStillMoved)
	sc.Step(`^the commit date on the tip is smaller than on its parent$`, w.commitDateOnTipIsSmallerThanParent)
}

// holdsTheLeaseInARealLane is S21's own acquisition: a real worktree
// on the plan branch, not the placeholder "/lanes/<holder>" path the
// shared "holds the lease" step records — so persistToken actually
// writes a token to disk, and ReadToken, OwnAdvance and Resume have a
// real lane to read it from and advance in.
func (w *world) holdsTheLeaseInARealLane(holder string, planID int) error {
	isolate(w.t)
	w.planID = planID
	repo := claimableRepo(w.t, w.t.TempDir(), "atlas", planID, "Shader unit")
	lane := filepath.Join(w.t.TempDir(), holder+"-lane")
	opts := leaseFor(holder, planID)
	opts.Lane = lane

	lease, err := claim.Acquire(repo, opts, gitwt.Exec)
	if err != nil {
		return err
	}
	git(w.t, repo, "worktree", "add", "-q", lane, claim.Branch(int64(planID)))
	w.clones[holder] = repo
	w.holder, w.lease = holder, lease
	w.pc().lanes[holder] = lane

	return nil
}

// theNetworkCutsOff arms holder's own Runner with the full partition:
// push, fetch and ls-remote all fail from here until a heal step
// restores it. A machine the scenario never introduced refuses rather
// than silently cutting nothing.
func (w *world) theNetworkCutsOff(holder string) error {
	if _, err := w.cloneOf(holder); err != nil {
		return err
	}
	s := w.pc()
	s.runners[holder] = partitionRunner(gitwt.Exec)
	s.cut[holder] = true

	return nil
}

// nextPushLandsButConfirmationIsLost arms holder's Runner with S21's
// own partition shape: the push runs for real, but the client is told
// it failed, and the confirming read is cut too.
func (w *world) nextPushLandsButConfirmationIsLost(holder string) error {
	if _, err := w.cloneOf(holder); err != nil {
		return err
	}
	s := w.pc()
	s.runners[holder] = landedButUnconfirmedRunner(gitwt.Exec)
	s.cut[holder] = true

	return nil
}

// thePartitionHealsFor restores holder's Runner to gitwt.Exec. A
// machine that was never cut off refuses — a heal names what it heals.
func (w *world) thePartitionHealsFor(holder string) error {
	if _, err := w.cloneOf(holder); err != nil {
		return err
	}
	s := w.pc()
	if !s.cut[holder] {
		return fmt.Errorf("%q was never cut off from origin; nothing to heal", holder)
	}
	delete(s.runners, holder)
	s.cut[holder] = false

	return nil
}

// leaseOptsFor is holder's LeaseOptions for this scenario: the shared
// placeholder lane bdd_lease_test.go's leaseFor mints, or the real
// worktree a step like holdsTheLeaseInARealLane built and recorded —
// so a renewal that lands can actually persist its token to disk.
func (w *world) leaseOptsFor(holder string) claim.LeaseOptions {
	opts := leaseFor(holder, w.planID)
	if lane, ok := w.pc().lanes[holder]; ok {
		opts.Lane = lane
	}

	return opts
}

// renewsItsLease is the section's own renewal: like
// bdd_lease_test.go's "comes back and renews its lease", but CASed
// from this section's own tracked chain tip — starting at the
// acquisition and advancing on every successful renewal — through
// holder's own current Runner, so a partitioned machine's renewal
// never quietly borrows the healed Runner. Only the holder renews.
func (w *world) renewsItsLease(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	s := w.pc()
	from := w.lastConfirmedBeat(holder)

	lease, err := claim.Renew(repo, w.leaseOptsFor(holder), from, w.runnerFor(holder))
	w.err = err
	if err == nil {
		s.tips[holder] = lease.Tip
		s.beats = append(s.beats, lease.Tip)
	}

	return nil
}

// theRenewalReportsUnconfirmed reads the last renewal's error as an
// UnconfirmedPushError: the push and its reconciliation read both
// failed, so the transition cannot be told a loss from a landed win.
func (w *world) theRenewalReportsUnconfirmed() error {
	var unconfirmed *claim.UnconfirmedPushError
	if !errors.As(w.err, &unconfirmed) {
		return fmt.Errorf("expected an unconfirmed push, got %v", w.err)
	}

	return nil
}

// originsTipHasNotMoved checks origin's work ref still carries the
// holder's own claim — a cut renewal's failed push left nothing on the
// remote to move it.
func (w *world) originsTipHasNotMoved() error {
	repo := w.clones[w.holder]
	got := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if got != w.lease.Tip {
		return fmt.Errorf("origin's tip moved to %s, want the unchanged claim %s", got, w.lease.Tip)
	}

	return nil
}

// lastConfirmedBeat is the tip this section's own bookkeeping last saw
// a successful transition land at for holder: the chain a renewal CASes
// from, falling back to the original claim before any renewal has
// succeeded.
func (w *world) lastConfirmedBeat(holder string) string {
	if tip, ok := w.pc().tips[holder]; ok {
		return tip
	}

	return w.lease.Tip
}

// originsTipHasMovedPast checks origin's tip is no longer holder's own
// last confirmed beat — S21's push landed for real even though the
// client read it as a failure.
func (w *world) originsTipHasMovedPast(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	last := w.lastConfirmedBeat(holder)
	got := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if got == "" || got == last {
		return fmt.Errorf("origin's tip is %q, want it moved past the last confirmed beat %s", got, last)
	}

	return nil
}

// observerWatchesTipGoStale folds one look at holder's current tip
// into the window, over and over on a clock this step advances itself
// — never time.Now — until the span exceeds the takeover window, the
// same accumulation a live observer would eventually reach by polling.
func (w *world) observerWatchesTipGoStale(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	tip := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if tip == "" {
		return fmt.Errorf("origin holds no tip for plan %d", w.planID)
	}
	s := w.pc()
	win, now := discovery.Window{}, s.clock
	for {
		win = discovery.Observe(win, tip, now, discovery.DefaultSampleGap)
		if win.Span() > discovery.DefaultTakeoverWindow {
			break
		}
		now = now.Add(discovery.DefaultSampleGap)
	}
	s.window, s.clock = win, now

	return nil
}

// theWindowReadsTheHoldStale checks the window this scenario matured
// reads as a stale hold right now, on the same clock that matured it.
func (w *world) theWindowReadsTheHoldStale() error {
	s := w.pc()
	if !discovery.StaleHold(s.window, s.clock, discovery.DefaultTakeoverWindow, discovery.DefaultSampleGap) {
		return fmt.Errorf("window spanning %s did not read stale", s.window.Span())
	}

	return nil
}

// tokenStillMatchesClaim reads holder's persisted token back off its
// real lane and checks it still names the last renewal that actually
// confirmed as landed: a renewal that only looked like it failed never
// got the chance to persist a newer one.
func (w *world) tokenStillMatchesClaim(holder string) error {
	lane, ok := w.pc().lanes[holder]
	if !ok {
		return fmt.Errorf("%q has no real lane recorded", holder)
	}
	want := w.lastConfirmedBeat(holder)
	got := claim.ReadToken(lane, int64(w.planID), gitwt.Exec)
	if got != want {
		return fmt.Errorf("persisted token is %q, want the last confirmed beat %s", got, want)
	}

	return nil
}

// recognizedAsOwnerOfOriginsTip checks OwnAdvance accepts origin's
// current tip as holder's own advance beyond its persisted token — the
// proof a self-resume stands on.
func (w *world) recognizedAsOwnerOfOriginsTip(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	lane, ok := w.pc().lanes[holder]
	if !ok {
		return fmt.Errorf("%q has no real lane recorded", holder)
	}
	token := claim.ReadToken(lane, int64(w.planID), gitwt.Exec)
	tip := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if !claim.OwnAdvance(repo, int64(w.planID), token, tip, gitwt.Exec) {
		return fmt.Errorf("OwnAdvance(%s -> %s) reported false", token, tip)
	}

	return nil
}

// resumesAtTheSameEpoch resumes holder from origin's current tip and
// checks the landed beat kept the original acquisition's epoch — a
// resume never bumps it.
func (w *world) resumesAtTheSameEpoch(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	if _, ok := w.pc().lanes[holder]; !ok {
		return fmt.Errorf("%q has no real lane recorded", holder)
	}
	tip := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)

	lease, err := claim.Resume(repo, w.leaseOptsFor(holder), tip, gitwt.Exec)
	if err != nil {
		return err
	}
	if lease.Epoch != w.lease.Epoch {
		return fmt.Errorf("resume landed at epoch %d, want %d", lease.Epoch, w.lease.Epoch)
	}

	return nil
}

// releasesFromItsRecordedTip releases holder's lease from this
// section's own tracked chain tip — the CAS a stale holder's own
// unwind rides, win or lose.
func (w *world) releasesFromItsRecordedTip(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	from := w.lastConfirmedBeat(holder)
	_, w.err = claim.Release(repo, w.leaseOptsFor(holder), from, w.runnerFor(holder))

	return nil
}

// theReleaseIsFencedNaming reuses the lease world's own fence check —
// a release lost to a foreign move fences exactly the way a renewal
// does, so its assertion is not repeated here, just aimed at a
// release's own error.
func (w *world) theReleaseIsFencedNaming(holder string) error {
	return w.theRenewalIsFencedNaming(holder)
}

// originStillHoldsTheTakeover reuses the lease world's own check that
// origin's tip still carries the takeover — a fenced release deletes
// nothing, so nothing here can have moved it.
func (w *world) originStillHoldsTheTakeover() error {
	return w.originHoldsTheTakeover()
}

// theWorkRefStillExists checks the plan's work ref is still readable
// on origin at all — there is no unleased delete for a fenced release
// to have fired.
func (w *world) theWorkRefStillExists() error {
	repo := w.clones[w.holder]
	out, err := gitCapture(w.t, repo, "ls-remote", "origin", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	if strings.TrimSpace(out) == "" {
		return fmt.Errorf("origin no longer carries %s", w.branch())
	}

	return nil
}

// pinnedInstant is the fixed commit date S33 renews under: any single
// instant does, since the row's whole point is that it never varies.
const pinnedInstant = "2026-01-01T00:00:00Z"

// backwardInstant is a commit date years before pinnedInstant, so a
// beat minted under it always reads with a smaller %ct than its
// parent, regardless of when the test itself runs.
const backwardInstant = "2018-01-01T00:00:00Z"

// commitClockIsPinned pins holder's commit clock to one instant for
// the rest of this subtest — GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
// through t.Setenv, so it cannot leak into a sibling scenario.
func (w *world) commitClockIsPinned(holder string) error {
	if _, err := w.cloneOf(holder); err != nil {
		return err
	}
	w.t.Setenv("GIT_AUTHOR_DATE", pinnedInstant)
	w.t.Setenv("GIT_COMMITTER_DATE", pinnedInstant)

	return nil
}

// commitClockStepsBackward moves holder's commit clock years before
// its last beat.
func (w *world) commitClockStepsBackward(holder string) error {
	if _, err := w.cloneOf(holder); err != nil {
		return err
	}
	w.t.Setenv("GIT_AUTHOR_DATE", backwardInstant)
	w.t.Setenv("GIT_COMMITTER_DATE", backwardInstant)

	return nil
}

// observerSamplesTheCurrentTip folds one look at the holder's current
// tip into the window at this section's own clock, then advances that
// clock — the granular counterpart to observerWatchesTipGoStale, for a
// scenario that cares what one specific sample does rather than
// whether the window eventually matures.
func (w *world) observerSamplesTheCurrentTip() error {
	repo := w.clones[w.holder]
	tip := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if tip == "" {
		return fmt.Errorf("origin holds no tip for plan %d", w.planID)
	}
	s := w.pc()
	s.window = discovery.Observe(s.window, tip, s.clock, discovery.DefaultSampleGap)
	s.clock = s.clock.Add(time.Minute)

	return nil
}

// theTwoBeatsShareADateNotASHA reads the last two beats this
// scenario's renewals produced and checks they carry the same commit
// date but distinct SHAs — the nonce every marker mints keeps a
// frozen clock from ever colliding two markers onto one commit.
func (w *world) theTwoBeatsShareADateNotASHA() error {
	s := w.pc()
	if len(s.beats) < 2 {
		return fmt.Errorf("only %d beats recorded, want 2", len(s.beats))
	}
	first, second := s.beats[len(s.beats)-2], s.beats[len(s.beats)-1]
	if first == second {
		return fmt.Errorf("both beats share one SHA %s", first)
	}
	repo := w.clones[w.holder]
	d1, err := gitCapture(w.t, repo, "log", "-1", "--format=%ct", first)
	if err != nil {
		return fmt.Errorf("%s: %w", d1, err)
	}
	d2, err := gitCapture(w.t, repo, "log", "-1", "--format=%ct", second)
	if err != nil {
		return fmt.Errorf("%s: %w", d2, err)
	}
	if d1 != d2 {
		return fmt.Errorf("commit dates differ: %s vs %s", d1, d2)
	}

	return nil
}

// theWindowResetsToOneSample checks the window this scenario's last
// two observer samples left behind carries exactly one sample and no
// void — a changed tip resets a window outright, it does not throw the
// gap-exceeded flag a stale reconnection would.
func (w *world) theWindowResetsToOneSample() error {
	s := w.pc()
	if s.window.Samples != 1 {
		return fmt.Errorf("window carries %d samples, want 1", s.window.Samples)
	}
	if s.window.Voided != "" {
		return fmt.Errorf("window voided: %q, want no void", s.window.Voided)
	}

	return nil
}

// theWindowReadsTheHoldLive checks StaleHold reads false for the
// window this scenario left behind, both moments after its last
// sample and long after — liveness is tip change, and no choice of
// "now" on its own can manufacture a stale reading over a window that
// never accumulated one.
func (w *world) theWindowReadsTheHoldLive() error {
	s := w.pc()
	near := s.window.Last.Add(time.Minute)
	if discovery.StaleHold(s.window, near, discovery.DefaultTakeoverWindow, discovery.DefaultSampleGap) {
		return errors.New("window read stale moments after its last sample")
	}
	far := s.window.Last.Add(365 * 24 * time.Hour)
	if discovery.StaleHold(s.window, far, discovery.DefaultTakeoverWindow, discovery.DefaultSampleGap) {
		return errors.New("window read stale a year after its last sample")
	}

	return nil
}

// theTipStillMoved checks the last two beats this scenario's renewals
// produced carry different SHAs — a clock stepping backward changes no
// decision a marker's own parent chain makes.
func (w *world) theTipStillMoved() error {
	s := w.pc()
	if len(s.beats) < 2 {
		return fmt.Errorf("only %d beats recorded, want 2", len(s.beats))
	}
	if s.beats[len(s.beats)-1] == s.beats[len(s.beats)-2] {
		return fmt.Errorf("tip did not move: still %s", s.beats[len(s.beats)-1])
	}

	return nil
}

// commitDateOnTipIsSmallerThanParent reads %ct off the last beat and
// its parent and checks the tip's own date is the smaller one — the
// date a human reading the log sees is misleading, and nothing else
// reads it.
func (w *world) commitDateOnTipIsSmallerThanParent() error {
	s := w.pc()
	if len(s.beats) == 0 {
		return errors.New("no beat recorded")
	}
	tip := s.beats[len(s.beats)-1]
	repo := w.clones[w.holder]
	tipDate, err := commitEpoch(w.t, repo, tip)
	if err != nil {
		return err
	}
	parentDate, err := commitEpoch(w.t, repo, tip+"^")
	if err != nil {
		return err
	}
	if tipDate >= parentDate {
		return fmt.Errorf("tip's commit date %d is not smaller than its parent's %d", tipDate, parentDate)
	}

	return nil
}

// commitEpoch reads a commit's %ct — its committer date as Unix
// seconds — as an int64, the plain numeric form %ct exists for so two
// dates can be compared without parsing git's own date syntax.
func commitEpoch(t *testing.T, repo, rev string) (int64, error) {
	t.Helper()
	out, err := gitCapture(t, repo, "log", "-1", "--format=%ct", rev)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", out, err)
	}

	return strconv.ParseInt(strings.TrimSpace(out), 10, 64)
}

// TestPartitionRunnerFailsTheThreeNetworkVerbsAndPassesEverythingElse:
// push, fetch and ls-remote fail as a cut network would; every other
// call — the local plumbing a marker is minted with — reaches base
// untouched.
func TestPartitionRunnerFailsTheThreeNetworkVerbsAndPassesEverythingElse(t *testing.T) {
	var calls []string
	base := func(dir string, args ...string) ([]byte, error) {
		calls = append(calls, args[0])

		return []byte("ok"), nil
	}
	run := partitionRunner(base)

	for _, verb := range []string{"push", "fetch", "ls-remote"} {
		_, err := run("/repo", verb, "origin")
		require.Error(t, err, verb)
		assert.Contains(t, err.Error(), verb)
	}
	for _, verb := range []string{"rev-parse", "commit-tree", "log", "update-ref"} {
		out, err := run("/repo", verb, "HEAD")
		require.NoError(t, err, verb)
		assert.Equal(t, "ok", string(out))
	}
	assert.Equal(t, []string{"rev-parse", "commit-tree", "log", "update-ref"}, calls,
		"the three network verbs never reach base at all")
}

// TestLandedButUnconfirmedRunnerRunsTheRealPushThenReportsItFailed:
// the push actually reaches base and its result is discarded either
// way — a synthetic error always comes back, and ls-remote always
// fails too, so the caller cannot reconcile the two.
func TestLandedButUnconfirmedRunnerRunsTheRealPushThenReportsItFailed(t *testing.T) {
	var pushed bool
	base := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "push" {
			pushed = true
		}

		return nil, nil
	}
	run := landedButUnconfirmedRunner(base)

	_, err := run("/repo", "push", "origin", "marker:ref")
	require.Error(t, err)
	assert.True(t, pushed, "the real push still ran")

	_, err = run("/repo", "ls-remote", "origin", "ref")
	require.Error(t, err)
}

// TestLandedButUnconfirmedRunnerPassesAFailedPushThrough: when the
// real push itself fails, that failure is reported as-is rather than
// papered over with the synthetic "landed" error.
func TestLandedButUnconfirmedRunnerPassesAFailedPushThrough(t *testing.T) {
	pushErr := errors.New("no route to host")
	base := func(dir string, args ...string) ([]byte, error) { return nil, pushErr }
	run := landedButUnconfirmedRunner(base)

	_, err := run("/repo", "push", "origin", "marker:ref")

	require.ErrorIs(t, err, pushErr)
}

// TestRunnerForDefaultsToExecUntilCut: a machine nobody has cut off
// runs on gitwt.Exec, and a heal restores exactly that default.
func TestRunnerForDefaultsToExecUntilCut(t *testing.T) {
	w := newWorld(t)
	w.clones["box-a"] = t.TempDir()

	require.NoError(t, w.theNetworkCutsOff("box-a"))
	assert.NotNil(t, w.runnerFor("box-a"))
	require.NoError(t, w.thePartitionHealsFor("box-a"))
	_, ok := w.pc().runners["box-a"]
	assert.False(t, ok, "a healed machine carries no override at all")
}

// TestTheNetworkCutsOffRefusesAMachineTheScenarioNeverIntroduced.
func TestTheNetworkCutsOffRefusesAMachineTheScenarioNeverIntroduced(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.theNetworkCutsOff("ghost"))
}

// TestThePartitionHealsForRefusesAMachineNeverCut: a heal names a
// machine a prior step cut off; one that was never cut refuses rather
// than silently doing nothing.
func TestThePartitionHealsForRefusesAMachineNeverCut(t *testing.T) {
	w := newWorld(t)
	w.clones["box-a"] = t.TempDir()

	err := w.thePartitionHealsFor("box-a")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "never cut off")
}

// TestNextPushLandsButConfirmationIsLostRefusesAMachineTheScenarioNeverIntroduced.
func TestNextPushLandsButConfirmationIsLostRefusesAMachineTheScenarioNeverIntroduced(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.nextPushLandsButConfirmationIsLost("ghost"))
}

// TestRenewsItsLeaseRefusesAMachineThatNeverHeldTheLease: only the
// holder renews, exactly as bdd_lease_test.go's own renewal step
// insists.
func TestRenewsItsLeaseRefusesAMachineThatNeverHeldTheLease(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	w.clones["box-a"] = t.TempDir()
	w.clones["box-b"] = t.TempDir()

	err := w.renewsItsLease("box-b")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "never held the lease")
}

// TestReleasesFromItsRecordedTipRefusesAMachineThatNeverHeldTheLease.
func TestReleasesFromItsRecordedTipRefusesAMachineThatNeverHeldTheLease(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	w.clones["box-a"] = t.TempDir()
	w.clones["box-b"] = t.TempDir()

	err := w.releasesFromItsRecordedTip("box-b")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "never held the lease")
}

// TestTheRenewalReportsUnconfirmedDistinguishesFromAPlainFault: only
// an UnconfirmedPushError reads as this Then step's promise, not any
// other error a renewal could carry.
func TestTheRenewalReportsUnconfirmedDistinguishesFromAPlainFault(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.theRenewalReportsUnconfirmed(), "no error is not unconfirmed")

	w.err = errors.New("some other fault")
	require.Error(t, w.theRenewalReportsUnconfirmed())

	w.err = &claim.UnconfirmedPushError{PlanID: 7, Err: errors.New("timed out")}
	require.NoError(t, w.theRenewalReportsUnconfirmed())
}

// TestTokenStillMatchesClaimRefusesAHolderWithNoRealLane: the section
// tracks a real lane only for the steps that build one; a holder that
// never got one refuses rather than reading an empty path.
func TestTokenStillMatchesClaimRefusesAHolderWithNoRealLane(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.tokenStillMatchesClaim("box-a"))
}

// TestRecognizedAsOwnerOfOriginsTipRefusesAHolderWithNoRealLane.
func TestRecognizedAsOwnerOfOriginsTipRefusesAHolderWithNoRealLane(t *testing.T) {
	w := newWorld(t)
	w.clones["box-a"] = t.TempDir()

	require.Error(t, w.recognizedAsOwnerOfOriginsTip("box-a"))
}

// TestResumesAtTheSameEpochRefusesAHolderWithNoRealLane.
func TestResumesAtTheSameEpochRefusesAHolderWithNoRealLane(t *testing.T) {
	w := newWorld(t)
	w.clones["box-a"] = t.TempDir()

	require.Error(t, w.resumesAtTheSameEpoch("box-a"))
}

// TestTheWindowResetsToOneSampleReadsSamplesAndVoid: the check fails
// on either more than one sample or a void note, and passes only on
// exactly a fresh, unvoided window.
func TestTheWindowResetsToOneSampleReadsSamplesAndVoid(t *testing.T) {
	w := newWorld(t)
	s := w.pc()

	s.window = discovery.Window{Samples: 2}
	require.Error(t, w.theWindowResetsToOneSample(), "more than one sample")

	s.window = discovery.Window{Samples: 1, Voided: "gap exceeded"}
	require.Error(t, w.theWindowResetsToOneSample(), "a void note")

	s.window = discovery.Window{Samples: 1}
	require.NoError(t, w.theWindowResetsToOneSample())
}

// TestTheWindowReadsTheHoldLiveFailsOnAMaturedWindow: a window that
// really did mature past the takeover window is correctly read as
// stale by StaleHold, so this Then step must fail on it — it exists to
// catch the opposite of what a frozen or backward clock ever produces.
func TestTheWindowReadsTheHoldLiveFailsOnAMaturedWindow(t *testing.T) {
	w := newWorld(t)
	s := w.pc()
	first := s.clock
	s.window = discovery.Window{
		Tip: "abc", First: first, Last: first.Add(3 * time.Hour), Samples: 6,
	}

	err := w.theWindowReadsTheHoldLive()

	require.Error(t, err)
}

// TestTheTwoBeatsShareADateNotASHARequiresTwoBeats.
func TestTheTwoBeatsShareADateNotASHARequiresTwoBeats(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.theTwoBeatsShareADateNotASHA())
}

// TestTheTipStillMovedRequiresTwoBeats.
func TestTheTipStillMovedRequiresTwoBeats(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.theTipStillMoved())

	s := w.pc()
	s.beats = []string{"same", "same"}
	require.Error(t, w.theTipStillMoved(), "an unmoved tip is not a move")

	s.beats = []string{"first", "second"}
	require.NoError(t, w.theTipStillMoved())
}

// TestCommitEpochParsesFormatCt pins commitEpoch to a known commit
// date, the same %ct reading the clock rows lean on to compare two
// commits without parsing git's own date syntax.
func TestCommitEpochParsesFormatCt(t *testing.T) {
	isolate(t)
	repo := t.TempDir()
	git(t, repo, "init", "-q", "-b", "main")
	git(t, repo, "config", "user.email", "t@example.com")
	git(t, repo, "config", "user.name", "frit-test")
	t.Setenv("GIT_AUTHOR_DATE", pinnedInstant)
	t.Setenv("GIT_COMMITTER_DATE", pinnedInstant)
	git(t, repo, "commit", "--allow-empty", "-q", "-m", "pinned")

	got, err := commitEpoch(t, repo, "HEAD")

	require.NoError(t, err)
	want, err := time.Parse(time.RFC3339, pinnedInstant)
	require.NoError(t, err)
	assert.Equal(t, want.Unix(), got)
}

// TestPCLazilyInitializesOncePerWorld: pc's maps and clock are built
// on first use and kept, the same one-per-scenario lifetime section
// itself guarantees.
func TestPCLazilyInitializesOncePerWorld(t *testing.T) {
	w := newWorld(t)

	first := w.pc()
	first.tips["box-a"] = "sha1"

	assert.Same(t, first, w.pc())
	assert.Equal(t, "sha1", w.pc().tips["box-a"])
}

// leaseWorldFixture builds a world with holder already holding a fresh
// lease for planID, mirroring what the shared "holds the lease" step
// does — for a step-level unit test that only needs a live lease on
// origin to act on, without pulling in the full BDD machinery.
func leaseWorldFixture(t *testing.T, holder string, planID int) *world {
	t.Helper()
	isolate(t)
	w := newWorld(t)
	w.planID = planID
	repo := claimableRepo(t, t.TempDir(), "atlas", planID, "Shader unit")
	w.clones[holder] = repo
	lease, err := claim.Acquire(repo, leaseFor(holder, planID), gitwt.Exec)
	require.NoError(t, err)
	w.holder, w.lease = holder, lease

	return w
}

// TestOriginsTipHasNotMovedPassesUntilSomethingRenewsItAway.
func TestOriginsTipHasNotMovedPassesUntilSomethingRenewsItAway(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 900)

	require.NoError(t, w.originsTipHasNotMoved())

	repo := w.clones["box-a"]
	_, err := claim.Renew(repo, leaseFor("box-a", 900), w.lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	require.Error(t, w.originsTipHasNotMoved())
}

// TestOriginsTipHasMovedPastReadsTheLastConfirmedBeat: it refuses
// while origin still sits at the last confirmed beat, and only passes
// once a further, untracked move leaves that beat behind.
func TestOriginsTipHasMovedPastReadsTheLastConfirmedBeat(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 901)
	require.Error(t, w.originsTipHasMovedPast("box-a"), "origin has not moved at all yet")

	repo := w.clones["box-a"]
	lease, err := claim.Renew(repo, leaseFor("box-a", 901), w.lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	w.pc().tips["box-a"] = lease.Tip
	require.Error(t, w.originsTipHasMovedPast("box-a"), "origin matches the last confirmed beat exactly")

	_, err = claim.Renew(repo, leaseFor("box-a", 901), lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	require.NoError(t, w.originsTipHasMovedPast("box-a"))
}

// TestOriginsTipHasMovedPastRefusesAMachineTheScenarioNeverIntroduced.
func TestOriginsTipHasMovedPastRefusesAMachineTheScenarioNeverIntroduced(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.originsTipHasMovedPast("ghost"))
}

// TestTheWindowReadsTheHoldStaleFailsOnAFreshWindow: a single sample
// has no span to mature, so this Then step must refuse it.
func TestTheWindowReadsTheHoldStaleFailsOnAFreshWindow(t *testing.T) {
	w := newWorld(t)
	now := w.pc().clock
	w.pc().window = discovery.Window{Tip: "abc", First: now, Last: now, Samples: 1}

	require.Error(t, w.theWindowReadsTheHoldStale())
}

// TestObserverWatchesTipGoStaleMaturesAStaleWindow: folding looks at
// an unchanged tip on an explicit, self-advancing clock accumulates a
// window StaleHold reads as matured — no sleep, no wall clock.
func TestObserverWatchesTipGoStaleMaturesAStaleWindow(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 908)

	require.NoError(t, w.observerWatchesTipGoStale("box-a"))
	require.NoError(t, w.theWindowReadsTheHoldStale())
	assert.Greater(t, w.pc().window.Span(), discovery.DefaultTakeoverWindow)
}

// TestObserverWatchesTipGoStaleRefusesAMachineWithNoTipOnOrigin: an
// unreadable remote reads as no tip at all, so the step refuses rather
// than folding "" into the window as if it meant something.
func TestObserverWatchesTipGoStaleRefusesAMachineWithNoTipOnOrigin(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	w.clones["box-a"] = t.TempDir()

	require.Error(t, w.observerWatchesTipGoStale("box-a"))
}

// TestObserverSamplesTheCurrentTipAdvancesTheClockAndTheWindow.
func TestObserverSamplesTheCurrentTipAdvancesTheClockAndTheWindow(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 907)
	before := w.pc().clock

	require.NoError(t, w.observerSamplesTheCurrentTip())

	s := w.pc()
	assert.Equal(t, w.lease.Tip, s.window.Tip)
	assert.Equal(t, 1, s.window.Samples)
	assert.True(t, s.clock.After(before))
}

// TestTheWorkRefStillExistsFailsWhenTheRefIsGone: it passes while
// origin still carries the plan's work ref, and fails the moment
// nothing does — there is no unleased delete for a fenced release to
// have fired, so this is the check that would catch one if there were.
func TestTheWorkRefStillExistsFailsWhenTheRefIsGone(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 902)
	require.NoError(t, w.theWorkRefStillExists())

	repo := w.clones["box-a"]
	origin, err := gitCapture(w.t, repo, "config", "--get", "remote.origin.url")
	require.NoError(t, err)
	git(w.t, repo, "push", "-q", strings.TrimSpace(origin), ":"+w.branch())

	require.Error(t, w.theWorkRefStillExists())
}

// TestCommitClockIsPinnedSetsBothDateEnvVars.
func TestCommitClockIsPinnedSetsBothDateEnvVars(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 905)

	require.NoError(t, w.commitClockIsPinned("box-a"))

	assert.Equal(t, pinnedInstant, os.Getenv("GIT_AUTHOR_DATE"))
	assert.Equal(t, pinnedInstant, os.Getenv("GIT_COMMITTER_DATE"))
	require.Error(t, w.commitClockIsPinned("ghost"))
}

// TestCommitClockStepsBackwardSetsBothDateEnvVars.
func TestCommitClockStepsBackwardSetsBothDateEnvVars(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 906)

	require.NoError(t, w.commitClockStepsBackward("box-a"))

	assert.Equal(t, backwardInstant, os.Getenv("GIT_AUTHOR_DATE"))
	assert.Equal(t, backwardInstant, os.Getenv("GIT_COMMITTER_DATE"))
	require.Error(t, w.commitClockStepsBackward("ghost"))
}

// TestCommitDateOnTipIsSmallerThanParentComparesFormatCt.
func TestCommitDateOnTipIsSmallerThanParentComparesFormatCt(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 904)
	require.Error(t, w.commitDateOnTipIsSmallerThanParent(), "no beat recorded yet")

	w.t.Setenv("GIT_AUTHOR_DATE", backwardInstant)
	w.t.Setenv("GIT_COMMITTER_DATE", backwardInstant)
	repo := w.clones["box-a"]
	lease, err := claim.Renew(repo, leaseFor("box-a", 904), w.lease.Tip, gitwt.Exec)
	require.NoError(t, err)
	w.pc().beats = append(w.pc().beats, lease.Tip)

	require.NoError(t, w.commitDateOnTipIsSmallerThanParent())
}

// TestLeaseOptsForUsesTheRealLaneWhenOneWasRecorded: the placeholder
// lane bdd_lease_test.go's leaseFor mints until a step like
// holdsTheLeaseInARealLane records the worktree it actually built.
func TestLeaseOptsForUsesTheRealLaneWhenOneWasRecorded(t *testing.T) {
	w := newWorld(t)
	w.planID = 909

	assert.Equal(t, "/lanes/box-a", w.leaseOptsFor("box-a").Lane)

	w.pc().lanes["box-a"] = "/real/lane"
	assert.Equal(t, "/real/lane", w.leaseOptsFor("box-a").Lane)
}

// TestLastConfirmedBeatFallsBackToTheOriginalClaim.
func TestLastConfirmedBeatFallsBackToTheOriginalClaim(t *testing.T) {
	w := newWorld(t)
	w.lease = claim.Lease{Tip: "claim-sha"}

	assert.Equal(t, "claim-sha", w.lastConfirmedBeat("box-a"))

	w.pc().tips["box-a"] = "beat-sha"
	assert.Equal(t, "beat-sha", w.lastConfirmedBeat("box-a"))
}

// TestTheReleaseIsFencedNamingDelegatesToTheSharedFenceCheck: it reuses
// bdd_lease_test.go's own fence assertion rather than repeating it, so
// this test pins the delegation, not the assertion's own logic.
func TestTheReleaseIsFencedNamingDelegatesToTheSharedFenceCheck(t *testing.T) {
	w := newWorld(t)
	w.err = &claim.FenceError{Known: true, Marker: claim.Marker{Holder: "box-b"}}

	require.NoError(t, w.theReleaseIsFencedNaming("box-b"))
	require.Error(t, w.theReleaseIsFencedNaming("box-c"))
}

// TestOriginStillHoldsTheTakeoverDelegatesToTheSharedCheck: it reuses
// the lease world's own originHoldsTheTakeover, so an origin that
// cannot back the recorded takeover reads as an error exactly as that
// check would report it directly.
func TestOriginStillHoldsTheTakeoverDelegatesToTheSharedCheck(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	w.clones["box-a"] = t.TempDir()
	w.taken = claim.Lease{Tip: "unreachable-sha"}

	require.Error(t, w.originStillHoldsTheTakeover())
}

// TestHoldsTheLeaseInARealLaneBuildsAWorktreeWithNoTokenPersistedYet:
// Acquire runs before the worktree exists, so persistToken is a no-op
// at this point — exactly the gap S21's own renewal step closes.
func TestHoldsTheLeaseInARealLaneBuildsAWorktreeWithNoTokenPersistedYet(t *testing.T) {
	w := newWorld(t)

	require.NoError(t, w.holdsTheLeaseInARealLane("box-a", 910))

	assert.Equal(t, "box-a", w.holder)
	lane, ok := w.pc().lanes["box-a"]
	require.True(t, ok)
	_, err := os.Stat(filepath.Join(lane, ".git"))
	require.NoError(t, err, "a real worktree was created")
	assert.Empty(t, claim.ReadToken(lane, 910, gitwt.Exec),
		"Acquire ran before the worktree existed: nothing persisted yet")
}
