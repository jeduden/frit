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
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/observe"
	"github.com/jeduden/frit/internal/report"
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
	registrars = append(registrars, (*world).registerVerbLevelPartitionsAndClocks)
	registrars = append(registrars, (*world).registerCrossHostClocks)
}

// pcState is this section's own state, kept beside the shared world
// via section — never a field on world itself. It carries a Runner per
// machine (gitwt.Exec until a partition step swaps one in and a heal
// step swaps it back — a machine's presence in runners is what "cut
// off" means, so no separate flag tracks it), each machine's real
// worktree lane where one was built, the chain of beats a scenario's
// own renewals produced, and the observation window a step advances
// on a clock it — never time.Now — chooses.
type pcState struct {
	runners map[string]gitwt.Runner
	lanes   map[string]string
	tips    map[string]string
	beats   []string
	window  discovery.Window
	clock   time.Time
	// heldPlans is S23's own synthetic fleet: a plan list built by hand
	// rather than gathered from a repository, since observeHolds only
	// ever compares HoldTip strings, never resolves them through git.
	heldPlans []discovery.Plan
	// board is the document a "reads the board" step last decoded, for
	// a following Then to read.
	board *report.BoardDoc
	// observerRoot and observerRepo are the --root directory and the
	// checkout under it an "observer" step built — a second, discoverable
	// clone distinct from any holder's own, so cutting it off from
	// origin cannot also cut off the holder's own renewals.
	observerRoot string
	observerRepo string
	// hostWindows and hostClocks are S36's own shape: more than one
	// observer's own window and clock, keyed by host, kept apart from
	// the singular window/clock pair every other row in this file
	// shares — two hosts converging on the same tip despite years of
	// skew between them, never one host's clock read against the
	// other's window.
	hostWindows map[string]discovery.Window
	hostClocks  map[string]time.Time
}

