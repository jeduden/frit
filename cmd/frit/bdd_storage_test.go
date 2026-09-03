package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The storage-anomalies vocabulary: a person with raw write access to
// origin — never through a frit verb — deletes, force-pushes or
// forges a marker on the work ref, or the remote itself is replaced,
// mirrored or restored. It registers itself, like every section's
// step file, so this section adds a file and never edits
// bdd_lease_test.go or bdd_test.go. "holds the lease", "comes back and
// renews its lease", "the renewal is fenced", "the error suggests
// yield" and "origin still holds the takeover" are the lease world's
// own steps, reused as-is: a storage step reads the clones and the
// lease those steps built.
func init() {
	registrars = append(registrars, (*world).registerStorageAnomalies)
}

// storageState is this section's own state, kept beside the shared
// world via section — never a field on world itself. It carries the
// one bare origin every machine in a storage scenario shares, a
// mirror backup taken of it, each machine's own tracked chain tip for
// the rows that renew more than once, and the leases a fresh
// acquisition on a replaced remote minted.
type storageState struct {
	origin   string
	backup   string
	tips     map[string]string
	acquired map[string]claim.Lease
}

// sw fetches this scenario's section state, lazily initialized on
// first use.
func (w *world) sw() *storageState {
	s := section[storageState](w)
	if s.tips == nil {
		s.tips = map[string]string{}
		s.acquired = map[string]claim.Lease{}
	}

	return s
}

// registerStorageAnomalies binds the step texts S37, S38, S39, S41,
// S69 and S71 add. A quoted machine name is checked against the role
// the scenario set it up in, the same discipline bdd_lease_test.go's
// own steps hold to.
func (w *world) registerStorageAnomalies(sc *godog.ScenarioContext) {
	sc.Step(`^a person deletes the work ref on origin$`, w.aPersonDeletesTheWorkRefOnOrigin)
	sc.Step(`^the renewal is refused$`, w.theRenewalIsRefused)
	sc.Step(`^origin's work ref is gone$`, w.originsWorkRefIsGone)
	sc.Step(`^"([^"]+)"'s local ref still points at its recorded tip$`, w.localRefStillPointsAtItsRecordedTip)
	sc.Step(`^a person force-pushes a marker forged to name "([^"]+)" onto the work ref$`,
		w.aPersonForcePushesAMarkerForgedToNameOntoTheWorkRef)
	sc.Step(`^a person force-pushes the work ref back to "([^"]+)"'s first tip$`,
		w.aPersonForcePushesTheWorkRefBackTo)
	sc.Step(`^origin is replaced by a fresh remote carrying only main$`,
		w.originIsReplacedByAFreshRemoteCarryingOnlyMain)
	sc.Step(`^every machine is pointed at the new remote$`, w.everyMachineIsPointedAtTheNewRemote)
	sc.Step(`^"([^"]+)" acquires the lease on the new remote$`, w.acquiresTheLeaseOnTheNewRemote)
	sc.Step(`^"([^"]+)" acquired at epoch 1 on the new remote$`, w.acquiredAtEpochOneOnTheNewRemote)
	sc.Step(`^a person pushes a beat marker forged to name "([^"]+)" onto the work ref$`,
		w.aPersonPushesABeatMarkerForgedToNameOntoTheWorkRef)
	sc.Step(`^a mirror backup of origin is taken$`, w.aMirrorBackupOfOriginIsTaken)
	sc.Step(`^"([^"]+)" renews its lease again$`, w.renewsItsLeaseAgain)
	sc.Step(`^origin is restored from the backup$`, w.originIsRestoredFromTheBackup)
	sc.Step(`^"([^"]+)" re-reads origin's tip and renews from it$`, w.reReadsOriginsTipAndRenewsFromIt)
	sc.Step(`^a person runs "git gc --prune=now" on origin$`, w.aPersonRunsGitGCPruneNowOnOrigin)
	sc.Step(`^"([^"]+)" acquires the lease again$`, w.acquiresTheLeaseAgain)
	sc.Step(`^orphans lists both tips as rescued for "([^"]+)"'s lane$`, w.orphansListsBothTipsAsRescued)
}

