package gitobj

import (
	"strconv"
	"strings"

	"github.com/jeduden/frit/internal/gitwt"
)

// refFormat asks for exactly the two fields ParseRefs reads.
const refFormat = "--format=%(objectname) %(refname)"

// Refs lists every ref in the repository — local branches, remote
// tracking branches and tags alike.
//
// The scope is deliberately everything. A plan can sit on a branch
// that was never checked out, never merged, and only ever seen on a
// peer's remote, and those are precisely the lanes a fleet board
// exists to surface.
func Refs(dir string, run gitwt.Runner) ([]Ref, error) {
	out, err := run(dir, "for-each-ref", refFormat)
	if err != nil {
		return nil, err
	}

	return ParseRefs(out), nil
}

// DefaultRef names the ref whose copy of a file is authoritative.
//
// It is emphatically not HEAD. A main worktree is routinely parked
// on a feature branch, so HEAD names whatever was last checked out
// rather than the branch work lands on. That is the common case on
// the machines this targets, not a corner case.
//
// The cascade asks, in order: what does the remote call its default
// branch, then the conventional names, then HEAD as a last resort.
// An empty answer is legitimate and means "no branch is special".
func DefaultRef(dir string, run gitwt.Runner) string {
	if out, err := run(dir, "symbolic-ref", "--quiet",
		"refs/remotes/origin/HEAD"); err == nil {
		if ref := strings.TrimSpace(string(out)); ref != "" {
			return ref
		}
	}

	for _, candidate := range []string{
		"refs/heads/main", "refs/heads/master",
	} {
		if _, err := run(dir, "rev-parse", "--verify", "--quiet",
			candidate); err == nil {
			return candidate
		}
	}

	if out, err := run(dir, "symbolic-ref", "--quiet",
		"HEAD"); err == nil {
		return strings.TrimSpace(string(out))
	}

	return ""
}

// MergedRefs returns every ref already merged into `into`.
//
// Without this, hold detection reports finished work as an active
// claim: landing a plan does not delete its branch, so the ref that
// once meant "I am working this" still exists and still matches the
// pattern. An empty `into` yields an empty set rather than an error —
// a repository with no default branch has nothing to be merged into.
func MergedRefs(
	dir, into string, run gitwt.Runner,
) (map[string]bool, error) {
	if into == "" {
		return map[string]bool{}, nil
	}

	out, err := run(dir, "for-each-ref", "--merged", into,
		"--format=%(refname)")
	if err != nil {
		return nil, err
	}

	merged := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			merged[name] = true
		}
	}

	return merged, nil
}

// RefTimes returns each ref's commit time as a Unix timestamp.
//
// One process answers for every ref, which is what makes an age report
// over hundreds of lanes cheap.
func RefTimes(dir string, run gitwt.Runner) (map[string]int64, error) {
	out, err := run(dir, "for-each-ref",
		"--format=%(refname) %(committerdate:unix)")
	if err != nil {
		return nil, err
	}

	times := map[string]int64{}
	for _, line := range strings.Split(string(out), "\n") {
		name, stamp, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || name == "" {
			continue
		}
		secs, err := strconv.ParseInt(strings.TrimSpace(stamp), 10, 64)
		if err != nil {
			// A tag object with no committerdate reports empty; it is
			// not a lane, so skipping it is correct.
			continue
		}
		times[name] = secs
	}

	return times, nil
}

// TreeOIDs resolves <ref>:<subdir> for every ref in one git process.
//
// The answer is a map from ref name to the tree object holding that
// directory. Refs without the directory are absent from the map
// rather than erroring: most branches in a large repository have no
// plans, and that is not a fault.
//
// This is the step that makes the walk cheap. Resolving one tree per
// ref costs a single `--batch-check`, and because branches sharing a
// plan directory share its tree object, the distinct values are far
// fewer than the refs.
func TreeOIDs(
	dir, subdir string, refs []Ref, pipe gitwt.PipeRunner,
) (map[string]string, error) {
	if len(refs) == 0 {
		return map[string]string{}, nil
	}

	var stdin strings.Builder
	for _, ref := range refs {
		stdin.WriteString(ref.Name)
		stdin.WriteString(":")
		stdin.WriteString(subdir)
		stdin.WriteString("\n")
	}

	out, err := pipe(dir, []byte(stdin.String()),
		"cat-file", "--batch-check")
	if err != nil {
		return nil, err
	}

	checks := ParseBatchCheck(out)
	trees := make(map[string]string, len(checks))
	for i, check := range checks {
		if i >= len(refs) || check.Missing || check.Type != "tree" {
			continue
		}
		trees[refs[i].Name] = check.OID
	}

	return trees, nil
}

// TreeEntries lists a tree recursively.
func TreeEntries(
	dir, tree string, run gitwt.Runner,
) ([]TreeEntry, error) {
	out, err := run(dir, "ls-tree", "-r", tree)
	if err != nil {
		return nil, err
	}

	return ParseLsTree(out), nil
}

// Blobs reads every named object in one git process, returning a map
// from object id to content. Missing objects are omitted.
func Blobs(
	dir string, oids []string, pipe gitwt.PipeRunner,
) (map[string][]byte, error) {
	if len(oids) == 0 {
		return map[string][]byte{}, nil
	}

	var stdin strings.Builder
	for _, oid := range oids {
		stdin.WriteString(oid)
		stdin.WriteString("\n")
	}

	out, err := pipe(dir, []byte(stdin.String()), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}

	objects, err := ParseBatch(out)
	if err != nil {
		return nil, err
	}

	blobs := make(map[string][]byte, len(objects))
	for _, obj := range objects {
		if obj.Missing {
			continue
		}
		blobs[obj.OID] = obj.Data
	}

	return blobs, nil
}
