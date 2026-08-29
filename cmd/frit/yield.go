package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/report"
)

type yieldCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
}

// Run ends a fenced lane's stake in a plan: its local divergence is
// parked to the rescue ref, its own worktree is torn down through
// herdr, and it exits clean. The lane that still holds the live lease
// is refused — yield is for the fenced, not an alias for release.
//
// It never guards a foreign hold the way claim, start, nudge and open
// do: acting on a lane another host's claim now covers is exactly what
// a fenced lane needs yield for.
func (yc *yieldCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	// The gather above reads the whole fleet, and rightly wants each
	// repository's fetch bounded independently. What follows is scoped
	// to one repository's own lease ref, so it shares a single deadline
	// instead: a stalled remote should cost roughly --git-timeout, not
	// a multiple of it across the pre-push read, the push and a retry.
	rt.git = gitwt.WithDeadline(gitwt.ExecContext, time.Now().Add(c.GitTimeout))

	plan, err := resolveSelector(rt, yc.Selector, res.Plans, false)
	if err != nil {
		return err
	}

	branch := claim.Branch(plan.ID)
	doc := report.NewYield(c.Root, plan.Repo, plan.ID, plan.Title, branch)
	carryProblems(doc, res.Problems, c.All)

	coord, ok := res.Coords[plan.Repo]
	if !ok {
		doc.Refuse(ambiguousRepo(plan.Repo))
		return renderYield(c, rt, doc)
	}

	local, err := localRef(rt, coord.Path, branch)
	if err != nil {
		return err
	}

	sc, err := claim.Yield(coord.Path, claim.LeaseOptions{
		PlanID: plan.ID,
		Remote: coord.Remote,
		Base:   coord.Base,
		Holder: hostname(),
		Lane:   defaultLanePath(coord.Path, plan.Path),
	}, local, rt.git)
	if err != nil {
		var still *claim.StillHeldError
		if errors.As(err, &still) {
			doc.Refuse("is still held by this lane; yield is for a " +
				"fenced lane, use release instead")
			return renderYield(c, rt, doc)
		}
		// A park conflict is a warning, not a command failure, the same
		// way scavengeRef treats it: the document still renders, and
		// under --json nothing is lost to stderr. The lane is left
		// standing rather than torn down — parking did not succeed, so
		// tearing the worktree down would discard exactly what it
		// failed to save.
		doc.Warn(fmt.Sprintf("park: %v", err))
		return renderYield(c, rt, doc)
	}
	doc.Parked(sc.Rescue)

	tearDownLane(rt, doc)

	return renderYield(c, rt, doc)
}

// localRef reads the tip a plan's work ref carries in this repository's
// own local git state — shared by every worktree of the same
// repository, so reading it from the coordinate's path sees the same
// ref a fenced lane's own checkout carries. "" when the ref was never
// fetched or minted locally.
//
// `rev-parse --verify --quiet` exits 1, with nothing on stderr, for
// exactly that absent-ref case — the one failure this reads as "".
// Any other exit is a real fault (a bad repoPath, a broken git dir),
// and must not be read as an absent ref: local feeds claim.Yield's
// still-held check and its park, so a fault silently taken for "" would
// either misreport a live plan as unheld or park nothing while telling
// the operator it did.
func localRef(rt *runtime, repoPath, branch string) (string, error) {
	out, err := rt.git(repoPath,
		"rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", nil
		}

		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// tearDownLane hands the calling pane's own worktree to herdr for
// removal. Yield acts on the lane it is itself running in, so the
// workspace to tear down is read off the current pane, not looked up
// by plan — the same metadata read start's escalation never needs, and
// never an agent read. A herdr frit could not reach, or with no pane
// open, is a warning: the parked rescue already stands regardless.
//
// The pane is still checked against the plan before anything is torn
// down: yield was given a plan id, not a workspace, and nothing
// otherwise stops that id from being a different plan than whatever
// happens to be running in the calling pane — a mistaken or explicit
// argument would then tear down an unrelated, possibly live, lane. The
// cwd is resolved back to a repository and plan id the same way an
// empty selector infers one (fleet.CurrentPlanID); a mismatch, like an
// unreachable herdr, is a warning that leaves the worktree standing
// rather than a guess acted on.
func tearDownLane(rt *runtime, doc *report.YieldDoc) {
	pane, err := herdr.CurrentPane(rt.herdr)
	if err != nil {
		doc.Warn(fmt.Sprintf("pane current: %v", err))
		return
	}
	if pane.Workspace == "" {
		doc.Warn("no pane open to tear down")
		return
	}
	repo, id, ok := fleet.CurrentPlanID(pane.CWD, rt.git, holdsForRoot)
	if !ok || repo != doc.Plan.Repo || id != doc.Plan.ID {
		doc.Warn(fmt.Sprintf(
			"the calling pane is not plan %d's own lane; its worktree "+
				"was left standing", doc.Plan.ID))
		return
	}
	if err := herdr.WorktreeRemove(rt.herdr, pane.Workspace); err != nil {
		doc.Warn(fmt.Sprintf("worktree remove: %v", err))
		return
	}
	doc.Torn()
}

// renderYield prints the yield as a table or emits it as JSON.
func renderYield(c *cli, rt *runtime, doc *report.YieldDoc) error {
	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printYield(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printYield reports what yield parked and tore down, or why it
// refused. A refusal is not a failure — the lane still holds its own
// lease — so the command still exits clean.
func printYield(out io.Writer, doc *report.YieldDoc) {
	if doc.Refused != "" {
		_, _ = fmt.Fprintf(out, "refused: plan %d %s\n",
			doc.Plan.ID, doc.Refused)
		return
	}

	_, _ = fmt.Fprintf(out, "yielded plan %d\n", doc.Plan.ID)
	if doc.Rescue != "" {
		_, _ = fmt.Fprintf(out, "  parked: %s\n", doc.Rescue)
	}
	if doc.TornDown {
		_, _ = fmt.Fprintln(out, "  torn down: yes")
	}
	if doc.Warning != "" {
		_, _ = fmt.Fprintf(out, "  warning: %s\n", doc.Warning)
	}
}
