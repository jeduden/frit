// Package scaffold writes the plan machinery frit's workflow assumes
// into a repository: the plan/proto.md schema, a default .mdsmith.yml,
// and a PLAN.md catalog seed.
//
// It is the same class of act as `frit init` writing .frit.yml and
// `frit skills` installing skills — a shipped default embedded in the
// binary, written once, refusing to clobber an edit without force. None
// of it is the claim, so none of it mutates a ref.
package scaffold

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jeduden/frit/internal/planmeta"
)

// ErrExists reports that a target file already exists. The writers
// refuse to clobber one without force: a scaffolded file is edited after
// it lands, and a stray re-run must not silently lose that edit — the
// same rule `frit init` holds for .frit.yml.
var ErrExists = errors.New("file already exists")

// The files a repository root carries, beside .frit.yml, once its
// mdsmith machinery is scaffolded.
const (
	mdsmithConfigName = ".mdsmith.yml"
	planIndexName     = "PLAN.md"
)

// protoSchema is the canonical plan schema. TestShippedProtoMatchesRepo
// pins it equal to this repo's plan/proto.md, so the copy frit ships
// cannot drift from the schema frit itself lints plans against.
//
//go:embed assets/proto.md
var protoSchema []byte

// mdsmithConfig is a default linter configuration for a repository using
// frit's plan workflow: enough to lint proto.md, plan files and the
// PLAN.md catalog. Without it a fresh repo cannot lint proto.md at all.
//
//go:embed assets/mdsmith.yml
var mdsmithConfig []byte

// planIndex is the empty PLAN.md catalog mdsmith renders for a repo with
// no plans yet — the seed it fills and maintains as plans accrue.
//
//go:embed assets/PLAN.md
var planIndex []byte

// WriteProto writes the embedded proto.md schema into planDir, creating
// the directory if it does not exist, and returns the written path.
//
// It refuses to clobber an existing proto.md unless force is set.
func WriteProto(planDir string, force bool) (string, error) {
	return writeAsset(protoSchema,
		filepath.Join(planDir, planmeta.ProtoName), force)
}

// WriteMdsmithConfig writes the embedded default .mdsmith.yml into
// repoDir and returns the written path, refusing to clobber an existing
// one unless force is set.
func WriteMdsmithConfig(repoDir string, force bool) (string, error) {
	return writeAsset(mdsmithConfig,
		filepath.Join(repoDir, mdsmithConfigName), force)
}

// WritePlanIndex writes the embedded PLAN.md catalog seed into repoDir
// and returns the written path, refusing to clobber an existing one
// unless force is set.
func WritePlanIndex(repoDir string, force bool) (string, error) {
	return writeAsset(planIndex,
		filepath.Join(repoDir, planIndexName), force)
}

// writeAsset writes content to path, creating the parent directory if
// absent, and returns path. It refuses to clobber an existing file
// unless force is set, and on that refusal writes nothing.
func writeAsset(content []byte, path string, force bool) (string, error) {
	if !force {
		_, err := os.Stat(path)
		if err == nil {
			return "", fmt.Errorf("%s: %w", path, ErrExists)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}

	return path, nil
}
