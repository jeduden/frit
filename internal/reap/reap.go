// Package reap decides what frit's own landed check clears to be torn
// down: a stranded lane's checkout, and the branch it stood on.
//
// It never trusts a caller's classification alone. A lane already
// called stranded — claimed but with no live hold — still has its own
// branch re-checked against the caller's landed evidence before
// anything is cleared for deletion, so a delete never rides on a
// guess.
package reap

import "github.com/jeduden/frit/internal/gitwt"

// Decision is one worktree of a stranded lane, and whether frit's own
// evidence clears it to be torn down.
type Decision struct {
	PlanID   int64
	Worktree gitwt.Worktree
	Branch   string
	// Refused is why the worktree is left standing, empty when Landed
	// cleared it for reap.
	Refused string
}

// Landed reports whether the caller's own evidence — an ordinary
// merge's ancestry, or a plan already done on the default branch —
// confirms a branch has landed. It is the one gate a delete rides on.
type Landed func(branch string) bool

// Decide classifies every worktree of a stranded lane on its own
// branch: one Landed confirms is cleared to reap, everything else is
// refused rather than deleted without frit's own evidence.
func Decide(planID int64, worktrees []gitwt.Worktree, landed Landed) []Decision {
	out := make([]Decision, 0, len(worktrees))
	for _, w := range worktrees {
		d := Decision{PlanID: planID, Worktree: w, Branch: w.Branch}
		if !landed(w.Branch) {
			d.Refused = "frit does not read this branch as landed"
		}
		out = append(out, d)
	}

	return out
}
