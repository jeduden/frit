package discovery

import "time"

// The staleness defaults, until `.frit.yml` carries them per repo. The
// window T is how long a work ref's tip must sit unchanged before its
// lease reads as stale — longer than any legitimate quiet stretch of a
// working lane. The sample-gap bound S_max is T/4, so a matured window
// holds at least five samples and an observer that slept cannot mature
// one from two far-apart looks.
const (
	DefaultTakeoverWindow = 2 * time.Hour
	DefaultSampleGap      = DefaultTakeoverWindow / 4
)

// Window is one host's observation of a work ref's tip: which tip,
// when this host first and last saw it, and how many looks agreed.
// Staleness is decided from a window and nothing else — one clock, no
// cross-machine timestamps.
type Window struct {
	Tip     string    `json:"tip"`
	First   time.Time `json:"first"`
	Last    time.Time `json:"last"`
	Samples int       `json:"samples"`
	// Voided says why the previous window was thrown away, "" when it
	// was not — so the state answers "why did no takeover fire".
	Voided string `json:"voided,omitempty"`
}

// Span is how long the window has observed one unchanged tip.
func (w Window) Span() time.Duration {
	return 0
}

// Observe folds one look at the tip into the window.
func Observe(w Window, tip string, now time.Time, sMax time.Duration) Window {
	return Window{}
}

// StaleHold reports whether the window shows a matured stale lease.
func StaleHold(w Window, now time.Time, t, sMax time.Duration) bool {
	return false
}
