package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
)

// C15 and C16 run the built frit on a host with no herdr installed —
// the headless shape a cloud agent container has. C15 claims and
// releases there; C16's pick --go and C18's start --go hand the prompt
// back with no pane to run it in; C19's message refuses naming herdr
// not installed; C20's yield removes a fenced lane with git; C21's
// harness session publishes its lane through its own branch; C22's
// start still sees a lane live on another host. They register as
// their own section file, per docs/development.md, and read and write
// the command section's own state.
func init() {
	registrars = append(registrars, (*world).registerMissingHerdr)
}

// missingHerdrState is C15, C16 and C18's own: the $PATH the built
// frit runs under, which carries git and nothing named herdr, and the
// lane the last claim or start stood up.
type missingHerdrState struct {
	path     string
	lane     string
	held     string
	leftover string
	// expectFail lets the next built-frit run exit non-zero, for a
	// step that asserts on the failure it reports.
	expectFail bool
	// herdrLog is where C24's recording herdr appends each call.
	herdrLog string
}

func (w *world) registerMissingHerdr(sc *godog.ScenarioContext) {
	sc.Step(`^herdr is not installed on this host$`, w.herdrIsNotInstalledOnThisHost)
	sc.Step(`^herdr is installed on this host$`, w.herdrIsInstalledOnThisHost)
	sc.Step(`^the built frit runs "([^"]+)"$`, w.theBuiltFritRuns)
	sc.Step(`^origin carries no work ref for plan (\d+)$`, w.originCarriesNoWorkRefForPlan)
	sc.Step(`^plan (\d+) is claimed in a worktree of its own, on origin's claim marker$`,
		w.planIsClaimedInAWorktreeOfItsOwn)
	sc.Step(`^origin's claim marker for plan (\d+) binds no session$`, w.originsClaimMarkerBindsNoSession)
	sc.Step(`^the lane holds its token for plan (\d+)$`, w.theLaneHoldsItsTokenForPlan)
	sc.Step(`^the built frit runs "([^"]+)" from the lane$`, w.theBuiltFritRunsFromTheLane)
	sc.Step(`^origin's tip for plan (\d+) is a release marker$`, w.originsTipForPlanIsAReleaseMarker)
	sc.Step(`^plan (\d+) is handed back in a worktree of its own, on origin's claim marker$`,
		w.planIsHandedBackInAWorktreeOfItsOwn)
	sc.Step(`^plan (\d+) is handed back again from the same lane, resumed$`,
		w.planIsHandedBackAgainResumed)
	sc.Step(`^it refuses, naming herdr not installed rather than unreachable$`,
		w.itRefusesNamingHerdrNotInstalled)
	sc.Step(`^another machine takes plan (\d+) over$`, w.anotherMachineTakesPlanOver)
	sc.Step(`^the lane's work is parked and its worktree removed with git$`,
		w.theLanesWorkIsParkedAndItsWorktreeRemoved)
	sc.Step(`^a commit made in the lane is pushed to plan (\d+) and copied to "([^"]+)"$`,
		w.aCommitInTheLaneIsPushedAndCopied)
	sc.Step(`^the board shows plan (\d+) (held|not held)$`, w.theBoardShowsPlan)
	sc.Step(`^"([^"]+)" is merged into origin's main$`, w.branchIsMergedIntoOriginsMain)
	sc.Step(`^plan 7's matured hold has a live agent in its lane on host "([^"]+)"$`,
		w.planSevensMaturedHoldIsLiveOnHost)
	sc.Step(`^start refuses, naming plan/7's live lane, and origin's lease is untouched$`,
		w.startRefusesNamingTheLiveLaneLeaseUntouched)
	sc.Step(`^the refusal carries no herdr problem$`, w.theRefusalCarriesNoHerdrProblem)
	sc.Step(`^a released lane's leftover worktree holds unlanded work on plan 7$`,
		w.aReleasedLanesLeftoverHoldsUnlandedWork)
	sc.Step(`^the built frit runs "([^"]+)" expecting it to fail$`, w.theBuiltFritRunsExpectingFailure)
	sc.Step(`^it names the leftover's path, its rescue ref and frit reap$`,
		w.itNamesTheLeftoversPathRescueAndReap)
	sc.Step(`^the leftover worktree still stands and origin's plan/7 is unchanged$`,
		w.theLeftoverStillStandsAndOriginUnchanged)
	sc.Step(`^the leftover worktree is gone$`, w.theLeftoverWorktreeIsGone)
	sc.Step(`^herdr is installed on this host, recording every call$`,
		w.herdrIsInstalledRecordingEveryCall)
	sc.Step(`^herdr was never called$`, w.herdrWasNeverCalled)
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

// herdrIsInstalledOnThisHost installs a herdr that answers with no
// agents. A scenario whose verb mints, or races to mint, names this so
// it reaches the mint on any build box: the missing-herdr gate would
// otherwise refuse first wherever herdr happens not to be installed.
func (w *world) herdrIsInstalledOnThisHost() error {
	withHerdr(w.t, herdrReturning())

	return nil
}

// theBuiltFritRuns runs the built binary as a subprocess from the
// fixture's root, under the herdr-less $PATH, so what is proven is the
// shipped command's behavior rather than an in-process stand-in.
func (w *world) theBuiltFritRuns(args string) error {
	repo := section[commandState](w).repo
	if repo == "" {
		return fmt.Errorf("no fixture repository to run frit beside")
	}

	return w.runBuiltFritIn(filepath.Dir(repo), args)
}

// theBuiltFritRunsFromTheLane runs the built binary from inside the
// lane C15's claim stood up, so a verb that infers its plan from the
// checkout reads it the way an agent working there would.
func (w *world) theBuiltFritRunsFromTheLane(args string) error {
	lane := section[missingHerdrState](w).lane
	if lane == "" {
		return fmt.Errorf("no lane was stood up to run from")
	}

	return w.runBuiltFritIn(lane, args)
}

// runBuiltFritIn is the shared body of both: the built binary, from
// dir, under the herdr-less $PATH, its output in the command section.
func (w *world) runBuiltFritIn(dir, args string) error {
	frit, err := builtFrit()
	if err != nil {
		return err
	}
	cs := section[commandState](w)
	cs.out.Reset()
	cs.errb.Reset()
	cmd := exec.Command(frit, strings.Fields(args)...)
	cmd.Dir = dir
	cmd.Env = append(envWithout("FRIT_ROOT", "FRIT_CONFIG", "PATH"),
		"PATH="+section[missingHerdrState](w).path)
	cmd.Stdout = &cs.out
	cmd.Stderr = &cs.errb
	if err := cmd.Run(); err != nil && !section[missingHerdrState](w).expectFail {
		return fmt.Errorf("frit %s: %w: %s", args, err, cs.errb.String())
	}

	return nil
}

// originCarriesNoWorkRefForPlan reads the bare remote directly: no
// claim marker and no release marker ever reached it for this plan.
func (w *world) originCarriesNoWorkRefForPlan(id string) error {
	cs := section[commandState](w)
	origin, err := gitCapture(w.t, cs.repo, "remote", "get-url", "origin")
	if err != nil {
		return err
	}

	return assertNoOriginBranch(w.t, strings.TrimSpace(origin), id)
}

// planIsClaimedInAWorktreeOfItsOwn reads claim's --json: claimed, with
// a worktree named, standing on origin's claim marker.
func (w *world) planIsClaimedInAWorktreeOfItsOwn(id string) error {
	var doc struct {
		Claimed  bool   `json:"claimed"`
		Worktree string `json:"worktree"`
	}
	cs := section[commandState](w)
	if err := json.Unmarshal(cs.out.Bytes(), &doc); err != nil {
		return fmt.Errorf("claim --json: %w: %s", err, cs.out.String())
	}
	if !doc.Claimed || doc.Worktree == "" {
		return fmt.Errorf("expected plan %s claimed with a worktree, got: %s", id, cs.out.String())
	}

	return w.laneStandsOnOriginsTip(id, doc.Worktree)
}

// startJSON is the slice of start's --json, and pick --go's, a
// handed-back escalation is read by.
type startJSON struct {
	Plan             struct{ ID int64 } `json:"plan"`
	Started          bool               `json:"started"`
	Resumed          bool               `json:"resumed"`
	Handoff          string             `json:"handoff"`
	PromptDispatched bool               `json:"prompt_dispatched"`
	Pane             string             `json:"pane"`
	Lane             string             `json:"lane"`
	Refused          string             `json:"refused"`
	Problems         []struct {
		Repo string `json:"repo"`
	} `json:"problems"`
}

// readHandedBack decodes the last run's start document and checks it
// handed plan id back: started, with no pane and nothing dispatched.
func (w *world) readHandedBack(id string) (startJSON, error) {
	doc, err := w.decodeStart()
	if err != nil {
		return doc, err
	}
	cs := section[commandState](w)
	if strconv.FormatInt(doc.Plan.ID, 10) != id || !doc.Started ||
		doc.Handoff != "handed-back" || doc.PromptDispatched || doc.Pane != "" {
		return doc, fmt.Errorf("expected plan %s handed back, got: %s", id, cs.out.String())
	}

	return doc, nil
}

// decodeStart decodes the last run's start document.
func (w *world) decodeStart() (startJSON, error) {
	var doc startJSON
	cs := section[commandState](w)
	if err := json.Unmarshal(cs.out.Bytes(), &doc); err != nil {
		return doc, fmt.Errorf("start --json: %w: %s", err, cs.out.String())
	}

	return doc, nil
}

// planIsHandedBackInAWorktreeOfItsOwn: the escalation ran with no pane
// — the lane stood up by git on origin's claim marker, the prompt left
// for the caller to run there.
func (w *world) planIsHandedBackInAWorktreeOfItsOwn(id string) error {
	doc, err := w.readHandedBack(id)
	if err != nil {
		return err
	}

	return w.laneStandsOnOriginsTip(id, doc.Lane)
}

// planIsHandedBackAgainResumed: run from inside the lane, start resumed
// on the lane's own token and handed back again, from the same lane.
func (w *world) planIsHandedBackAgainResumed(id string) error {
	doc, err := w.readHandedBack(id)
	if err != nil {
		return err
	}
	if !doc.Resumed {
		return fmt.Errorf("expected plan %s resumed on its token: %s", id, section[commandState](w).out.String())
	}
	if lane := section[missingHerdrState](w).lane; doc.Lane != lane {
		return fmt.Errorf("resumed in %s, not the lane %s", doc.Lane, lane)
	}

	return nil
}

// laneStandsOnOriginsTip checks lane is checked out on plan id's claim
// branch at the very tip origin holds — as herdr's own stand-up would
// have left it — and records it as the lane later steps run from.
func (w *world) laneStandsOnOriginsTip(id, lane string) error {
	branch, err := gitCapture(w.t, lane, "branch", "--show-current")
	if err != nil {
		return err
	}
	if branch != "plan/"+id {
		return fmt.Errorf("lane is on %q, not plan/%s", branch, id)
	}
	head, err := gitCapture(w.t, lane, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if tip := originTip(w, id); tip != head {
		return fmt.Errorf("lane stands on %s, origin's claim is %s", head, tip)
	}
	section[missingHerdrState](w).lane = lane

	return nil
}

// originsClaimMarkerBindsNoSession: no herdr ever named a session, so
// the marker's session trailer is empty and only the staleness window
// guards the hold — the shape S61 already pins.
func (w *world) originsClaimMarkerBindsNoSession(id string) error {
	body, err := gitCapture(w.t, originPath(w), "log", "-1", "--format=%B", "refs/heads/plan/"+id)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(body, "plan "+id+": claim") || !strings.Contains(body, "session: -") {
		return fmt.Errorf("origin's tip is not a claim marker binding no session: %s", body)
	}

	return nil
}

// theLaneHoldsItsTokenForPlan: the minted tip persisted in the lane's
// own git dir, so release resumes on it with no window to wait out.
func (w *world) theLaneHoldsItsTokenForPlan(id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	token := claim.ReadToken(section[missingHerdrState](w).lane, n, gitwt.Exec)
	if tip := originTip(w, id); token != tip {
		return fmt.Errorf("lane token %q is not origin's tip %q", token, tip)
	}

	return nil
}

// originsTipForPlanIsAReleaseMarker reads the bare remote's plan
// branch: the lease ended by a release marker, not a delete.
func (w *world) originsTipForPlanIsAReleaseMarker(id string) error {
	body, err := gitCapture(w.t, originPath(w), "log", "-1", "--format=%B", "refs/heads/plan/"+id)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(body, "plan "+id+": release") {
		return fmt.Errorf("origin's tip for plan %s is not a release marker: %s", id, body)
	}

	return nil
}

// originPath is the fixture's bare remote, read off the clone.
func originPath(w *world) string {
	origin, err := gitCapture(w.t, section[commandState](w).repo, "remote", "get-url", "origin")
	if err != nil {
		w.t.Fatalf("origin url: %v", err)
	}

	return strings.TrimSpace(origin)
}

// originTip is plan id's branch tip on the bare remote, "" when absent.
func originTip(w *world, id string) string {
	tip, err := gitCapture(w.t, originPath(w), "rev-parse", "--verify", "--quiet", "refs/heads/plan/"+id)
	if err != nil {
		return ""
	}

	return tip
}

// itRefusesNamingHerdrNotInstalled reads message's --json: refused on
// the certain absence, not the "unreachable" a socket that may come
// back earns, and with no herdr problem carried for this host's mode.
func (w *world) itRefusesNamingHerdrNotInstalled() error {
	var doc struct {
		Refused  string `json:"refused"`
		Problems []struct {
			Repo string `json:"repo"`
		} `json:"problems"`
	}
	cs := section[commandState](w)
	if err := json.Unmarshal(cs.out.Bytes(), &doc); err != nil {
		return fmt.Errorf("--json: %w: %s", err, cs.out.String())
	}
	if !strings.Contains(doc.Refused, "herdr not installed") || strings.Contains(doc.Refused, "unreachable") {
		return fmt.Errorf("expected a not-installed refusal, got: %s", cs.out.String())
	}
	for _, p := range doc.Problems {
		if p.Repo == "herdr" {
			return fmt.Errorf("a herdr problem was carried: %s", cs.out.String())
		}
	}

	return nil
}

// anotherMachineTakesPlanOver fences the lane: a second clone of the
// same origin seizes plan id's work ref at its current tip, leaving
// the lane's own local ref behind it.
func (w *world) anotherMachineTakesPlanOver(id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	other := cloneAgain(w.t, section[commandState](w).repo)
	_, err = claim.Takeover(other, claim.LeaseOptions{
		PlanID: n, Remote: "origin", Base: "origin/main",
		Holder: "elsewhere", Lane: "/lanes/x",
	}, originTip(w, id), gitwt.Exec)

	return err
}

// theLanesWorkIsParkedAndItsWorktreeRemoved reads yield's --json: the
// fenced lane's tip parked to a rescue ref and the lane torn down, its
// directory gone from disk while the primary checkout stands.
func (w *world) theLanesWorkIsParkedAndItsWorktreeRemoved() error {
	var doc struct {
		Rescue   string `json:"rescue"`
		TornDown bool   `json:"torn_down"`
	}
	cs := section[commandState](w)
	if err := json.Unmarshal(cs.out.Bytes(), &doc); err != nil {
		return fmt.Errorf("yield --json: %w: %s", err, cs.out.String())
	}
	if doc.Rescue == "" || !doc.TornDown {
		return fmt.Errorf("expected the lane parked and torn down, got: %s", cs.out.String())
	}
	if _, err := os.Stat(section[missingHerdrState](w).lane); !os.IsNotExist(err) {
		return fmt.Errorf("the lane's worktree still stands: %v", err)
	}
	if _, err := os.Stat(cs.repo); err != nil {
		return fmt.Errorf("the primary checkout is gone: %w", err)
	}

	return nil
}

// aCommitInTheLaneIsPushedAndCopied is a harness session's own work
// step: commit in the lane, push the lease ref — a renewal — then push
// the same tip to the branch its harness publishes, with plain git.
func (w *world) aCommitInTheLaneIsPushedAndCopied(id, harness string) error {
	lane := section[missingHerdrState](w).lane
	if err := os.WriteFile(filepath.Join(lane, "work.txt"), []byte("plan "+id+"\n"), 0o600); err != nil {
		return err
	}
	branch := "plan/" + id
	for _, args := range [][]string{
		{"add", "work.txt"},
		{"-c", "commit.gpgsign=false", "commit", "-q", "-m", "plan " + id + ": the work"},
		{"push", "-q", "origin", branch},
		{"push", "-q", "origin", branch + ":" + harness},
	} {
		if out, err := gitCapture(w.t, lane, args...); err != nil {
			return fmt.Errorf("git %v: %w: %s", args, err, out)
		}
	}

	return nil
}

// theBoardShowsPlan reads board --json for plan id's held flag.
func (w *world) theBoardShowsPlan(id, want string) error {
	var doc struct {
		Plans []struct {
			ID   int64 `json:"id"`
			Held bool  `json:"held"`
		} `json:"plans"`
	}
	cs := section[commandState](w)
	if err := json.Unmarshal(cs.out.Bytes(), &doc); err != nil {
		return fmt.Errorf("board --json: %w: %s", err, cs.out.String())
	}
	for _, p := range doc.Plans {
		if strconv.FormatInt(p.ID, 10) != id {
			continue
		}
		if p.Held != (want == "held") {
			return fmt.Errorf("plan %s held is %v, want %s", id, p.Held, want)
		}
		return nil
	}

	return fmt.Errorf("board does not list plan %s: %s", id, cs.out.String())
}

// branchIsMergedIntoOriginsMain stands in for the forge merging the
// session's PR: the primary checkout merges the harness branch into
// main with a merge commit and pushes main.
func (w *world) branchIsMergedIntoOriginsMain(harness string) error {
	repo := section[commandState](w).repo
	for _, args := range [][]string{
		{"fetch", "-q", "origin", harness},
		{"-c", "commit.gpgsign=false", "merge", "-q", "--no-ff", "-m", "merge " + harness, "FETCH_HEAD"},
		{"push", "-q", "origin", "main"},
	} {
		if out, err := gitCapture(w.t, repo, args...); err != nil {
			return fmt.Errorf("git %v: %w: %s", args, err, out)
		}
	}

	return nil
}

// planSevensMaturedHoldIsLiveOnHost leases plan 7 to another machine,
// matures its takeover window, and puts a working agent in a checkout
// of its branch that only host can see: a fake ssh, written into the
// herdr-less $PATH, answers that host's herdr with the pane and runs
// any other remote command — the git a remote cwd resolves with —
// locally.
func (w *world) planSevensMaturedHoldIsLiveOnHost(host string) error {
	repo := section[commandState](w).repo
	if repo == "" {
		return fmt.Errorf("no fixture repository; a plan nobody has ever held comes first")
	}
	lease, err := claim.Acquire(repo, claim.LeaseOptions{PlanID: 7, Remote: "origin",
		Base: "origin/main", Holder: "elsewhere", Lane: "/lanes/x"}, gitwt.Exec)
	if err != nil {
		return err
	}
	seedWindow(w.t, "atlas", 7, lease.Tip, 3*time.Hour)
	lane := liveLeaseLane(w.t, repo, "plan/7")
	body, err := json.Marshal(map[string]any{"result": map[string]any{"agents": []map[string]any{{
		"agent": "claude", "agent_status": "working", "pane_id": "wBox:p1", "cwd": lane,
	}}}})
	if err != nil {
		return err
	}
	st := section[missingHerdrState](w)
	if st.path == "" {
		return fmt.Errorf("the herdr-less $PATH comes first")
	}
	script := "#!/bin/sh\n[ \"$1\" = " + host + " ] || exit 255\nshift\n" +
		"if [ \"$1\" = herdr ]; then printf '%s\\n' '" + string(body) + "'; exit 0; fi\nexec \"$@\"\n"
	st.held = lease.Tip

	return os.WriteFile(filepath.Join(st.path, "ssh"), []byte(script), 0o700)
}

// startRefusesNamingTheLiveLaneLeaseUntouched reads start's --json: a
// refusal naming the live lane's branch, no hand-back, and origin's
// lease exactly where the other machine left it.
func (w *world) startRefusesNamingTheLiveLaneLeaseUntouched() error {
	doc, err := w.decodeStart()
	if err != nil {
		return err
	}
	if !strings.Contains(doc.Refused, "plan/7") || doc.Handoff == "handed-back" {
		return fmt.Errorf("expected a live-lane refusal, got: %s", section[commandState](w).out.String())
	}
	if tip := originTip(w, "7"); tip != section[missingHerdrState](w).held {
		return fmt.Errorf("origin's lease moved to %s", tip)
	}

	return nil
}

// theRefusalCarriesNoHerdrProblem: a host with no herdr refuses for a
// remote live lane without listing the missing herdr as a fault — the
// same mode a hand-back carries no problem for.
func (w *world) theRefusalCarriesNoHerdrProblem() error {
	doc, err := w.decodeStart()
	if err != nil {
		return err
	}
	for _, p := range doc.Problems {
		if p.Repo == "herdr" {
			return fmt.Errorf("a herdr problem was carried: %s", section[commandState](w).out.String())
		}
	}

	return nil
}

// aReleasedLanesLeftoverHoldsUnlandedWork leaves what a released lane
// leaves behind: a worktree on plan/7, its unlanded commit pushed, the
// lease ended by a release marker, the checkout still on disk.
func (w *world) aReleasedLanesLeftoverHoldsUnlandedWork() error {
	cs := section[commandState](w)
	if cs.repo == "" {
		return fmt.Errorf("no fixture repository; a plan nobody has ever held comes first")
	}
	path, tip := leftoverWorktree(w.t, filepath.Dir(cs.repo), cs.repo, 7)
	st := section[missingHerdrState](w)
	st.leftover, st.held = path, tip

	return nil
}

// theBuiltFritRunsExpectingFailure runs the built frit from the root,
// letting it exit non-zero so the next step can read what it reported.
func (w *world) theBuiltFritRunsExpectingFailure(args string) error {
	st := section[missingHerdrState](w)
	st.expectFail = true
	defer func() { st.expectFail = false }()

	return w.theBuiltFritRuns(args)
}

// itNamesTheLeftoversPathRescueAndReap: the refusal is explicit — the
// worktree it kept, where the work was parked, and the cleanup.
func (w *world) itNamesTheLeftoversPathRescueAndReap() error {
	got := section[commandState](w).errb.String()
	for _, want := range []string{section[missingHerdrState](w).leftover,
		"refs/frit/rescue/7/", "left standing", "frit reap --go"} {
		if !strings.Contains(got, want) {
			return fmt.Errorf("the refusal does not name %q: %s", want, got)
		}
	}

	return nil
}

// theLeftoverStillStandsAndOriginUnchanged: nothing was removed, and
// nothing was minted — plan/7 is exactly where release left it.
func (w *world) theLeftoverStillStandsAndOriginUnchanged() error {
	st := section[missingHerdrState](w)
	if _, err := os.Stat(st.leftover); err != nil {
		return fmt.Errorf("the leftover was removed: %w", err)
	}
	if tip := originTip(w, "7"); tip != st.held {
		return fmt.Errorf("origin's plan/7 moved to %s, want %s", tip, st.held)
	}

	return nil
}

// theLeftoverWorktreeIsGone: reap --go, the cleanup the refusal names,
// removed the leftover once its owner ran it.
func (w *world) theLeftoverWorktreeIsGone() error {
	if _, err := os.Stat(section[missingHerdrState](w).leftover); !os.IsNotExist(err) {
		return fmt.Errorf("the leftover still stands: %v; %s", err, section[commandState](w).out.String())
	}

	return nil
}

// herdrIsInstalledRecordingEveryCall builds the git-only $PATH and puts
// a herdr on it that answers with no agents and appends every call it
// gets to a log, so a scenario can prove the binary was never run.
func (w *world) herdrIsInstalledRecordingEveryCall() error {
	if err := w.herdrIsNotInstalledOnThisHost(); err != nil {
		return err
	}
	st := section[missingHerdrState](w)
	st.herdrLog = filepath.Join(w.t.TempDir(), "herdr.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + st.herdrLog +
		"\nprintf '%s\\n' '{\"result\":{\"agents\":[]}}'\n"

	return os.WriteFile(filepath.Join(st.path, "herdr"), []byte(script), 0o700)
}

// herdrWasNeverCalled: the recording herdr's log is absent or empty.
func (w *world) herdrWasNeverCalled() error {
	body, err := os.ReadFile(section[missingHerdrState](w).herdrLog)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(body) > 0 {
		return fmt.Errorf("herdr was called: %s", body)
	}

	return nil
}
