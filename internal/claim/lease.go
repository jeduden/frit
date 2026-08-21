package claim

import (
	"errors"
	"fmt"

	"github.com/jeduden/frit/internal/gitwt"
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

// Acquire leases the work ref for a plan: refs/heads/plan/<id>.
func Acquire(repoDir string, opts LeaseOptions, run gitwt.Runner) (Lease, error) {
	return Lease{}, errors.New("not implemented")
}

// Renew advances the lease from the holder's recorded tip.
func Renew(
	repoDir string, opts LeaseOptions, from string, run gitwt.Runner,
) (Lease, error) {
	return Lease{}, errors.New("not implemented")
}

// Release marks the lease released, deleting nothing.
func Release(
	repoDir string, opts LeaseOptions, from string, run gitwt.Runner,
) (Lease, error) {
	return Lease{}, errors.New("not implemented")
}

// Released reports whether a ref tip is a release marker for a plan.
func Released(repoDir, tip string, planID int64, run gitwt.Runner) bool {
	return false
}

// leaseMessage builds a lease marker's commit body.
func leaseMessage(
	kind string, opts LeaseOptions, epoch int, nonce, base string,
) string {
	return ""
}

// parseMarker reads a lease marker from a commit body.
func parseMarker(planID int64, body string) (Marker, bool) {
	return Marker{}, false
}

// newNonce mints the random token that keeps every marker SHA unique.
func newNonce() (string, error) {
	return "", errors.New("not implemented")
}