// originOf finds the bare origin every machine in a storage scenario
// shares, reading remote.origin.url off holder's own clone the first
// time it is needed and caching it, exactly the way cloneAgain finds
// it for a second machine's own clone.
func (w *world) originOf(holder string) (string, error) {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return "", err
	}
	s := w.sw()
	if s.origin != "" {
		return s.origin, nil
	}
	origin, err := gitCapture(w.t, repo, "config", "--get", "remote.origin.url")
	if err != nil {
		return "", fmt.Errorf("%s: %w", origin, err)
	}
	s.origin = origin

	return origin, nil
}

// forgeMarker mints a lease marker commit the way a person with raw
// write access would: commit-tree straight off git plumbing, in the
// exact trailer shape leaseMessage builds, but naming whatever holder
// the forger chooses. frit never verifies who minted a marker, only
// whose SHA currently sits on the ref — the token is the fence, the
// trailer only reports.
func (w *world) forgeMarker(repo, parent, kind, holder, lane string, epoch int) (string, error) {
	tree, err := gitCapture(w.t, repo, "rev-parse", parent+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("%s: %w", tree, err)
	}
	msg := fmt.Sprintf(
		"plan %d: %s\n\nepoch:   %d\nnonce:   forged\nholder:  %s\n"+
			"lane:    %s\nsession: -",
		w.planID, kind, epoch, holder, lane)

	return gitCapture(w.t, repo, "commit-tree", tree, "-p", parent, "-m", msg)
}

// personForgesAndPushes is the shape behind S38 and S69: a person
// with raw write access to origin — never through a frit verb —
// force-pushes a marker forged to name holder, minted as a child of
// the lease's own recorded tip, straight onto the work ref.
func (w *world) personForgesAndPushes(kind, holder string, epoch int) error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	forged, err := w.forgeMarker(repo, w.lease.Tip, kind, holder, "/lanes/"+holder, epoch)
	if err != nil {
		return fmt.Errorf("%s: %w", forged, err)
	}
	out, err := gitCapture(w.t, repo, "push", "-q", "-f", "origin", forged+":"+w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}

	return nil
}

// aPersonForcePushesAMarkerForgedToNameOntoTheWorkRef is S38's own
// anomaly: a forged takeover, one epoch past the live lease, as a
// hand-force-push would look.
func (w *world) aPersonForcePushesAMarkerForgedToNameOntoTheWorkRef(holder string) error {
	return w.personForgesAndPushes(markerTakeoverText, holder, w.lease.Epoch+1)
}

// aPersonPushesABeatMarkerForgedToNameOntoTheWorkRef is S69's own
// anomaly: a forged beat, same epoch, naming the very holder it
// fences — the marker body's claim is never the fence, the CAS is.
func (w *world) aPersonPushesABeatMarkerForgedToNameOntoTheWorkRef(holder string) error {
	return w.personForgesAndPushes(markerBeatText, holder, w.lease.Epoch)
}

// markerTakeoverText and markerBeatText are the marker-kind subjects
// a forged commit's message carries — the same words leaseMessage
// would write, spelled out here since claim's own kind constants are
// unexported.
const (
	markerTakeoverText = "takeover"
	markerBeatText     = "beat"
)

// aPersonDeletesTheWorkRefOnOrigin is S37's own anomaly: raw write
// access to the bare origin, never through a frit verb.
func (w *world) aPersonDeletesTheWorkRefOnOrigin() error {
	origin, err := w.originOf(w.holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, origin, "update-ref", "-d", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}

	return nil
}

// theRenewalIsRefused checks the last renewal failed with a plain
// error — casPush's absent-ref branch — rather than a fence: an
// absent ref is a fault the CAS never arbitrated, so wrapping it as a
// FenceError would claim a mover this run never saw.
func (w *world) theRenewalIsRefused() error {
	if w.err == nil {
		return fmt.Errorf("expected the renewal to be refused, got a plain win")
	}
	var fenced *claim.FenceError
	if errors.As(w.err, &fenced) {
		return fmt.Errorf("expected a plain refusal, got a fence naming %q", fenced.Marker.Holder)
	}

	return nil
}

// originsWorkRefIsGone checks origin carries no ref at all for this
// plan.
func (w *world) originsWorkRefIsGone() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, repo, "ls-remote", "origin", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	if out != "" {
		return fmt.Errorf("origin still carries the work ref: %s", out)
	}

	return nil
}

