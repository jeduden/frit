package repocfg

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrExists reports that a repository already carries a config file.
var ErrExists = errors.New(FileName + " already exists")

// Template is the file `frit init` writes: every setting present with
// its default value, and a comment explaining what it does.
//
// The defaults are written out rather than left commented, so the
// file answers "what is frit doing here" without a second lookup, and
// so editing it is a change rather than an uncommenting.
const Template = `# frit settings for this repository.
#
# frit reads this file from the repository it indexes, so a project's
# conventions travel with the project rather than living on one
# machine. Every value below is frit's default; delete a key to keep
# the default, or change it to override.

# Where plan files live, relative to the repository root.
plan-dir: ` + DefaultPlanDir + `

# Ref names that count as a claim on a plan — a "hold".
#
#   {id}  the plan id, one run of digits (required, exactly once)
#   *     any run of characters except a slash
#   **    any run of characters, slashes included
#
# Patterns match the branch name with any remote prefix stripped, so
# one entry covers a local branch and its copy on every remote. The
# match is anchored, so backup/plan/123-x is not a claim on plan 123.
# The first pattern that matches wins.
#
# List every shape the project actually uses — conventions decorate
# the id freely, and a ref matching no pattern is simply not a hold:
#
#   holds:
#     - "plan/{id}-*"
#     - "*/plan-{id}-*"
#     - "plan-{id}-*"
#
# An empty list says this repository has no claims at all.
holds:
  - "` + LeaseHoldPattern + `"
  - "` + DefaultHoldPattern + `"

# The git remote a claim lease is pushed to.
remote: ` + DefaultRemote + `

# The ref a claim lease is dated against.
#
# Left unset, base is derived from git through the
# origin/HEAD → main → master → HEAD cascade — so it has no literal
# default and is not written as an active key. Set it only to pin the
# ref the lease is dated against, e.g.
#
# base: origin/main
`

// Init writes the template into repoDir.
//
// It refuses to clobber an existing file unless force is set: the file
// is hand-edited after creation, and overwriting someone's pattern
// list on a stray re-run would be a silent loss.
func Init(repoDir string, force bool) (string, error) {
	path := filepath.Join(repoDir, FileName)

	if !force {
		_, err := os.Stat(path)
		if err == nil {
			return "", fmt.Errorf("%s: %w", path, ErrExists)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}

	if err := os.WriteFile(path, []byte(Template), 0o600); err != nil {
		return "", err
	}

	return path, nil
}
