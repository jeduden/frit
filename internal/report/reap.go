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

// ReapRepo is one repository's stranded checkouts, reaped or refused.
type ReapRepo struct {
	Name    string        `json:"name"`
	Reaped  []ReapedLane  `json:"reaped"`
	Refused []RefusedLane `json:"refused"`
}

// Any reports whether this repository had anything to reap or refuse,
// so a table can stay quiet about one in good order.
func (r ReapRepo) Any() bool {
	return len(r.Reaped) > 0 || len(r.Refused) > 0
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

// AddRepo records one repository's reaped and refused stranded
// checkouts, clean or not.
func (d *ReapDoc) AddRepo(name string, reaped []ReapedLane, refused []RefusedLane) {
	repo := ReapRepo{
		Name:    name,
		Reaped:  reaped,
		Refused: refused,
	}
	if repo.Reaped == nil {
		repo.Reaped = []ReapedLane{}
	}
	if repo.Refused == nil {
		repo.Refused = []RefusedLane{}
	}
	d.Repos = append(d.Repos, repo)
}

// AddProblem records a repository frit could not read.
func (d *ReapDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
