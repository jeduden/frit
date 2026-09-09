package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/report"
	"github.com/jeduden/frit/internal/skills"
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
// the commit subject C2 and C4 expect drift to name back, and C5's
// second plan id — done, so kept apart from the world's own planID,
// which the mid-flight plan already claims.
type commandState struct {
	repo    string
	out     bytes.Buffer
	errb    bytes.Buffer
	subject string
	doneID  int
	// origin, skillCmd and herdrLog are C7 and C8's own: the bare
	// remote a named start's effect on a ref is read from, the
	// installed skill's own command line with <selector> still open,
	// and the log the fake herdr on $PATH appends every call to.
	origin   string
	skillCmd string
	herdrLog string
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
	sc.Step(`^a multi-phase plan in progress whose last phase's commit is on main$`,
		w.aMultiPhasePlanInProgressWhoseLastPhasesCommitIsOnMain)
	sc.Step(`^drift reports that a commit names the plan's final phase$`,
		w.driftReportsThatACommitNamesThePlansFinalPhase)
	sc.Step(`^a plan mid-flight whose work has not merged into main$`,
		w.aPlanMidFlightWhoseWorkHasNotMergedIntoMain)
	sc.Step(`^a plan already marked done$`, w.aPlanAlreadyMarkedDone)
	sc.Step(`^drift raises nothing for the mid-flight plan$`, w.driftRaisesNothingForTheMidFlightPlan)
	sc.Step(`^drift does not list the done plan$`, w.driftDoesNotListTheDonePlan)
	sc.Step(`^a plan freshly claimed by this lane$`, w.aPlanFreshlyClaimedByThisLane)
	sc.Step(`^yield refuses it, naming release as the way out$`, w.yieldRefusesItNamingReleaseAsTheWayOut)
	sc.Step(`^the bundled plan-start skill is installed with the built frit invocation$`,
		w.theBundledPlanStartSkillIsInstalledWithTheBuiltFritInvocation)
	sc.Step(`^plans 7 and 8 are ready with plan 8 ranked above plan 7$`,
		w.plans7And8AreReadyWithPlan8RankedAbovePlan7)
	sc.Step(`^plan 7 has an unmet dependency while plan 8 is ready$`,
		w.plan7HasAnUnmetDependencyWhilePlan8IsReady)
	sc.Step(`^the installed skill's start command runs for plan (\d+) with --go --json$`,
		w.theInstalledSkillsStartCommandRunsForPlanWithGoJSON)
	sc.Step(`^plan (\d+) gains one lane and one dispatched agent$`,
		w.planGainsOneLaneAndOneDispatchedAgent)
	sc.Step(`^the JSON handoff names plan (\d+) and its pane with prompt_dispatched true$`,
		w.theJSONHandoffNamesPlanAndItsPaneWithPromptDispatchedTrue)
	sc.Step(`^plan (\d+) remains unheld with no agent$`, w.planRemainsUnheldWithNoAgent)
	sc.Step(`^the JSON refusal names plan (\d+) and its unmet dependency$`,
		w.theJSONRefusalNamesPlanAndItsUnmetDependency)
	sc.Step(`^neither plan gains a hold or an agent$`, w.neitherPlanGainsAHoldOrAnAgent)
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

// driftDocFor decodes the whole drift document from the command's own
// --json output, shared by driftRowFor and by any Then step that needs
// to reason over the full row set rather than one plan's row alone.
func driftDocFor(w *world) (report.DriftDoc, error) {
	cs := section[commandState](w)
	var doc report.DriftDoc
	if err := json.Unmarshal(cs.out.Bytes(), &doc); err != nil {
		return report.DriftDoc{}, fmt.Errorf("drift did not emit valid json: %w, got: %s", err, cs.out.String())
	}

	return doc, nil
}

// driftRowFor decodes the drift row for the world's plan from the
// command's own --json output — the evidence a Then step reads,
// never a re-derivation.
func driftRowFor(w *world) (report.DriftRow, error) {
	doc, err := driftDocFor(w)
	if err != nil {
		return report.DriftRow{}, err
	}
	for _, r := range doc.Rows {
		if r.ID == int64(w.planID) {
			return r, nil
		}
	}

	return report.DriftRow{}, fmt.Errorf("no drift row for plan %d in: %s",
		w.planID, section[commandState](w).out.String())
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

// aMultiPhasePlanInProgressWhoseLastPhasesCommitIsOnMain is C4's own
// setup: a plan still marked in progress whose phase ledger names a
// final phase, with a commit naming that phase already on main — the
// namesLastPhase signal drift's phase-level check reads. The fixture
// is shared with drift_test.go's own unit test, so the two layers
// cannot drift apart on what "the last phase landed" means.
func (w *world) aMultiPhasePlanInProgressWhoseLastPhasesCommitIsOnMain() error {
	isolate(w.t)
	w.planID = 800
	root := w.t.TempDir()

	cs := section[commandState](w)
	cs.repo, cs.subject = lastPhasePlanRepo(w.t, root, w.planID)

	return nil
}

// driftReportsThatACommitNamesThePlansFinalPhase is C4's own Then:
// the row for the plan built in Given carries the last-phase flag,
// read from drift's own --json output rather than an internal call.
func (w *world) driftReportsThatACommitNamesThePlansFinalPhase() error {
	row, err := driftRowFor(w)
	if err != nil {
		return err
	}
	if !row.LastPhaseCommit {
		return fmt.Errorf("expected plan %d to name its final phase, got: %s",
			w.planID, section[commandState](w).out.String())
	}

	return nil
}

// aPlanMidFlightWhoseWorkHasNotMergedIntoMain is C5's first Given: a
// plan still marked in progress whose own creation commit is the only
// evidence naming it — no branch merged, no tip content matching main
// — the shape namesLastPhase and the landed check both read as quiet.
func (w *world) aPlanMidFlightWhoseWorkHasNotMergedIntoMain() error {
	isolate(w.t)
	w.planID = 900
	root := w.t.TempDir()

	cs := section[commandState](w)
	cs.repo = initRepo(w.t, root, "atlas")
	commitPlan(w.t, cs.repo, w.planID, "🔳", "Underway", nil, "")

	return nil
}

// aPlanAlreadyMarkedDone is C5's second Given: a second plan, marked
// done, committed into the same repository the mid-flight plan
// already lives in — its id kept in commandState.doneID, apart from
// the world's own planID, so both Then steps read the right row.
func (w *world) aPlanAlreadyMarkedDone() error {
	cs := section[commandState](w)
	cs.doneID = 950
	commitPlan(w.t, cs.repo, cs.doneID, "✅", "Finished", nil, "")

	return nil
}

// driftRaisesNothingForTheMidFlightPlan is C5's first Then: the
// mid-flight plan's row carries neither the landed nor the last-phase
// flag — present, but with nothing alarming to report.
func (w *world) driftRaisesNothingForTheMidFlightPlan() error {
	row, err := driftRowFor(w)
	if err != nil {
		return err
	}
	if row.Landed {
		return fmt.Errorf("expected plan %d to read not landed, got landed: %s",
			w.planID, section[commandState](w).out.String())
	}
	if row.LastPhaseCommit {
		return fmt.Errorf("expected plan %d to name no final phase, got: %s",
			w.planID, section[commandState](w).out.String())
	}

	return nil
}

// driftDoesNotListTheDonePlan is C5's second Then: the done plan's id
// carries no row at all — Unfinished() skips it before drift ever
// walks its evidence, not merely reports it quiet.
func (w *world) driftDoesNotListTheDonePlan() error {
	cs := section[commandState](w)
	doc, err := driftDocFor(w)
	if err != nil {
		return err
	}
	for _, r := range doc.Rows {
		if r.ID == int64(cs.doneID) {
			return fmt.Errorf("expected no drift row for done plan %d, got one: %v", cs.doneID, r)
		}
	}

	return nil
}

// aPlanFreshlyClaimedByThisLane is C6's own setup: this lane claims a
// plan first, the same way TestClaimMintsAPickablePlan does, leaving
// the lease branch at origin's own tip — no divergence yet, the shape
// claim.Yield reads as still held rather than fenced.
// herdrReturningWithWorktree fakes the worktree.create call claim's
// own stand-up needs, the same fake TestClaimStandsUpItsWorktree uses.
func (w *world) aPlanFreshlyClaimedByThisLane() error {
	isolate(w.t)
	withHerdr(w.t, herdrReturningWithWorktree())
	w.planID = 7
	root := w.t.TempDir()
	cs := section[commandState](w)
	cs.repo = claimableRepo(w.t, root, "atlas", w.planID, "Shader unit")

	runCLI(&cs.out, &cs.errb, "claim", strconv.Itoa(w.planID), "--root", root)
	if !strings.Contains(cs.out.String(), "claimed plan") {
		return fmt.Errorf("expected the claim to succeed, got: %s%s",
			cs.out.String(), cs.errb.String())
	}

	return nil
}

// yieldRefusesItNamingReleaseAsTheWayOut is C6's own Then: the live
// holder's own yield is refused, not silently parked, and the
// refusal names release as the way out — read from the command's own
// stdout, never an internal call. Nothing was parked: the still-held
// case returns before park ever runs.
func (w *world) yieldRefusesItNamingReleaseAsTheWayOut() error {
	cs := section[commandState](w)
	got := cs.out.String()
	if !strings.Contains(got, "refused") {
		return fmt.Errorf("expected the command to report a refusal, got: %s", got)
	}
	if !strings.Contains(got, "release") {
		return fmt.Errorf("expected the refusal to name release, got: %s", got)
	}
	if strings.Contains(got, "parked:") {
		return fmt.Errorf("expected nothing parked: %s", got)
	}

	return nil
}

// cmdFritTestDir is captured by init, before any scenario's own
// isolate() ever moves the process cwd with t.Chdir — go test always
// starts a package's tests with its own directory as cwd, so this is
// cmd/frit's own package directory, the one `go build` must run from
// to produce a real frit binary regardless of which directory a later
// step's fixture stands in.
var cmdFritTestDir string

func init() {
	if d, err := os.Getwd(); err == nil {
		cmdFritTestDir = d
	}
}

var (
	builtFritOnce sync.Once
	builtFritPath string
	builtFritErr  error
)

// builtFrit builds a real frit binary once per test binary run and
// returns its path: C7 and C8 install the skill with a custom
// invocation pointing at it, and run its example command as a real
// subprocess, never the in-process dispatch every other command
// scenario drives through runCLI.
func builtFrit() (string, error) {
	builtFritOnce.Do(func() {
		dir, err := os.MkdirTemp("", "frit-bdd-bin-")
		if err != nil {
			builtFritErr = err
			return
		}
		path := filepath.Join(dir, "frit")
		cmd := exec.Command("go", "build", "-o", path, ".")
		cmd.Dir = cmdFritTestDir
		var errb bytes.Buffer
		cmd.Stderr = &errb
		if err := cmd.Run(); err != nil {
			builtFritErr = fmt.Errorf("go build cmd/frit: %w: %s", err, errb.String())
			return
		}
		builtFritPath = path
	})

	return builtFritPath, builtFritErr
}

// startCommandPattern finds the plan-start skill's own main command
// span, backticked in its Method section, with the selector still open
// for the caller to fill in.
var startCommandPattern = regexp.MustCompile("`([^`]* start <selector> --go --json)`")

// extractStartCommand reads the installed plan-start skill's own
// command line rather than composing an equivalent by hand, so the
// scenario proves the shipped instructions actually run, not a
// hand-rolled stand-in for them.
func extractStartCommand(skillText string) (string, error) {
	m := startCommandPattern.FindStringSubmatch(skillText)
	if m == nil {
		return "", fmt.Errorf(
			"plan-start skill names no `start <selector> --go --json` command")
	}

	return m[1], nil
}

// writeFakeHerdr puts a throwaway herdr on $PATH ahead of any real one,
// the way fakeHerdrOnPath and fakeEditorOnPath already stand a script
// in for a real binary that ExecContext hardcodes by name (internal/herdr).
// It logs every call to $HERDR_LOG for the Then steps to read, answers
// worktree.create by actually creating the git worktree the caller
// named — so the real frit subprocess that follows finds a lane on
// disk, exactly as the real herdr would have left one — and answers
// agent.list with no agents, so the fresh-acquire pre-flight never
// refuses a live pane that was never there. Every other call succeeds
// silently: start never reads their output, only their error.
func writeFakeHerdr(dir string) error {
	body := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$HERDR_LOG\"\n" +
		"case \"$1 $2\" in\n" +
		"\"worktree create\")\n" +
		"  cwd=\"\"; path=\"\"; branch=\"\"; prev=\"\"\n" +
		"  for a in \"$@\"; do\n" +
		"    case \"$prev\" in\n" +
		"      --cwd) cwd=\"$a\" ;;\n" +
		"      --path) path=\"$a\" ;;\n" +
		"      --branch) branch=\"$a\" ;;\n" +
		"    esac\n" +
		"    prev=\"$a\"\n" +
		"  done\n" +
		"  git -C \"$cwd\" worktree add -q \"$path\" \"$branch\" || exit 1\n" +
		"  echo '{\"result\":{\"root_pane\":{\"pane_id\":\"wZ:p1\"}}}'\n" +
		"  ;;\n" +
		"\"agent list\")\n" +
		"  echo '{\"result\":{\"agents\":[]}}'\n" +
		"  ;;\n" +
		"*)\n" +
		"  exit 0\n" +
		"  ;;\n" +
		"esac\n"

	return os.WriteFile(filepath.Join(dir, "herdr"), []byte(body), 0o755)
}

// theBundledPlanStartSkillIsInstalledWithTheBuiltFritInvocation is
// C7 and C8's shared first Given: build a real frit binary, install
// the canonical plan-start skill with it as the invocation, and read
// back the installed command line — the same file an agent would
// read — for the When step to run. It also stands a fake herdr up on
// $PATH for the whole scenario, since the subprocess the When step
// runs shares no memory with this process's own herdrRunner fake.
func (w *world) theBundledPlanStartSkillIsInstalledWithTheBuiltFritInvocation() error {
	frit, err := builtFrit()
	if err != nil {
		return err
	}
	cs := section[commandState](w)

	dir := w.t.TempDir()
	if _, err := skills.Install(dir, false, frit); err != nil {
		return fmt.Errorf("installing the skill bundle: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(
		dir, ".claude", "skills", "plan-start", "SKILL.md"))
	if err != nil {
		return fmt.Errorf("reading the installed plan-start skill: %w", err)
	}
	cmdLine, err := extractStartCommand(string(data))
	if err != nil {
		return err
	}
	cs.skillCmd = cmdLine

	herdrDir := w.t.TempDir()
	cs.herdrLog = filepath.Join(herdrDir, "herdr.log")
	if err := os.WriteFile(cs.herdrLog, nil, 0o600); err != nil {
		return err
	}
	if err := writeFakeHerdr(herdrDir); err != nil {
		return err
	}
	w.t.Setenv("PATH", herdrDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	w.t.Setenv("HERDR_LOG", cs.herdrLog)

	return nil
}

// rankedReadyPlansRepo is C7's own Given: a repository with plans 7
// and 8 both ready, and a third plan depending on 8 so it unblocks
// something 7 does not — the exact shape rankByUnblock (internal/discovery)
// ranks first, ties otherwise breaking by id. Pushed to a real bare
// origin, the coordinate a named start's claim is minted against.
func (w *world) plans7And8AreReadyWithPlan8RankedAbovePlan7() error {
	isolate(w.t)
	root := w.t.TempDir()
	cs := section[commandState](w)

	repo := initRepo(w.t, root, "atlas")
	commitPlan(w.t, repo, 7, "🔲", "Shader unit", nil, "")
	commitPlan(w.t, repo, 8, "🔲", "Lighting pass", nil, "")
	commitPlan(w.t, repo, 9, "🔲", "Depends on lighting", []int{8}, "")
	cs.repo, cs.origin = pushedOrigin(w.t, repo, "atlas")

	return nil
}

// plan7HasAnUnmetDependencyWhilePlan8IsReady is C8's own Given: plan 7
// depends on an unfinished plan 6, so it is not startable, while plan
// 8 sits ready alongside it with nothing about the fixture giving it
// any priority — the selector alone must decide which is refused.
func (w *world) plan7HasAnUnmetDependencyWhilePlan8IsReady() error {
	isolate(w.t)
	root := w.t.TempDir()
	cs := section[commandState](w)

	repo := initRepo(w.t, root, "atlas")
	commitPlan(w.t, repo, 6, "🔲", "Prereq", nil, "")
	commitPlan(w.t, repo, 7, "🔲", "Shader unit", []int{6}, "")
	commitPlan(w.t, repo, 8, "🔲", "Lighting pass", nil, "")
	cs.repo, cs.origin = pushedOrigin(w.t, repo, "atlas")

	return nil
}

// pushedOrigin gives repo a real bare remote and pushes main to it,
// the same shape claimableRepo already builds for a single-plan
// fixture, factored out here so C7 and C8's own multi-plan repos can
// reuse it.
func pushedOrigin(t *testing.T, repo, name string) (string, string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), name+"-origin.git")
	git(t, repo, "init", "-q", "--bare", "-b", "main", origin)
	git(t, repo, "remote", "add", "origin", origin)
	git(t, repo, "push", "-q", "origin", "main")

	return repo, origin
}

// theInstalledSkillsStartCommandRunsForPlanWithGoJSON is C7 and C8's
// shared When: the installed skill's own command line, its <selector>
// filled in, run as a real subprocess from the fixture's root — the
// directory holding the plan repository, exactly where an agent whose
// cwd sits there would run it, root inferred rather than passed.
func (w *world) theInstalledSkillsStartCommandRunsForPlanWithGoJSON(id string) error {
	cs := section[commandState](w)
	cmdLine := strings.ReplaceAll(cs.skillCmd, "<selector>", id)
	fields := strings.Fields(cmdLine)
	if len(fields) == 0 {
		return fmt.Errorf("empty start command extracted from the skill")
	}

	cs.out.Reset()
	cs.errb.Reset()
	cmd := exec.Command(fields[0], fields[1:]...)
	cmd.Dir = filepath.Dir(cs.repo)
	// isolate() sets FRIT_ROOT to "" so the in-process tests never see
	// a developer's own env leak in — but that empty value is itself a
	// value, and kong's envResolver reads it ahead of --root's "."
	// default, unlike a var that was never set. The real skill's
	// command never names --root at all, trusting root inference from
	// cwd exactly like this subprocess's cmd.Dir does, so the empty
	// override is stripped rather than inherited.
	cmd.Env = envWithout("FRIT_ROOT", "FRIT_CONFIG")
	cmd.Stdout = &cs.out
	cmd.Stderr = &cs.errb
	_ = cmd.Run()

	return nil
}

// envWithout returns the current process environment with the named
// variables removed entirely, rather than set empty — the difference
// kong's env resolver reads as "the caller chose nothing" versus "the
// caller chose empty".
func envWithout(names ...string) []string {
	drop := make(map[string]bool, len(names))
	for _, n := range names {
		drop[n] = true
	}
	env := os.Environ()
	kept := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if !drop[name] {
			kept = append(kept, kv)
		}
	}

	return kept
}

