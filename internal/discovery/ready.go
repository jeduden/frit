package discovery

import "sort"

// Ready returns the plans that can be started now: not yet begun,
// claimed by nobody, and with every dependency done — plus the matured
// takeovers, held plans whose lease has been observed stale.
//
// This is the question the whole index exists to answer, and it cannot
// be answered from one file. A dependency names a plan id, resolved
// within the same repository because ids are only unique there; a plan
// whose every upstream is ✅ and which no lane holds is startable, and
// anything else is withheld. A dependency that resolves to no known
// plan is treated as unmet — an edge frit cannot confirm is done is not
// assumed to be.
//
// Results are ordered by repository then id, so the board reads the
// same way twice.
func Ready(plans []Plan) []Plan {
	done := doneByRepo(plans)

	out := make([]Plan, 0)
	for _, p := range plans {
		if !candidate(p) {
			continue
		}
		if allDone(p, done) {
			out = append(out, p)
		}
	}

	sortByRepoID(out)

	return out
}

// candidate is the per-plan half of the readiness rule: a fresh start
// — not begun, held by nobody — or a matured takeover, a held plan
// whose lease reads stale and whose work is still outstanding. A
// live-tip hold stays hidden.
func candidate(p Plan) bool {
	if p.Held {
		return p.Stale && p.Unfinished()
	}

	return p.NotStarted()
}

// doneByRepo indexes which plan ids are done, keyed by repository so a
// dependency edge is resolved against the right repository's plans.
func doneByRepo(plans []Plan) map[string]map[int64]bool {
	done := map[string]map[int64]bool{}
	for _, p := range plans {
		if !p.Done() {
			continue
		}
		repo, ok := done[p.Repo]
		if !ok {
			repo = map[int64]bool{}
			done[p.Repo] = repo
		}
		repo[p.ID] = true
	}

	return done
}

// allDone reports whether every dependency of a plan is a done plan in
// the same repository.
func allDone(p Plan, done map[string]map[int64]bool) bool {
	for _, dep := range p.DependsOn {
		if !done[p.Repo][dep] {
			return false
		}
	}

	return true
}

// sortByRepoID orders plans by repository then id in place.
func sortByRepoID(plans []Plan) {
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].Repo != plans[j].Repo {
			return plans[i].Repo < plans[j].Repo
		}

		return plans[i].ID < plans[j].ID
	})
}
