package report

import (
	"time"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/lanes"
)

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
	Name       string         `json:"name"`
	Unstaffed  []Lane         `json:"unstaffed"`
	Stranded   []StrandedLane `json:"stranded"`
	Empty      []Worktree     `json:"empty"`
	Prunable   []Worktree     `json:"prunable"`
	Migratable []Migratable   `json:"migratable"`
	// StaleHolds are held plans ready to be taken over — a takeover
	// window that has matured — nobody has acted on yet, read from the
	// same observation fold board and claim use rather than lanes.Find's
	// git-ref sweep.
	StaleHolds []StaleHold `json:"stale_holds"`
	// Deserted are held plans herdr confirms have lost their bound
	// session, before the takeover window matures, that no worktree's
	// own token can self-resume. A matured hold reads as a StaleHold
	// instead — the two kinds never collide.
	Deserted []Deserted `json:"deserted"`
}

// StaleHold is one held plan ready for a takeover: its window matured
// (StaleSeconds > 0), its bound session is confirmed gone (Dead), or
// both.
type StaleHold struct {
	PlanID       int64  `json:"plan_id"`
	Branch       string `json:"branch"`
	StaleSeconds int64  `json:"stale_seconds"`
	Dead         bool   `json:"dead"`
}

// Migratable is a hold that still reads as a claim but is decorated
// rather than the id-only shape the lease protocol writes.
type Migratable struct {
	PlanID int64  `json:"plan_id"`
	From   string `json:"from"`
	To     string `json:"to"`
}

// Deserted is one held plan whose bound session herdr reports gone and
// that self-resume cannot recover — a dead end regardless of whether
// its takeover window has matured yet.
type Deserted struct {
	PlanID int64  `json:"plan_id"`
	Branch string `json:"branch"`
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
		len(r.Empty) > 0 || len(r.Prunable) > 0 || len(r.Migratable) > 0 ||
		len(r.StaleHolds) > 0 || len(r.Deserted) > 0
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
		Name:       name,
		Unstaffed:  make([]Lane, 0, len(found.Unstaffed)),
		Stranded:   make([]StrandedLane, 0, len(found.Stranded)),
		Empty:      worktreesOf(found.Empty),
		Prunable:   worktreesOf(found.Prunable),
		Migratable: make([]Migratable, 0, len(found.Migratable)),
		StaleHolds: []StaleHold{},
		Deserted:   []Deserted{},
	}

	for _, lane := range found.Unstaffed {
		repo.Unstaffed = append(repo.Unstaffed, laneOf(lane))
	}

	for _, lane := range found.Stranded {
		repo.Stranded = append(repo.Stranded, strandedOf(lane))
	}

	for _, m := range found.Migratable {
		repo.Migratable = append(repo.Migratable,
			Migratable{PlanID: m.PlanID, From: m.From, To: m.To})
	}

	d.Repos = append(d.Repos, repo)
}

// AddStale records the plans in one repository whose lease has
// matured, beside the kinds AddRepo already recorded for it — the
// held-stale cell of the verb-state table, read from the same
// observation fold board and claim use rather than lanes.Find's
// git-ref sweep. A no-op when AddRepo was never called for the name.
func (d *OrphansDoc) AddStale(name string, plans []discovery.Plan) {
	for i := range d.Repos {
		if d.Repos[i].Name != name {
			continue
		}
		for _, p := range plans {
			d.Repos[i].StaleHolds = append(
				d.Repos[i].StaleHolds, staleHoldOf(p))
		}

		return
	}
}

// staleHoldOf projects a matured plan into its wire shape.
func staleHoldOf(p discovery.Plan) StaleHold {
	branch := ""
	if len(p.Holds) > 0 {
		branch = p.Holds[0]
	}

	return StaleHold{
		PlanID:       p.ID,
		Branch:       branch,
		StaleSeconds: int64(p.StaleFor / time.Second),
		Dead:         p.Dead,
	}
}

// AddDeserted records the plans in one repository that are a dead
// end herdr's veto surfaced before any window matured, beside the
// kinds AddRepo already recorded for it — its own cell of the
// verb-state table, distinct from the matured StaleHolds cell. A
// no-op when AddRepo was never called for the name.
func (d *OrphansDoc) AddDeserted(name string, plans []discovery.Plan) {
	for i := range d.Repos {
		if d.Repos[i].Name != name {
			continue
		}
		for _, p := range plans {
			d.Repos[i].Deserted = append(d.Repos[i].Deserted, desertedOf(p))
		}

		return
	}
}

// desertedOf projects a deserted plan into its wire shape.
func desertedOf(p discovery.Plan) Deserted {
	branch := ""
	if len(p.Holds) > 0 {
		branch = p.Holds[0]
	}

	return Deserted{PlanID: p.ID, Branch: branch}
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
