package discovery

import "sort"

// Board returns the outstanding plans, ordered for a status board:
// in-progress first so active work floats to the top, then by
// repository and id. With wipOnly it narrows to the plans actually
// under way; otherwise it carries everything unfinished, begun or not.
//
// It answers what to keep an eye on — the work in flight and the work
// still to pick up — leaving the holder and the live agent for the
// caller to join, since those come from git refs and the herdr socket
// rather than the plan itself.
func Board(plans []Plan, wipOnly bool) []Plan {
	out := make([]Plan, 0)
	for _, p := range plans {
		if wipOnly && !p.InProgress() {
			continue
		}
		if !wipOnly && !p.Unfinished() {
			continue
		}
		out = append(out, p)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if r := rank(out[i]); r != rank(out[j]) {
			return r < rank(out[j])
		}
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}

		return out[i].ID < out[j].ID
	})

	return out
}

// rank orders the statuses on the board: in-progress before
// not-started, so what is moving reads first.
func rank(p Plan) int {
	if p.InProgress() {
		return 0
	}

	return 1
}
