package gitobj

import (
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
