package discovery

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// The staleness tests state exact times: every observation is an
// offset from t0 on one clock, folded through Observe, and the verdict
// is asked at an explicit now. No sleeps, no wall clock.
var t0 = time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)

const (
	testT    = 2 * time.Hour
	testSMax = 30 * time.Minute
)

// obs is one look at the ref: which tip, at what offset from t0.
type obs struct {
	tip string
	at  time.Duration
}

// fold runs a sequence of observations through Observe.
func fold(seq []obs) Window {
	var w Window
	for _, o := range seq {
		w = Observe(w, o.tip, t0.Add(o.at), testSMax)
	}

	return w
}

// every20m observes one tip every 20 minutes from 0 through span.
func every20m(tip string, span time.Duration) []obs {
	seq := []obs{}
	for at := time.Duration(0); at <= span; at += 20 * time.Minute {
		seq = append(seq, obs{tip: tip, at: at})
	}

	return seq
}

// TestObserveAndStaleHold drives the window rules table-first: a
// matured window is one unchanged tip spanning more than T with every
// gap under S_max; a moved tip resets it, a long gap voids it, and
// absent state starts fresh so nothing fires early.
func TestObserveAndStaleHold(t *testing.T) {
	cases := []struct {
		name      string
		seq       []obs
		staleAt   time.Duration // when StaleHold is asked, offset from t0
		wantStale bool
	}{
		{
			name:      "one unchanged tip spanning more than T is stale",
			seq:       every20m("aaa", 140*time.Minute),
			staleAt:   140 * time.Minute,
			wantStale: true,
		},
		{
			name:      "a span of exactly T has not matured",
			seq:       every20m("aaa", testT),
			staleAt:   testT,
			wantStale: false,
		},
		{
			name: "the tip moving resets the window",
			seq: append(every20m("aaa", 140*time.Minute),
				obs{tip: "bbb", at: 150 * time.Minute}),
			staleAt:   150 * time.Minute,
			wantStale: false,
		},
		{
			name: "a gap over S_max voids the window",
			seq: []obs{
				{tip: "aaa", at: 0},
				{tip: "aaa", at: 20 * time.Minute},
				{tip: "aaa", at: 20*time.Minute + testSMax + time.Minute},
				{tip: "aaa", at: 20*time.Minute + testSMax + 2*time.Minute},
			},
			staleAt:   20*time.Minute + testSMax + 2*time.Minute,
			wantStale: false,
		},
		{
			name:      "lost state starts fresh; nothing fires early",
			seq:       []obs{{tip: "aaa", at: 0}},
			staleAt:   0,
			wantStale: false,
		},
		{
			name:      "a window gone dark is not acted on",
			seq:       every20m("aaa", 140*time.Minute),
			staleAt:   140*time.Minute + testSMax + time.Minute,
			wantStale: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := fold(tc.seq)
			got := StaleHold(w, t0.Add(tc.staleAt), testT, testSMax)
			assert.Equal(t, tc.wantStale, got)
		})
	}
}

// TestObserveGrowsOneWindow pins the fold itself: samples on one tip
// extend the window and count up, and the span is last minus first.
func TestObserveGrowsOneWindow(t *testing.T) {
	w := fold(every20m("aaa", 60*time.Minute))

	assert.Equal(t, "aaa", w.Tip)
	assert.Equal(t, t0, w.First)
	assert.Equal(t, t0.Add(60*time.Minute), w.Last)
	assert.Equal(t, 4, w.Samples)
	assert.Empty(t, w.Voided)
	assert.Equal(t, 60*time.Minute, w.Span())
}

// TestObserveResetOnAMovedTip: a new tip is a live lease — the window
// restarts at the new tip with this look as its first.
func TestObserveResetOnAMovedTip(t *testing.T) {
	w := fold([]obs{
		{tip: "aaa", at: 0},
		{tip: "aaa", at: 20 * time.Minute},
		{tip: "bbb", at: 40 * time.Minute},
	})

	assert.Equal(t, "bbb", w.Tip)
	assert.Equal(t, t0.Add(40*time.Minute), w.First)
	assert.Equal(t, 1, w.Samples)
	assert.Empty(t, w.Voided, "a moved tip is progress, not a fault to explain")
}

// TestObserveVoidsOnAGapAndSaysWhy: a gap over S_max restarts the
// window, and the state records why — so "why did no takeover fire"
// has an answer in the file.
func TestObserveVoidsOnAGapAndSaysWhy(t *testing.T) {
	w := fold([]obs{
		{tip: "aaa", at: 0},
		{tip: "aaa", at: testSMax + time.Minute},
	})

	assert.Equal(t, t0.Add(testSMax+time.Minute), w.First,
		"the window restarted at the late look")
	assert.Equal(t, 1, w.Samples)
	assert.Contains(t, w.Voided, "gap", "the state says why it restarted")
}

// TestStaleDefaults pins the shipped parameters: S_max is T/4, so a
// matured window holds at least five samples.
func TestStaleDefaults(t *testing.T) {
	assert.Equal(t, DefaultTakeoverWindow/4, DefaultSampleGap)
}

// TestPlanDesertedIsHeldDeadAndUnmatured pins the one spelling of the
// deserted reading every acting site shares: held, bound session
// confirmed gone, takeover window not yet matured — and nothing short
// of all three.
func TestPlanDesertedIsHeldDeadAndUnmatured(t *testing.T) {
	deserted := Plan{Held: true, Dead: true}

	assert.True(t, deserted.Deserted())

	unheld := deserted
	unheld.Held = false
	assert.False(t, unheld.Deserted(), "nobody holds it")

	alive := deserted
	alive.Dead = false
	assert.False(t, alive.Deserted(), "its bound session is not confirmed gone")

	matured := deserted
	matured.Stale = true
	assert.False(t, matured.Deserted(), "a matured window is the stale reading's own cell")
}
