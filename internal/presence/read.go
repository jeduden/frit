package presence

import (
	"time"

	"github.com/jeduden/frit/internal/herdr"
)

// Options tunes a fan-out read. TTL is how long a snapshot is served
// before a re-probe, so a live fleet is not re-read on every
// invocation; Timeout bounds each host so a slow one renders stale
// rather than stalling the board.
type Options struct {
	TTL     time.Duration
	Timeout time.Duration
}

// Read fans presence out over hosts and reconciles the result against
// the cache at path. A remote host whose snapshot is younger than the
// TTL is served from cache without a re-probe; the local host — the
// empty Host — is always probed, having no round-trip to spend. Every
// host's panes, fresh reads and stale fallbacks and cache hits alike,
// are returned as one flat list so a caller wanting the union is
// unchanged, while the per-host statuses carry the staleness a caller
// can surface. A failed read never propagates, and a cache-write
// failure is swallowed: neither one may block the board.
func Read(
	hosts []herdr.Host, exec herdr.ExecFunc,
	path string, opt Options, now time.Time,
) ([]herdr.Pane, []Status) {
	cache := Load(path)

	toProbe := make([]herdr.Host, 0, len(hosts))
	skipped := make([]herdr.Host, 0, len(hosts))
	for _, host := range hosts {
		if host != "" && cache.Fresh(host, opt.TTL, now) {
			skipped = append(skipped, host)

			continue
		}
		toProbe = append(toProbe, host)
	}

	results := herdr.ListHosts(toProbe, WithTimeout(exec, opt.Timeout))
	statuses, next := Reconcile(results, cache, now)

	// Drop any host no longer in the roster, so a machine dropped from
	// the config does not linger in the cache across host churn.
	wanted := make(map[herdr.Host]bool, len(hosts))
	for _, host := range hosts {
		wanted[host] = true
	}
	for host := range next {
		if !wanted[host] {
			delete(next, host)
		}
	}

	for _, host := range skipped {
		snap := cache[host]
		statuses = append(statuses, Status{
			Host:  host,
			Panes: snap.Panes,
			Seen:  true,
			Age:   now.Sub(snap.At),
		})
	}

	// Best effort: a cache the board could not persist is next run's
	// cold start, not this run's failure.
	_ = Store(path, next)

	panes := []herdr.Pane{}
	for _, s := range statuses {
		panes = append(panes, s.Panes...)
	}

	return panes, statuses
}
