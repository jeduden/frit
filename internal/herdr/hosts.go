package herdr

import (
	"context"
	"sync"
)

// ExecFunc runs a process and returns its stdout. It is the seam
// ListHosts fans out over and WithTimeout wraps: run in production, a
// fake in tests, or a timeout-bounded run when a slow host must not
// stall the board.
type ExecFunc func(name string, args ...string) ([]byte, error)

// ContextExecFunc is the context-aware form of ExecFunc, the way
// ContextRunner is the context-aware form of Runner. RunContext
// satisfies it directly, so presence.WithTimeout can wrap the real
// exec.CommandContext-backed core rather than an adapter around it.
type ContextExecFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

// Host is where a herdr socket lives. The empty Host is this machine,
// read through the local herdr binary; any other value is an ssh
// target reached as `ssh <host> herdr …`. The plan key already carries
// the host dimension, so growing the fleet is adding hosts, not a
// migration.
type Host string

// HostResult ties one host's panes, or the error reaching it, back to
// the host. A fanned-out read keeps the host dimension rather than
// flattening every pane into one list, so the board can name where a
// pane lives and an unreachable host stays a host with an error rather
// than a silent gap.
type HostResult struct {
	Host  Host
	Panes []Pane
	Err   error
}

// command builds the argv that reaches a host's herdr. The local host
// is the herdr binary with the bare args; a named host is that same
// call wrapped in `ssh <host> herdr …`, which is the read this fan-out
// runs against every remote.
func command(host Host, args []string) (name string, argv []string) {
	if host == "" {
		return "herdr", args
	}

	return "ssh", append([]string{string(host), "herdr"}, args...)
}

// ListHosts reads panes from every host concurrently, one goroutine per
// host, because a serial walk would pay every host's latency in turn.
// exec is the process runner — run in production, a fake in tests — and
// each host's own error is kept in its own result so one unreachable
// machine does not fail the read for the rest.
func ListHosts(hosts []Host, exec ExecFunc) []HostResult {
	results := make([]HostResult, len(hosts))

	var wg sync.WaitGroup
	for i, host := range hosts {
		wg.Add(1)
		go func(i int, host Host) {
			defer wg.Done()

			runner := func(args ...string) ([]byte, error) {
				name, argv := command(host, args)

				return exec(name, argv...)
			}

			panes, err := List(runner)
			for j := range panes {
				panes[j].Host = host
			}
			results[i] = HostResult{Host: host, Panes: panes, Err: err}
		}(i, host)
	}
	wg.Wait()

	return results
}
