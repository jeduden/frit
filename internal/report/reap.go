package report

// ReapDoc is what `frit reap` did, or would do, across the fleet: for
// a stranded lane whose branch frit's own evidence confirms landed,
// the checkout removed and the branch deleted; for one it does not
// confirm, a refusal rather than a guessed delete.
//
// Go records whether --go was given, so a consumer reading Reaped
// alone knows whether it already happened or is only what would.
type ReapDoc struct {
	header
	Root     string     `json:"root"`
	Go       bool       `json:"go"`
	Repos    []ReapRepo `json:"repos"`
	Problems []Problem  `json:"problems"`
}

// ReapRepo is one repository's orphans, reaped or refused: a stranded
// checkout, an unstaffed hold, a prunable or never-started worktree.
// The kinds stay separate rather than merged into a count, the same
// reason orphans keeps them apart: each calls for a different
// response, and a consumer reading one kind should not have to sift
// it out of the others.
type ReapRepo struct {
	Name          string            `json:"name"`
	Reaped        []ReapedLane      `json:"reaped"`
	Refused       []RefusedLane     `json:"refused"`
	Dropped       []DroppedHold     `json:"dropped"`
	RefusedHolds  []RefusedHold     `json:"refused_holds"`
	Pruned        []PrunedWorktree  `json:"pruned"`
	RefusedPruned []RefusedWorktree `json:"refused_pruned"`
}

// Any reports whether this repository had anything to reap or refuse,
// so a table can stay quiet about one in good order.
func (r ReapRepo) Any() bool {
	return len(r.Reaped) > 0 || len(r.Refused) > 0 ||
		len(r.Dropped) > 0 || len(r.RefusedHolds) > 0 ||
		len(r.Pruned) > 0 || len(r.RefusedPruned) > 0
}

// ReapedLane is one stranded checkout reap removed, or would remove
// under --go, along with the branch it deleted.
type ReapedLane struct {
	PlanID   int64    `json:"plan_id"`
	Worktree Worktree `json:"worktree"`
	Branch   string   `json:"branch"`
}

// RefusedLane is one stranded checkout reap left standing because
// frit's own landed check did not confirm its branch landed.
type RefusedLane struct {
	PlanID   int64    `json:"plan_id"`
	Worktree Worktree `json:"worktree"`
	Branch   string   `json:"branch"`
	Reason   string   `json:"reason"`
}

// DroppedHold is one unstaffed lane's canonical hold dropped through
// claim.Scavenge, or would be dropped under a dry run. Rescue is the
// ref any unlanded work was parked to, "" when the chain held nothing
// a delete could destroy.
type DroppedHold struct {
	PlanID int64  `json:"plan_id"`
	Branch string `json:"branch"`
	Rescue string `json:"rescue"`
}

// RefusedHold is one unstaffed lane's hold left standing: a decorated
// legacy branch claim.Scavenge has no ref to CAS against, or a
// canonical one Scavenge could not drop — fenced by another machine
// since observed, or already gone.
type RefusedHold struct {
	PlanID int64  `json:"plan_id"`
	Branch string `json:"branch"`
	Reason string `json:"reason"`
}

// PrunedWorktree is one prunable or never-started worktree reap
// removed, or would remove under a dry run. Kind names which orphan
// kind this was: "prunable" or "empty".
type PrunedWorktree struct {
	Worktree Worktree `json:"worktree"`
	Kind     string   `json:"kind"`
}

// RefusedWorktree is one prunable or never-started worktree reap left
// standing because git itself refused to remove it: real content on
// disk it could not reconcile against a resolvable commit — the S79
// shape, where a worktree's branch ref vanished without landing and
// so reads as zero-commit exactly like one that never started.
type RefusedWorktree struct {
	Worktree Worktree `json:"worktree"`
	Kind     string   `json:"kind"`
	Reason   string   `json:"reason"`
}

// NewReap opens a reap report; goFlag is whether --go was given.
func NewReap(root string, goFlag bool) *ReapDoc {
	return &ReapDoc{
		header:   newHeader("reap"),
		Root:     root,
		Go:       goFlag,
		Repos:    []ReapRepo{},
		Problems: []Problem{},
	}
}

// AddRepo records one repository's reaped and refused orphans, clean
// or not, across every kind reap acts on.
func (d *ReapDoc) AddRepo(
	name string,
	reaped []ReapedLane, refused []RefusedLane,
	dropped []DroppedHold, refusedHolds []RefusedHold,
	pruned []PrunedWorktree, refusedPruned []RefusedWorktree,
) {
	repo := ReapRepo{
		Name: name, Reaped: reaped, Refused: refused,
		Dropped: dropped, RefusedHolds: refusedHolds,
		Pruned: pruned, RefusedPruned: refusedPruned,
	}
	if repo.Reaped == nil {
		repo.Reaped = []ReapedLane{}
	}
	if repo.Refused == nil {
		repo.Refused = []RefusedLane{}
	}
	if repo.Dropped == nil {
		repo.Dropped = []DroppedHold{}
	}
	if repo.RefusedHolds == nil {
		repo.RefusedHolds = []RefusedHold{}
	}
	if repo.Pruned == nil {
		repo.Pruned = []PrunedWorktree{}
	}
	if repo.RefusedPruned == nil {
		repo.RefusedPruned = []RefusedWorktree{}
	}
	d.Repos = append(d.Repos, repo)
}

// AddProblem records a repository frit could not read.
func (d *ReapDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
