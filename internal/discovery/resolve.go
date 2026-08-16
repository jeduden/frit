package discovery

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ErrNotFound reports a selector that matched no plan.
var ErrNotFound = fmt.Errorf("no plan matches")

// Ambiguous reports a selector that matched more than one plan. It
// carries the candidates so a caller can print them and let the person
// narrow the selector rather than guessing on their behalf.
type Ambiguous struct {
	Selector   string
	Candidates []Plan
}

// Error lists the candidates one per line, so the message a command
// prints is itself the disambiguation.
func (e *Ambiguous) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%q matches %d plans:", e.Selector, len(e.Candidates))
	for _, p := range e.Candidates {
		fmt.Fprintf(&b, "\n  %d  %s", p.ID, p.Title)
	}

	return b.String()
}

// Resolve turns a selector into exactly one plan.
//
// Two forms are tried in order, because typing a ten-digit timestamp is
// its own friction:
//
//   - an exact id — unambiguous within a repository, but the same id
//     can exist in two repositories, so several matches is an ambiguity
//     to report rather than a match to pick from.
//   - a slug fragment — matched against titles, paths and branch names,
//     the identifiers a person remembers when the id is gone.
//
// The third form, inference from the current directory, is the caller's
// to resolve: it needs git, and this package stays pure. The caller
// turns a worktree into an id and hands that id here.
//
// Zero matches is ErrNotFound; more than one is an *Ambiguous carrying
// the candidates. Exactly one is the answer.
func Resolve(selector string, plans []Plan) (Plan, error) {
	if id, err := strconv.ParseInt(selector, 10, 64); err == nil {
		if byID := withID(id, plans); len(byID) > 0 {
			return one(selector, byID)
		}
	}

	return one(selector, bySlug(selector, plans))
}

// ByRepoID resolves the one plan a repository knows by id.
//
// It is the target of the cwd selector, where both halves of the fleet
// key are already known: the worktree names the repository and its
// branch names the id. Going through the string resolver instead would
// throw the repository away and match the id fleet-wide, so the same id
// in another repository would read as ambiguous when it is not.
func ByRepoID(repo string, id int64, plans []Plan) (Plan, error) {
	for _, p := range plans {
		if p.Repo == repo && p.ID == id {
			return p, nil
		}
	}

	return Plan{}, fmt.Errorf("%s:%d: %w", repo, id, ErrNotFound)
}

// withID collects every plan carrying an id, across repositories.
func withID(id int64, plans []Plan) []Plan {
	var out []Plan
	for _, p := range plans {
		if p.ID == id {
			out = append(out, p)
		}
	}

	return out
}

// bySlug collects every plan a lowered fragment appears in.
func bySlug(selector string, plans []Plan) []Plan {
	fragment := strings.ToLower(selector)

	var out []Plan
	for _, p := range plans {
		if p.matchesSlug(fragment) {
			out = append(out, p)
		}
	}

	return out
}

// one returns the sole match, or the error that says why there is not
// one. Candidates are ordered by repo then id so the disambiguation
// reads the same way twice.
func one(selector string, matches []Plan) (Plan, error) {
	switch len(matches) {
	case 0:
		return Plan{}, fmt.Errorf("%q: %w", selector, ErrNotFound)
	case 1:
		return matches[0], nil
	default:
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].Repo != matches[j].Repo {
				return matches[i].Repo < matches[j].Repo
			}

			return matches[i].ID < matches[j].ID
		})

		return Plan{}, &Ambiguous{Selector: selector, Candidates: matches}
	}
}