// localRefStillPointsAtItsRecordedTip checks holder's own local copy
// of the work ref never moved — a lost CAS syncs nothing locally, so
// the fenced-out or refused holder's own branch is exactly where it
// was minted.
func (w *world) localRefStillPointsAtItsRecordedTip(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	tip, err := gitCapture(w.t, repo, "rev-parse", w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", tip, err)
	}
	if tip != w.lease.Tip {
		return fmt.Errorf("the local work ref moved to %s, want the recorded %s", tip, w.lease.Tip)
	}

	return nil
}

// aPersonForcePushesTheWorkRefBackTo is S39's own anomaly: origin's
// ref is force-pushed back to exactly holder's first tip — the same
// object, not a fresh commit — the ABA hazard a naive value-only
// compare would miss.
func (w *world) aPersonForcePushesTheWorkRefBackTo(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, repo, "push", "-q", "-f", "origin", w.lease.Tip+":"+w.branch())
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}

	return nil
}

// originIsReplacedByAFreshRemoteCarryingOnlyMain is S41's own
// anomaly: a brand-new bare remote, seeded from holder's own main
// branch alone — no plan ref survives the migration.
func (w *world) originIsReplacedByAFreshRemoteCarryingOnlyMain() error {
	repo, err := w.cloneOf(w.holder)
	if err != nil {
		return err
	}
	migrated := filepath.Join(w.t.TempDir(), "migrated-origin.git")
	out, err := gitCapture(w.t, repo, "init", "-q", "--bare", "-b", "main", migrated)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	out, err = gitCapture(w.t, repo, "push", "-q", migrated, "main:main")
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	w.sw().origin = migrated

	return nil
}

// everyMachineIsPointedAtTheNewRemote repoints every clone this
// scenario has already built at the replacement remote — the fleet's
// own migration, not another anomaly.
func (w *world) everyMachineIsPointedAtTheNewRemote() error {
	origin := w.sw().origin
	if origin == "" {
		return fmt.Errorf("no replacement remote yet; the migration step comes first")
	}
	for _, repo := range w.clones {
		out, err := gitCapture(w.t, repo, "remote", "set-url", "origin", origin)
		if err != nil {
			return fmt.Errorf("%s: %w", out, err)
		}
	}

	return nil
}

// acquiresTheLeaseOnTheNewRemote is holder's fresh acquisition on the
// migrated remote: a new clone of it, since holder's old checkout
// belongs to the old plan/<id> history this remote never carried.
func (w *world) acquiresTheLeaseOnTheNewRemote(holder string) error {
	origin := w.sw().origin
	if origin == "" {
		return fmt.Errorf("no replacement remote yet; the migration step comes first")
	}
	dst := w.t.TempDir()
	git(w.t, dst, "clone", "-q", origin, dst)
	git(w.t, dst, "config", "user.email", "t3@example.com")
	git(w.t, dst, "config", "user.name", "frit-test-3")
	w.clones[holder] = dst

	lease, err := claim.Acquire(dst, leaseFor(holder, w.planID), gitwt.Exec)
	if err != nil {
		return err
	}
	w.sw().acquired[holder] = lease

	return nil
}

// acquiredAtEpochOneOnTheNewRemote checks holder's acquisition on the
// migrated remote landed as a fresh epoch-1 claim, never a takeover of
// anything the old remote carried.
func (w *world) acquiredAtEpochOneOnTheNewRemote(holder string) error {
	lease, ok := w.sw().acquired[holder]
	if !ok {
		return fmt.Errorf("%q never acquired on the new remote", holder)
	}
	if lease.Epoch != 1 {
		return fmt.Errorf("%q acquired at epoch %d, want 1", holder, lease.Epoch)
	}

	return nil
}

// aMirrorBackupOfOriginIsTaken is S71's own setup: a full mirror clone
// of origin as it stands right now, the snapshot a later restore
// replays.
func (w *world) aMirrorBackupOfOriginIsTaken() error {
	origin, err := w.originOf(w.holder)
	if err != nil {
		return err
	}
	backup := filepath.Join(w.t.TempDir(), "origin-backup.git")
	out, err := gitCapture(w.t, filepath.Dir(backup), "clone", "-q", "--mirror", origin, backup)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	w.sw().backup = backup

	return nil
}

// storedTip is holder's own last-known tip for this section: the
// value a storage step last confirmed, falling back to the original
// acquisition before any tracked renewal has succeeded.
func (w *world) storedTip(holder string) (string, error) {
	s := w.sw()
	if tip, ok := s.tips[holder]; ok {
		return tip, nil
	}
	if holder == w.holder {
		return w.lease.Tip, nil
	}

	return "", fmt.Errorf("%q has no recorded tip in this scenario", holder)
}

