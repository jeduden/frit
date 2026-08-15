package report

import "github.com/jeduden/frit/internal/discover"

// ReposDoc is what `frit repos` found: every repository under the
// root, with every worktree attached to it — including worktrees
// living outside the root, which is how a repository's lanes are found
// wherever they were put.
type ReposDoc struct {
	header
	Root  string `json:"root"`
	Repos []Repo `json:"repos"`
}

// Repo is one repository and its checkouts.
type Repo struct {
	Name      string     `json:"name"`
	Path      string     `json:"path"`
	Worktrees []Worktree `json:"worktrees"`
}

// Repos builds the repository listing.
func Repos(root string, repos []discover.Repo) ReposDoc {
	doc := ReposDoc{
		header: newHeader("repos"),
		Root:   root,
		Repos:  make([]Repo, 0, len(repos)),
	}

	for _, r := range repos {
		doc.Repos = append(doc.Repos, Repo{
			Name:      r.Name,
			Path:      r.Path,
			Worktrees: worktreesOf(r.Worktrees),
		})
	}

	return doc
}
