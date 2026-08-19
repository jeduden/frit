package herdr

import (
	"path/filepath"
	"strings"

	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/repocfg"
)

// Site is where a pane sits: the worktree it is in and the branch that
// worktree is on. Either may be empty — a pane in no repository has no
// root, and a detached checkout has no branch.
type Site struct {
	Root   string
	Branch string
}

// Resolve walks a pane's cwd back to the worktree it sits in.
//
// The cwd is the pane's shell directory, which drifts below the
// worktree root as the shell cds around, so the root is found with
// `rev-parse --show-toplevel` — git walks up for us — rather than by
// matching cwd against a known worktree path. A cwd in no git
// repository yields an empty Site rather than an error: a pane outside
// any worktree is a fact to report, not a fault to fail on.
func Resolve(cwd string, run gitwt.Runner) Site {
	out, err := run(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return Site{}
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return Site{}
	}

	branch := ""
	if b, err := run(root, "symbolic-ref", "--quiet", "--short",
		"HEAD"); err == nil {
		branch = strings.TrimSpace(string(b))
	}

	return Site{Root: root, Branch: branch}
}

// Lane is a pane resolved back to the plan it sits on.
//
// PlanID is zero when the branch claims no plan — a pane on a branch
// outside the convention, or in no repository at all. The lane is kept
// regardless: an agent doing real work off the convention is a fact
// the board must not hide.
type Lane struct {
	Pane   Pane
	Root   string
	Repo   string
	Branch string
	PlanID int64
}

// HasPlan reports whether the pane resolved to a known plan.
func (l Lane) HasPlan() bool {
	return l.PlanID != 0
}

// Join resolves every pane to its lane through the cwd join.
//
// gitFor returns the git runner that reaches a pane's host: the local
// git for the empty Host, and one that runs over ssh for a remote one.
// A pane's cwd is a path on the machine it lives on, so resolving it
// against any other host's git would either fail or, worse, match a
// coincidental local checkout — the wrong-lane hazard this join exists
// to prevent.
//
// holdsFor returns the hold patterns for a worktree root; it is a
// callback rather than a hard dependency on repocfg so this package
// stays about panes and git, and the caller owns config loading and
// its caching. The roots resolved here are memoised so a fleet of
// panes in one repository costs one config read, not one per pane.
func Join(
	panes []Pane, gitFor func(Host) gitwt.Runner,
	holdsFor func(root string) repocfg.Holds,
) []Lane {
	holdsByRoot := map[string]repocfg.Holds{}

	lanes := make([]Lane, 0, len(panes))
	for _, p := range panes {
		site := Resolve(p.CWD, gitFor(p.Host))
		lane := Lane{
			Pane:   p,
			Root:   site.Root,
			Branch: site.Branch,
		}
		if site.Root != "" {
			lane.Repo = filepath.Base(site.Root)
			lane.PlanID = matchPlan(site, holdsByRoot, holdsFor)
		}
		lanes = append(lanes, lane)
	}

	return lanes
}

// LiveRoots returns the set of worktree roots that have an agent on
// them right now.
//
// It is what sharpens staleness: a branch that has not moved but still
// has an agent in its worktree is being worked, not abandoned. A pane
// with no agent, or one git cannot resolve to a root, contributes
// nothing, and a root with several panes appears once.
//
// Only local panes count. Staleness is a fact about this machine's
// worktrees, and a remote root is a path on another host that could
// collide with a local one — so a pane from elsewhere never marks a
// local worktree live.
func LiveRoots(panes []Pane, run gitwt.Runner) map[string]bool {
	roots := map[string]bool{}
	for _, p := range panes {
		if !p.HasAgent() || p.Host != "" {
			continue
		}
		if site := Resolve(p.CWD, run); site.Root != "" {
			roots[site.Root] = true
		}
	}

	return roots
}

// matchPlan reads the plan id a site's branch claims, loading and
// caching the root's hold patterns on first sight.
func matchPlan(
	site Site, cache map[string]repocfg.Holds,
	holdsFor func(root string) repocfg.Holds,
) int64 {
	if site.Branch == "" {
		return 0
	}

	holds, ok := cache[site.Root]
	if !ok {
		holds = holdsFor(site.Root)
		cache[site.Root] = holds
	}

	id, _ := holds.Match(site.Branch)

	return id
}
