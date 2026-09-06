package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
)

// The command-scenario vocabulary is single-host: no clone, no second
// machine, no lease race — the shape docs/research/command-scenarios.md
// exists to catalog. It registers itself, like every section's step
// file, so a section adds a file and never a line to bdd_test.go.
func init() {
	registrars = append(registrars, (*world).registerCommands)
}

// commandState holds one command scenario's own state beside the
// shared world: the repository a verb runs against, and its CLI
// output.
type commandState struct {
	repo string
	out  bytes.Buffer
	errb bytes.Buffer
}

func (w *world) registerCommands(sc *godog.ScenarioContext) {
	sc.Step(`^a plan nobody has ever held$`, w.aPlanNobodyHasEverHeld)
	sc.Step(`^it is released$`, w.itIsReleased)
	sc.Step(`^the release is a no-op, not a refusal$`, w.theReleaseIsANoOpNotARefusal)
	sc.Step(`^it is yielded$`, w.itIsYielded)
	sc.Step(`^yield parks nothing and refuses nothing$`, w.yieldParksNothingAndRefusesNothing)
}

// aPlanNobodyHasEverHeld is C1's own setup: a claimable plan with no
// lease ever minted for it — the state `frit release` sees when
// nothing has ever claimed the plan it names.
func (w *world) aPlanNobodyHasEverHeld() error {
	isolate(w.t)
	w.planID = 7
	root := w.t.TempDir()
	cs := section[commandState](w)
	cs.repo = claimableRepo(w.t, root, "atlas", w.planID, "Shader unit")

	return nil
}

// itIsReleased drives the real `frit release` CLI, the same way a
// lease-protocol scenario drives claim or yield: the behavior under
// test is the command's own dispatch, never a re-implementation.
func (w *world) itIsReleased() error {
	cs := section[commandState](w)
	runCLI(&cs.out, &cs.errb, "release", strconv.Itoa(w.planID), "--root", filepath.Dir(cs.repo))

	return nil
}

// theReleaseIsANoOpNotARefusal is C1's own Then: nothing ever held the
// plan, so there is nothing to end — reported as a plain no-op, never
// worded as a refusal.
func (w *world) theReleaseIsANoOpNotARefusal() error {
	got := section[commandState](w).out.String()
	if strings.Contains(got, "refused") {
		return fmt.Errorf("expected a no-op, got a refusal: %s", got)
	}
	if !strings.Contains(got, "nothing") {
		return fmt.Errorf("the output does not read as a no-op: %s", got)
	}

	return nil
}

// itIsYielded is C2's own When: drives the real `frit yield` CLI, the
// same way itIsReleased drives release — the behavior under test is
// the command's own dispatch, never a re-implementation.
func (w *world) itIsYielded() error {
	cs := section[commandState](w)
	runCLI(&cs.out, &cs.errb, "yield", strconv.Itoa(w.planID), "--root", filepath.Dir(cs.repo))

	return nil
}

// yieldParksNothingAndRefusesNothing is C2's own Then: a plan nobody
// holds has nothing of this lane's own to park, so yieldNothingLocal
// reports the clean no-op — no "refused:" line and no "parked:" line,
// read off the command's own output rather than an internal call.
func (w *world) yieldParksNothingAndRefusesNothing() error {
	got := section[commandState](w).out.String()
	if strings.Contains(got, "refused") {
		return fmt.Errorf("expected nothing refused, got a refusal: %s", got)
	}
	if strings.Contains(got, "parked:") {
		return fmt.Errorf("expected nothing parked: %s", got)
	}

	return nil
}
