package herdr

import (
	"bytes"
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

// Exec is the Runner that shells out to the local herdr binary.
func Exec(args ...string) ([]byte, error) {
	return run("herdr", args...)
}

// Run is the ExecFunc that shells out to an arbitrary process, the seam
// ListHosts fans out over in production: the local herdr binary for the
// local host, and `ssh <host> herdr …` for a remote one.
func Run(name string, args ...string) ([]byte, error) {
	return run(name, args...)
}

// run invokes name with args, taking the binary name so a test can
// drive the success and failure paths without a herdr on the machine.
func run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)

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

// WithTimeout bounds a Runner so a stalled herdr call fails fast
// instead of hanging the verb that made it, the way gitwt.WithTimeout
// bounds a git Runner. Every single-host herdr read — board, who, and
// the held-plan presence check release and claim take — goes through
// rt.herdr, so wrapping it once at the dispatch seam bounds them all.
//
// It bounds the wait, not the process: the underlying herdr subprocess
// is not killed and is orphaned when frit exits. The buffered channel
// lets the wrapped call deliver into nothing and exit, so bounding the
// wait does not leak the goroutine once herdr eventually returns.
func WithTimeout(run Runner, d time.Duration) Runner {
	return func(args ...string) ([]byte, error) {
		type reply struct {
			out []byte
			err error
		}

		done := make(chan reply, 1)
		go func() {
			out, err := run(args...)
			done <- reply{out: out, err: err}
		}()

		select {
		case r := <-done:
			return r.out, r.err
		case <-time.After(d):
			return nil, fmt.Errorf("herdr %s: timed out after %s",
				strings.Join(args, " "), d)
		}
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
