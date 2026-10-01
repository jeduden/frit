package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// C15 and C16 run the built frit on a host with no herdr installed —
// the headless shape a cloud agent container has. They register as
// their own section file, per docs/development.md, and read and write
// the command section's own state.
func init() {
	registrars = append(registrars, (*world).registerMissingHerdr)
}

// missingHerdrState is C15 and C16's own: the $PATH the built frit
// runs under, which carries git and nothing named herdr.
type missingHerdrState struct {
	path string
}

func (w *world) registerMissingHerdr(sc *godog.ScenarioContext) {
	sc.Step(`^herdr is not installed on this host$`, w.herdrIsNotInstalledOnThisHost)
	sc.Step(`^the built frit runs "([^"]+)"$`, w.theBuiltFritRuns)
	sc.Step(`^it refuses, naming herdr not found and nothing claimed$`,
		w.itRefusesNamingHerdrNotFoundAndNothingClaimed)
	sc.Step(`^origin carries no work ref for plan (\d+)$`, w.originCarriesNoWorkRefForPlan)
}

// herdrIsNotInstalledOnThisHost builds a $PATH holding git alone, so
// the built frit's own lookup of herdr fails exactly as it does on a
// host that never installed it — no fake runner stands in for the
// exec error the fix reads.
func (w *world) herdrIsNotInstalledOnThisHost() error {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	dir := w.t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(dir, "git")); err != nil {
		return err
	}
	section[missingHerdrState](w).path = dir

	return nil
}

// theBuiltFritRuns runs the built binary as a subprocess from the
// fixture's root, under the herdr-less $PATH, so what is proven is the
// shipped command's behavior rather than an in-process stand-in.
func (w *world) theBuiltFritRuns(args string) error {
	frit, err := builtFrit()
	if err != nil {
		return err
	}
	cs := section[commandState](w)
	cs.out.Reset()
	cs.errb.Reset()
	cmd := exec.Command(frit, strings.Fields(args)...)
	cmd.Dir = filepath.Dir(cs.repo)
	cmd.Env = append(envWithout("FRIT_ROOT", "FRIT_CONFIG", "PATH"),
		"PATH="+section[missingHerdrState](w).path)
	cmd.Stdout = &cs.out
	cmd.Stderr = &cs.errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("frit %s: %w: %s", args, err, cs.errb.String())
	}

	return nil
}

// itRefusesNamingHerdrNotFoundAndNothingClaimed is the shared Then: a
// refusal, in plan terms, that names the missing herdr and says no
// claim was made — never the old "worktree not stood up" unwind.
func (w *world) itRefusesNamingHerdrNotFoundAndNothingClaimed() error {
	got := section[commandState](w).out.String()
	if !strings.Contains(got, "refused: plan") ||
		!strings.Contains(got, "herdr not found; nothing claimed") {
		return fmt.Errorf("expected a missing-herdr refusal, got: %s", got)
	}
	if strings.Contains(got, "worktree not stood up") {
		return fmt.Errorf("the claim was minted and unwound: %s", got)
	}

	return nil
}

// originCarriesNoWorkRefForPlan reads the bare remote directly: the
// refusal happened before the mint, so no claim marker and no release
// marker ever reached it.
func (w *world) originCarriesNoWorkRefForPlan(id string) error {
	cs := section[commandState](w)
	origin, err := gitCapture(w.t, cs.repo, "remote", "get-url", "origin")
	if err != nil {
		return err
	}

	return assertNoOriginBranch(w.t, strings.TrimSpace(origin), id)
}
