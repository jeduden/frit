// Package fleet gathers the cross-repo, cross-branch plan index the
// discovery verbs read, and infers the plan a worktree is working from
// the current directory.
//
// The discovery package is pure and works over a slice of plans; this
// package is where that slice is built out of git, and where "no
// selector at all" is turned into a plan id by running the cwd join
// backwards.
package fleet

import (
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/repocfg"
)

// CurrentPlanID infers the plan a directory is working, by resolving
// the directory to its worktree root, reading the branch that worktree
// is on, and matching it against that repository's hold patterns.
//
// This is the cwd join the who command runs, in reverse: who starts
// from an agent and asks which plan, this starts from a directory and
// asks the same. A directory in no repository, on a branch outside the
// convention, or under a repository declaring no patterns, claims no
// plan and reports false rather than an error — being outside a lane is
// a fact, not a fault.
func CurrentPlanID(
	cwd string, run gitwt.Runner,
	holdsFor func(root string) repocfg.Holds,
) (int64, bool) {
	site := herdr.Resolve(cwd, run)
	if site.Root == "" || site.Branch == "" {
		return 0, false
	}

	id, ok := holdsFor(site.Root).Match(site.Branch)
	if !ok {
		return 0, false
	}

	return id, true
}
