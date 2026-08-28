// Package discover walks a directory tree for git repositories and
// groups the worktrees it finds by the repository they belong to.
//
// The walk is deliberately shallow-stopping: once a directory is
// recognised as a working tree, its subtree is not descended. A
// repository's own files are never other repositories worth
// indexing, and vendored checkouts are noise on a fleet board.
package discover

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jeduden/frit/internal/gitwt"
)

// Skipped is a repository candidate git refused to answer for — its
// directory and the error CommonDir or List returned — so a caller
// that keeps a diagnostic channel can name it instead of letting it
// vanish from every command's output with nothing said about why.
type Skipped struct {
	Dir string
	Err error
}

// Repo is one git repository and every worktree attached to it.
type Repo struct {
	// Name is the main worktree's directory basename.
	Name string
	// Path is the main worktree's absolute path.
	Path string
	// CommonDir is the shared git directory, and the identity the
	// grouping is keyed on.
	CommonDir string
	// Worktrees is every worktree of this repository, including ones
	// that live outside the walked root.
	Worktrees []gitwt.Worktree
}

// skipDirs are never descended into. They cannot contain a
// repository worth indexing, and node_modules in particular can hold
// thousands of directories that would dominate the walk.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"zig-cache":    true,
	"target":       true,
}

// Repos finds every git repository under root.
//
// Grouping is by git common directory, so the many sibling
// worktrees of one repository collapse into a single Repo rather
// than appearing as dozens of unrelated checkouts. A candidate that
// git refuses to answer for does not fail the walk: it is reported
// back in skipped instead, so one broken checkout does not blind the
// whole board but is not silently unaccounted for either.
func Repos(root string, run gitwt.Runner) ([]Repo, []Skipped, error) {
	candidates, err := findWorkTrees(root)
	if err != nil {
		return nil, nil, err
	}

	seen := make(map[string]bool, len(candidates))
	out := make([]Repo, 0, len(candidates))
	skipped := make([]Skipped, 0)

	for _, dir := range candidates {
		common, err := gitwt.CommonDir(dir, run)
		if err != nil {
			skipped = append(skipped, Skipped{Dir: dir, Err: err})
			continue
		}
		if seen[common] {
			continue
		}
		seen[common] = true

		worktrees, err := gitwt.List(dir, run)
		if err != nil {
			skipped = append(skipped, Skipped{Dir: dir, Err: err})
			continue
		}
		if len(worktrees) == 0 {
			continue
		}

		main := worktrees[0]
		out = append(out, Repo{
			Name:      filepath.Base(main.Path),
			Path:      main.Path,
			CommonDir: common,
			Worktrees: worktrees,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Path < out[j].Path
	})

	return out, skipped, nil
}

// findWorkTrees returns every directory under root that carries a
// .git entry, in lexical order and without descending into any of
// them.
func findWorkTrees(root string) ([]string, error) {
	var found []string

	err := filepath.WalkDir(root, func(
		path string, d fs.DirEntry, err error,
	) error {
		if err != nil {
			// A root that cannot be read is a caller mistake and is
			// reported. Anything deeper is skipped rather than fatal:
			// a board that dies on one bad permission is useless.
			if path == root {
				return err
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && skipDir(d.Name()) {
			return fs.SkipDir
		}
		if isWorkTree(path) {
			found = append(found, path)
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return found, nil
}

// skipDir reports whether a directory name is never worth entering.
func skipDir(name string) bool {
	if skipDirs[name] {
		return true
	}

	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

// isWorkTree reports whether dir carries a .git entry. It is a file
// for a linked worktree and a directory for the main one, so the
// entry's kind is deliberately not checked.
func isWorkTree(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, ".git"))

	return err == nil
}
