package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitwt"
)

// C15 and C16 run the built frit on a host with no herdr installed —
// the headless shape a cloud agent container has. C15 claims and
// releases there; C16's pick --go still refuses, needing a pane. They register as
// their own section file, per docs/development.md, and read and write
// the command section's own state.
func init() {
	registrars = append(registrars, (*world).registerMissingHerdr)
}

// missingHerdrState is C15 and C16's own: the $PATH the built frit
// runs under, which carries git and nothing named herdr, and the lane
// C15's claim stood up.
type missingHerdrState struct {
	path string
	lane string
}

func (w *world) registerMissingHerdr(sc *godog.ScenarioContext) {
	sc.Step(`^herdr is not installed on this host$`, w.herdrIsNotInstalledOnThisHost)
	sc.Step(`^herdr is installed on this host$`, w.herdrIsInstalledOnThisHost)
	sc.Step(`^the built frit runs "([^"]+)"$`, w.theBuiltFritRuns)
	sc.Step(`^it refuses, naming herdr not found and nothing claimed$`,
		w.itRefusesNamingHerdrNotFoundAndNothingClaimed)
	sc.Step(`^origin carries no work ref for plan (\d+)$`, w.originCarriesNoWorkRefForPlan)
	sc.Step(`^plan (\d+) is claimed in a worktree of its own, on origin's claim marker$`,
		w.planIsClaimedInAWorktreeOfItsOwn)
	sc.Step(`^origin's claim marker for plan (\d+) binds no session$`, w.originsClaimMarkerBindsNoSession)
	sc.Step(`^the lane holds its token for plan (\d+)$`, w.theLaneHoldsItsTokenForPlan)
	sc.Step(`^the built frit runs "([^"]+)" from the lane$`, w.theBuiltFritRunsFromTheLane)
	sc.Step(`^origin's tip for plan (\d+) is a release marker$`, w.originsTipForPlanIsAReleaseMarker)
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
	return w.runBuiltFritIn(filepath.Dir(section[commandState](w).repo), args)
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

// planIsClaimedInAWorktreeOfItsOwn reads claim's --json: claimed, with
// a worktree named, and that worktree checked out on the claim branch
// at the very tip origin holds — the claim marker, as herdr's own
// stand-up would have left it.
func (w *world) planIsClaimedInAWorktreeOfItsOwn(id string) error {
	var doc struct {
		Claimed  bool   `json:"claimed"`
		Refused  string `json:"refused"`
		Worktree string `json:"worktree"`
	}
	cs := section[commandState](w)
	if err := json.Unmarshal(cs.out.Bytes(), &doc); err != nil {
		return fmt.Errorf("claim --json: %w: %s", err, cs.out.String())
	}
	if !doc.Claimed || doc.Worktree == "" {
		return fmt.Errorf("expected plan %s claimed with a worktree, got: %s", id, cs.out.String())
	}
	branch, err := gitCapture(w.t, doc.Worktree, "branch", "--show-current")
	if err != nil {
		return err
	}
	if branch != "plan/"+id {
		return fmt.Errorf("lane is on %q, not plan/%s", branch, id)
	}
	head, err := gitCapture(w.t, doc.Worktree, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if tip := originTip(w, id); tip != head {
		return fmt.Errorf("lane stands on %s, origin's claim is %s", head, tip)
	}
	section[missingHerdrState](w).lane = doc.Worktree

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
