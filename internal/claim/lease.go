// This file is the lease on the work ref. A plan's hold is
// refs/heads/plan/<id> — id only, so no local state reaches the name —
// and the ref itself is the lease: the tip SHA is the token, and every
// transition is one server-side CAS (--force-with-lease with an exact
// expected value). Epoch, a fresh nonce and the holder identity ride
// as trailers on marker commits. The remote is the arbiter; frit never
// decides holdership from a local view. The protocol's full record is
// docs/research/lease-protocol.md.

package claim

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jeduden/frit/internal/gitwt"
)

// The marker kinds, as they appear in the subject line.
const (
	markerClaim    = "claim"
	markerBeat     = "beat"
	markerRelease  = "release"
	markerTakeover = "takeover"
)

// LeaseOptions describes the lease transitions on a plan's work ref:
// who is acting, from which machine and lane, against which remote.
type LeaseOptions struct {
	PlanID  int64
	Remote  string // e.g. "origin"
	Base    string // the ref an acquisition is dated against, e.g. "origin/main"
	Holder  string // stable machine id recorded in the marker
	Lane    string // absolute worktree path on the holder; "" records "-"
	Session string // the herdr session the lease is bound to; "" records "-"
}

// Lease is the state a transition left the work ref in: the tip SHA is
// the lease token, and the epoch tells this acquisition from every
// earlier one.
type Lease struct {
	Branch  string
	Tip     string
	Epoch   int
	BaseSHA string // acquire only: the base the claim was dated against
}

// Marker is one lease marker read off the work ref: its kind from the
// subject line and the trailers frit minted beneath it.
type Marker struct {
	Kind    string // claim | beat | release | takeover
	Epoch   int
	Nonce   string
	Holder  string
	Lane    string
	Session string
	Base    string
}

// HeldError reports an acquire that lost: the work ref already carries
// a live lease. It carries the winner's marker so the refusal can name
// the holder's epoch, machine and lane rather than guess.
type HeldError struct {
	PlanID     int64
	Tip        string // the tip that holds the ref on the remote
	Marker     Marker // the winner's latest lease marker; zero if unread
	Known      bool   // the marker was read; false → report no holder facts
	ThisHolder bool   // the winner's holder id is this run's own
	Landed     bool   // the winner's tip is merged into the base
}

func (e *HeldError) Error() string {
	return fmt.Sprintf(
		"plan %d is held: the work ref carries a live lease", e.PlanID)
}

func (e *HeldError) Unwrap() error { return ErrLostRace }

// FenceError reports a renewal or release whose CAS lost: the work ref
// moved under the lease, so this holder is fenced. It carries the
// mover's marker so the report can name who moved it.
type FenceError struct {
	PlanID int64
	Tip    string // where the ref is now, "" when it could not be read
	Marker Marker // the mover's latest lease marker; zero if unread
	Known  bool
}

func (e *FenceError) Error() string {
	if e.Known && e.Marker.Holder != "" {
		return fmt.Sprintf("fenced: the work ref for plan %d was moved by %s",
			e.PlanID, e.Marker.Holder)
	}

	return fmt.Sprintf(
		"fenced: the work ref for plan %d moved under the lease", e.PlanID)
}

// VetoError reports a takeover refused because herdr positively
// confirms the holder's bound session is live (F3, S61). It is never
// returned by this package — the veto needs herdr, which the lease
// atom does not import — and lives here only so its wording sits
// beside the errors it stands in for. Renewed says whether the beat
// pushed on the holder's behalf landed.
type VetoError struct {
	PlanID  int64
	Marker  Marker
	Renewed bool
}

func (e *VetoError) Error() string {
	return fmt.Sprintf(
		"plan %d is held by a live session; takeover vetoed", e.PlanID)
}

func (e *VetoError) Unwrap() error { return ErrLostRace }

// StillHeldError reports a yield run from the lane that still holds
// the live lease: nothing is fenced, so there is nothing to rescue.
// Yield is for the fenced, not an alias for release.
type StillHeldError struct {
	PlanID int64
}

