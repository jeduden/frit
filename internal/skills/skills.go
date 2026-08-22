// Package skills ships the Claude Code skills that teach an agent to
// drive frit, and lays them into a repository frit works on.
//
// frit already scaffolds one file into a repo — `frit init` writes
// `.frit.yml`. A skill is the same class of act: not the claim, not a
// mutation of any ref, just instructions that travel with the tool so
// the repos frit indexes also carry the knowledge to drive it. The
// skill markdown is embedded in the binary, so a shipped frit needs no
// companion files to install one.
package skills

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrExists reports that a skill file already exists at the target.
//
// Install refuses to clobber it without force: a skill is edited after
// it lands, and overwriting a local change on a stray re-run would be
// a silent loss — the same rule `frit init` holds for `.frit.yml`.
var ErrExists = errors.New("skill file already exists")

// assets holds the bundled skills, one directory per skill, each with
// a SKILL.md. The tree under assets/ is mirrored into .claude/skills/.
//
//go:embed all:assets
var assets embed.FS

// destRoot is the directory, relative to a repository root, that
// Claude Code reads skills from.
var destRoot = filepath.Join(".claude", "skills")

// invokeToken marks a command span in an asset that names how to run
// frit. Install replaces it with the caller's chosen invocation; the
// prose that names frit the tool is never tokened, so it survives
// untouched.
const invokeToken = "{{frit}}"

// Install writes every bundled skill into repoDir under
// .claude/skills/<name>/SKILL.md, mirroring the embedded tree, and
// returns the written paths sorted.
//
// invoke is substituted for every invokeToken in each asset's bytes,
// so the laid-down skill's commands read invoke <verb> instead of the
// token. An empty invoke means bare frit.
//
// It refuses to clobber an existing skill file unless force is set. On
// that refusal it writes nothing further and returns ErrExists, so a
// re-run without force is inert rather than half-applied.
func Install(repoDir string, force bool, invoke string) ([]string, error) {
	if invoke == "" {
		invoke = "frit"
	}

	files, err := bundledFiles()
	if err != nil {
		return nil, err
	}

	if !force {
		for _, rel := range files {
			dst := filepath.Join(repoDir, destRoot, rel)
			if _, err := os.Stat(dst); err == nil {
				return nil, fmt.Errorf("%s: %w", dst, ErrExists)
			} else if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}
	}

	written := make([]string, 0, len(files))
	for _, rel := range files {
		data, err := assets.ReadFile(path("assets", rel))
		if err != nil {
			return nil, err
		}
		data = []byte(strings.ReplaceAll(string(data), invokeToken, invoke))

		dst := filepath.Join(repoDir, destRoot, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return nil, err
		}
		written = append(written, dst)
	}

	sort.Strings(written)

	return written, nil
}

// bundledFiles lists every embedded skill file, as a slash path
// relative to assets/, sorted.
func bundledFiles() ([]string, error) {
	var rels []string
	err := fs.WalkDir(assets, "assets", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("assets", filepath.FromSlash(p))
		if err != nil {
			return err
		}
		rels = append(rels, rel)

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(rels)

	return rels, nil
}

// path joins an embed path, which always uses forward slashes,
// regardless of the host separator.
func path(elem ...string) string {
	joined := ""
	for i, e := range elem {
		if i > 0 {
			joined += "/"
		}
		joined += filepath.ToSlash(e)
	}

	return joined
}