// renewsItsLeaseAgain is S71's own tracked renewal: like the lease
// world's "comes back and renews its lease", but CASed from this
// section's own tracked chain tip so a second and third renewal in
// the same scenario advance from where the first one actually landed,
// not from the stale acquisition tip the shared world never updates.
func (w *world) renewsItsLeaseAgain(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	from, err := w.storedTip(holder)
	if err != nil {
		return err
	}
	lease, err := claim.Renew(repo, leaseFor(holder, w.planID), from, gitwt.Exec)
	w.err = err
	if err == nil {
		w.sw().tips[holder] = lease.Tip
	}

	return nil
}

// originIsRestoredFromTheBackup is S71's own anomaly: the mirror
// backup is force-mirrored back onto origin, so every ref — the work
// ref chief among them — reverts to the snapshot taken before it.
func (w *world) originIsRestoredFromTheBackup() error {
	s := w.sw()
	if s.backup == "" {
		return fmt.Errorf("no backup was taken; the backup step comes first")
	}
	origin, err := w.originOf(w.holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, s.backup, "push", "-q", "--mirror", "-f", origin)
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}

	return nil
}

// reReadsOriginsTipAndRenewsFromIt is S71's recovery: a fresh read of
// origin's actual current tip, then a renewal CASed from exactly that
// — the way a fenced holder re-orients after a restore, rather than
// retrying its own stale memory forever.
func (w *world) reReadsOriginsTipAndRenewsFromIt(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	fresh := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec)
	if fresh == "" {
		return fmt.Errorf("origin holds no tip for plan %d", w.planID)
	}
	lease, err := claim.Renew(repo, leaseFor(holder, w.planID), fresh, gitwt.Exec)
	w.err = err
	if err == nil {
		w.sw().tips[holder] = lease.Tip
	}

	return nil
}

// aPersonRunsGitGCPruneNowOnOrigin is S40's own anomaly: a remote GC,
// run for real against the bare origin, immediately eligible to
// collect anything no ref reaches — the marker chain a scavenge just
// deleted the work ref for, and everything the rescue ref does not
// keep alive.
func (w *world) aPersonRunsGitGCPruneNowOnOrigin() error {
	origin, err := w.originOf(w.holder)
	if err != nil {
		return err
	}
	out, err := gitCapture(w.t, origin, "gc", "--prune=now", "--quiet")
	if err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}

	return nil
}

// acquiresTheLeaseAgain is S78's own second cycle: a fresh claim on
// the very plan a scavenge just deleted the ref for, so a second
// scavenge later parks a second, different tip from the same lane. It
// refuses unless the ref is actually gone — a caller that skipped the
// scavenge step would otherwise land a takeover, not the fresh
// epoch-1 claim this row's shape depends on.
func (w *world) acquiresTheLeaseAgain(holder string) error {
	if holder != w.holder {
		return fmt.Errorf("%q never held the lease; %q did", holder, w.holder)
	}
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	if tip := claim.RemoteTip(repo, "origin", int64(w.planID), gitwt.Exec); tip != "" {
		return fmt.Errorf("origin still carries the work ref at %s; the scavenge step comes first", tip)
	}
	lease, err := claim.Acquire(repo, leaseFor(holder, w.planID), gitwt.Exec)
	if err != nil {
		return err
	}
	w.lease = lease

	return nil
}

// orphansListsBothTipsAsRescued checks S78's own promise from both
// ends: the primitive, claim.RescueRefs, must carry exactly two
// distinct refs for the plan, and the verb an operator actually runs,
// frit orphans, must report the same plan with the same count.
func (w *world) orphansListsBothTipsAsRescued(holder string) error {
	repo, err := w.cloneOf(holder)
	if err != nil {
		return err
	}
	refs := claim.RescueRefs(repo, "origin", int64(w.planID), gitwt.Exec)
	if len(refs) != 2 {
		return fmt.Errorf("origin carries %d rescue refs for plan %d, want 2: %v", len(refs), w.planID, refs)
	}

	var doc report.OrphansDoc
	stderr := emit(w.t, &doc, "orphans", "--root", filepath.Dir(repo))
	if stderr != "" {
		return fmt.Errorf("orphans reported stderr: %s", stderr)
	}
	for _, r := range doc.Repos {
		for _, rescued := range r.Rescued {
			if rescued.PlanID != int64(w.planID) {
				continue
			}
			if len(rescued.Refs) != 2 {
				return fmt.Errorf("orphans lists %d rescue refs for plan %d, want 2: %v",
					len(rescued.Refs), w.planID, rescued.Refs)
			}

			return nil
		}
	}

	return fmt.Errorf("orphans lists no rescued entry for plan %d", w.planID)
}

