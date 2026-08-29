package gitwt

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner runs a git subcommand inside dir and returns its stdout.
//
// It is a function type rather than an interface so tests can fake
// git with a closure, and so the real implementation stays a single
// function with no state to construct.
type Runner func(dir string, args ...string) ([]byte, error)

// ContextRunner is the context-aware form of Runner: a bound handed
// in through ctx, rather than baked into the closure, so
// exec.CommandContext can kill the subprocess when the caller's
// context is done. WithTimeout and WithDeadline wrap one of these and
// hand back a plain Runner.
type ContextRunner func(ctx context.Context, dir string, args ...string) ([]byte, error)

// waitDelay bounds how long runOutput waits for stdout/stderr to hit
// EOF once the child has been killed or has exited. Killing the
// direct child (git) does not close pipe descriptors a grandchild —
// an ssh transport or credential helper — inherited and kept open;
// without a bound, Wait would then block on that orphan instead of on
// git, silently reopening the hang this package exists to close.
const waitDelay = 5 * time.Second

// runOutput runs the already-built cmd and words stdout, stderr and
// the error the way gitwt always has. It is the piece runContext and
// ExecPipeContext share, since a pipe call needs cmd.Stdin set before
// running but otherwise behaves identically.
func runOutput(cmd *exec.Cmd, name string, args []string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = waitDelay

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("%s %s: %w",
				name, strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("%s %s: %w: %s",
			name, strings.Join(args, " "), err, msg)
	}

	return stdout.Bytes(), nil
}

// runContext is the shared exec core: it runs name with args under
// ctx. exec.CommandContext kills the child the moment ctx is done, so
// Run cannot return before that kill completes — the bound moves from
// the caller's wait into the process itself.
func runContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	return runOutput(exec.CommandContext(ctx, name, args...), name, args)
}

// ExecContext is the context-aware form of Exec: dir is always passed
// as `-C <dir>` rather than by setting the process working directory,
// since frit walks many repositories in one run and a shared cwd
// would make the result order-dependent.
func ExecContext(ctx context.Context, dir string, args ...string) ([]byte, error) {
	full := append([]string{"-C", dir}, args...)
	return runContext(ctx, "git", full...)
}

// Exec is the Runner that actually shells out to git. It is
// ExecContext against a context that is never cancelled, so any
// caller still on the plain Runner shape is unaffected.
func Exec(dir string, args ...string) ([]byte, error) {
	return ExecContext(context.Background(), dir, args...)
}

// WithTimeout bounds a Runner so a stalled call fails fast instead of
// hanging the command that made it, mirroring presence.WithTimeout for
// gitwt's own Runner shape.
//
// It builds a context bounded by d and calls the context-aware base
// through it, so exec.CommandContext kills the underlying git
// subprocess the moment the bound fires rather than leaving it to
// run on after Runner returns.
func WithTimeout(run ContextRunner, d time.Duration) Runner {
	return func(dir string, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()

		out, err := run(ctx, dir, args...)
		if err != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("git %s: timed out after %s",
				strings.Join(args, " "), d)
		}
		return out, err
	}
}

// WithTimeoutPipe bounds a context-aware PipeRunner the same way
// WithTimeout bounds a Runner. The batch plumbing this wraps is
// normally local, but a partial clone's promisor remote can pull a
// missing object over the network on demand — the same network bound
// applies here too.
func WithTimeoutPipe(run ContextPipeRunner, d time.Duration) PipeRunner {
	return func(dir string, stdin []byte, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), d)
		defer cancel()

		out, err := run(ctx, dir, stdin, args...)
		if err != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("git %s: timed out after %s",
				strings.Join(args, " "), d)
		}
		return out, err
	}
}

// WithDeadline bounds a context-aware Runner the way WithTimeout
// does, but every call made through the same wrapped Runner shares
// one clock instead of each re-arming a fixed duration. A mutating
// verb like release or claim chains several sequential git calls
// against one remote — the gather's fetch, an ls-remote, the push, a
// retry's ls-remote — and each independently re-armed with the full
// --git-timeout can cost a multiple of it against a fully stalled
// remote. WithDeadline instead spends down one shared budget: a call
// made after it is exhausted returns at once, without starting, and a
// call still running when the deadline fires has its subprocess
// killed by a context built against that same deadline.
func WithDeadline(run ContextRunner, deadline time.Time) Runner {
	return func(dir string, args ...string) ([]byte, error) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf(
				"git %s: git-timeout budget exhausted by an earlier call",
				strings.Join(args, " "))
		}

		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		out, err := run(ctx, dir, args...)
		if err != nil && ctx.Err() != nil {
			return nil, fmt.Errorf("git %s: timed out after %s",
				strings.Join(args, " "), remaining)
		}
		return out, err
	}
}

// PipeRunner runs a git subcommand with stdin attached and returns
// its stdout.
//
// The batch plumbing (`cat-file --batch`, `--batch-check`) reads its
// request list from stdin, which is the whole point of it: one
// process answers thousands of lookups. Runner cannot express that,
// so it gets its own type rather than growing an unused parameter on
// every ordinary call.
type PipeRunner func(dir string, stdin []byte, args ...string) ([]byte, error)

// ContextPipeRunner is the context-aware form of PipeRunner, the way
// ContextRunner is the context-aware form of Runner.
type ContextPipeRunner func(ctx context.Context, dir string, stdin []byte, args ...string) ([]byte, error)

// ExecPipeContext is the context-aware form of ExecPipe.
func ExecPipeContext(ctx context.Context, dir string, stdin []byte, args ...string) ([]byte, error) {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Stdin = bytes.NewReader(stdin)

	return runOutput(cmd, "git", full)
}

// ExecPipe is the PipeRunner that shells out to git. It is
// ExecPipeContext against a context that is never cancelled, so any
// caller still on the plain PipeRunner shape is unaffected.
func ExecPipe(dir string, stdin []byte, args ...string) ([]byte, error) {
	return ExecPipeContext(context.Background(), dir, stdin, args...)
}

// List returns every worktree of the repository containing dir.
//
// The answer covers the whole repository, not just dir, so calling it
// once per repository is enough — and calling it from a linked
// worktree returns the same set as calling it from the main one.
func List(dir string, run Runner) ([]Worktree, error) {
	out, err := run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	return ParseWorktreeList(out), nil
}

// GitDir returns the worktree's own git directory.
//
// This is not CommonDir: every linked worktree of a repository shares
// the common dir, while --git-dir names the directory belonging to
// this worktree alone (<main>/.git/worktrees/<name>). Per-lane state —
// the lease token — belongs there, so two lanes of one repository
// never overwrite each other's.
func GitDir(dir string, run Runner) (string, error) {
	out, err := run(dir, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// CommonDir returns the repository's shared git directory.
//
// This is the identity frit groups by: every linked worktree of one
// repository reports the same common dir, while two clones of the
// same upstream report different ones. Grouping by remote URL would
// wrongly merge two clones; grouping by path would wrongly split a
// repository's worktrees apart.
func CommonDir(dir string, run Runner) (string, error) {
	out, err := run(dir, "rev-parse", "--path-format=absolute",
		"--git-common-dir")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}
