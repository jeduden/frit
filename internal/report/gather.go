package report

import "fmt"

// Gather is how much of the fleet a gathering verb's walk covered, so a
// consumer — reading --json or the table — can tell a complete answer
// from a partial one rather than inferring coverage from the plans it
// happens to see. Every count is always present, even at zero, per the
// JSON contract.
//
// Elapsed is carried in milliseconds under a fixed key rather than as a
// duration string, so the shape is stable for a consumer and
// deterministic for a golden test.
type Gather struct {
	Discovered int   `json:"discovered"`
	Read       int   `json:"read"`
	Fetched    int   `json:"fetched"`
	Problems   int   `json:"problems"`
	ElapsedMS  int64 `json:"elapsed_ms"`
}

// StatusLine renders the gather status as the one line the table shows,
// built from the same struct the JSON carries so the two renderings
// cannot drift apart.
func (g Gather) StatusLine() string {
	return fmt.Sprintf(
		"gathered %d/%d repositories, %d fetched, %d problem(s), in %dms",
		g.Read, g.Discovered, g.Fetched, g.Problems, g.ElapsedMS)
}

// gathered embeds into every document a gathering verb produces, so the
// coverage summary rides on all of them from one definition and a
// consumer reads it the same way on each. The zero value carries every
// key at zero, so a document that has not recorded a walk still honors
// the contract.
type gathered struct {
	Gather Gather `json:"gather"`
}

// SetGather records how much of the fleet the walk covered. It is
// promoted onto every gathering document, so the projection reads the
// same at each call site.
func (g *gathered) SetGather(s Gather) {
	g.Gather = s
}
