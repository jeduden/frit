// Package plans reads plan files out of every ref in a repository,
// without checking any of them out.
package plans

import (
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
)

// DefaultDir is where plan files live by convention.
const DefaultDir = "plan"

// FixedName is a plan folder's one fixed inner file, the folder
// counterpart to a flat plan/<id>_<slug>.md file.
const FixedName = "plan.md"

// IsFolderPlanFile reports whether a path's final segment names a
// folder plan's fixed inner file, as opposed to a flat plan file. It
// is the one predicate discovery, lane naming and doctor share for
// that question, so the three cannot silently drift on what counts as
// a folder plan — discovery still owns depth (isPlanPath below), the
// question this answers alone. filepath.Base is used rather than
// path.Base so a caller may pass either a git-relative path (always
// "/") or an OS path (native separators; "/" is a folder plan's
// contract too on Windows, filepath.Base already recognizes both).
func IsFolderPlanFile(p string) bool {
	return filepath.Base(p) == FixedName
}

// mislaidPlan matches a dropped file's base name against the same
// <id>_<slug>.md shape a flat plan carries, so a plan-like file left
// in the wrong place is reported rather than silently lost.
var mislaidPlan = regexp.MustCompile(`^[0-9].*_.*\.md$`)

// mislaidFolderPlan matches a dropped plan.md's parent directory name
// against the same <id>_<slug> shape a plan folder carries. A folder
// plan's own base name is always the fixed plan.md, which carries no
// id, so a folder nested deeper than one level is invisible to
// mislaidPlan unless its parent name is checked instead.
var mislaidFolderPlan = regexp.MustCompile(`^[0-9].*_.*$`)

// File is one plan file as it exists on one ref.
//
// The same plan usually appears on many refs. Content is shared
// between them rather than copied: identical files resolve to one
// git object, and the byte slice is that object read once.
type File struct {
	// Ref is the full ref name the file was read from.
	Ref string
	// Path is the repository-relative path of the file.
	Path string
	// OID is the blob's object id, and is equal for identical
	// content across refs.
	OID string
	// Content is the file's bytes.
	Content []byte
}

// Short is the ref name without its category prefix.
func (f File) Short() string {
	return gitobj.Ref{Name: f.Ref}.Short()
}

// Collect reads every *.md file under subdir, on every ref.
//
// The walk costs one git process for the refs, one to resolve a tree
// per ref, one per distinct tree, and one for all the blobs — not
// one checkout per ref. On a repository with 313 refs sharing a
// handful of plan directories, that is a few processes rather than
// a few hundred working trees.
func Collect(
	dir, subdir string, run gitwt.Runner, pipe gitwt.PipeRunner,
) ([]File, []string, error) {
	refs, err := gitobj.Refs(dir, run)
	if err != nil {
		return nil, nil, err
	}

	trees, err := gitobj.TreeOIDs(dir, subdir, refs, pipe)
	if err != nil {
		return nil, nil, err
	}

	entries, ignored, err := entriesByTree(dir, subdir, trees, run)
	if err != nil {
		return nil, nil, err
	}

	blobs, err := gitobj.Blobs(dir, blobOIDs(entries), pipe)
	if err != nil {
		return nil, nil, err
	}

	return assemble(refs, trees, entries, blobs), ignored, nil
}

// entriesByTree lists each distinct tree exactly once, and collects
// every mislaid plan-like path across all of them, deduplicated and
// sorted so the report is stable.
func entriesByTree(
	dir, subdir string, trees map[string]string, run gitwt.Runner,
) (map[string][]gitobj.TreeEntry, []string, error) {
	out := make(map[string][]gitobj.TreeEntry)
	seen := map[string]bool{}
	var ignored []string

	for _, tree := range trees {
		if _, done := out[tree]; done {
			continue
		}
		listed, err := gitobj.TreeEntries(dir, tree, run)
		if err != nil {
			return nil, nil, err
		}
		kept, mislaid := markdownOnly(subdir, listed)
		out[tree] = kept
		for _, p := range mislaid {
			if seen[p] {
				continue
			}
			seen[p] = true
			ignored = append(ignored, p)
		}
	}
	sort.Strings(ignored)

	return out, ignored, nil
}

// markdownOnly keeps the blobs that are plan files and restores
// their repository-relative path: a flat plan/*.md file, or a
// folder's one fixed plan.md one level deep. Everything else beneath
// the plan directory is not a plan; among what is dropped, a path
// whose base still looks like a plan file name — or whose base is the
// fixed plan.md sitting under a directory that looks like a plan
// folder — is a mislaid plan, returned separately so it is reported
// rather than lost. A folder plan's own base name carries no id, so
// the parent directory is what a too-deep folder plan is recognized
// by.
//
// The tree being listed is the subdirectory's own tree, so ls-tree
// reports "a.md" where the repository holds "plan/a.md", and
// "folder/plan.md" where the repository holds "plan/folder/plan.md".
// Depth is measured on that subdir-relative path, before the prefix
// is rejoined, so a nested plan-dir still counts a folder as one
// level deep.
func markdownOnly(
	subdir string, entries []gitobj.TreeEntry,
) ([]gitobj.TreeEntry, []string) {
	kept := make([]gitobj.TreeEntry, 0, len(entries))
	var mislaid []string
	for _, e := range entries {
		if e.Type != "blob" || !strings.HasSuffix(e.Path, ".md") {
			continue
		}
		if isPlanPath(e.Path) {
			e.Path = path.Join(subdir, e.Path)
			kept = append(kept, e)
			continue
		}
		base := path.Base(e.Path)
		lost := mislaidPlan.MatchString(base) ||
			(IsFolderPlanFile(base) &&
				mislaidFolderPlan.MatchString(path.Base(path.Dir(e.Path))))
		if lost {
			mislaid = append(mislaid, path.Join(subdir, e.Path))
		}
	}

	return kept, mislaid
}

// isPlanPath reports whether a subdir-relative path is a flat plan
// (one segment) or a folder plan's fixed plan.md (one folder deep).
func isPlanPath(relPath string) bool {
	segments := strings.Split(relPath, "/")
	switch len(segments) {
	case 1:
		return true
	case 2:
		return IsFolderPlanFile(segments[1])
	default:
		return false
	}
}

// blobOIDs collects the distinct object ids to fetch, sorted so the
// request order is stable and the walk is reproducible.
func blobOIDs(entries map[string][]gitobj.TreeEntry) []string {
	seen := map[string]bool{}
	for _, list := range entries {
		for _, e := range list {
			seen[e.OID] = true
		}
	}

	oids := make([]string, 0, len(seen))
	for oid := range seen {
		oids = append(oids, oid)
	}
	sort.Strings(oids)

	return oids
}

// assemble joins refs to their tree's entries and to blob content,
// in a stable order: by ref name, then by path.
func assemble(
	refs []gitobj.Ref,
	trees map[string]string,
	entries map[string][]gitobj.TreeEntry,
	blobs map[string][]byte,
) []File {
	var out []File

	for _, ref := range refs {
		tree, ok := trees[ref.Name]
		if !ok {
			continue
		}
		for _, e := range entries[tree] {
			content, ok := blobs[e.OID]
			if !ok {
				continue
			}
			out = append(out, File{
				Ref:     ref.Name,
				Path:    e.Path,
				OID:     e.OID,
				Content: content,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Ref != out[j].Ref {
			return out[i].Ref < out[j].Ref
		}
		return out[i].Path < out[j].Path
	})

	return out
}