// pc fetches this scenario's section state, lazily initialized on
// first use: a fresh set of maps and a clock pinned to a fixed instant
// rather than wherever time.Now happens to sit when the test runs.
func (w *world) pc() *pcState {
	s := section[pcState](w)
	if s.runners == nil {
		s.runners = map[string]gitwt.Runner{}
		s.lanes = map[string]string{}
		s.tips = map[string]string{}
		s.clock = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		s.hostWindows = map[string]discovery.Window{}
		s.hostClocks = map[string]time.Time{}
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

// registerVerbLevelPartitionsAndClocks binds the step texts S22, S23,
// S24 and S35 add — a second registrar, kept apart from
// registerPartitionsAndClocks only to stay under this file's own
// statement-count lint budget, not a different section of state.
func (w *world) registerVerbLevelPartitionsAndClocks(sc *godog.ScenarioContext) {
	sc.Step(`^an observer has already watched "([^"]+)"'s tip for a while$`, w.observerHasAlreadyWatchedTipForAWhile)
	sc.Step(`^the network cuts the observer off from origin$`, w.theNetworkCutsTheObserverOffFromOrigin)
	sc.Step(`^the observer reads the board$`, w.theObserverReadsTheBoard)
	sc.Step(`^the board reports "([^"]+)"'s observed-at age$`, w.theBoardReportsObservedAtAge)
	sc.Step(`^the board reports the observer's fetch as unreachable$`, w.theBoardReportsTheObserversFetchAsUnreachable)
	sc.Step(`^several held plans were each observed a while ago$`, w.severalHeldPlansWereEachObservedAWhileAgo)
	sc.Step(`^the gap since each one's last sample exceeds the sample-gap bound$`,
		w.theGapSinceEachOnesLastSampleExceedsTheBound)
	sc.Step(`^the fleet is observed again, now that origin is reachable$`,
		w.theFleetIsObservedAgainNowThatOriginIsReachable)
	sc.Step(`^every window resets to one sample$`, w.everyWindowResetsToOneSample)
	sc.Step(`^no plan reads its takeover window matured$`, w.noPlanReadsItsTakeoverWindowMatured)
	sc.Step(`^the renewal is a plain win$`, w.theRenewalIsAPlainWin)
	sc.Step(`^an observer clones origin, catching up with the renewed tip$`,
		w.anObserverClonesOriginCatchingUpWithTheRenewedTip)
	sc.Step(`^the board still reports "([^"]+)" held at the renewed tip$`, w.theBoardStillReportsHeldAtTheRenewedTip)
	sc.Step(`^origin holds the takeover$`, w.originHoldsTheTakeover)
	sc.Step(`^a further observer watches "([^"]+)"'s tip mature by the same span$`, w.observerWatchesTipGoStale)
	sc.Step(`^that span does not read stale once the takeover count backs the threshold off$`,
		w.thatSpanDoesNotReadStaleOnceBackedOff)
}

// registerCrossHostClocks binds the step texts S36 adds — its own
// registrar, kept apart only for this file's own lint budget, not a
// different section of state.
func (w *world) registerCrossHostClocks(sc *godog.ScenarioContext) {
	sc.Step(`^a second host's clock is skewed years from the first's$`, w.aSecondHostsClockIsSkewedYearsFromTheFirsts)
	sc.Step(`^both hosts watch "([^"]+)"'s tip go stale, each on its own clock$`,
		w.bothHostsWatchTipGoStaleEachOnItsOwnClock)
	sc.Step(`^both hosts' windows read the hold stale$`, w.bothHostsWindowsReadTheHoldStale)
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

	return nil
}

// thePartitionHealsFor restores holder's Runner to gitwt.Exec. A
// machine that was never cut off refuses — a heal names what it heals.
func (w *world) thePartitionHealsFor(holder string) error {
	if _, err := w.cloneOf(holder); err != nil {
		return err
	}
	s := w.pc()
	if _, ok := s.runners[holder]; !ok {
		return fmt.Errorf("%q was never cut off from origin; nothing to heal", holder)
	}
	delete(s.runners, holder)

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
//
// The two step texts disagree about what "the current tip" is:
// bdd_lease_test.go's own comesBackAndRenews always CASes from the
// shared w.lease.Tip and never updates this section's tips/beats. No
// scenario in this file mixes the two after this file's own chain has
// advanced, so today they agree by coincidence; a scenario that did
// mix them would CAS from a stale tip and fail with a spurious fence.
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
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
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

// matureWindow folds one look at tip into a fresh window, over and
// over on a clock the caller chose — never time.Now — until the span
// exceeds the takeover window, the same accumulation a live observer
// would eventually reach by polling. It returns the matured window and
// the clock value its last sample landed on, so a caller can fold
// either back into whatever state it tracks; two independent callers
// sharing this one loop (a single observer's own maturation, and each
// of several hosts maturing a window of its own) can never drift apart
// on what "matured" means.
func matureWindow(tip string, now time.Time) (discovery.Window, time.Time) {
	win := discovery.Window{}
	for {
		win = discovery.Observe(win, tip, now, discovery.DefaultSampleGap)
		if win.Span() > discovery.DefaultTakeoverWindow {
			break
		}
		now = now.Add(discovery.DefaultSampleGap)
	}

	return win, now
}

// observerWatchesTipGoStale matures a window over holder's current
// tip on this section's own clock, via matureWindow, and stores both
// on w.pc() — the shape S20, S25 and S35 all reuse.
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
	s.window, s.clock = matureWindow(tip, s.clock)

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
	s := w.pc()
	if _, ok := s.lanes[holder]; !ok {
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
	// A resume is a landed beat exactly as a renewal is: record it on
	// the section's own tracked chain, so a later step CASing from
	// lastConfirmedBeat starts from what actually landed rather than
	// the pre-resume tip — resume happens to be every row's own last
	// step today, but nothing should silently depend on that staying
	// true.
	s.tips[holder] = lease.Tip
	s.beats = append(s.beats, lease.Tip)

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
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
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
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
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
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	d1, err := commitEpoch(w.t, repo, first)
	if err != nil {
		return err
	}
	d2, err := commitEpoch(w.t, repo, second)
	if err != nil {
		return err
	}
	if d1 != d2 {
		return fmt.Errorf("commit dates differ: %d vs %d", d1, d2)
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
	// StaleHold refuses to act on a window it has not confirmed
	// recently (now.Sub(Last) > sMax), so a window gone quiet for a
	// year also reads live — but that guard alone would make this
	// check pass on any window, matured or not. A hypothetical window
	// that really had matured a full takeover span, sampled at the
	// same near moment, must still read stale — proving this
	// scenario's own window reads live because it is genuinely fresh,
	// not because the check is structurally toothless.
	far := s.window.Last.Add(365 * 24 * time.Hour)
	if discovery.StaleHold(s.window, far, discovery.DefaultTakeoverWindow, discovery.DefaultSampleGap) {
		return errors.New("window read stale a year after its last sample")
	}
	matured := discovery.Window{
		Tip: s.window.Tip, First: s.window.Last.Add(-3 * time.Hour), Last: s.window.Last, Samples: 6,
	}
	if !discovery.StaleHold(matured, near, discovery.DefaultTakeoverWindow, discovery.DefaultSampleGap) {
		return errors.New("a genuinely matured window at the same moment reads live too; StaleHold itself looks broken")
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
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
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

// observerClone clones originURL into filepath.Join(root, name) — a
// checkout under a shared --root that `board --root root` actually
// discovers, unlike cloneAgain's own throwaway t.TempDir(). name is
// the repo name a scenario's plans are keyed under, so the clone reads
// as the same repository a holder's own checkout does.
func observerClone(t *testing.T, root, name, originURL string) string {
	t.Helper()
	dst := filepath.Join(root, name)
	git(t, root, "clone", "-q", originURL, dst)
	git(t, dst, "config", "user.email", "observer@example.com")
	git(t, dst, "config", "user.name", "frit-observer")

	return dst
}

// buildObserverClone builds a second, discoverable checkout of
// holder's own origin, under a fresh --root of its own, and records it
// on this section's state for the network-cut and board-read steps
// that follow.
func (w *world) buildObserverClone(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	origin, err := gitCapture(w.t, repo, "config", "--get", "remote.origin.url")
	if err != nil {
		return fmt.Errorf("%s: %w", origin, err)
	}
	s := w.pc()
	s.observerRoot = w.t.TempDir()
	s.observerRepo = observerClone(w.t, s.observerRoot, "atlas", strings.TrimSpace(origin))

	return nil
}

// observedSpan is how long S22's own seeded window has already
// watched the tip before anything is cut — well past any takeover
// threshold, and the span theBoardReportsObservedAtAge checks the
// board's own displayed age against, so the two can never drift
// apart.
const observedSpan = 3 * time.Hour

// observerHasAlreadyWatchedTipForAWhile builds the observer's checkout
// and seeds it a window already matured to observedSpan — S22's own
// shape, so a following cut and read is not a race against the test's
// own runtime.
func (w *world) observerHasAlreadyWatchedTipForAWhile(holder string) error {
	if err := w.buildObserverClone(holder); err != nil {
		return err
	}
	seedWindow(w.t, "atlas", int64(w.planID), w.lease.Tip, observedSpan)

	return nil
}

// anObserverClonesOriginCatchingUpWithTheRenewedTip builds the
// observer checkout only after a renewal has already landed — S24's
// own shape, so the clone already carries the fresh tip with no fetch
// needed to see it, and seeds no window: this row asserts
// classification, never a staleness age.
//
// Why a second checkout rather than one holder's own, with `pushurl`
// split from `url`: `ls-remote` — the read casPush confirms a push
// through — travels over `url`, the same one `fetch` does, so a
// checkout whose `url` is broken can never land a *confirmed* renewal
// at all; it would read unconfirmed, S21's own shape, not this row's
// clean win. A confirmed renewal and a broken read can only coexist
// on two different checkouts, so that is what this row builds.
func (w *world) anObserverClonesOriginCatchingUpWithTheRenewedTip() error {
	return w.buildObserverClone(w.holder)
}

// theNetworkCutsTheObserverOffFromOrigin points the observer's own
// remote at a path that does not exist — the holder's remote is
// untouched, so nothing here can affect its own ability to renew.
func (w *world) theNetworkCutsTheObserverOffFromOrigin() error {
	s := w.pc()
	if s.observerRepo == "" {
		return errors.New("no observer checkout recorded; a step that builds one comes first")
	}
	git(w.t, s.observerRepo, "remote", "set-url", "origin", filepath.Join(w.t.TempDir(), "gone"))

	return nil
}

// theObserverReadsTheBoard runs the real board verb, --root pointed at
// the observer's own directory, and decodes its JSON document for a
// following Then to read.
func (w *world) theObserverReadsTheBoard() error {
	s := w.pc()
	if s.observerRoot == "" {
		return errors.New("no observer root recorded; a step that builds one comes first")
	}
	var doc report.BoardDoc
	emit(w.t, &doc, "board", "--root", s.observerRoot)
	s.board = &doc

	return nil
}

// theBoardReportsObservedAtAge checks the last board read carries a
// nonzero observed-at age for holder's plan — the display an observer
// cut off from origin still owes a reader, from whatever it last saw.
func (w *world) theBoardReportsObservedAtAge(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	p, err := w.boardPlan()
	if err != nil {
		return err
	}
	// A lower bound, not an exact value: the real wall clock ran a
	// little further between the seed and this read. But it must be
	// close to observedSpan, not merely positive — a bug that folded
	// the wrong window in (a fresh one, say) would still report some
	// small positive age and slip past a bare ">0" check.
	want := int64((observedSpan - 2*time.Minute).Seconds())
	if p.StaleSeconds < want {
		return fmt.Errorf("plan %d's observed-at age is %ds, want at least %ds", w.planID, p.StaleSeconds, want)
	}

	return nil
}

// theBoardReportsTheObserversFetchAsUnreachable checks the last board
// read named the observer's own failed fetch — a Problem, not a
// crash, not a silently-dropped plan.
func (w *world) theBoardReportsTheObserversFetchAsUnreachable() error {
	s := w.pc()
	if s.board == nil {
		return errors.New("the board was never read")
	}
	for _, p := range s.board.Problems {
		if strings.Contains(p.Message, "could not fetch") {
			return nil
		}
	}

	return fmt.Errorf("no problem names a failed fetch: %+v", s.board.Problems)
}

// theBoardStillReportsHeldAtTheRenewedTip checks holder's plan still
// reads held on the board an observer read after cutting itself off —
// the classification a renewal elsewhere never corrupts.
func (w *world) theBoardStillReportsHeldAtTheRenewedTip(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	p, err := w.boardPlan()
	if err != nil {
		return err
	}
	if !p.Held {
		return fmt.Errorf("plan %d no longer reads held", w.planID)
	}
	// report.BoardPlan carries no tip field to compare a JSON value
	// against, so "at the renewed tip" is checked the way it actually
	// has to be: the observer's own checkout — the one board itself
	// walked to answer Held — must carry the same tip origin does,
	// read off its own local remote-tracking ref (its own fetch is
	// already cut by this point, so a live probe of origin would fail
	// here even though the cached view board itself read is correct).
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	want := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	s := w.pc()
	got, err := gitCapture(w.t, s.observerRepo, "rev-parse", "refs/remotes/origin/"+claim.Branch(int64(w.planID)))
	if err != nil {
		return fmt.Errorf("%s: %w", got, err)
	}
	if strings.TrimSpace(got) != want {
		return fmt.Errorf("the observer's own cached view carries %s, want the renewed tip %s",
			strings.TrimSpace(got), want)
	}

	return nil
}

// boardPlan finds this scenario's own plan on the last board document
// read, refusing when no read happened yet or the plan is missing from
// it — a Then step should fail loudly on either rather than reading a
// zero BoardPlan as if it meant something.
func (w *world) boardPlan() (report.BoardPlan, error) {
	s := w.pc()
	if s.board == nil {
		return report.BoardPlan{}, errors.New("the board was never read")
	}
	for _, p := range s.board.Plans {
		// Repo, not just ID: every scenario in this file mints its
		// plan in "atlas", the same repo claimableRepo and
		// observerClone both name, so a board carrying more than one
		// repository can never match this section's own plan by
		// coincidence of ID alone.
		if p.ID == int64(w.planID) && p.Repo == "atlas" {
			return p, nil
		}
	}

	return report.BoardPlan{}, fmt.Errorf("the board does not carry plan %d in atlas", w.planID)
}

// theRenewalIsAPlainWin checks the last renewal landed with no error
// at all — S24's own positive case, the mirror of every "reports the
// push unconfirmed" Then the earlier rows already carry.
func (w *world) theRenewalIsAPlainWin() error {
	if w.err != nil {
		return fmt.Errorf("expected a plain win, got %v", w.err)
	}

	return nil
}

// severalHeldPlansWereEachObservedAWhileAgo builds S23's own synthetic
// fleet — several plans with no repository behind them at all, since
// observeHolds only ever compares HoldTip strings — and seeds each a
// window already freshly observed at this section's own clock.
func (w *world) severalHeldPlansWereEachObservedAWhileAgo() error {
	isolate(w.t)
	s := w.pc()
	path, err := observe.Path()
	if err != nil {
		return err
	}
	state := observe.State{}
	var plans []discovery.Plan
	for _, id := range []int64{23, 24, 25} {
		tip := fmt.Sprintf("tip-%d", id)
		plans = append(plans, discovery.Plan{Repo: "atlas", ID: id, HoldTip: tip, Held: true})
		state[observe.Key("atlas", id)] = discovery.Window{
			Tip: tip, First: s.clock.Add(-3 * time.Hour), Last: s.clock, Samples: 9,
		}
	}
	if err := observe.Save(path, state); err != nil {
		return err
	}
	s.heldPlans = plans

	return nil
}

// theGapSinceEachOnesLastSampleExceedsTheBound advances this section's
// own clock past DefaultSampleGap with no new sample taken — the
// partition itself, on the explicit clock every row in this file
// reads decisions from, never the wall clock.
func (w *world) theGapSinceEachOnesLastSampleExceedsTheBound() error {
	s := w.pc()
	if len(s.heldPlans) == 0 {
		return errors.New("no held plans recorded; a step that seeds them comes first")
	}
	s.clock = s.clock.Add(discovery.DefaultSampleGap + time.Minute)

	return nil
}

// theFleetIsObservedAgainNowThatOriginIsReachable calls observeHolds
// directly against S23's synthetic fleet and this section's own clock
// — no repository, no gitwt.Runner beyond the unused default main
// wires up, since no plan here carries a coordinate for TakeoverCount
// to read.
func (w *world) theFleetIsObservedAgainNowThatOriginIsReachable() error {
	s := w.pc()
	if len(s.heldPlans) == 0 {
		return errors.New("no held plans recorded; a step that seeds them comes first")
	}
	res := &fleet.Result{Plans: s.heldPlans}
	observeHolds(res, &runtime{git: gitwt.Exec}, s.clock)
	s.heldPlans = res.Plans

	return nil
}

// everyWindowResetsToOneSample checks the persisted state left every
// seeded plan's window a fresh, voided single sample — a restart, not
// the false maturity a naive elapsed-time reading would have shown.
func (w *world) everyWindowResetsToOneSample() error {
	s := w.pc()
	path, err := observe.Path()
	if err != nil {
		return err
	}
	state := observe.Load(path)
	for _, p := range s.heldPlans {
		key := observe.Key(p.Repo, p.ID)
		win, ok := state[key]
		if !ok {
			return fmt.Errorf("no window recorded for %s", key)
		}
		if win.Samples != 1 {
			return fmt.Errorf("plan %d window carries %d samples, want 1", p.ID, win.Samples)
		}
		if win.Voided == "" {
			return fmt.Errorf("plan %d window carries no void note", p.ID)
		}
	}

	return nil
}

// noPlanReadsItsTakeoverWindowMatured checks observeHolds left every
// seeded plan un-stale — no mass takeover fires on heal.
func (w *world) noPlanReadsItsTakeoverWindowMatured() error {
	s := w.pc()
	for _, p := range s.heldPlans {
		if p.Stale {
			return fmt.Errorf("plan %d reads stale after the partition healed", p.ID)
		}
	}

	return nil
}

// thatSpanDoesNotReadStaleOnceBackedOff reads the takeover count off
// the real chain w.taker's own takeover minted, computes the same
// backed-off threshold observeHolds does, and checks the span this
// section's window last matured to reads stale under the bare window
// — proving the row actually matured something — but not under the
// backed-off one, the pair that pins the damping rather than either
// half alone.
func (w *world) thatSpanDoesNotReadStaleOnceBackedOff() error {
	s := w.pc()
	repo, err := w.cloneOf(w.taker)
	if err != nil {
		return err
	}
	base, err := gitCapture(w.t, repo, "rev-parse", "origin/main")
	if err != nil {
		return fmt.Errorf("%s: %w", base, err)
	}
	k := claim.TakeoverCount(repo, int64(w.planID), strings.TrimSpace(base), w.taken.Tip, gitwt.Exec)
	if k == 0 {
		return errors.New("no takeover marker found in the chain; the backoff was never tested")
	}
	threshold := time.Duration(k+1) * discovery.DefaultTakeoverWindow
	if !discovery.StaleHold(s.window, s.clock, discovery.DefaultTakeoverWindow, discovery.DefaultSampleGap) {
		return errors.New("the span did not mature past the base window; the backoff was never tested")
	}
	if discovery.StaleHold(s.window, s.clock, threshold, discovery.DefaultSampleGap) {
		return fmt.Errorf("window still reads stale under the backed-off threshold %s (k=%d)", threshold, k)
	}

	return nil
}

// aSecondHostsClockIsSkewedYearsFromTheFirsts seeds two hosts' own
// starting clocks, years apart, with no window yet for either — the
// premise S36's own maturation and Then steps build on.
func (w *world) aSecondHostsClockIsSkewedYearsFromTheFirsts() error {
	s := w.pc()
	s.hostClocks["host-1"] = s.clock
	s.hostClocks["host-2"] = s.clock.AddDate(7, 0, 0)

	return nil
}

// bothHostsWatchTipGoStaleEachOnItsOwnClock matures a fresh window per
// host, independently, on that host's own clock alone —
// observerWatchesTipGoStale's own loop, run once per entry in
// hostClocks, so no host's clock ever informs another's window.
func (w *world) bothHostsWatchTipGoStaleEachOnItsOwnClock(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	tip := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if tip == "" {
		return fmt.Errorf("origin holds no tip for plan %d", w.planID)
	}
	s := w.pc()
	if len(s.hostClocks) == 0 {
		return errors.New("no hosts recorded; a step that skews their clocks comes first")
	}
	for host, now := range s.hostClocks {
		s.hostWindows[host], s.hostClocks[host] = matureWindow(tip, now)
	}

	return nil
}

// bothHostsWindowsReadTheHoldStale checks every host's own window
// reads stale against only that host's own clock — the convergence
// S36 promises, proven by construction: no host's clock is ever read
// here against another host's window.
func (w *world) bothHostsWindowsReadTheHoldStale() error {
	s := w.pc()
	if len(s.hostWindows) < 2 {
		return fmt.Errorf("only %d host windows recorded, want 2", len(s.hostWindows))
	}
	for host, win := range s.hostWindows {
		now := s.hostClocks[host]
		if !discovery.StaleHold(win, now, discovery.DefaultTakeoverWindow, discovery.DefaultSampleGap) {
			return fmt.Errorf("host %q's window did not read stale", host)
		}
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

// TestResumesAtTheSameEpochRecordsTheLandedBeat: a resume is a landed
// beat exactly as a renewal is — it must update the section's own
// tracked chain, or a later step CASing from lastConfirmedBeat would
// start from the stale pre-resume tip instead of what actually landed.
func TestResumesAtTheSameEpochRecordsTheLandedBeat(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, w.holdsTheLeaseInARealLane("box-a", 923))
	require.NoError(t, w.renewsItsLease("box-a"))
	require.NoError(t, w.nextPushLandsButConfirmationIsLost("box-a"))
	require.NoError(t, w.renewsItsLease("box-a"))
	require.NoError(t, w.thePartitionHealsFor("box-a"))
	repo := w.clones["box-a"]

	require.NoError(t, w.resumesAtTheSameEpoch("box-a"))

	landed := claim.RemoteTip(repo, "origin", 923, gitwt.Exec)
	assert.Equal(t, landed, w.lastConfirmedBeat("box-a"),
		"the section's own chain now starts from the resumed beat, not the pre-resume tip")
	assert.Equal(t, landed, w.pc().beats[len(w.pc().beats)-1])
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

// TestObserverCloneBuildsADiscoverableCheckout: the clone lands at
// filepath.Join(root, name), the layout board --root root actually
// walks, and it carries the origin's own history.
func TestObserverCloneBuildsADiscoverableCheckout(t *testing.T) {
	isolate(t)
	origin := claimableRepo(t, t.TempDir(), "atlas", 911, "Shader unit")
	url, err := gitCapture(t, origin, "config", "--get", "remote.origin.url")
	require.NoError(t, err)
	root := t.TempDir()

	dst := observerClone(t, root, "atlas", strings.TrimSpace(url))

	assert.Equal(t, filepath.Join(root, "atlas"), dst)
	out, err := gitCapture(t, dst, "log", "-1", "--format=%s")
	require.NoError(t, err)
	assert.NotEmpty(t, strings.TrimSpace(out))
}

// TestBuildObserverCloneRefusesAMachineTheScenarioNeverIntroduced.
func TestBuildObserverCloneRefusesAMachineTheScenarioNeverIntroduced(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.buildObserverClone("ghost"))
}

// TestObserverHasAlreadyWatchedTipForAWhileBuildsAMaturedWindow: the
// clone lands under a fresh --root and the seeded window already
// spans well past the takeover threshold, on the observe store the
// same isolate(t) call the holder step made points at.
func TestObserverHasAlreadyWatchedTipForAWhileBuildsAMaturedWindow(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 912)

	require.NoError(t, w.observerHasAlreadyWatchedTipForAWhile("box-a"))

	s := w.pc()
	assert.NotEmpty(t, s.observerRoot)
	_, err := os.Stat(filepath.Join(s.observerRepo, ".git"))
	require.NoError(t, err)
	path, err := observe.Path()
	require.NoError(t, err)
	win := observe.Load(path)[observe.Key("atlas", 912)]
	assert.Greater(t, win.Span(), discovery.DefaultTakeoverWindow)
}

// TestAnObserverClonesOriginCatchingUpWithTheRenewedTipSeedsNoWindow:
// S24's own shape needs only a fresh clone, never a staleness age.
func TestAnObserverClonesOriginCatchingUpWithTheRenewedTipSeedsNoWindow(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 913)

	require.NoError(t, w.anObserverClonesOriginCatchingUpWithTheRenewedTip())

	path, err := observe.Path()
	require.NoError(t, err)
	_, ok := observe.Load(path)[observe.Key("atlas", 913)]
	assert.False(t, ok, "this row never seeds a window")
}

// TestTheNetworkCutsTheObserverOffFromOriginRefusesWithNoObserver.
func TestTheNetworkCutsTheObserverOffFromOriginRefusesWithNoObserver(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.theNetworkCutsTheObserverOffFromOrigin())
}

// TestTheNetworkCutsTheObserverOffFromOriginBreaksItsFetchAlone: the
// observer's own remote stops resolving; a sibling holder checkout is
// never touched by this step at all.
func TestTheNetworkCutsTheObserverOffFromOriginBreaksItsFetchAlone(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 914)
	require.NoError(t, w.observerHasAlreadyWatchedTipForAWhile("box-a"))

	require.NoError(t, w.theNetworkCutsTheObserverOffFromOrigin())

	_, err := gitCapture(w.t, w.pc().observerRepo, "fetch", "origin")
	assert.Error(t, err, "the observer's own fetch now fails")
	_, err = gitCapture(w.t, w.clones["box-a"], "fetch", "origin")
	assert.NoError(t, err, "the holder's own remote is untouched")
}

// TestTheObserverReadsTheBoardRefusesWithNoObserverRoot.
func TestTheObserverReadsTheBoardRefusesWithNoObserverRoot(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.theObserverReadsTheBoard())
}

// TestTheObserverReadsTheBoardDecodesTheDocument: a real board read,
// through the CLI, lands the plan on w.pc().board for a Then step to
// read.
func TestTheObserverReadsTheBoardDecodesTheDocument(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 915)
	require.NoError(t, w.observerHasAlreadyWatchedTipForAWhile("box-a"))

	require.NoError(t, w.theObserverReadsTheBoard())

	require.NotNil(t, w.pc().board)
	found := false
	for _, p := range w.pc().board.Plans {
		if p.ID == 915 {
			found = true
		}
	}
	assert.True(t, found, "the observer's own board carries the plan")
}

// TestBoardPlanRefusesWithNoReadOrAMissingPlan.
func TestBoardPlanRefusesWithNoReadOrAMissingPlan(t *testing.T) {
	w := newWorld(t)
	w.planID = 916

	_, err := w.boardPlan()
	require.Error(t, err, "the board was never read")

	w.pc().board = &report.BoardDoc{}
	_, err = w.boardPlan()
	require.Error(t, err, "the plan is missing from the document")

	w.pc().board.Plans = []report.BoardPlan{{ID: 916, Repo: "other", Held: true}}
	_, err = w.boardPlan()
	require.Error(t, err, "same ID, wrong repo, must not match")

	w.pc().board.Plans = []report.BoardPlan{{ID: 916, Repo: "atlas", Held: true}}
	got, err := w.boardPlan()
	require.NoError(t, err)
	assert.True(t, got.Held)
}

// TestTheBoardReportsObservedAtAgeReadsStaleSecondsAndTheHolder.
func TestTheBoardReportsObservedAtAgeReadsStaleSecondsAndTheHolder(t *testing.T) {
	w := newWorld(t)
	w.holder, w.planID = "box-a", 917
	w.pc().board = &report.BoardDoc{
		Plans: []report.BoardPlan{{ID: 917, Repo: "atlas", StaleSeconds: int64(observedSpan.Seconds())}},
	}

	require.NoError(t, w.theBoardReportsObservedAtAge("box-a"))
	require.Error(t, w.theBoardReportsObservedAtAge("box-b"), "box-a held it, not box-b")

	w.pc().board.Plans[0].StaleSeconds = 60
	require.Error(t, w.theBoardReportsObservedAtAge("box-a"),
		"an age far short of the seeded span is not the display this row promises")
}

// TestTheBoardReportsTheObserversFetchAsUnreachableReadsProblems.
func TestTheBoardReportsTheObserversFetchAsUnreachableReadsProblems(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.theBoardReportsTheObserversFetchAsUnreachable(), "the board was never read")

	w.pc().board = &report.BoardDoc{Problems: []report.Problem{{Repo: "atlas", Message: "some other fault"}}}
	require.Error(t, w.theBoardReportsTheObserversFetchAsUnreachable())

	w.pc().board.Problems[0].Message = "could not fetch origin; remote-tracking view may be stale"
	require.NoError(t, w.theBoardReportsTheObserversFetchAsUnreachable())
}