func (e *StillHeldError) Error() string {
	return fmt.Sprintf(
		"plan %d is still held by this lane; yield is for a fenced lane, "+
			"use release instead", e.PlanID)
}

// Acquire leases the work ref for a plan: refs/heads/plan/<id>.
//
// An absent ref is acquired fresh at epoch 1 on the base; a ref whose
// tip is a release marker is re-acquired at epoch E+1 as a child of
// that marker, so history is appended, never rewritten. Anything else
// on the tip is a live lease, and the returned HeldError names it.
func Acquire(repoDir string, opts LeaseOptions, run gitwt.Runner) (Lease, error) {
	ref := "refs/heads/" + leaseBranch(opts.PlanID)
	tip := remoteHolder(repoDir, opts.Remote, ref, run)
	if tip == "" {
		return pushClaimMarker(repoDir, opts, ref, "", 1, run)
	}

	// The ref exists. Its objects may not be local — the lease could be
	// another machine's — so bring them in once before reading the tip.
	_, _ = run(repoDir, "fetch", "--quiet", opts.Remote, ref)
	m, ok := commitMarker(repoDir, opts.PlanID, tip, run)
	if ok && m.Kind == markerRelease {
		return pushClaimMarker(repoDir, opts, ref, tip, m.Epoch+1, run)
	}

	return Lease{}, heldError(repoDir, opts, tip, run)
}

// Renew advances the lease from the holder's recorded tip: a beat
// marker, child of that tip, same epoch. A foreign move fences the
// renewal and the error names the mover.
func Renew(
	repoDir string, opts LeaseOptions, from string, run gitwt.Runner,
) (Lease, error) {
	lease, err := advance(repoDir, opts, markerBeat, from, run)
	if err == nil {
		persistToken(opts, lease.Tip, run)
	}

	return lease, err
}

// Resume is the self-resume transition (F9, F11, S3, S21): mechanically
// a renewal — a beat CASed from the lane's own recorded tip, same
// epoch — named for what the caller is doing with it. A lane resumes
// on the strength of its own persisted token matching origin's current
// tip, with no staleness window consulted at all; that decision is the
// caller's, not this function's.
func Resume(
	repoDir string, opts LeaseOptions, from string, run gitwt.Runner,
) (Lease, error) {
	return Renew(repoDir, opts, from, run)
}

// Release marks the lease released: a release marker, child of the
// holder's recorded tip, same epoch. It deletes nothing — the history
// stays for the next acquisition to build on, and a later Acquire
// CASes exactly on the release marker at epoch E+1.
func Release(
	repoDir string, opts LeaseOptions, from string, run gitwt.Runner,
) (Lease, error) {
	return advance(repoDir, opts, markerRelease, from, run)
}

// Takeover seizes a stale lease: a takeover marker minted as a child
// of exactly the observed stale tip, at epoch E+1, CASed so the server
// arbitrates. A holder that was merely quiet renews first and wins —
// the takeover then loses the CAS, re-reads, and the returned
// HeldError names the live holder; the loser moves nothing and retries
// never, it just resets to observing.
func Takeover(
	repoDir string, opts LeaseOptions, from string, run gitwt.Runner,
) (Lease, error) {
	m, ok := fetchedMarker(repoDir, opts, from, run)
	if !ok {
		return Lease{}, fmt.Errorf(
			"no lease marker for plan %d is reachable from %s",
			opts.PlanID, from)
	}
	marker, err := mintMarker(
		repoDir, markerTakeover, from, opts, m.Epoch+1, "", run)
	if err != nil {
		return Lease{}, err
	}

	ref := "refs/heads/" + leaseBranch(opts.PlanID)
	lost, tip, err := casPush(repoDir, ref, opts, marker, from, run)
	if err != nil {
		return Lease{}, fmt.Errorf(
			"push takeover for plan %d: %w", opts.PlanID, err)
	}
	if lost {
		return Lease{}, heldError(repoDir, opts, tip, run)
	}
	persistToken(opts, marker, run)

	return Lease{
		Branch: leaseBranch(opts.PlanID), Tip: marker, Epoch: m.Epoch + 1,
	}, nil
}

