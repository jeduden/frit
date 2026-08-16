package discovery

// DepNode is one plan in an upstream dependency walk, together with the
// plans it in turn waits on. It answers "what blocks this" by carrying
// the whole tree rather than a flat list, so a person can see not just
// what is unfinished but why.
type DepNode struct {
	// Plan is the plan at this node. For an unresolved edge only ID and
	// Repo are set.
	Plan Plan
	// Found reports whether the edge resolved to a known plan. An
	// unknown upstream is carried, not dropped: an edge frit cannot see
	// is itself a thing blocking the plan.
	Found bool
	// Deps are this plan's own upstreams, walked in the order declared.
	Deps []DepNode
}

// Dependencies walks a plan's upstream DAG within its repository.
//
// Each edge names a plan id, resolved among the same repository's plans
// because ids are only unique there. The walk expands every plan once:
// a plan reached by two paths, or a cycle, is shown where it is first
// met and left unexpanded afterwards, so the walk always terminates.
func Dependencies(root Plan, plans []Plan) DepNode {
	byRepoID := indexByRepoID(plans)
	expanded := map[int64]bool{}

	return walk(root, root.Repo, byRepoID, expanded)
}

// walk builds the node for one plan, recursing into upstreams it has
// not already expanded.
func walk(
	p Plan, repo string,
	byRepoID map[string]map[int64]Plan, expanded map[int64]bool,
) DepNode {
	node := DepNode{Plan: p, Found: true}
	if expanded[p.ID] {
		return node
	}
	expanded[p.ID] = true

	for _, dep := range p.DependsOn {
		child, ok := byRepoID[repo][dep]
		if !ok {
			node.Deps = append(node.Deps, DepNode{
				Plan:  Plan{ID: dep, Repo: repo},
				Found: false,
			})
			continue
		}
		node.Deps = append(node.Deps, walk(child, repo, byRepoID, expanded))
	}

	return node
}

// indexByRepoID keys plans by repository then id, the shape a
// dependency edge is resolved through.
func indexByRepoID(plans []Plan) map[string]map[int64]Plan {
	out := map[string]map[int64]Plan{}
	for _, p := range plans {
		repo, ok := out[p.Repo]
		if !ok {
			repo = map[int64]Plan{}
			out[p.Repo] = repo
		}
		repo[p.ID] = p
	}

	return out
}