// TestTheBoardStillReportsHeldAtTheRenewedTipRefusesBeforeTouchingGit:
// the not-held and wrong-holder refusals both fire before this step
// ever reads a checkout, so neither needs one set up.
func TestTheBoardStillReportsHeldAtTheRenewedTipRefusesBeforeTouchingGit(t *testing.T) {
	w := newWorld(t)
	w.holder, w.planID = "box-a", 918
	w.pc().board = &report.BoardDoc{Plans: []report.BoardPlan{{ID: 918, Repo: "atlas", Held: false}}}

	require.Error(t, w.theBoardStillReportsHeldAtTheRenewedTip("box-a"), "not held")
	require.Error(t, w.theBoardStillReportsHeldAtTheRenewedTip("box-b"), "box-a held it, not box-b")
}

// TestTheBoardStillReportsHeldAtTheRenewedTipComparesTheObserversCachedView:
// once Held is true, the step reads the observer's own local
// remote-tracking ref (its fetch is already cut) and compares it
// against origin's real tip — the check S24 exists to run.
func TestTheBoardStillReportsHeldAtTheRenewedTipComparesTheObserversCachedView(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 919)
	require.NoError(t, w.buildObserverClone("box-a"))
	w.pc().board = &report.BoardDoc{Plans: []report.BoardPlan{{ID: 919, Repo: "atlas", Held: true}}}

	require.NoError(t, w.theBoardStillReportsHeldAtTheRenewedTip("box-a"))

	wrong, err := gitCapture(w.t, w.pc().observerRepo, "rev-parse", "HEAD")
	require.NoError(t, err)
	git(w.t, w.pc().observerRepo, "update-ref", "refs/remotes/origin/"+claim.Branch(919), strings.TrimSpace(wrong))

	require.Error(t, w.theBoardStillReportsHeldAtTheRenewedTip("box-a"),
		"a cached view stuck on the wrong tip must not pass")
}

