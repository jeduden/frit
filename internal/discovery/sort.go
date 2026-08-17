package discovery

import (
	"sort"

	"github.com/jeduden/frit/internal/planmeta"
)

// SortKey names an order a list of plans can be arranged in, chosen
// over the command's own default.
type SortKey int

// The orders a person can ask for.
const (
	SortStatus SortKey = iota // in-progress first, then not-started, ...
	SortRepo                  // grouped by repository
	SortID                    // by id, which is creation time — oldest first
	SortHeld                  // claimed lanes first
)

// ParseSortKey resolves a flag value to a key, reporting whether it
// named one. "age" is accepted for id, since the id is a timestamp and
// that is what a person means by it.
func ParseSortKey(s string) (SortKey, bool) {
	switch s {
	case "status":
		return SortStatus, true
	case "repo":
		return SortRepo, true
	case "id", "age":
		return SortID, true
	case "held":
		return SortHeld, true
	}

	return 0, false
}

// Sort orders plans in place by a key, and reverses the result when
// asked. The sort is stable, so plans equal on the key keep the order
// they arrived in, and every key falls back to repo then id so the
// arrangement is total and reproducible.
func Sort(plans []Plan, key SortKey, reverse bool) {
	sort.SliceStable(plans, func(i, j int) bool {
		return less(key, plans[i], plans[j])
	})
	if reverse {
		Reverse(plans)
	}
}

// Reverse flips a list in place, so a command's own order can be turned
// end to end without naming a sort key.
func Reverse(plans []Plan) {
	for i, j := 0, len(plans)-1; i < j; i, j = i+1, j-1 {
		plans[i], plans[j] = plans[j], plans[i]
	}
}

// less is the ordering for one key, each falling through to repo then
// id so ties resolve the same way every run.
func less(key SortKey, a, b Plan) bool {
	switch key {
	case SortRepo:
		return byRepoID(a, b)
	case SortID:
		if a.ID != b.ID {
			return a.ID < b.ID
		}

		return a.Repo < b.Repo
	case SortHeld:
		if a.Held != b.Held {
			return a.Held // held before unheld
		}

		return byRepoID(a, b)
	default: // SortStatus
		if ra, rb := statusRank(a), statusRank(b); ra != rb {
			return ra < rb
		}

		return byRepoID(a, b)
	}
}

// byRepoID is the common tiebreak: repository, then id.
func byRepoID(a, b Plan) bool {
	if a.Repo != b.Repo {
		return a.Repo < b.Repo
	}

	return a.ID < b.ID
}

// statusRank orders the lifecycle for a board: what is moving first,
// then what is waiting, then what is settled.
func statusRank(p Plan) int {
	switch p.Status {
	case planmeta.StatusInProgress:
		return 0
	case planmeta.StatusNotStarted:
		return 1
	case planmeta.StatusDone:
		return 2
	case planmeta.StatusSuperseded:
		return 3
	default:
		return 4
	}
}