// Scavenged describes what a scavenge did: the rescue ref unlanded
// work was parked to, "" when the chain held nothing but markers and
// landed commits — nothing a delete could destroy.
type Scavenged struct {
	Rescue string
}

// Scavenge removes a work ref on landed or matured evidence: park any
// unlanded work to the rescue ref first, then delete by CAS on
// exactly the observed tip. The evidence is tied to that tip — a
// holder that renewed moved it, so the scavenge refuses before it
// parks anything, and a renewing holder can never be scavenged (A2).
// Retries are idempotent: a ref already gone is a clean no-op, and a
// rescue an earlier half-done run parked is recognized, never
// clobbered.
func Scavenge(
	repoDir string, opts LeaseOptions, from string, run gitwt.Runner,
) (Scavenged, error) {
	ref := "refs/heads/" + leaseBranch(opts.PlanID)
	switch now := remoteHolder(repoDir, opts.Remote, ref, run); now {
	case "":
		// Already gone — an earlier run, or another machine's, finished
		// the job. Clean the stale local copy and report a no-op.
		_, _ = run(repoDir, "update-ref", "-d", ref)
		return Scavenged{}, nil
	case from:
	default:
		return Scavenged{}, fenceError(repoDir, opts, now, run)
	}

	res := Scavenged{}
	unlanded, err := hasUnlanded(repoDir, opts, from, run)
	if err != nil {
		return res, err
	}
	if unlanded {
		rescue := rescueRef(opts.PlanID, opts.Holder)
		if err := park(repoDir, opts, rescue, from, run); err != nil {
			return res, err
		}
		res.Rescue = rescue
	}

	if _, err := run(repoDir, "push",
		"--force-with-lease="+ref+":"+from,
		opts.Remote, ":"+ref); err != nil {
		// The delete is classified like every push: by what holds the
		// ref now, never by stderr. Gone is a win; anything else is not.
		if remoteHolder(repoDir, opts.Remote, ref, run) != "" {
			return res, fmt.Errorf(
				"delete %s for plan %d: %w", ref, opts.PlanID, err)
		}
	}
	_, _ = run(repoDir, "update-ref", "-d", ref)

	return res, nil
}

// Yield ends a fenced lane's stake in a plan without racing the work
// ref itself: whatever this lane's own copy of it carries — local is
// the lane's recorded tip — is parked to the rescue ref, create-only,
// the same park a scavenge runs. It never CASes the work ref, so a
// lane whose local tip still matches origin's current tip is not
// fenced at all; that is the live holder, and yielding it would
// silently discard the lease rather than release it, so that case is
// refused instead (F4, F5).
//
// An empty local is its own case, checked first: nothing was ever
// fetched or minted here, so there is nothing of this lane's own to
// rescue. It must not fall into the still-held comparison — an absent
// remote ref reads as "" too, and "" == "" would misreport a plan
// nobody holds as still held by this lane. It must not reach park
// either: park's push is tip+":"+rescue, and an empty tip turns that
// into a delete of the rescue ref, which git accepts whether or not
// the ref exists — a silent false "parked" for a rescue that was
// never written.
func Yield(
	repoDir string, opts LeaseOptions, local string, run gitwt.Runner,
) (Scavenged, error) {
	if local == "" {
		return Scavenged{}, nil
	}

	ref := "refs/heads/" + leaseBranch(opts.PlanID)
	if remoteHolder(repoDir, opts.Remote, ref, run) == local {
		return Scavenged{}, &StillHeldError{PlanID: opts.PlanID}
	}

	rescue := rescueRef(opts.PlanID, opts.Holder)
	if err := park(repoDir, opts, rescue, local, run); err != nil {
		return Scavenged{}, err
	}

	return Scavenged{Rescue: rescue}, nil
}