// TestSwLazilyInitializesItsMapsOncePerWorld: the section's maps are
// built on first use and stay the same value for the rest of the
// scenario, a fresh pair for another scenario's own world.
func TestSwLazilyInitializesItsMapsOncePerWorld(t *testing.T) {
	w := newWorld(t)

	first := w.sw()
	first.tips["box-a"] = "sha"

	assert.Same(t, first, w.sw())
	assert.Equal(t, "sha", w.sw().tips["box-a"])
	assert.NotSame(t, first, newWorld(t).sw(), "another scenario, another state")
}

// TestOriginOfCachesTheFirstReadAndRefusesAnUnknownMachine: the bare
// origin is discovered once, off any machine the scenario already
// introduced, and a machine it never met refuses rather than reading
// a zero value.
func TestOriginOfCachesTheFirstReadAndRefusesAnUnknownMachine(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 37
	repo := claimableRepo(t, t.TempDir(), "atlas", 37, "Shader unit")
	w.holder = "box-a"
	w.clones["box-a"] = repo

	origin, err := w.originOf("box-a")
	require.NoError(t, err)
	assert.NotEmpty(t, origin)
	assert.Equal(t, origin, w.sw().origin, "the discovery is cached")

	_, err = w.originOf("ghost")
	require.Error(t, err)
}

// TestForgeMarkerMintsALeaseMessageShapeMarkerNamingTheForgedHolder:
// the minted commit is a child of parent, and its body carries the
// forger's chosen kind and holder — the same trailer shape a real
// marker carries, minted through raw plumbing rather than the claim
// package's own path.
func TestForgeMarkerMintsALeaseMessageShapeMarkerNamingTheForgedHolder(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 38
	repo := claimableRepo(t, t.TempDir(), "atlas", 38, "Shader unit")
	lease, err := claim.Acquire(repo, leaseFor("box-a", 38), gitwt.Exec)
	require.NoError(t, err)

	forged, err := w.forgeMarker(repo, lease.Tip, "takeover", "mallory", "/lanes/mallory", 2)

	require.NoError(t, err)
	parent, err := gitCapture(t, repo, "rev-parse", forged+"^")
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, parent)
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", forged)
	require.NoError(t, err)
	assert.Contains(t, body, "plan 38: takeover")
	assert.Contains(t, body, "holder:  mallory")
	assert.Contains(t, body, "lane:    /lanes/mallory")
	assert.Contains(t, body, "epoch:   2")
}

// TestPersonForgesAndPushesLandsTheForgedMarkerOnOrigin: the forged
// commit ends up as origin's own tip for the plan — a hand-force-push
// never goes through casPush's arbitration.
func TestPersonForgesAndPushesLandsTheForgedMarkerOnOrigin(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 38
	repo := claimableRepo(t, t.TempDir(), "atlas", 38, "Shader unit")
	lease, err := claim.Acquire(repo, leaseFor("box-a", 38), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.lease = lease
	w.clones["box-a"] = repo

	err = w.personForgesAndPushes("takeover", "mallory", lease.Epoch+1)

	require.NoError(t, err)
	remote, err := gitCapture(t, repo, "ls-remote", "origin", w.branch())
	require.NoError(t, err)
	fields := strings.Fields(remote)
	require.NotEmpty(t, fields)
	assert.NotEqual(t, lease.Tip, fields[0], "origin no longer holds the original tip")
	body, err := gitCapture(t, repo, "log", "-1", "--format=%B", fields[0])
	require.NoError(t, err)
	assert.Contains(t, body, "holder:  mallory", "the forged commit landed on origin")
}

// TestAPersonDeletesTheWorkRefOnOriginRemovesTheRefFromOrigin: the
// delete runs against the bare origin directly, never through a frit
// verb, and leaves holder's own local copy untouched.
func TestAPersonDeletesTheWorkRefOnOriginRemovesTheRefFromOrigin(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 37
	repo := claimableRepo(t, t.TempDir(), "atlas", 37, "Shader unit")
	lease, err := claim.Acquire(repo, leaseFor("box-a", 37), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.lease = lease
	w.clones["box-a"] = repo

	require.NoError(t, w.aPersonDeletesTheWorkRefOnOrigin())

	remote, err := gitCapture(t, repo, "ls-remote", "origin", w.branch())
	require.NoError(t, err)
	assert.Empty(t, remote)
	local, err := gitCapture(t, repo, "rev-parse", w.branch())
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, local, "the local ref is untouched")
}

// TestTheRenewalIsRefusedAcceptsAPlainErrorButRejectsAFenceOrNil: the
// step wants a fault the CAS never arbitrated — a fence, or no error
// at all, is not that.
func TestTheRenewalIsRefusedAcceptsAPlainErrorButRejectsAFenceOrNil(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.theRenewalIsRefused(), "no error is no refusal")

	w.err = &claim.FenceError{Marker: claim.Marker{Holder: "box-b"}, Known: true}
	require.Error(t, w.theRenewalIsRefused(), "a fence is not a plain refusal")

	w.err = fmt.Errorf("push beat for plan 37: exit status 1")
	assert.NoError(t, w.theRenewalIsRefused())
}

