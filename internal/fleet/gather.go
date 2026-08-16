package fleet

import (
	"sort"

	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/index"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/jeduden/frit/internal/plans"
	"github.com/jeduden/frit/internal/repocfg"
)

// Problem is one repository, or one file within it, frit could not
// read. It travels with the result rather than ending the gather, so a
// single broken checkout does not blind the whole board.
type Problem struct {
	Repo string
	Err  error
}

// Result is a gathered fleet: every plan's authoritative version across
// every repository and ref, and the problems met on the way.
type Result struct {
	Plans    []discovery.Plan
	Problems []Problem
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

	res := Result{Plans: []discovery.Plan{}, Problems: []Problem{}}
	for _, repo := range repos {
		entries, held, problems, err := gatherRepo(host, repo, run, pipe)
		if err != nil {
			res.Problems = append(res.Problems, Problem{repo.Name, err})
			continue
		}
		res.Problems = append(res.Problems, problems...)
		for _, e := range entries {
			res.Plans = append(res.Plans, planOf(repo.Name, e, held))
		}
	}

	return res, nil
}

// gatherRepo reads one repository's plans and the ids its lanes hold.
func gatherRepo(
	host string, repo discover.Repo,
	run gitwt.Runner, pipe gitwt.PipeRunner,
) ([]index.Entry, map[int64]bool, []Problem, error) {
	cfg, err := repocfg.Load(repo.Path)
	if err != nil {
		return nil, nil, nil, err
	}

	files, err := plans.Collect(repo.Path, cfg.PlanDir, run, pipe)
	if err != nil {
		return nil, nil, nil, err
	}

	preferred := gitobj.DefaultRef(repo.Path, run)
	entries, errs := index.Build(host, repo.Name, preferred, files)
	problems := make([]Problem, 0, len(errs))
	for _, e := range errs {
		problems = append(problems, Problem{repo.Name, e})
	}

	held, err := heldIDs(repo, cfg, preferred, run)
	if err != nil {
		return nil, nil, nil, err
	}

	return entries, held, problems, nil
}

// heldIDs is the set of plan ids a lane currently claims: the same
// hold read the orphan report is built on, merged refs already filtered
// out so landed work does not read as a live claim.
func heldIDs(
	repo discover.Repo, cfg repocfg.Config, preferred string,
	run gitwt.Runner,
) (map[int64]bool, error) {
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

	held := map[int64]bool{}
	for _, lane := range lanes.Build(repo.Worktrees, refs, merged, holds) {
		if len(lane.Holds) > 0 {
			held[lane.PlanID] = true
		}
	}

	return held, nil
}

// planOf projects a plan's authoritative version into the discovery
// view, tagging it held when a lane claims its id.
func planOf(
	repoName string, e index.Entry, held map[int64]bool,
) discovery.Plan {
	v := e.Primary()

	return discovery.Plan{
		Key:       e.Key.String(),
		Repo:      repoName,
		ID:        e.Key.ID,
		Status:    v.Plan.Status,
		Title:     v.Plan.Title,
		Summary:   v.Plan.Summary,
		Model:     v.Plan.Model,
		DependsOn: v.Plan.DependsOn,
		Phases:    v.Plan.Phases,
		Path:      v.Path,
		Branches:  shortBranches(e),
		Held:      held[e.Key.ID],
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