// RescueRefs lists the rescue refs a plan's scavenges and yields have
// parked work to, one per machine that ever parked, so stranded
// commits are found again. The remote is read directly — a rescue ref
// is pushed straight to origin and never fetched into a local branch —
// and an unreadable remote answers an empty list rather than a fault,
// the same tolerance every discovery read gives a repository it cannot
// reach.
func RescueRefs(
	repoDir, remote string, planID int64, run gitwt.Runner,
) []string {
	pattern := fmt.Sprintf("refs/frit/rescue/%d/*", planID)
	out, err := run(repoDir, "ls-remote", remote, pattern)
	if err != nil {
		return []string{}
	}

	refs := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		refs = append(refs, fields[1])
	}
	sort.Strings(refs)

	return refs
}

// rescueRef names where scavenge and yield park a plan's unlanded
// work: per plan and per machine, so two scavengers never contend for
// one name and parked work says whose run parked it.
func rescueRef(planID int64, holder string) string {
	return fmt.Sprintf("refs/frit/rescue/%d/%s", planID, holder)
}

// park pushes the tip to the rescue ref, create-only. A rescue that
// already exists at the same tip is an earlier half-done run's work,
// already parked; one at a different tip is somebody's parked work,
// and clobbering it would destroy exactly what the rescue exists to
// keep — so park refuses, and the caller must not delete.
func park(
	repoDir string, opts LeaseOptions, rescue, tip string, run gitwt.Runner,
) error {
	if _, err := run(repoDir, "push",
		"--force-with-lease="+rescue+":",
		opts.Remote, tip+":"+rescue); err == nil {
		return nil
	}
	if remoteHolder(repoDir, opts.Remote, rescue, run) == tip {
		return nil
	}

	return fmt.Errorf(
		"rescue ref %s already holds other work; not deleting plan %d's ref",
		rescue, opts.PlanID)
}

// hasUnlanded reports whether the chain from the base to the tip
// holds anything besides frit's own markers — work a delete would
// destroy. The base is refreshed the way the landed check refreshes
// it, so the answer is against origin's view, not a stale local one.
func hasUnlanded(
	repoDir string, opts LeaseOptions, tip string, run gitwt.Runner,
) (bool, error) {
	base := freshBase(repoDir, opts.Base, opts.Remote, run)
	out, err := run(repoDir, "log", "--format=%s", tip, "^"+base)
	if err != nil {
		return false, fmt.Errorf(
			"read the chain for plan %d: %w", opts.PlanID, err)
	}
	for _, subject := range strings.Split(string(out), "\n") {
		if subject == "" {
			continue
		}
		if !markerSubject(subject, opts.PlanID) {
			return true, nil
		}
	}

	return false, nil
}

// markerSubject reports whether a subject line is one of frit's own
// lease markers for this plan — the droppings a delete may discard.
// The legacy decorated subjects carry a slug behind the kind and are
// deliberately not markers here: parking them is harmless, dropping
// them is not provably so.
func markerSubject(subject string, planID int64) bool {
	rest, ok := strings.CutPrefix(subject, fmt.Sprintf("plan %d: ", planID))
	if !ok {
		return false
	}
	switch rest {
	case markerClaim, markerBeat, markerRelease, markerTakeover:
		return true
	}

	return false
}

// Released reports whether a ref tip is a release marker for a plan.
// It reads only the subject, so a fleet walk can ask it per hold ref
// without parsing bodies; an unreadable object reads as not released,
// which fails safe — the ref then still counts as a hold.
func Released(repoDir, tip string, planID int64, run gitwt.Runner) bool {
	subject, err := trimmed(run(repoDir, "log", "-1", "--format=%s", tip))

	return err == nil &&
		subject == fmt.Sprintf("plan %d: %s", planID, markerRelease)
}