// TestTheRenewalIsAPlainWinReadsWErr.
func TestTheRenewalIsAPlainWinReadsWErr(t *testing.T) {
	w := newWorld(t)

	require.NoError(t, w.theRenewalIsAPlainWin())

	w.err = errors.New("fenced")
	require.Error(t, w.theRenewalIsAPlainWin())
}

// TestSeveralHeldPlansWereEachObservedAWhileAgoSeedsThreeFreshWindows.
func TestSeveralHeldPlansWereEachObservedAWhileAgoSeedsThreeFreshWindows(t *testing.T) {
	w := newWorld(t)

	require.NoError(t, w.severalHeldPlansWereEachObservedAWhileAgo())

	s := w.pc()
	require.Len(t, s.heldPlans, 3)
	path, err := observe.Path()
	require.NoError(t, err)
	state := observe.Load(path)
	for _, p := range s.heldPlans {
		win := state[observe.Key(p.Repo, p.ID)]
		assert.Equal(t, p.HoldTip, win.Tip)
		assert.Equal(t, s.clock, win.Last, "each window was last sampled at the section's own clock")
	}
}

// TestTheGapSinceEachOnesLastSampleExceedsTheBoundRefusesWithNoPlans.
func TestTheGapSinceEachOnesLastSampleExceedsTheBoundRefusesWithNoPlans(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.theGapSinceEachOnesLastSampleExceedsTheBound())
}

