package discovery

import "strings"

// Find returns the plans whose title or summary contains the query,
// case-insensitively, across every repository and ref.
//
// It is the verb for when you remember the topic but not the id. It
// reads the plans the index already gathered — no checkout, no fresh
// walk — and matches the authoritative version of each, so a plan is
// found once however many refs carry it. Results are ordered by
// repository then id.
func Find(query string, plans []Plan) []Plan {
	needle := strings.ToLower(strings.TrimSpace(query))

	// A blank query is not a wildcard, and matching against it would
	// walk every plan for nothing.
	out := make([]Plan, 0)
	if needle == "" {
		return out
	}

	for _, p := range plans {
		if strings.Contains(strings.ToLower(p.Title), needle) ||
			strings.Contains(strings.ToLower(p.Summary), needle) {
			out = append(out, p)
		}
	}

	sortByRepoID(out)

	return out
}