// leaseBranch is the work ref's branch name: plan/<id>, id only, so
// nothing derived from local state — a slug, a title — reaches the ref
// and two machines can never name the same plan differently.
func leaseBranch(planID int64) string {
	return fmt.Sprintf("plan/%d", planID)
}

// advance is renew and release: mint a marker child of the holder's
// recorded tip carrying the epoch read beneath that tip, and CAS the
// ref from exactly that tip. A lost CAS is a fence, not a fault.
func advance(
	repoDir string, opts LeaseOptions, kind, from string, run gitwt.Runner,
) (Lease, error) {
	m, ok := latestMarker(repoDir, opts.PlanID, from, run)
	if !ok {
		return Lease{}, fmt.Errorf(
			"no lease marker for plan %d is reachable from %s",
			opts.PlanID, from)
	}
	marker, err := mintMarker(repoDir, kind, from, opts, m.Epoch, "", run)
	if err != nil {
		return Lease{}, err
	}

	ref := "refs/heads/" + leaseBranch(opts.PlanID)
	lost, tip, err := casPush(repoDir, ref, opts, marker, from, run)
	if err != nil {
		return Lease{}, fmt.Errorf(
			"push %s for plan %d: %w", kind, opts.PlanID, err)
	}
	if lost {
		return Lease{}, fenceError(repoDir, opts, tip, run)
	}

	return Lease{
		Branch: leaseBranch(opts.PlanID), Tip: marker, Epoch: m.Epoch,
	}, nil
}

// pushClaimMarker mints the claim marker for an acquisition and CASes
// the work ref onto it: from absence when parent is "", else from
// exactly the release marker the new claim is a child of.
func pushClaimMarker(
	repoDir string, opts LeaseOptions, ref, parent string, epoch int,
	run gitwt.Runner,
) (Lease, error) {
	baseSHA, err := trimmed(run(repoDir, "rev-parse", opts.Base))
	if err != nil {
		return Lease{}, err
	}
	par := parent
	if par == "" {
		par = baseSHA
	}
	marker, err := mintMarker(
		repoDir, markerClaim, par, opts, epoch, baseSHA, run)
	if err != nil {
		return Lease{}, err
	}

	lost, tip, err := casPush(repoDir, ref, opts, marker, parent, run)
	if err != nil {
		return Lease{}, fmt.Errorf(
			"push claim for plan %d: %w", opts.PlanID, err)
	}
	if lost {
		return Lease{}, heldError(repoDir, opts, tip, run)
	}
	persistToken(opts, marker, run)

	return Lease{
		Branch:  leaseBranch(opts.PlanID),
		Tip:     marker,
		Epoch:   epoch,
		BaseSHA: baseSHA,
	}, nil
}

// mintMarker writes one marker commit: the parent's own tree, so the
// marker touches no file, with the lease trailers as its body.
func mintMarker(
	repoDir, kind, parent string, opts LeaseOptions, epoch int, base string,
	run gitwt.Runner,
) (string, error) {
	tree, err := trimmed(run(repoDir, "rev-parse", parent+"^{tree}"))
	if err != nil {
		return "", err
	}
	nonce, err := newNonce()
	if err != nil {
		return "", err
	}

	return trimmed(run(repoDir, "commit-tree", tree, "-p", parent,
		"-m", leaseMessage(kind, opts, epoch, nonce, base)))
}

