package herdr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunReturnsStdout drives the success path with a real external
// command, so the plumbing is exercised without a herdr on the box.
func TestRunReturnsStdout(t *testing.T) {
	out, err := runContext(context.Background(), "echo", "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(out))
}

// TestRunSurfacesAMissingBinary is the unreachable-socket case at its
// root: the binary is not there, and run says so rather than returning
// empty output that would read as "no agents".
func TestRunSurfacesAMissingBinary(t *testing.T) {
	_, err := runContext(context.Background(), "frit-no-such-herdr-binary")
	assert.Error(t, err)
}

// TestRunShellsOutToTheNamedBinary covers the exported seam the
// multi-host fan-out shells ssh through: Run takes the process name, so
// it reaches an arbitrary binary rather than only the local herdr.
func TestRunShellsOutToTheNamedBinary(t *testing.T) {
	out, err := Run("echo", "over-here")
	require.NoError(t, err)
	assert.Equal(t, "over-here\n", string(out))
}

// TestRunSurfacesAFailingProcess: a non-zero exit is an error, the same
// way an unreachable ssh target must be, not silent empty output.
func TestRunSurfacesAFailingProcess(t *testing.T) {
	_, err := Run("false")
	assert.Error(t, err)
}

// TestRunSurfacesStderrAlongsideTheFailure: a failing process that
// also writes to stderr must have that text folded into the error, the
// branch neither TestRunSurfacesAMissingBinary (no process ever ran)
// nor TestRunSurfacesAFailingProcess (a bare exit, no stderr) reaches.
func TestRunSurfacesStderrAlongsideTheFailure(t *testing.T) {
	_, err := runContext(context.Background(),
		"sh", "-c", "echo boom >&2; exit 1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom")
}

// fakeHerdrOnPath puts a throwaway script named herdr first on $PATH,
// so Exec and ExecContext — which hardcode "herdr" as the binary name
// — run a real process without needing a herdr installation or a
// socket, the same way runContext's own tests already stand in real
// binaries (echo, false, sh) for the process this package would
// otherwise shell out to.
func fakeHerdrOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "herdr")
	require.NoError(t, os.WriteFile(
		script, []byte("#!/bin/sh\necho \"$@\"\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestExecShellsOutToTheLocalHerdrBinary drives Exec's real code path
// — through ExecContext and runContext — against a fake herdr on
// $PATH, since a real installation is not something a unit test can
// rely on.
func TestExecShellsOutToTheLocalHerdrBinary(t *testing.T) {
	fakeHerdrOnPath(t)

	out, err := Exec("agent", "list")

	require.NoError(t, err)
	assert.Equal(t, "agent list\n", string(out))
}

// TestExecContextShellsOutToTheLocalHerdrBinary covers ExecContext
// directly: it is the ContextRunner cmd/frit wires as herdrRunner,
// wrapped by WithTimeout, so it is exercised on its own rather than
// only transitively through Exec.
func TestExecContextShellsOutToTheLocalHerdrBinary(t *testing.T) {
	fakeHerdrOnPath(t)

	out, err := ExecContext(context.Background(), "pane", "current")

	require.NoError(t, err)
	assert.Equal(t, "pane current\n", string(out))
}

// TestListParsesWhatTheRunnerReturns joins the Runner seam to the
// parser with a canned response.
func TestListParsesWhatTheRunnerReturns(t *testing.T) {
	runner := func(args ...string) ([]byte, error) {
		assert.Equal(t, []string{"agent", "list"}, args)

		return []byte(`{"result":{"agents":[
			{"agent":"claude","agent_status":"working",
			 "cwd":"/w","pane_id":"w1:p1"}]}}`), nil
	}

	panes, err := List(runner)
	require.NoError(t, err)
	require.Len(t, panes, 1)
	assert.Equal(t, "claude", panes[0].Agent)
}

// TestListReturnsTheRunnerError leaves the fatal-or-not decision to
// the caller by handing the failure straight up.
func TestListReturnsTheRunnerError(t *testing.T) {
	want := errors.New("dial unix: no such file")
	_, err := List(func(...string) ([]byte, error) { return nil, want })
	assert.ErrorIs(t, err, want)
}
