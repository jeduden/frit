package repocfg

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// idToken is the placeholder a hold pattern uses for the plan id.
const idToken = "{id}"

// ErrNoID reports a pattern that names no plan id. Such a pattern
// could tell you a ref looks like a claim but not what it claims, so
// it is rejected rather than silently ignored.
var ErrNoID = errors.New("pattern must contain exactly one " + idToken)

// Pattern matches a branch name and extracts the plan id it claims.
//
// Patterns are matched against the branch name with any remote prefix
// already stripped, so one pattern covers a local branch and its copy
// on every remote — which is what a claim pushed to a shared forge
// looks like from here.
type Pattern struct {
	source string
	re     *regexp.Regexp
}

// String returns the pattern as it was written, for error messages.
func (p *Pattern) String() string { return p.source }

// Compile turns a hold pattern into a matcher.
//
// The syntax is deliberately small:
//
//	{id}  the plan id, one run of digits
//	*     any run of characters except a slash
//	**    any run of characters, slashes included
//
// Everything else is literal, so a dot is a dot rather than "any
// character". The match is anchored at both ends: `plan/{id}-*` must
// not accept `backup/plan/123-x`, or every archived branch would read
// as an active claim.
func Compile(pattern string) (*Pattern, error) {
	if pattern == "" {
		return nil, errors.New("pattern is empty")
	}
	if strings.Count(pattern, idToken) != 1 {
		return nil, fmt.Errorf("%q: %w", pattern, ErrNoID)
	}

	// Exactly one {id} is guaranteed above, so the pattern splits
	// into what precedes the id and what follows it.
	before, after, _ := strings.Cut(pattern, idToken)
	expr := "^" + expandWildcards(before) +
		`([0-9]+)` + expandWildcards(after) + "$"

	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", pattern, err)
	}

	return &Pattern{source: pattern, re: re}, nil
}

// expandWildcards regexp-quotes a literal segment, then reinstates the
// two wildcards. Quoting first is what keeps a dot literal.
func expandWildcards(segment string) string {
	quoted := regexp.QuoteMeta(segment)
	// QuoteMeta turns * into \*, so the doublestar is \*\*.
	quoted = strings.ReplaceAll(quoted, `\*\*`, "\x00")
	quoted = strings.ReplaceAll(quoted, `\*`, `[^/]*`)
	quoted = strings.ReplaceAll(quoted, "\x00", `.*`)

	return quoted
}

// Match reports the plan id this branch claims, if it claims one.
//
// An id too large for an int64 is not treated as a match: it cannot be
// a plan id, and parsing it would otherwise wrap silently.
func (p *Pattern) Match(branch string) (int64, bool) {
	m := p.re.FindStringSubmatch(branch)
	if m == nil {
		return 0, false
	}

	id, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}

// Holds is a repository's full set of hold patterns.
//
// A list rather than a single pattern because conventions decorate the
// id freely: one repository can carry `plan/<id>-<slug>`,
// `owner/plan-<id>-<slug>` and a bare `plan-<id>` at the same time,
// and a single pattern would see only a fraction of its lanes.
type Holds []*Pattern

// CompileAll compiles every pattern, reporting which one is at fault.
func CompileAll(patterns []string) (Holds, error) {
	out := make(Holds, 0, len(patterns))
	for _, raw := range patterns {
		p, err := Compile(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	return out, nil
}

// Match returns the plan id claimed by this branch, trying each
// pattern in order. An empty Holds matches nothing, which is the
// honest answer for a repository that declares no convention.
func (h Holds) Match(branch string) (int64, bool) {
	for _, p := range h {
		if id, ok := p.Match(branch); ok {
			return id, true
		}
	}

	return 0, false
}