// casPush is the one server-side arbitration every transition rides:
// push the marker over the expected old value — "" means the ref must
// be absent — and classify a failure by what actually holds the ref on
// the remote, never by git's stderr (see Mint for why). lost reports a
// CAS the remote decided against us, with the tip that beat it; an
// error is a real fault. The remote is the truth either way, so the
// local copy of the ref is synced on a win and untouched on a loss.
func casPush(
	repoDir, ref string, opts LeaseOptions, marker, expected string,
	run gitwt.Runner,
) (lost bool, tip string, err error) {
	_, pushErr := run(repoDir, "push",
		"--force-with-lease="+ref+":"+expected,
		opts.Remote, marker+":"+ref)
	if pushErr == nil {
		syncLocalRef(repoDir, ref, marker, run)
		return false, marker, nil
	}

	now := remoteHolder(repoDir, opts.Remote, ref, run)
	switch now {
	case marker:
		// Our own marker is on the remote: the push landed even though
		// the client reported an error — a connection dropped after the
		// ref transaction committed. The transition is ours.
		syncLocalRef(repoDir, ref, marker, run)
		return false, marker, nil
	case "":
		// Nothing on the remote, or it could not be read: the push left
		// no ref, so this is a real fault, not a lost arbitration.
		return false, "", pushErr
	default:
		return true, now, nil
	}
}

// syncLocalRef moves the local copy of the work ref to the tip the
// remote just accepted. Best-effort: the branch may be checked out in
// the lane's worktree, where git refuses an update from outside, and a
// stale local copy is a stale view, not a lost lease.
func syncLocalRef(repoDir, ref, tip string, run gitwt.Runner) {
	_, _ = run(repoDir, "update-ref", ref, tip)
}

// heldError names the lease that won an acquire: epoch, holder and
// lane read off the winner's latest marker, plus whether that holder
// is this same machine and whether the work already landed on the base
// — the facts a refusal reports instead of a guess.
func heldError(
	repoDir string, opts LeaseOptions, tip string, run gitwt.Runner,
) error {
	e := &HeldError{PlanID: opts.PlanID, Tip: tip}
	m, ok := fetchedMarker(repoDir, opts, tip, run)
	if !ok {
		return e
	}
	e.Marker = m
	e.Known = true
	e.ThisHolder = m.Holder != "" && m.Holder == opts.Holder
	e.Landed = landedTip(repoDir, opts.Base, opts.Remote, tip, run)

	return e
}

// fenceError names the mover that fenced a renewal or release, read
// off the marker now holding the ref.
func fenceError(
	repoDir string, opts LeaseOptions, tip string, run gitwt.Runner,
) error {
	e := &FenceError{PlanID: opts.PlanID, Tip: tip}
	if m, ok := fetchedMarker(repoDir, opts, tip, run); ok {
		e.Marker = m
		e.Known = true
	}

	return e
}

// ReadMarker reads the lease marker a work ref's tip carries — epoch,
// machine, lane and bound session — fetching the ref once when the
// objects are not local. ok is false when no marker for this plan is
// reachable from tip. It is the takeover veto's way to learn who
// currently holds a lease without minting anything.
func ReadMarker(
	repoDir string, opts LeaseOptions, tip string, run gitwt.Runner,
) (Marker, bool) {
	return fetchedMarker(repoDir, opts, tip, run)
}

// TakeoverCount reads the backoff factor k straight off a plan's work
// ref: how many takeover markers already sit in the chain since base
// (F3). Every observer reads the same chain, so every observer
// computes the same k, and a takeover waits k·T instead of T — the
// damping that keeps two live but quiet agents from oscillating.
//
// A git fault answers zero rather than fail the caller: the count only
// ever widens the window, so a wrong zero costs cost, never
// correctness, the same tolerance every part of staleness gets.
func TakeoverCount(
	repoDir string, planID int64, base, tip string, run gitwt.Runner,
) int {
	out, err := run(repoDir, "log", "--format=%s", tip, "^"+base)
	if err != nil {
		return 0
	}
	want := fmt.Sprintf("plan %d: %s", planID, markerTakeover)
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		if line == want {
			n++
		}
	}

	return n
}

// RemoteTip is the tip origin holds for a plan's work ref right now,
// "" when the ref is absent or the remote could not be read. Unlike a
// gathered discovery.Plan's HoldTip, which may be this clone's stale
// local view of a ref another lane moved, this is always a fresh read
// — the one self-resume compares its persisted token against.
func RemoteTip(repoDir, remote string, planID int64, run gitwt.Runner) string {
	ref := "refs/heads/" + leaseBranch(planID)

	return remoteHolder(repoDir, remote, ref, run)
}