// TestOriginsWorkRefIsGoneFailsWhenARefRemains: the check reads
// origin directly, so a ref left behind is caught rather than assumed
// gone.
func TestOriginsWorkRefIsGoneFailsWhenARefRemains(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 37
	repo := claimableRepo(t, t.TempDir(), "atlas", 37, "Shader unit")
	_, err := claim.Acquire(repo, leaseFor("box-a", 37), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.clones["box-a"] = repo

	require.Error(t, w.originsWorkRefIsGone())
}

// TestLocalRefStillPointsAtItsRecordedTipFailsWhenTheLocalRefMoved:
// the check compares against the world's own recorded acquisition
// tip, not whatever origin currently holds.
func TestLocalRefStillPointsAtItsRecordedTipFailsWhenTheLocalRefMoved(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 37
	repo := claimableRepo(t, t.TempDir(), "atlas", 37, "Shader unit")
	lease, err := claim.Acquire(repo, leaseFor("box-a", 37), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.lease = lease
	w.clones["box-a"] = repo

	require.NoError(t, w.localRefStillPointsAtItsRecordedTip("box-a"))

	_, err = claim.Renew(repo, leaseFor("box-a", 37), lease.Tip, gitwt.Exec)
	require.NoError(t, err)

	require.Error(t, w.localRefStillPointsAtItsRecordedTip("box-a"))
}

// TestAPersonForcePushesTheWorkRefBackToRefusesAMachineThatNeverHeldTheLease:
// only the holder's own first tip is a meaningful target to push
// back to.
func TestAPersonForcePushesTheWorkRefBackToRefusesAMachineThatNeverHeldTheLease(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"

	require.Error(t, w.aPersonForcePushesTheWorkRefBackTo("box-b"))
}

// TestOriginIsReplacedByAFreshRemoteCarryingOnlyMainCopiesJustMain:
// the new bare remote carries the old repository's main branch and
// nothing else — no plan ref survives the migration.
func TestOriginIsReplacedByAFreshRemoteCarryingOnlyMainCopiesJustMain(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 41
	repo := claimableRepo(t, t.TempDir(), "atlas", 41, "Shader unit")
	_, err := claim.Acquire(repo, leaseFor("box-a", 41), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.clones["box-a"] = repo

	require.NoError(t, w.originIsReplacedByAFreshRemoteCarryingOnlyMain())

	migrated := w.sw().origin
	require.NotEmpty(t, migrated)
	main, err := gitCapture(t, repo, "ls-remote", migrated, "refs/heads/main")
	require.NoError(t, err)
	assert.NotEmpty(t, main)
	plan, err := gitCapture(t, repo, "ls-remote", migrated, "refs/heads/plan/41")
	require.NoError(t, err)
	assert.Empty(t, plan, "the plan ref did not survive the migration")
}

// TestEveryMachineIsPointedAtTheNewRemoteRefusesWithNoReplacementYet:
// repointing before the migration step ran would silently point every
// clone at nothing.
func TestEveryMachineIsPointedAtTheNewRemoteRefusesWithNoReplacementYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.everyMachineIsPointedAtTheNewRemote())
}

// TestAcquiresTheLeaseOnTheNewRemoteRefusesWithNoReplacementYet: a
// fresh acquisition needs a replacement remote to clone.
func TestAcquiresTheLeaseOnTheNewRemoteRefusesWithNoReplacementYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.acquiresTheLeaseOnTheNewRemote("box-b"))
}

