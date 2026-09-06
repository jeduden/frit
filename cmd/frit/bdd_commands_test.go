package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/report"
)

// The command-scenario vocabulary is single-host: no clone, no second
// machine, no lease race — the shape docs/research/command-scenarios.md
// exists to catalog. It registers itself, like every section's step
// file, so a section adds a file and never a line to bdd_test.go.
func init() {
	registrars = append(registrars, (*world).registerCommands)
}

// commandState holds one command scenario's own state beside the
// shared world: the repository a verb runs against, its CLI output,
// and the commit subject C2 expects drift to name back.
type commandState struct {
	repo    string
	out     bytes.Buffer
	errb    bytes.Buffer
	subject string
}

func (w *world) registerCommands(sc *godog.ScenarioContext) {
	sc.Step(`^a plan nobody has ever held$`, w.aPlanNobodyHasEverHeld)
	sc.Step(`^it is released$`, w.itIsReleased)
	sc.Step(`^the release is a no-op, not a refusal$`, w.theReleaseIsANoOpNotARefusal)
	sc.Step(`^a plan in progress whose work has merged into main$`, w.aPlanInProgressWhoseWorkHasMergedIntoMain)
	sc.Step(`^frit drift is run$`, w.fritDriftIsRun)
	sc.Step(`^drift reports the plan's work has landed$`, w.driftReportsThePlansWorkHasLanded)
	sc.Step(`^drift names the commit that carries the plan's id$`, w.driftNamesTheCommitThatCarriesThePlansID)
	sc.Step(`^it is yielded$`, w.itIsYielded)
	sc.Step(`^yield parks nothing and refuses nothing$`, w.yieldParksNothingAndRefusesNothing)
}

// aPlanNobodyHasEverHeld is C1's own setup: a claimable plan with no
// lease ever minted for it — the state `frit release` sees when
// nothing has ever claimed the plan it names. withHerdr fakes the
// herdr socket so a command reaching for it — as C3's yield does,
// tearing its own pane down — never shells out to a real herdr
// subprocess in this single-host, no-agent fixture.
func (w *world) aPlanNobodyHasEverHeld() error {
	isolate(w.t)
	withHerdr(w.t, herdrReturning())
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

// aPlanInProgressWhoseWorkHasMergedIntoMain is C2's own setup: a plan
// still marked in progress whose hold branch already merged into main
// by an ordinary merge commit — the ancestor-merge signal drift's
// landed check reads — with the plan's own creation commit as the
// only evidence naming its id. The fixture itself is shared with
// drift_test.go's own unit test, so the two layers cannot drift apart
// on what "landed" means.
func (w *world) aPlanInProgressWhoseWorkHasMergedIntoMain() error {
	isolate(w.t)
	w.planID = 100
	root := w.t.TempDir()

	cs := section[commandState](w)
	cs.repo, cs.subject = mergedPlanRepo(w.t, root, w.planID)

	return nil
}

// fritDriftIsRun drives the real `frit drift` CLI with --json, the
// only rendering that carries the commits naming a plan's id — the
// plain table only counts them.
func (w *world) fritDriftIsRun() error {
	cs := section[commandState](w)
	runCLI(&cs.out, &cs.errb, "drift", "--root", filepath.Dir(cs.repo), "--json")

	return nil
}

// driftReportsThePlansWorkHasLanded is C2's first Then: the row for
// the plan built in Given reads landed, read from drift's own --json
// output rather than an internal call.
func (w *world) driftReportsThePlansWorkHasLanded() error {
	row, err := driftRowFor(w)
	if err != nil {
		return err
	}
	if !row.Landed {
		return fmt.Errorf("expected plan %d to read landed, got: %s",
			w.planID, section[commandState](w).out.String())
	}

	return nil
}

// driftNamesTheCommitThatCarriesThePlansID is C2's second Then: the
// commit Given built to name the plan's id is among the commits
// drift's row reports for it.
func (w *world) driftNamesTheCommitThatCarriesThePlansID() error {
	cs := section[commandState](w)
	row, err := driftRowFor(w)
	if err != nil {
		return err
	}
	for _, c := range row.Commits {
		if c.Subject == cs.subject {
			return nil
		}
	}

	return fmt.Errorf("no commit named %q among drift's commits: %v", cs.subject, row.Commits)
}

// driftRowFor decodes the drift row for the world's plan from the
// command's own --json output — the evidence a Then step reads,
// never a re-derivation.
func driftRowFor(w *world) (report.DriftRow, error) {
	cs := section[commandState](w)
	var doc report.DriftDoc
	if err := json.Unmarshal(cs.out.Bytes(), &doc); err != nil {
		return report.DriftRow{}, fmt.Errorf("drift did not emit valid json: %w, got: %s", err, cs.out.String())
	}
	for _, r := range doc.Rows {
		if r.ID == int64(w.planID) {
			return r, nil
		}
	}

	return report.DriftRow{}, fmt.Errorf("no drift row for plan %d in: %s", w.planID, cs.out.String())
}

// itIsYielded is C3's own When: drives the real `frit yield` CLI, the
// same way itIsReleased drives release — the behavior under test is
// the command's own dispatch, never a re-implementation.
func (w *world) itIsYielded() error {
	cs := section[commandState](w)
	runCLI(&cs.out, &cs.errb, "yield", strconv.Itoa(w.planID), "--root", filepath.Dir(cs.repo))

	return nil
}

// yieldParksNothingAndRefusesNothing is C3's own Then: a plan nobody
// holds has nothing of this lane's own to park, so yieldNothingLocal
// reports the clean no-op — no "refused:" line and no "parked:" line,
// read off the command's own output rather than an internal call.
func (w *world) yieldParksNothingAndRefusesNothing() error {
	cs := section[commandState](w)
	got := cs.out.String()
	if !strings.Contains(got, "yielded plan") {
		return fmt.Errorf("expected the command to report yielding, got: %s", got)
	}
	if strings.Contains(got, "refused") {
		return fmt.Errorf("expected nothing refused, got a refusal: %s", got)
	}
	if strings.Contains(got, "parked:") {
		return fmt.Errorf("expected nothing parked: %s", got)
	}

	return nil
}
