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
