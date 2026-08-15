// Package repocfg reads the per-repository settings frit needs to
// index a repository it does not own.
//
// These are distinct from frit's own settings. `--root` says which
// repositories to walk and is answered by flags, the environment and
// the user config; the settings here belong to each indexed
// repository and travel with it, which is why they live in a
// `.frit.yml` committed beside its plans.
package repocfg

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FileName is the per-repository config file.
const FileName = ".frit.yml"

// DefaultPlanDir is where plan files live by convention.
const DefaultPlanDir = "plan"

// DefaultHoldPattern is the canonical claim shape: a branch named for
// the plan it claims. A repository that uses it needs no config at
// all; one that decorates the id differently declares its own.
const DefaultHoldPattern = "plan/{id}-*"

// Config is one repository's settings.
type Config struct {
	// PlanDir is where plan files live, relative to the repository
	// root.
	PlanDir string `yaml:"plan-dir"`
	// Holds are the ref-name patterns that count as a claim on a
	// plan. Order is significant only in that the first match wins.
	Holds []string `yaml:"holds"`
}

// Default is the configuration a repository gets when it ships no
// `.frit.yml` — the canonical convention, so the common case needs no
// file at all.
func Default() Config {
	return Config{
		PlanDir: DefaultPlanDir,
		Holds:   []string{DefaultHoldPattern},
	}
}

// Load reads repoDir/.frit.yml.
//
// A missing file yields Default(), because most repositories follow
// the convention and should not have to say so. A present but
// malformed file is an error: it means someone tried to configure
// something and got it wrong, and silently falling back to defaults
// would hide that.
//
// Fields the file omits keep their default, so a repository can
// override the hold patterns without restating where plans live.
func Load(repoDir string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(filepath.Join(repoDir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}

	var file Config
	if err := yaml.Unmarshal(data, &file); err != nil {
		return Config{}, fmt.Errorf("%s: %w", FileName, err)
	}

	if file.PlanDir != "" {
		cfg.PlanDir = file.PlanDir
	}
	if file.Holds != nil {
		// A declared empty list means "this repository has no holds",
		// which is different from omitting the key.
		cfg.Holds = file.Holds
	}

	return cfg, nil
}

// Compiled compiles the config's hold patterns.
func (c Config) Compiled() (Holds, error) {
	return CompileAll(c.Holds)
}
