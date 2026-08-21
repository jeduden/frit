package discovery

import "sort"

// Pick ranks the startable plans and returns at most n of them, for
// when ready lists more than a person wants to read.
//
// The candidates are exactly what ready returns; the ranking is by how
// much each unblocks — the number of plans still waiting on it — most
// first, ties broken by repo then id so the order is stable. A plan
// whose dependents are all finished frees no waiting work and does not
// count, so it never outranks one with a genuinely blocked dependent.
// Starting the plan that frees the most waiting work is the honest
// reading of "what should I start next". A non-positive n means all of
// them.
func Pick(plans []Plan, n int) []Plan {
	ranked := rankByUnblock(Ready(plans), plans)

	if n > 0 && n < len(ranked) {
		return ranked[:n]
	}

	return ranked
}

// Candidates orders every plan pick --go will try, most worth starting
// first: the fresh startable plans Pick ranks, then the resume tail —
// in progress, held by nobody — ranked the same way. A fresh plan is a
// real new pick, so it precedes a resume; the resume tail is the
// fallback for a lane that merged away and left prescribed work with no
// lane to run it. pick --go walks this list, taking the next when a
// claim loses its race.
func Candidates(plans []Plan) []Plan {
	return append(Pick(plans, 0), rankByUnblock(resumable(plans), plans)...)
}

// resumable is the resume tail: plans in progress that no lane holds,
// whose lane vanished when a phase merged. Held is checked so a plan a
// lane is actively working is not offered twice.
func resumable(plans []Plan) []Plan {
	out := make([]Plan, 0)
	for _, p := range plans {
		if !p.Held && p.InProgress() {
			out = append(out, p)
		}
	}

	return out
}

// rankByUnblock orders subset by how much each plan unblocks across the
// whole fleet — the number of still-waiting plans that depend on it,
// most first — ties broken by repo then id so the order is stable. The
// counts are drawn from all plans, not just the subset, so a resume is
// ranked by the same downstream weight a fresh pick is.
func rankByUnblock(subset, all []Plan) []Plan {
	ranked := append([]Plan(nil), subset...)
	unblocks := downstreamCounts(all)

	sort.SliceStable(ranked, func(i, j int) bool {
		ci := unblocks[ranked[i].Repo][ranked[i].ID]
		cj := unblocks[ranked[j].Repo][ranked[j].ID]
		if ci != cj {
			return ci > cj
		}
		if ranked[i].Repo != ranked[j].Repo {
			return ranked[i].Repo < ranked[j].Repo
		}

		return ranked[i].ID < ranked[j].ID
	})

	return ranked
}

// downstreamCounts counts, per plan, how many still-waiting plans
// depend on it, resolved within each repository. A dependent that is
// already done or superseded is waiting on nothing, so it is not
// counted: finishing its upstream would free no blocked work.
func downstreamCounts(plans []Plan) map[string]map[int64]int {
	counts := map[string]map[int64]int{}
	for _, p := range plans {
		if p.Done() || p.Superseded() {
			continue
		}
		for _, dep := range p.DependsOn {
			repo, ok := counts[p.Repo]
			if !ok {
				repo = map[int64]int{}
				counts[p.Repo] = repo
			}
			repo[dep]++
		}
	}

	return counts
}
