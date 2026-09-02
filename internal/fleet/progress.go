package fleet

import "time"

// Reporter is how Gather tells a caller what it is doing as it walks
// the fleet. Gather takes one as a required argument and emits into it
// unconditionally — a Start before the walk, a Repo per repository, a
// Done at the end — so no verb can gather in silence. Whether the
// events are rendered is the reporter's own concern: the CLI wires one
// that prints to stderr, a test wires DiscardReporter.
type Reporter interface {
	// Start opens the walk, naming how many repositories were
	// discovered under the root.
	Start(repos int)
	// Repo announces the repository about to be read, one-based index
	// of total, so a caller can show "3 of 12" as the walk advances.
	Repo(name string, index, total int)
	// Done closes the walk with the status the gather summarised.
	Done(Summary)
}

// Summary is the status of one gather: what the walk covered. It is
// returned on every Result by construction, so a caller cannot hold a
// gathered fleet without knowing how much of the fleet it reflects.
type Summary struct {
	// Discovered is every repository found under the root, whether or
	// not it could then be read.
	Discovered int
	// Read is the repositories successfully gathered into plans.
	// Discovered minus Read is the repositories a fault stepped over.
	Read int
	// Fetched is the repositories whose remote-tracking refs this walk
	// actually refreshed — zero when the walk ran with fetch off.
	Fetched int
	// Problems is the count of faults met on the walk, the same faults
	// carried on Result.Problems.
	Problems int
	// Elapsed is how long the walk took.
	Elapsed time.Duration
}

// DiscardReporter satisfies Reporter and renders nothing. It is the
// reporter a test or a caller that wants no progress passes, so the
// required argument is never a reason to reach for a nil that Gather
// would have to guard.
type DiscardReporter struct{}

// Start renders nothing.
func (DiscardReporter) Start(int) {}

// Repo renders nothing.
func (DiscardReporter) Repo(string, int, int) {}

// Done renders nothing.
func (DiscardReporter) Done(Summary) {}
