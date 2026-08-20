// Package scaffold writes the plan machinery frit's workflow assumes
// into a repository: the plan/proto.md schema today, the PLAN.md index
// next.
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

// ErrExists reports that a target file already exists. WriteProto
// refuses to clobber it without force: proto.md is edited after it
// lands, and a stray re-run must not silently lose that edit — the same
// rule `frit init` holds for .frit.yml.
var ErrExists = errors.New("proto.md already exists")

// protoSchema is the canonical plan schema, embedded so a shipped frit
// carries it with no companion file. TestShippedProtoMatchesRepo pins
// it equal to this repo's plan/proto.md, so the copy frit ships cannot
// drift from the schema frit itself lints plans against.
//
//go:embed assets/proto.md
var protoSchema []byte

// WriteProto writes the embedded proto.md schema into planDir, creating
// the directory if it does not exist, and returns the written path.
//
// It refuses to clobber an existing proto.md unless force is set, and on
// that refusal writes nothing.
func WriteProto(planDir string, force bool) (string, error) {
	path := filepath.Join(planDir, planmeta.ProtoName)

	if !force {
		_, err := os.Stat(path)
		if err == nil {
			return "", fmt.Errorf("%s: %w", path, ErrExists)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}

	if err := os.MkdirAll(planDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, protoSchema, 0o644); err != nil {
		return "", err
	}

	return path, nil
}
