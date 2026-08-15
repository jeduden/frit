// Package lanes joins claims to checkouts.
//
// A lane is one plan being worked: the refs that claim it, and the
// worktrees standing on those refs. Reporting what is abandoned means
// finding the lanes where those two halves disagree.
package lanes

import (
	"sort"
	"time"

	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/repocfg"
)

// Hold is one ref that claims a plan.
type Hold struct {
	// Ref is the full ref name, e.g. refs/heads/plan/42-x.
	Ref string
	// Branch is the branch name with any remote prefix stripped, so a
	// local claim and its copy on a remote read the same.
	Branch string
	// PlanID is the plan the ref claims.
	PlanID int64
}

// Lane is one plan, its claims, and the checkouts working them.
type Lane struct {
	PlanID    int64
	Holds     []Hold
	Worktrees []gitwt.Worktree
}

// Unstaffed reports a plan that is claimed but has no checkout: the
// work was taken and then either never set up or cleaned away.
func (l Lane) Unstaffed() bool {
	return len(l.Holds) > 0 && len(l.Worktrees) == 0
}

// Build joins one repository's refs and worktrees into lanes.
//
// Merged refs are excluded before anything else. Landing a plan does
// not delete its branch, so without that filter every finished plan
// would report as an active claim — the single largest source of false
// holds in a long-lived repository.
func Build(
	worktrees []gitwt.Worktree,
	refs []gitobj.Ref,
	merged map[string]bool,
	holds repocfg.Holds,
) []Lane {
	byID := map[int64]*Lane{}

	for _, ref := range refs {
		if merged[ref.Name] {
			continue
		}
		branch, ok := ref.Branch()
		if !ok {
			continue
		}
		id, ok := holds.Match(branch)
		if !ok {
			continue
		}
		lane := laneFor(byID, id)
		lane.Holds = append(lane.Holds, Hold{
			Ref: ref.Name, Branch: branch, PlanID: id,
		})
	}

	for _, wt := range worktrees {
		if wt.Branch == "" {
			continue
		}
		id, ok := holds.Match(wt.Branch)
		if !ok {
			continue
		}
		lane := laneFor(byID, id)
		lane.Worktrees = append(lane.Worktrees, wt)
	}

	return collect(byID)
}

// laneFor returns the lane for a plan id, creating it on first use.
func laneFor(byID map[int64]*Lane, id int64) *Lane {
	lane, ok := byID[id]
	if !ok {
		lane = &Lane{PlanID: id}
		byID[id] = lane
	}

	return lane
}

// collect flattens the grouping into a stable order.
func collect(byID map[int64]*Lane) []Lane {
	out := make([]Lane, 0, len(byID))
	for _, lane := range byID {
		sort.Slice(lane.Holds, func(i, j int) bool {
			return lane.Holds[i].Ref < lane.Holds[j].Ref
		})
		sort.Slice(lane.Worktrees, func(i, j int) bool {
			return lane.Worktrees[i].Path < lane.Worktrees[j].Path
		})
		out = append(out, *lane)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].PlanID < out[j].PlanID
	})

	return out
}

// Orphans are the lanes and checkouts that no longer add up.
type Orphans struct {
	// Unstaffed are plans claimed with no checkout behind them.
	Unstaffed []Lane
	// Empty are worktrees that never received a commit.
	Empty []gitwt.Worktree
	// Prunable are worktrees git considers removable.
	Prunable []gitwt.Worktree
}

// Any reports whether anything was found, so a caller can stay quiet
// when a repository is in good order.
func (o Orphans) Any() bool {
	return len(o.Unstaffed) > 0 || len(o.Empty) > 0 ||
		len(o.Prunable) > 0
}

// Find classifies what is abandoned.
//
// The three kinds are deliberately separate rather than one count:
// an unstaffed lane means work was claimed and dropped, an empty
// worktree means a lane was prepared and never started, and a prunable
// one means the checkout is already gone. They call for different
// responses.
//
// A bare repository is never an orphan; it has no working tree by
// definition.
func Find(built []Lane, worktrees []gitwt.Worktree) Orphans {
	var o Orphans

	for _, lane := range built {
		if lane.Unstaffed() {
			o.Unstaffed = append(o.Unstaffed, lane)
		}
	}

	for _, wt := range worktrees {
		if wt.Bare {
			continue
		}
		if wt.Prunable {
			o.Prunable = append(o.Prunable, wt)
			continue
		}
		if !wt.HasCommit() {
			o.Empty = append(o.Empty, wt)
		}
	}

	return o
}

// Aged is a worktree that has not moved for a while.
type Aged struct {
	Worktree gitwt.Worktree
	Age      time.Duration
}

// Stale reports worktrees whose branch has not received a commit for
// longer than olderThan.
//
// Age is measured from the branch tip rather than from the checkout's
// mtime: a directory is touched by builds, greps and editors, none of
// which mean work happened. A worktree whose branch has no recorded
// time — detached, or never committed — is skipped here and caught by
// the orphan report instead.
func Stale(
	worktrees []gitwt.Worktree,
	times map[string]int64,
	now time.Time,
	olderThan time.Duration,
) []Aged {
	var out []Aged

	for _, wt := range worktrees {
		if wt.Bare || wt.Branch == "" || !wt.HasCommit() {
			continue
		}
		stamp, ok := times["refs/heads/"+wt.Branch]
		if !ok {
			continue
		}
		age := now.Sub(time.Unix(stamp, 0))
		if age > olderThan {
			out = append(out, Aged{Worktree: wt, Age: age})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Age > out[j].Age
	})

	return out
}