// TestTheGapSinceEachOnesLastSampleExceedsTheBoundAdvancesTheClock.
func TestTheGapSinceEachOnesLastSampleExceedsTheBoundAdvancesTheClock(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, w.severalHeldPlansWereEachObservedAWhileAgo())
	before := w.pc().clock

	require.NoError(t, w.theGapSinceEachOnesLastSampleExceedsTheBound())

	assert.Greater(t, w.pc().clock.Sub(before), discovery.DefaultSampleGap)
}

// TestTheFleetIsObservedAgainNowThatOriginIsReachableRefusesWithNoPlans.
func TestTheFleetIsObservedAgainNowThatOriginIsReachableRefusesWithNoPlans(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.theFleetIsObservedAgainNowThatOriginIsReachable())
}

// TestTheFleetIsObservedAgainNowThatOriginIsReachableVoidsAGoneQuietWindow:
// the full S23 pipeline — seed, let the gap pass, observe again —
// leaves every plan un-stale, the mass-takeover guard the row exists
// to pin.
func TestTheFleetIsObservedAgainNowThatOriginIsReachableVoidsAGoneQuietWindow(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, w.severalHeldPlansWereEachObservedAWhileAgo())
	require.NoError(t, w.theGapSinceEachOnesLastSampleExceedsTheBound())

	require.NoError(t, w.theFleetIsObservedAgainNowThatOriginIsReachable())

	require.NoError(t, w.everyWindowResetsToOneSample())
	require.NoError(t, w.noPlanReadsItsTakeoverWindowMatured())
}