// startDocFor decodes the real subprocess's own --json output into a
// StartDoc, the wire shape frit start actually reports, never an
// internal re-derivation.
func startDocFor(w *world) (report.StartDoc, error) {
	cs := section[commandState](w)
	var doc report.StartDoc
	if err := json.Unmarshal(cs.out.Bytes(), &doc); err != nil {
		return report.StartDoc{}, fmt.Errorf(
			"start did not emit valid json: %w, got stdout: %s stderr: %s",
			err, cs.out.String(), cs.errb.String())
	}

	return doc, nil
}

// planGainsOneLaneAndOneDispatchedAgent is C7's first Then: the
// escalation actually ran — a lane stood up on disk at the path the
// document names, through exactly one worktree.create and one
// agent.start against the fake herdr's own log, not a refusal.
func (w *world) planGainsOneLaneAndOneDispatchedAgent(id string) error {
	doc, err := startDocFor(w)
	if err != nil {
		return err
	}
	want, _ := strconv.ParseInt(id, 10, 64)
	if doc.Refused != "" {
		return fmt.Errorf("expected plan %s to start, got refused: %s", id, doc.Refused)
	}
	if doc.Plan.ID != want || !doc.Started {
		return fmt.Errorf("expected plan %s to be started, got: %+v", id, doc)
	}
	if _, err := os.Stat(doc.Lane); err != nil {
		return fmt.Errorf("expected a lane on disk at %s: %v", doc.Lane, err)
	}

	cs := section[commandState](w)
	log, err := os.ReadFile(cs.herdrLog)
	if err != nil {
		return err
	}
	calls := string(log)
	if got := strings.Count(calls, "worktree create"); got != 1 {
		return fmt.Errorf("expected exactly one worktree create, got %d in: %s", got, calls)
	}
	if got := strings.Count(calls, "agent start"); got != 1 {
		return fmt.Errorf("expected exactly one dispatched agent, got %d in: %s", got, calls)
	}

	return nil
}

