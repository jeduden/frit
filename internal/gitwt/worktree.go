// Package gitwt reads git worktree state through porcelain formats.
//
// Only porcelain output is parsed here. The human-readable form of
// `git worktree list` aligns columns and elides fields, and neither
// behaviour is a stable contract across git versions; the porcelain
// form is documented as one `key value` pair per line with records
// separated by a blank line, and that is what this file assumes.
package gitwt

import (
	"path/filepath"
	"strings"
)

// zeroSHA is what git reports for a worktree that has no commit —
// created with --no-checkout, or sitting on an unborn branch. Ten of
// the 80 worktrees on the machine this was written for are in that
// state, so it is the common case, not an edge case.
const zeroSHA = "0000000000000000000000000000000000000000"

// Worktree is one record of `git worktree list --porcelain`.
type Worktree struct {
	// Path is the absolute path of the working tree.
	Path string
	// Head is the commit the worktree is on, or zeroSHA when none.
	// Empty for a bare repository, which has no working tree.
	Head string
	// Branch is the short branch name, without the refs/heads/
	// prefix. Empty when the worktree is detached or bare.
	Branch string
	// Detached reports a detached HEAD.
	Detached bool
	// Bare reports the record is the bare repository itself.
	Bare bool
	// Locked reports the worktree is locked against pruning.
	Locked bool
	// LockReason is the operator's reason, empty when none was given.
	LockReason string
	// Prunable reports git considers the worktree removable.
	Prunable bool
	// PruneReason is git's explanation, empty when none was given.
	PruneReason string
}

// HasCommit reports whether the worktree has a commit checked out.
//
// A bare record and an all-zero HEAD both answer false: neither has
// work in it, which is what "is this lane abandoned" turns on.
func (w Worktree) HasCommit() bool {
	return w.Head != "" && w.Head != zeroSHA
}

// Name is the worktree directory's basename, which is how lanes are
// referred to conversationally ("atlas-shader-unit-tests").
func (w Worktree) Name() string {
	return filepath.Base(w.Path)
}

// ParseWorktreeList turns porcelain output into records.
//
// Unknown keys are ignored rather than rejected, so a newer git that
// adds an attribute does not break the walk. A record is flushed on a
// blank line or at the next `worktree` line, so trailing records
// survive output that does not end in a blank line.
func ParseWorktreeList(data []byte) []Worktree {
	var out []Worktree
	var cur *Worktree

	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")
		if line == "" {
			flush()
			continue
		}
		key, val, _ := strings.Cut(line, " ")
		if key == "worktree" {
			flush()
			cur = &Worktree{Path: val}
			continue
		}
		if cur == nil {
			// An attribute before any worktree line: malformed
			// input, not something to guess at.
			continue
		}
		cur.apply(key, val)
	}
	flush()

	return out
}

// apply sets one porcelain attribute on the record under construction.
func (w *Worktree) apply(key, val string) {
	switch key {
	case "HEAD":
		w.Head = val
	case "branch":
		// TrimPrefix removes one occurrence, so a branch literally
		// named refs/heads/odd keeps its own prefix.
		w.Branch = strings.TrimPrefix(val, "refs/heads/")
	case "bare":
		w.Bare = true
	case "detached":
		w.Detached = true
	case "locked":
		w.Locked = true
		w.LockReason = val
	case "prunable":
		w.Prunable = true
		w.PruneReason = val
	}
}
