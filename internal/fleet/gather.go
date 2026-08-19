package fleet

import (
	"errors"
	"fmt"
	"sort"

	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/index"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/jeduden/frit/internal/plans"
	"github.com/jeduden/frit/internal/repocfg"
)

// Problem is one repository, or one file within it, frit could not
// read. It travels with the result rather than ending the gather, so a
// single broken checkout does not blind the whole board.
//
// NotPlan marks the benign kind: a markdown file in the plan directory
// that carries no front matter is simply not a plan, not a fault. A
// repository keeps a PLAN.md index and old notes beside its plans, so
// these are common and drowning the real failures. They are hidden
// unless a caller asks for everything.
type Problem struct {
	Repo    string
	Err     error
	NotPlan bool
}

// Coord is the per-repository state a lease is minted from: where the
// repository lives, the remote a claim is pushed to, and the base a
// lease is dated against. The gather already resolves all three walking
// the fleet, so a mutating verb reads them off the result rather than
// walking the whole fleet a second time to re-derive them.
//
// The base is the config's when set, otherwise the default-ref cascade
// the gather already runs for its merged-ref filter — the same cascade
// a lease dates against.
type Coord struct {
	Path   string
	Remote string
	Base   string
}

// Result is a gathered fleet: every plan's authoritative version across
// every repository and ref, the coordinate a lease is minted from for
// each repository, and the problems met on the way.
//
// The coordinates are keyed by repository name — the same name a plan
// carries — and are per repository, not per plan, so they travel beside
// the plans rather than copied onto each one a repository holds.
type Result struct {
	Plans    []discovery.Plan
	Problems []Problem
	Coords   map[string]Coord
}

// Gather reads every repository under root and flattens its plan index
// into the view discovery works over.
//
// It is the plans walk plus holds: the same cross-ref index frit plans
// builds, joined to the claims frit orphans reads, so a plan arrives
// carrying both its dependency edges and whether a lane already holds
// it. One unreadable repository is recorded and stepped over; a bad
// front matter file within a good repository is recorded too.
func Gather(
	root, host string, run gitwt.Runner, pipe gitwt.PipeRunner,
) (Result, error) {
	repos, err := discover.Repos(root, run)
	if err != nil {
		return Result{}, err
	}

	res := Result{
		Plans:    []discovery.Plan{},
		Problems: []Problem{},
		Coords:   map[string]Coord{},
	}
	ambiguous := map[string]bool{}
	for _, repo := range repos {
		entries, held, coord, problems, err := gatherRepo(host, repo, run, pipe)
		if err != nil {
			res.Problems = append(res.Problems,
				Problem{Repo: repo.Name, Err: err})
			continue
		}
		res.Problems = append(res.Problems, problems...)
		recordCoord(&res, ambiguous, repo.Name, coord)
		for _, e := range entries {
			res.Plans = append(res.Plans, planOf(repo.Name, e, held))
		}
	}

	return res, nil
}

// recordCoord files a repository's lease coordinate under its name,
// unless another repository under the root already claimed that name.
//
// The fleet keys every plan by its repository's basename, so two
// checkouts sharing one cannot be told apart — a coordinate under that
// name could mint a lease into the wrong repository. When two collide
// the coordinate is dropped and the collision recorded as a problem, so
// a mutating verb refuses rather than guesses. The plans of both repos
// are still gathered, so the read-only board stays useful; only the
// lease, which must land in one exact checkout, is withheld.
func recordCoord(
	res *Result, ambiguous map[string]bool, name string, coord Coord,
) {
	if ambiguous[name] {
		return
	}
	if _, dup := res.Coords[name]; dup {
		delete(res.Coords, name)
		ambiguous[name] = true
		res.Problems = append(res.Problems, Problem{
			Repo: name,
			Err: fmt.Errorf("repository name %q is not unique under the "+
				"root; rename one so a claim lands in the right checkout",
				name),
		})
		return
	}
	res.Coords[name] = coord
}