// theJSONHandoffNamesPlanAndItsPaneWithPromptDispatchedTrue is C7's
// second Then: the field an agent's own skill branches on —
// prompt_dispatched — reads true, alongside the plan and the pane it
// names, read from the same JSON the skill's caller would read.
func (w *world) theJSONHandoffNamesPlanAndItsPaneWithPromptDispatchedTrue(id string) error {
	doc, err := startDocFor(w)
	if err != nil {
		return err
	}
	want, _ := strconv.ParseInt(id, 10, 64)
	if doc.Plan.ID != want {
		return fmt.Errorf("expected the handoff to name plan %s, got: %+v", id, doc.Plan)
	}
	if !doc.PromptDispatched {
		return fmt.Errorf("expected prompt_dispatched true, got: %+v", doc)
	}
	if doc.Pane == "" {
		return fmt.Errorf("expected a pane name, got none: %+v", doc)
	}

	return nil
}

// planRemainsUnheldWithNoAgent is C7's third Then: the other, higher
// ranked plan never gained a hold — no branch pushed for it on
// origin — and the fixture's fake herdr recorded no more than the one
// dispatched agent the named plan alone got.
func (w *world) planRemainsUnheldWithNoAgent(id string) error {
	cs := section[commandState](w)
	if err := assertNoOriginBranch(w.t, cs.origin, id); err != nil {
		return err
	}
	log, err := os.ReadFile(cs.herdrLog)
	if err != nil {
		return err
	}
	if got := strings.Count(string(log), "agent start"); got != 1 {
		return fmt.Errorf("expected exactly one dispatched agent overall, got %d in: %s",
			got, string(log))
	}

	return nil
}