// TestEveryWindowResetsToOneSampleFailsOnAMaturedOrUnvoidedWindow.
func TestEveryWindowResetsToOneSampleFailsOnAMaturedOrUnvoidedWindow(t *testing.T) {
	w := newWorld(t)
	require.NoError(t, w.severalHeldPlansWereEachObservedAWhileAgo())
	s := w.pc()
	path, err := observe.Path()
	require.NoError(t, err)

	require.Error(t, w.everyWindowResetsToOneSample(), "still carries the seeded 9-sample windows")

	state := observe.State{}
	for _, p := range s.heldPlans {
		state[observe.Key(p.Repo, p.ID)] = discovery.Window{Tip: p.HoldTip, Samples: 1}
	}
	require.NoError(t, observe.Save(path, state))
	require.Error(t, w.everyWindowResetsToOneSample(), "one sample with no void note is not a reset")

	for k, win := range state {
		win.Voided = "window restarted"
		state[k] = win
	}
	require.NoError(t, observe.Save(path, state))
	require.NoError(t, w.everyWindowResetsToOneSample())
}

// TestNoPlanReadsItsTakeoverWindowMaturedFailsOnAnyStalePlan.
func TestNoPlanReadsItsTakeoverWindowMaturedFailsOnAnyStalePlan(t *testing.T) {
	w := newWorld(t)
	w.pc().heldPlans = []discovery.Plan{{ID: 1}, {ID: 2}}

	require.NoError(t, w.noPlanReadsItsTakeoverWindowMatured())

	w.pc().heldPlans[1].Stale = true
	require.Error(t, w.noPlanReadsItsTakeoverWindowMatured())
}