// TestAcquiredAtEpochOneOnTheNewRemoteRefusesAMachineThatNeverAcquired:
// the check reads this section's own record of what actually landed,
// not a guess.
func TestAcquiredAtEpochOneOnTheNewRemoteRefusesAMachineThatNeverAcquired(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.acquiredAtEpochOneOnTheNewRemote("box-b"))

	w.sw().acquired["box-b"] = claim.Lease{Epoch: 2}
	require.Error(t, w.acquiredAtEpochOneOnTheNewRemote("box-b"), "epoch 2 is a takeover, not a fresh claim")
}

// TestAMirrorBackupOfOriginIsTakenCopiesTheWorkRef: the backup is a
// full mirror, so the plan's work ref is in it too, not just main.
func TestAMirrorBackupOfOriginIsTakenCopiesTheWorkRef(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 71
	repo := claimableRepo(t, t.TempDir(), "atlas", 71, "Shader unit")
	lease, err := claim.Acquire(repo, leaseFor("box-a", 71), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.lease = lease
	w.clones["box-a"] = repo

	require.NoError(t, w.aMirrorBackupOfOriginIsTaken())

	backup := w.sw().backup
	require.NotEmpty(t, backup)
	tip, err := gitCapture(t, backup, "rev-parse", w.branch())
	require.NoError(t, err)
	assert.Equal(t, lease.Tip, tip)
}

// TestStoredTipFallsBackToTheOriginalClaimAndRefusesAnUnknownMachine:
// before any tracked renewal, the section defers to the world's own
// acquisition tip for the holder; any other machine has none.
func TestStoredTipFallsBackToTheOriginalClaimAndRefusesAnUnknownMachine(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"
	w.lease = claim.Lease{Tip: "claim-sha"}

	got, err := w.storedTip("box-a")
	require.NoError(t, err)
	assert.Equal(t, "claim-sha", got)

	_, err = w.storedTip("box-b")
	require.Error(t, err)

	w.sw().tips["box-a"] = "beat-sha"
	got, err = w.storedTip("box-a")
	require.NoError(t, err)
	assert.Equal(t, "beat-sha", got, "a tracked renewal shadows the original claim")
}

// TestRenewsItsLeaseAgainRefusesAMachineThatNeverHeldTheLease: only
// the holder renews its own lease.
func TestRenewsItsLeaseAgainRefusesAMachineThatNeverHeldTheLease(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"

	require.Error(t, w.renewsItsLeaseAgain("box-b"))
}

// TestOriginIsRestoredFromTheBackupRefusesWithNoBackupYet: a restore
// with nothing backed up would silently push nothing.
func TestOriginIsRestoredFromTheBackupRefusesWithNoBackupYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.originIsRestoredFromTheBackup())
}

// TestReReadsOriginsTipAndRenewsFromItRefusesAMachineThatNeverHeldTheLease:
// only the holder re-orients its own lease from a fresh read.
func TestReReadsOriginsTipAndRenewsFromItRefusesAMachineThatNeverHeldTheLease(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"

	require.Error(t, w.reReadsOriginsTipAndRenewsFromIt("box-b"))
}

// TestAPersonRunsGitGCPruneNowOnOriginRunsAgainstTheBareOrigin: the gc
// runs against the bare origin itself, not any clone, and leaves it
// readable with its refs intact.
func TestAPersonRunsGitGCPruneNowOnOriginRunsAgainstTheBareOrigin(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 40
	repo := claimableRepo(t, t.TempDir(), "atlas", 40, "Shader unit")
	_, err := claim.Acquire(repo, leaseFor("box-a", 40), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.clones["box-a"] = repo

	require.NoError(t, w.aPersonRunsGitGCPruneNowOnOrigin())

	origin, err := w.originOf("box-a")
	require.NoError(t, err)
	main, err := gitCapture(t, origin, "rev-parse", "main")
	require.NoError(t, err)
	assert.NotEmpty(t, main, "origin survives the gc with its refs intact")
}

// TestAPersonRunsGitGCPruneNowOnOriginRefusesWithNoOriginYet: the step
// needs a clone to discover the bare origin through.
func TestAPersonRunsGitGCPruneNowOnOriginRefusesWithNoOriginYet(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.aPersonRunsGitGCPruneNowOnOrigin())
}

