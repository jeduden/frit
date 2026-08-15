// Package plans reads plan files out of every ref in a repository,
// without checking any of them out.
package plans

import (
	"path"
	"sort"
	"strings"

	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
)

// DefaultDir is where plan files live by convention.
const DefaultDir = "plan"

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
) ([]File, error) {
	refs, err := gitobj.Refs(dir, run)
	if err != nil {
		return nil, err
	}

	trees, err := gitobj.TreeOIDs(dir, subdir, refs, pipe)
	if err != nil {
		return nil, err
	}

	entries, err := entriesByTree(dir, subdir, trees, run)
	if err != nil {
		return nil, err
	}

	blobs, err := gitobj.Blobs(dir, blobOIDs(entries), pipe)
	if err != nil {
		return nil, err
	}

	return assemble(refs, trees, entries, blobs), nil
}

// entriesByTree lists each distinct tree exactly once.
func entriesByTree(
	dir, subdir string, trees map[string]string, run gitwt.Runner,
) (map[string][]gitobj.TreeEntry, error) {
	out := make(map[string][]gitobj.TreeEntry)

	for _, tree := range trees {
		if _, done := out[tree]; done {
			continue
		}
		listed, err := gitobj.TreeEntries(dir, tree, run)
		if err != nil {
			return nil, err
		}
		out[tree] = markdownOnly(subdir, listed)
	}

	return out, nil
}

// markdownOnly keeps the blobs that are plan files and restores
// their repository-relative path.
//
// The tree being listed is the subdirectory's own tree, so ls-tree
// reports "a.md" where the repository holds "plan/a.md". Rejoining
// the prefix here keeps Path meaning the same thing everywhere else
// in frit. Sub-trees and non-markdown attachments are not plans.
func markdownOnly(
	subdir string, entries []gitobj.TreeEntry,
) []gitobj.TreeEntry {
	kept := make([]gitobj.TreeEntry, 0, len(entries))
	for _, e := range entries {
		if e.Type != "blob" || !strings.HasSuffix(e.Path, ".md") {
			continue
		}
		e.Path = path.Join(subdir, e.Path)
		kept = append(kept, e)
	}

	return kept
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