// theJSONRefusalNamesPlanAndItsUnmetDependency is C8's first Then: the
// refusal stays on the plan the caller named, and reads as the
// unmet-dependency reason claimRefusal (cmd/frit/claim.go) gives it,
// rather than any other refusal wording.
func (w *world) theJSONRefusalNamesPlanAndItsUnmetDependency(id string) error {
	doc, err := startDocFor(w)
	if err != nil {
		return err
	}
	want, _ := strconv.ParseInt(id, 10, 64)
	if doc.Plan.ID != want {
		return fmt.Errorf("expected the refusal to name plan %s, got: %+v", id, doc.Plan)
	}
	if !strings.Contains(doc.Refused, "unfinished dependency") {
		return fmt.Errorf("expected an unmet-dependency refusal, got: %q", doc.Refused)
	}

	return nil
}

// neitherPlanGainsAHoldOrAnAgent is C8's second Then: a refused named
// start touches neither the plan it named nor the one it left alone —
// no branch for either on origin, and the fake herdr's log, read once
// for the whole scenario, never saw a single call.
func (w *world) neitherPlanGainsAHoldOrAnAgent() error {
	cs := section[commandState](w)
	for _, id := range []string{"7", "8"} {
		if err := assertNoOriginBranch(w.t, cs.origin, id); err != nil {
			return err
		}
	}
	log, err := os.ReadFile(cs.herdrLog)
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(string(log)); got != "" {
		return fmt.Errorf("expected no herdr calls at all, got: %s", got)
	}

	return nil
}

// assertNoOriginBranch reports an error unless origin carries no
// plan/<id> branch — the ref a claim mints, read directly off the
// remote rather than the local clone, since a lost race or a refusal
// leaves nothing to push in the first place.
func assertNoOriginBranch(t *testing.T, origin, id string) error {
	t.Helper()
	if _, err := gitCapture(t, origin, "show-ref", "--verify", "refs/heads/plan/"+id); err == nil {
		return fmt.Errorf("expected plan %s to hold no branch on origin", id)
	}

	return nil
}
