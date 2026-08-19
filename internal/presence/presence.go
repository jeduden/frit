// Package presence turns a round of multi-host herdr reads into a
// board that survives an unreachable host. herdr.ListHosts fans the
// read out; this package reconciles the results against a per-host
// cache so a dead or slow machine renders its last-known panes with an
// age rather than dropping its lanes or failing the whole command.
//
// It is pure over an injected clock, the same way lanes.Stale is: an
// age is now minus when a snapshot was read, so a test states an exact
// duration instead of racing the wall.
package presence

import (
	"time"

	"github.com/jeduden/frit/internal/herdr"
)

// Snapshot is the last successful read from one host: its panes and
// when they were read, so a later failure can render them with an age.
type Snapshot struct {
	Panes []herdr.Pane `json:"panes"`
	At    time.Time    `json:"at"`
}

// Cache is the last-known snapshot per host. It is reconciled as a
// value rather than mutated in place: Reconcile returns the next cache
// so the storage layer, not the reconciler, decides when to persist.
type Cache map[herdr.Host]Snapshot

// Status is one host's presence after reconciliation. Fresh marks a
// read that just succeeded; a failed read falls back to the cache and
// is not fresh. Seen is true whenever panes exist to show, fresh or
// cached, and false only for a host never read successfully. Age is
// zero for a fresh read and the time since the last good read for a
// stale one.
type Status struct {
	Host  herdr.Host
	Panes []herdr.Pane
	Fresh bool
	Seen  bool
	Age   time.Duration
}

// Reconcile turns fan-out results into per-host presence against a
// prior cache, and returns the presence and the updated cache. A
// successful read is rendered live and refreshes that host's snapshot;
// a failed read never propagates — it falls back to the last-known
// panes with the age since they were read, or an unseen status when the
// cache holds nothing for that host. A failure leaves the prior
// snapshot intact, so one unreachable machine neither blocks the board
// nor forgets what it last saw.
func Reconcile(
	results []herdr.HostResult, prior Cache, now time.Time,
) ([]Status, Cache) {
	next := make(Cache, len(prior))
	for host, snap := range prior {
		next[host] = snap
	}

	statuses := make([]Status, 0, len(results))
	for _, r := range results {
		if r.Err == nil {
			next[r.Host] = Snapshot{Panes: r.Panes, At: now}
			statuses = append(statuses, Status{
				Host:  r.Host,
				Panes: r.Panes,
				Fresh: true,
				Seen:  true,
			})

			continue
		}

		if snap, ok := prior[r.Host]; ok {
			statuses = append(statuses, Status{
				Host:  r.Host,
				Panes: snap.Panes,
				Seen:  true,
				Age:   now.Sub(snap.At),
			})

			continue
		}

		statuses = append(statuses, Status{Host: r.Host})
	}

	return statuses, next
}
