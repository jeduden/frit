package herdr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner runs a herdr subcommand and returns its stdout.
//
// It is a function type rather than an interface for the same reason
// gitwt.Runner is: tests fake the socket with a closure, and the real
// implementation stays one function with no state to construct.
type Runner func(args ...string) ([]byte, error)

// ContextRunner is the context-aware form of Runner: a bound handed in
// through ctx, rather than baked into the closure, so
// exec.CommandContext can kill the subprocess when the caller's
// context is done. WithTimeout wraps one of these and hands back a
// plain Runner, mirroring gitwt.ContextRunner.
type ContextRunner func(ctx context.Context, args ...string) ([]byte, error)

// waitDelay bounds how long runContext waits for stdout/stderr to hit
// EOF once the child has been killed or has exited, the same fix
// gitwt's core needed: killing the direct child does not close pipe
// descriptors a grandchild — ssh, in the remote-host case — inherited
// and kept open.
const waitDelay = 5 * time.Second

// runContext is the shared exec core: it runs name with args under
// ctx. exec.CommandContext kills the child the moment ctx is done, so
// Run cannot return before that kill completes — the bound moves from
// the caller's wait into the process itself.
func runContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = waitDelay

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("herdr %s: %w",
				strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("herdr %s: %w: %s",
			strings.Join(args, " "), err, msg)
	}

	return stdout.Bytes(), nil
}

// ExecContext is the context-aware form of Exec.
func ExecContext(ctx context.Context, args ...string) ([]byte, error) {
	return runContext(ctx, "herdr", args...)
}

// Exec is the Runner that shells out to the local herdr binary. It is
// ExecContext against a context that is never cancelled, so any
// caller still on the plain Runner shape is unaffected.
func Exec(args ...string) ([]byte, error) {
	return ExecContext(context.Background(), args...)
}

// RunContext is the context-aware form of Run.
func RunContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	return runContext(ctx, name, args...)
}

// Run is the ExecFunc that shells out to an arbitrary process, the seam
// ListHosts fans out over in production: the local herdr binary for the
// local host, and `ssh <host> herdr …` for a remote one. It is
// RunContext against a context that is never cancelled, so any caller
// still on the plain ExecFunc shape is unaffected.
func Run(name string, args ...string) ([]byte, error) {
	return RunContext(context.Background(), name, args...)
}

// WithTimeout bounds a context-aware Runner so a stalled herdr call
// fails fast instead of hanging the verb that made it, the way
// gitwt.WithTimeout bounds a git Runner. Every single-host herdr read
// — board, who, and the held-plan presence check release and claim
// take — goes through rt.herdr, so wrapping it once at the dispatch
// seam bounds them all.
//
// It builds a context bounded by d and calls the context-aware base
// through it, so exec.CommandContext kills the underlying herdr
// subprocess the moment the bound fires rather than leaving it to run
// on after Runner returns.
func WithTimeout(run ContextRunner, d time.Duration) Runner {
	return func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()

		out, err := run(ctx, args...)
		if err != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("herdr %s: timed out after %s",
				strings.Join(args, " "), d)
		}
		return out, err
	}
}

// List reads the live panes from a herdr server.
//
// A failing Runner is returned as-is rather than swallowed: the caller
// decides whether an unreachable socket is fatal. For a read-only
// board it is not — presence is simply absent and the git answer
// stands — but that call is the command's to make, not this function's.
func List(runner Runner) ([]Pane, error) {
	out, err := runner("agent", "list")
	if err != nil {
		return nil, err
	}

	return ParseAgentList(out)
}

// Missing reports a herdr that is not installed at all: the probe's
// call failed because the executable is not on $PATH, the one answer
// that makes every later herdr call certain to fail too. A verb about
// to push a lease it would only have to unwind asks this first, so a
// headless host refuses locally rather than writing to origin twice.
//
// Any other failure — a socket that refuses a dial, a timeout — is not
// proof of absence and answers nil: the server may yet come up, and
// the lease protocol's own unwind already covers it (S60).
func Missing(runner Runner) error {
	_, err := runner("agent", "list")

	return NotFound(err)
}

// NotFound is Missing's reading of an error some herdr call already
// returned, for a caller that has asked herdr once and need not ask
// again: err itself when exec could not run the binary at all — not on
// $PATH, or found only through a relative entry exec refuses to run —
// and nil for any other answer, a failure or none.
func NotFound(err error) error {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, exec.ErrDot) {
		return err
	}

	return nil
}