// fetchedMarker reads the latest lease marker reachable from tip,
// fetching the work ref once when the objects are not local — a lease
// another machine pushed that this clone has never seen.
func fetchedMarker(
	repoDir string, opts LeaseOptions, tip string, run gitwt.Runner,
) (Marker, bool) {
	if m, ok := latestMarker(repoDir, opts.PlanID, tip, run); ok {
		return m, true
	}
	ref := "refs/heads/" + leaseBranch(opts.PlanID)
	if _, err := run(repoDir, "fetch", "--quiet", opts.Remote, ref); err != nil {
		return Marker{}, false
	}

	return latestMarker(repoDir, opts.PlanID, tip, run)
}

// latestMarker finds the most recent lease marker reachable from tip —
// the tip itself, or the marker beneath a run of work commits — and
// parses it. ok is false when no marker for this plan is reachable.
func latestMarker(
	repoDir string, planID int64, tip string, run gitwt.Runner,
) (Marker, bool) {
	pattern := fmt.Sprintf("^plan %d: ", planID)
	body, err := trimmed(run(repoDir, "log", "-1",
		"--grep="+pattern, "--format=%B", tip))
	if err != nil || body == "" {
		return Marker{}, false
	}

	return parseMarker(planID, body)
}

// commitMarker reads the lease marker a single commit carries — the
// tip's own message, not the chain beneath it, which is how a release
// marker is told apart from work pushed on top of one.
func commitMarker(
	repoDir string, planID int64, sha string, run gitwt.Runner,
) (Marker, bool) {
	body, err := trimmed(run(repoDir, "log", "-1", "--format=%B", sha))
	if err != nil {
		return Marker{}, false
	}

	return parseMarker(planID, body)
}

// leaseMessage builds a lease marker's commit body: the kind in the
// subject, the trailers beneath, "-" standing in for an empty lane or
// session so every key is always present, and the base trailer only on
// the claim that was dated against it.
func leaseMessage(
	kind string, opts LeaseOptions, epoch int, nonce, base string,
) string {
	lane := opts.Lane
	if lane == "" {
		lane = "-"
	}
	session := opts.Session
	if session == "" {
		session = "-"
	}
	msg := fmt.Sprintf(
		"plan %d: %s\n\n"+
			"epoch:   %d\n"+
			"nonce:   %s\n"+
			"holder:  %s\n"+
			"lane:    %s\n"+
			"session: %s",
		opts.PlanID, kind, epoch, nonce, opts.Holder, lane, session)
	if base != "" {
		msg += "\nbase:    " + base
	}

	return msg
}

// parseMarker reads a lease marker from a commit body: the kind off
// the subject line, the trailers beneath it. ok is false for a body
// that is not this plan's marker — a work commit, or another plan's.
func parseMarker(planID int64, body string) (Marker, bool) {
	lines := strings.Split(body, "\n")
	kind, ok := strings.CutPrefix(lines[0], fmt.Sprintf("plan %d: ", planID))
	if !ok || kind == "" {
		return Marker{}, false
	}

	m := Marker{Kind: strings.TrimSpace(kind)}
	for _, line := range lines[1:] {
		key, val, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "epoch":
			if n, err := strconv.Atoi(val); err == nil {
				m.Epoch = n
			}
		case "nonce":
			m.Nonce = val
		case "holder":
			m.Holder = val
		case "lane":
			m.Lane = val
		case "session":
			m.Session = val
		case "base":
			m.Base = val
		}
	}

	return m, true
}

// newNonce mints the random token that keeps every marker SHA unique.
// The nonce is required for correctness (A3): SHA-based CAS is only
// ABA-proof if no two commits can hash alike, and a deterministic
// marker could be recreated at an old SHA.
func newNonce() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
