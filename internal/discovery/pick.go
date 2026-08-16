package discovery

import "sort"

// Pick ranks the startable plans and returns at most n of them, for
// when ready lists more than a person wants to read.
//
// The candidates are exactly what ready returns; the ranking is by how
// much each unblocks — the number of other plans that wait on it —
// most first, ties broken by repo then id so the order is stable.
// Starting the plan that frees the most downstream work is the honest
// reading of "what should I start next". A non-positive n means all of
// them.
func Pick(plans []Plan, n int) []Plan {
	ranked := Ready(plans)
	unblocks := downstreamCounts(plans)

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

	if n > 0 && n < len(ranked) {
		return ranked[:n]
	}

	return ranked
}

// downstreamCounts counts, per plan, how many plans depend on it,
// resolved within each repository.
func downstreamCounts(plans []Plan) map[string]map[int64]int {
	counts := map[string]map[int64]int{}
	for _, p := range plans {
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
