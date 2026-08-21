package discovery

import (
	"fmt"
	"time"
)

// The staleness defaults, until `.frit.yml` carries them per repo. The
// window T is how long a work ref's tip must sit unchanged before its
// lease reads as stale — longer than any legitimate quiet stretch of a
// working lane. The sample-gap bound S_max is T/4, so a matured window
// holds at least five samples and an observer that slept cannot mature
// one from two far-apart looks. T affects cost, never correctness: a
// wrong takeover is CAS-safe.
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
	if w.First.IsZero() {
		return 0
	}

	return w.Last.Sub(w.First)
}

// Observe folds one look at the tip into the window.
//
// The same tip within S_max of the last look extends the window; a
// different tip is progress and restarts it; a gap over S_max voids it
// — the observer slept, or the fleet went unread — and the restarted
// window records why, so a takeover that did not fire can be
// explained. Absent state (the zero Window) reads as first seen now,
// which only ever delays a takeover.
func Observe(w Window, tip string, now time.Time, sMax time.Duration) Window {
	fresh := Window{Tip: tip, First: now, Last: now, Samples: 1}
	if w.Tip == "" || w.Tip != tip {
		return fresh
	}
	if gap := now.Sub(w.Last); gap > sMax {
		fresh.Voided = fmt.Sprintf(
			"window restarted: a %s gap between samples exceeded the %s bound",
			gap.Round(time.Second), sMax)

		return fresh
	}

	w.Last = now
	w.Samples++

	return w
}

// StaleHold reports whether the window shows a matured stale lease:
// one unchanged tip observed for more than t, with the latest look
// still within sMax of now — a window gone dark is not acted on, the
// same rule that voids it on the next Observe.
func StaleHold(w Window, now time.Time, t, sMax time.Duration) bool {
	if w.Tip == "" {
		return false
	}
	if now.Sub(w.Last) > sMax {
		return false
	}

	return w.Span() > t
}
