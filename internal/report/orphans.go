package report

import "github.com/jeduden/frit/internal/lanes"

// OrphansDoc is what `frit orphans` found.
//
// Every repository walked is listed, including the ones with nothing
// wrong. The table skips those to stay short, which leaves a reader
// unable to tell a clean repository from one that was never reached;
// the document says both, one with empty lists and one in Problems.
type OrphansDoc struct {
	header
	Root     string       `json:"root"`
	Repos    []OrphanRepo `json:"repos"`
	Problems []Problem    `json:"problems"`
}

// OrphanRepo is one repository's abandoned work, kept in kinds rather
// than one count: a claimed lane with no checkout means work was taken
// and dropped, a stranded lane means a checkout outlived its now-landed
// claim, an empty worktree means a lane was prepared and never started,
// and a prunable one means the checkout is already gone. They call for
// different responses.
type OrphanRepo struct {
	Name      string         `json:"name"`
	Unstaffed []Lane         `json:"unstaffed"`
	Stranded  []StrandedLane `json:"stranded"`
	Empty     []Worktree     `json:"empty"`
	Prunable  []Worktree     `json:"prunable"`
}

// Lane is one plan and the refs claiming it.
type Lane struct {
	PlanID int64  `json:"plan_id"`
	Holds  []Hold `json:"holds"`
}

// StrandedLane is one plan whose checkout outlived its claim: the branch
// has merged, so no ref holds it, but a worktree is still standing on
// it. It carries the worktrees rather than holds, because holds is empty
// by definition — the landed ref is exactly what is missing.
type StrandedLane struct {
	PlanID    int64      `json:"plan_id"`
	Worktrees []Worktree `json:"worktrees"`
}

// Hold is one ref that claims a plan.
type Hold struct {
	Ref    string `json:"ref"`
	Branch string `json:"branch"`
}

// Any reports whether anything was found, so a renderer can stay quiet
// about a repository in good order.
func (r OrphanRepo) Any() bool {
	return len(r.Unstaffed) > 0 || len(r.Stranded) > 0 ||
		len(r.Empty) > 0 || len(r.Prunable) > 0
}

// NewOrphans opens an orphan report.
func NewOrphans(root string) *OrphansDoc {
	return &OrphansDoc{
		header:   newHeader("orphans"),
		Root:     root,
		Repos:    []OrphanRepo{},
		Problems: []Problem{},
	}
}

// AddRepo records what one repository turned up, clean or not.
func (d *OrphansDoc) AddRepo(name string, found lanes.Orphans) {
	repo := OrphanRepo{
		Name:      name,
		Unstaffed: make([]Lane, 0, len(found.Unstaffed)),
		Stranded:  make([]StrandedLane, 0, len(found.Stranded)),
		Empty:     worktreesOf(found.Empty),
		Prunable:  worktreesOf(found.Prunable),
	}

	for _, lane := range found.Unstaffed {
		repo.Unstaffed = append(repo.Unstaffed, laneOf(lane))
	}

	for _, lane := range found.Stranded {
		repo.Stranded = append(repo.Stranded, strandedOf(lane))
	}

	d.Repos = append(d.Repos, repo)
}

// AddProblem records a repository whose lanes could not be read.
func (d *OrphansDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// laneOf carries a lane's claims across. The plan id is on the lane
// already, so a hold repeats only what distinguishes it: the ref it
// is, and the branch name that reads the same for a local claim and
// its copy on a remote.
func laneOf(lane lanes.Lane) Lane {
	out := Lane{
		PlanID: lane.PlanID,
		Holds:  make([]Hold, 0, len(lane.Holds)),
	}

	for _, h := range lane.Holds {
		out.Holds = append(out.Holds, Hold{Ref: h.Ref, Branch: h.Branch})
	}

	return out
}

// strandedOf carries a stranded lane's checkouts across. It surfaces the
// worktrees rather than holds, because a landed lane has none: the ref
// that would have named the claim is the very thing that merged away.
func strandedOf(lane lanes.Lane) StrandedLane {
	return StrandedLane{
		PlanID:    lane.PlanID,
		Worktrees: worktreesOf(lane.Worktrees),
	}
}
