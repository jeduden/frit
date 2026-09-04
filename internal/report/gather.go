package report

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
	return ""
}
