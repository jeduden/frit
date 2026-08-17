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
	"path/filepath"

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
//
// It returns the repository alongside the id, because a plan id is only
// unique within a repository: the caller needs both halves of the key
// to resolve the exact plan without colliding with the same id in
// another repository.
func CurrentPlanID(
	cwd string, run gitwt.Runner,
	holdsFor func(root string) repocfg.Holds,
) (repo string, id int64, ok bool) {
	site := herdr.Resolve(cwd, run)
	if site.Root == "" || site.Branch == "" {
		return "", 0, false
	}

	id, ok = holdsFor(site.Root).Match(site.Branch)
	if !ok {
		return "", 0, false
	}

	return RepoName(site.Root, run), id, true
}

// RepoName is the indexed name of the repository a worktree belongs to:
// the basename of its main worktree, which is how discover keys it. A
// linked worktree sits under its own directory, so its own basename
// would be the lane's name, not the repository's — the main worktree,
// always first in the list, is the one to name. If git cannot be
// asked, the worktree's own basename is the honest fallback.
//
// It is exported because a dispatch verb resolves a live lane back to
// its repository this way: a hold branch name is repo-local, so the
// lane's repository must match the plan's before frit acts on it.
func RepoName(root string, run gitwt.Runner) string {
	worktrees, err := gitwt.List(root, run)
	if err != nil || len(worktrees) == 0 {
		return filepath.Base(root)
	}

	return filepath.Base(worktrees[0].Path)
}
