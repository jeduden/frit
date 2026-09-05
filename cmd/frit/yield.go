package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
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
// A foreign hold this lane is genuinely fenced under — its own local
// copy of the work ref still carries the divergence to park — is what
// yield exists for, and is acted on the way claim, start, nudge and
// open never act on a foreign hold. A foreign hold with nothing local
// to park is a different case: this lane never fetched or minted the
// ref at all, so there is nothing fenced here to end, and yield
// refuses it the way release does.
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
	doc.SetGather(gatherStatus(res))

	coord, ok := res.Coords[plan.Repo]
	if !ok {
		doc.Refuse(ambiguousRepo(plan.Repo))
		return renderYield(c, rt, doc)
	}

	local, err := localRef(rt, coord.Path, branch)
	if err != nil {
		return err
	}

	if reason, ok := foreignYieldRefusal(plan, local); ok {
		doc.RefuseUnproven(reason, plan.ID)
		return renderYield(c, rt, doc)
	}

	return performYield(c, rt, doc, coord, plan, local)
}

// performYield runs claim.Yield for a lane that is either fenced —
// local diverges from the remote lease — or holds nothing anyone else
// claims, and reports what it did: parked and torn down, a refusal,
// or a non-fatal warning. Split out of Run so the dispatch above it —
// the ambiguous-repo, foreign-hold and fenced/no-op cases — reads as
// one flat sequence of early returns.
func performYield(
	c *cli, rt *runtime, doc *report.YieldDoc, coord fleet.Coord,
	plan discovery.Plan, local string,
) error {
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
		var unconfirmed *claim.UnconfirmedYieldError
		if errors.As(err, &unconfirmed) {
			// The still-held read itself failed, before park was ever
			// attempted — nothing was parked or torn down, the same as
			// StillHeldError, so this is a refusal too, not a warning
			// tacked onto a "yielded" that never happened.
			doc.Refuse(fmt.Sprintf(
				"could not confirm whether it is still held: %v",
				unconfirmed.Err))
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

// foreignYieldRefusal reports why a foreign hold with nothing of this
// lane's own to park must refuse, worded from foreignHoldRefusal the
// same way release words it, rather than let claim.Yield's own
// empty-local no-op read as a success it never performed. ok is false
// when yield should proceed to claim.Yield as usual: a plan nobody
// holds (plan.HoldTip == "") keeps its existing empty-local no-op and
// its cleanup of a stray local branch nobody holds remotely, and a
// non-empty local is this lane's own copy of the ref — fenced or
// still current — for claim.Yield's own checks to sort out.
func foreignYieldRefusal(plan discovery.Plan, local string) (string, bool) {
	if plan.HoldTip == "" || local != "" {
		return "", false
	}

	return foreignHoldRefusal(plan), true
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
	printGather(rt.stdout, doc.Gather)
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
		if doc.NextAction != "" {
			_, _ = fmt.Fprintf(out, "  %s\n", doc.NextAction)
		}
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