// gatherRepo reads one repository's plans, the ids its lanes hold, and
// the coordinate a lease is minted from. The coordinate reuses the
// config and the default-ref cascade this loop already resolved, so it
// costs no new walk or subprocess.
func gatherRepo(
	host string, repo discover.Repo,
	run gitwt.Runner, pipe gitwt.PipeRunner,
) ([]index.Entry, map[int64][]string, Coord, []Problem, error) {
	cfg, err := repocfg.Load(repo.Path)
	if err != nil {
		return nil, nil, Coord{}, nil, err
	}

	files, err := plans.Collect(repo.Path, cfg.PlanDir, run, pipe)
	if err != nil {
		return nil, nil, Coord{}, nil, err
	}

	preferred := gitobj.DefaultRef(repo.Path, run)
	entries, errs := index.Build(host, repo.Name, preferred, files)
	problems := make([]Problem, 0, len(errs))
	for _, e := range errs {
		problems = append(problems, Problem{
			Repo:    repo.Name,
			Err:     e,
			NotPlan: errors.Is(e, planmeta.ErrNoFrontMatter),
		})
	}

	held, err := heldBranches(
		repo, cfg, preferred, index.LandedIDs(entries), run)
	if err != nil {
		return nil, nil, Coord{}, nil, err
	}

	return entries, held, coordOf(repo, cfg, preferred), problems, nil
}

// coordOf resolves the lease coordinate from what the gather already
// read: the repository path, the config's remote, and the base — the
// config's when set, otherwise the default-ref cascade the gather ran.
func coordOf(repo discover.Repo, cfg repocfg.Config, preferred string) Coord {
	base := cfg.Base
	if base == "" {
		base = preferred
	}

	return Coord{Path: repo.Path, Remote: cfg.Remote, Base: base}
}

// heldBranches maps each claimed plan id to the branches that claim it:
// the same holds the orphan report is built on, merged refs already
// filtered out so landed work does not read as a live claim. The branch
// names are the lanes a holder works the plan on, deduplicated so a
// claim pushed to a remote does not read as two.
func heldBranches(
	repo discover.Repo, cfg repocfg.Config, preferred string,
	landed map[int64]bool, run gitwt.Runner,
) (map[int64][]string, error) {
	holds, err := cfg.Compiled()
	if err != nil {
		return nil, err
	}
	refs, err := gitobj.Refs(repo.Path, run)
	if err != nil {
		return nil, err
	}
	merged, err := gitobj.MergedRefs(repo.Path, preferred, run)
	if err != nil {
		return nil, err
	}

	held := map[int64][]string{}
	for _, lane := range lanes.Build(
		repo.Worktrees, refs, merged, landed, holds) {
		seen := map[string]bool{}
		for _, h := range lane.Holds {
			if seen[h.Branch] {
				continue
			}
			seen[h.Branch] = true
			held[lane.PlanID] = append(held[lane.PlanID], h.Branch)
		}
	}

	return held, nil
}

// planOf projects a plan's authoritative version into the discovery
// view, tagging it held when a lane claims its id.
func planOf(
	repoName string, e index.Entry, held map[int64][]string,
) discovery.Plan {
	v := e.Primary()
	holds := held[e.Key.ID]

	return discovery.Plan{
		Key:       e.Key.String(),
		Repo:      repoName,
		ID:        e.Key.ID,
		Status:    v.Plan.Status,
		Title:     v.Plan.Title,
		Summary:   v.Plan.Summary,
		Model:     v.Plan.Model,
		Goal:      v.Plan.Goal,
		DependsOn: v.Plan.DependsOn,
		Phases:    v.Plan.Phases,
		Path:      v.Path,
		Branches:  shortBranches(e),
		Held:      len(holds) > 0,
		Holds:     holds,
	}
}

// shortBranches collects the short names of every ref carrying the
// plan, in any version, deduplicated and sorted so a selector's slug
// match is stable. These are what a plan is remembered by when the id
// is gone.
func shortBranches(e index.Entry) []string {
	seen := map[string]bool{}
	for _, v := range e.Versions {
		for _, ref := range v.Refs {
			seen[gitobj.Ref{Name: ref}.Short()] = true
		}
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)

	return out
}
