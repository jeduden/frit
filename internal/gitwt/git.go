package gitwt

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Runner runs a git subcommand inside dir and returns its stdout.
//
// It is a function type rather than an interface so tests can fake
// git with a closure, and so the real implementation stays a single
// function with no state to construct.
type Runner func(dir string, args ...string) ([]byte, error)

// Exec is the Runner that actually shells out to git.
//
// dir is always passed as `-C <dir>` rather than by setting the
// process working directory: frit walks many repositories in one run,
// and a shared cwd would make the result order-dependent.
func Exec(dir string, args ...string) ([]byte, error) {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("git %s: %w",
				strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, msg)
	}

	return stdout.Bytes(), nil
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

// ExecPipe is the PipeRunner that shells out to git.
func ExecPipe(dir string, stdin []byte, args ...string) ([]byte, error) {
	full := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", full...)
	cmd.Stdin = bytes.NewReader(stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("git %s: %w",
				strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, msg)
	}

	return stdout.Bytes(), nil
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
