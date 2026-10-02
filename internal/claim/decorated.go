package claim

import (
	"errors"
	"fmt"
	"sort"

	"github.com/jeduden/frit/internal/gitwt"
)

// Retired is one decorated branch a decorated takeover retired: the
// branch, the rescue ref its unlanded work was parked to ("" when the
// chain held nothing a delete could destroy), and what was actually
// removed. DeletedOnOrigin says origin's copy is gone; LocalKept says
// this clone's own copy still stands — a worktree is on it, or it
// carries commits past the observed tip — so it still reads as a hold
// on this host until that checkout is dealt with. A branch never
// pushed and kept locally had nothing removed at all.
type Retired struct {
	Branch          string
	Rescue          string
	DeletedOnOrigin bool
	LocalKept       bool
}

// TakeoverDecorated seizes a matured hold made of legacy decorated
// branches alone — plan/<id>-<slug>, with no id-only work ref beside
// it for Takeover to CAS a takeover marker onto (#204). tips maps each
// decorated branch to the tip its staleness window matured on.
//
// Every branch is read on origin first, and the takeover refuses as a
// lost race, naming the new tip, if any has moved: a holder that
// pushed since is not deserted, and nothing is parked or deleted for
// it (A2). A branch already gone from origin and this clone alike has
// nothing left to retire and is skipped. Then each branch is retired the way a scavenge retires a
// work ref — unlanded work parked to the rescue ref, then a delete
// CASed on exactly the observed tip — and only then is the id-only
// lease acquired, create-only, at epoch 1: a legacy claim carries no
// epoch chain to extend. The Acquire is still the arbiter against any
// other machine taking the plan meanwhile. A failure past the first
// retirement still returns the branches already retired beside the
// error, so a deletion that already happened is reported, not lost.
//
// A local copy of a branch is dropped only when it still sits at the
// observed tip and no worktree stands on it (S79); one carrying
// commits past that tip is this host's own unparked work and is kept.
func TakeoverDecorated(
	repoDir string, opts LeaseOptions, tips map[string]string,
	run gitwt.Runner,
) (Lease, error) {
	branches := make([]string, 0, len(tips))
	for b := range tips {
		branches = append(branches, b)
	}
	sort.Strings(branches)

	pushed := map[string]bool{}
	standing := make([]string, 0, len(branches))
	for _, b := range branches {
		now, onOrigin, err := decoratedTip(repoDir, opts, b, run)
		if err != nil {
			return Lease{}, err
		}
		switch now {
		case "":
			continue
		case tips[b]:
		default:
			return Lease{}, &HeldError{PlanID: opts.PlanID, Tip: now}
		}
		pushed[b] = onOrigin
		standing = append(standing, b)
	}

	retired := make([]Retired, 0, len(standing))
	for _, b := range standing {
		r, err := retireDecorated(repoDir, opts, b, tips[b], pushed[b], run)
		if err != nil {
			return Lease{Retired: retired}, err
		}
		retired = append(retired, r)
	}

	lease, err := Acquire(repoDir, opts, run)
	if err != nil {
		return Lease{Retired: retired}, err
	}
	lease.Retired = retired

	return lease, nil
}

// decoratedTip reads where a decorated branch stands now: origin's
// copy when origin carries one, else this clone's local copy — a
// decorated hold never pushed lives here alone; onOrigin says which.
// An unreadable origin is a fault, never folded into "not on origin".
func decoratedTip(
	repoDir string, opts LeaseOptions, branch string, run gitwt.Runner,
) (tip string, onOrigin bool, err error) {
	ref := "refs/heads/" + branch
	now, err := remoteHolderErr(repoDir, opts.Remote, ref, run)
	if err != nil {
		return "", false, fmt.Errorf(
			"read %s from %s for plan %d: %w", ref, opts.Remote, opts.PlanID, err)
	}
	if now != "" {
		return now, true, nil
	}

	return localTip(repoDir, ref, run), false, nil
}

// retireDecorated parks one decorated branch's unlanded work and then
// deletes it: on origin by the same CAS delete a scavenge runs, when
// origin carries it, and locally when the local copy still sits at tip
// with no worktree on it. A delete the remote refused because the
// branch moved is a lost race naming the new tip; one it refused
// while still holding the observed tip — a protected branch, a hook —
// is a fault, returned as is, never dressed up as a race (A2).
func retireDecorated(
	repoDir string, opts LeaseOptions, branch, tip string, pushed bool,
	run gitwt.Runner,
) (Retired, error) {
	parked, err := ParkUnlanded(repoDir, opts, tip, run)
	if err != nil {
		return Retired{}, fmt.Errorf(
			"%w; not deleting %s for plan %d", err, branch, opts.PlanID)
	}
	ref := "refs/heads/" + branch
	if pushed {
		err := casDelete(repoDir, opts, ref, tip, run)
		var refused *DeleteRefusedError
		if errors.As(err, &refused) && refused.Holder != tip {
			return Retired{}, &HeldError{PlanID: opts.PlanID, Tip: refused.Holder}
		}
		if err != nil {
			return Retired{}, err
		}
	}
	if localTip(repoDir, ref, run) == tip && !checkedOut(repoDir, branch, run) {
		_, _ = run(repoDir, "update-ref", "-d", ref, tip)
	}

	return Retired{
		Branch: branch, Rescue: parked.Rescue, DeletedOnOrigin: pushed,
		LocalKept: localTip(repoDir, ref, run) != "",
	}, nil
}