// TestAcquiresTheLeaseAgainRefusesAMachineThatNeverHeldTheLease: only
// the holder re-claims its own plan.
func TestAcquiresTheLeaseAgainRefusesAMachineThatNeverHeldTheLease(t *testing.T) {
	w := newWorld(t)
	w.holder = "box-a"

	require.Error(t, w.acquiresTheLeaseAgain("box-b"))
}

// TestAcquiresTheLeaseAgainRefusesWhileOriginStillCarriesTheRef: a
// caller that skipped the scavenge step must not silently land a
// takeover instead of the fresh claim this row's shape depends on.
func TestAcquiresTheLeaseAgainRefusesWhileOriginStillCarriesTheRef(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 78
	repo := claimableRepo(t, t.TempDir(), "atlas", 78, "Shader unit")
	lease, err := claim.Acquire(repo, leaseFor("box-a", 78), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.lease = lease
	w.clones["box-a"] = repo

	require.Error(t, w.acquiresTheLeaseAgain("box-a"))
}

// TestAcquiresTheLeaseAgainLandsAFreshEpochOneClaimOnceTheRefIsGone:
// once the ref is truly gone, the second cycle acquires exactly as a
// first claim would — epoch 1, not a takeover of anything.
func TestAcquiresTheLeaseAgainLandsAFreshEpochOneClaimOnceTheRefIsGone(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 78
	repo := claimableRepo(t, t.TempDir(), "atlas", 78, "Shader unit")
	lease, err := claim.Acquire(repo, leaseFor("box-a", 78), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.lease = lease
	w.clones["box-a"] = repo
	require.NoError(t, w.theRefIsScavenged(), "a bare claim marker carries no unlanded work to park")

	require.NoError(t, w.acquiresTheLeaseAgain("box-a"))

	assert.Equal(t, 1, w.lease.Epoch)
}

// TestOrphansListsBothTipsAsRescuedRefusesAnUnknownMachine: the check
// reads the machine's own clone to find origin.
func TestOrphansListsBothTipsAsRescuedRefusesAnUnknownMachine(t *testing.T) {
	w := newWorld(t)
	require.Error(t, w.orphansListsBothTipsAsRescued("box-a"))
}

// TestOrphansListsBothTipsAsRescuedRefusesWithOnlyOneParked: the row's
// own point is two tips landing side by side; a single park is not
// yet the promise this check makes.
func TestOrphansListsBothTipsAsRescuedRefusesWithOnlyOneParked(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 78
	repo := claimableRepo(t, t.TempDir(), "atlas", 78, "Shader unit")
	lease, err := claim.Acquire(repo, leaseFor("box-a", 78), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.lease = lease
	w.clones["box-a"] = repo
	out, err := gitCapture(t, repo, "push", "-q", "origin",
		lease.Tip+":refs/frit/rescue/78/box-a-"+lease.Tip)
	require.NoError(t, err, out)

	err = w.orphansListsBothTipsAsRescued("box-a")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "want 2")
}

// TestOrphansListsBothTipsAsRescuedFindsTwoRescueRefsThroughTheVerb:
// two distinct rescue refs for the same plan, counted both by the
// primitive, claim.RescueRefs, and by the orphans verb reading the
// same fleet root.
func TestOrphansListsBothTipsAsRescuedFindsTwoRescueRefsThroughTheVerb(t *testing.T) {
	isolate(t)
	w := newWorld(t)
	w.planID = 78
	repo := claimableRepo(t, t.TempDir(), "atlas", 78, "Shader unit")
	lease, err := claim.Acquire(repo, leaseFor("box-a", 78), gitwt.Exec)
	require.NoError(t, err)
	w.holder = "box-a"
	w.lease = lease
	w.clones["box-a"] = repo
	tip2, err := gitCapture(t, repo, "commit-tree", lease.Tip+"^{tree}", "-p", lease.Tip, "-m", "second")
	require.NoError(t, err)
	for _, tip := range []string{lease.Tip, tip2} {
		out, err := gitCapture(t, repo, "push", "-q", "origin", tip+":refs/frit/rescue/78/box-a-"+tip)
		require.NoError(t, err, out)
	}

	require.NoError(t, w.orphansListsBothTipsAsRescued("box-a"))
}