// TestThatSpanDoesNotReadStaleOnceBackedOffPinsTheDamping: the same
// span that reads stale under the bare takeover window no longer does
// once one real takeover has minted a marker and TakeoverCount backs
// the threshold off.
func TestThatSpanDoesNotReadStaleOnceBackedOffPinsTheDamping(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 919)
	require.NoError(t, w.observerWatchesTipGoStale("box-a"))
	require.NoError(t, w.takesTheLeaseOver("box-b"))
	require.NoError(t, w.observerWatchesTipGoStale("box-b"))

	require.NoError(t, w.thatSpanDoesNotReadStaleOnceBackedOff())
}

// TestThatSpanDoesNotReadStaleOnceBackedOffRefusesWithNoTakeover: a
// chain carrying no takeover marker at all reads k=0, so the row's own
// premise — a backoff to test — was never set up.
func TestThatSpanDoesNotReadStaleOnceBackedOffRefusesWithNoTakeover(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 920)
	w.taker, w.taken = "box-a", w.lease

	require.Error(t, w.thatSpanDoesNotReadStaleOnceBackedOff())
}

// TestASecondHostsClockIsSkewedYearsFromTheFirstsSeedsTwoDistinctClocks.
func TestASecondHostsClockIsSkewedYearsFromTheFirstsSeedsTwoDistinctClocks(t *testing.T) {
	w := newWorld(t)

	require.NoError(t, w.aSecondHostsClockIsSkewedYearsFromTheFirsts())

	s := w.pc()
	require.Len(t, s.hostClocks, 2)
	assert.Greater(t, s.hostClocks["host-2"].Sub(s.hostClocks["host-1"]), 5*365*24*time.Hour)
}

