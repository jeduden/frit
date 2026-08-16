package herdr

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runner runs a herdr subcommand and returns its stdout.
//
// It is a function type rather than an interface for the same reason
// gitwt.Runner is: tests fake the socket with a closure, and the real
// implementation stays one function with no state to construct.
type Runner func(args ...string) ([]byte, error)

// Exec is the Runner that shells out to the herdr binary.
func Exec(args ...string) ([]byte, error) {
	return run("herdr", args...)
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