// TestBothHostsWatchTipGoStaleEachOnItsOwnClockMaturesTwoIndependentWindows.
func TestBothHostsWatchTipGoStaleEachOnItsOwnClockMaturesTwoIndependentWindows(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 921)
	require.NoError(t, w.aSecondHostsClockIsSkewedYearsFromTheFirsts())

	require.NoError(t, w.bothHostsWatchTipGoStaleEachOnItsOwnClock("box-a"))

	s := w.pc()
	require.Len(t, s.hostWindows, 2)
	for host, win := range s.hostWindows {
		assert.Greater(t, win.Span(), discovery.DefaultTakeoverWindow, host)
	}
}

// TestBothHostsWatchTipGoStaleEachOnItsOwnClockRefusesWithNoHosts.
func TestBothHostsWatchTipGoStaleEachOnItsOwnClockRefusesWithNoHosts(t *testing.T) {
	w := leaseWorldFixture(t, "box-a", 922)

	require.Error(t, w.bothHostsWatchTipGoStaleEachOnItsOwnClock("box-a"))
}

// TestBothHostsWatchTipGoStaleEachOnItsOwnClockRefusesAMachineTheScenarioNeverIntroduced.
func TestBothHostsWatchTipGoStaleEachOnItsOwnClockRefusesAMachineTheScenarioNeverIntroduced(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.bothHostsWatchTipGoStaleEachOnItsOwnClock("ghost"))
}

// TestBothHostsWindowsReadTheHoldStaleFailsWithFewerThanTwoWindows.
func TestBothHostsWindowsReadTheHoldStaleFailsWithFewerThanTwoWindows(t *testing.T) {
	w := newWorld(t)

	require.Error(t, w.bothHostsWindowsReadTheHoldStale())

	s := w.pc()
	s.hostWindows["host-1"] = discovery.Window{Tip: "abc", Samples: 1}
	require.Error(t, w.bothHostsWindowsReadTheHoldStale())
}

// TestBothHostsWindowsReadTheHoldStaleFailsIfEitherHostIsNotStale:
// convergence means both, so one host's own window falling short of
// the takeover window fails the check even though the other's
// matured — pairing bugs (a swapped clock, a shared window) would
// otherwise slip through on the host that happens to be checked
// first.
func TestBothHostsWindowsReadTheHoldStaleFailsIfEitherHostIsNotStale(t *testing.T) {
	w := newWorld(t)
	s := w.pc()
	first := s.clock
	matured := discovery.Window{Tip: "abc", First: first, Last: first.Add(3 * time.Hour), Samples: 6}
	fresh := discovery.Window{Tip: "abc", First: first, Last: first, Samples: 1}
	s.hostWindows["host-1"] = matured
	s.hostWindows["host-2"] = fresh
	s.hostClocks["host-1"] = matured.Last
	s.hostClocks["host-2"] = fresh.Last

	require.Error(t, w.bothHostsWindowsReadTheHoldStale(), "host-2 never matured")

	s.hostWindows["host-2"] = matured
	require.NoError(t, w.bothHostsWindowsReadTheHoldStale())
}
