package main

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/report"
)

type claimCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
}

// Run mints frit's own hold on a plan — the atomic claim the dispatch
// ladder's rung 3 stands a lane on. It refuses a plan that is not
// startable: one already held, one blocked by an unfinished dependency,
// or one another machine wins the push for. On a pickable plan it leases
// the branch and reports the base it dated the lease against.
//
// The claim mints the ref and stops. The worktree and the pane are
// herdr's, reached by `frit start`; this command owns only the lease.
func (cc *claimCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}
	plan, err := resolveSelector(rt, cc.Selector, res.Plans)
	if err != nil {
		return err
	}

	branch := claim.Branch(plan.ID, plan.Path)
	doc := report.NewClaim(c.Root, plan.Repo, plan.ID, plan.Title, branch)
	carryProblems(doc, res.Problems, c.All)

	if reason := claimRefusal(plan, discovery.Ready(res.Plans)); reason != "" {
		doc.Refuse(reason)
		return renderClaim(c, rt, doc)
	}

	if err := mintClaim(rt, doc, plan, branch, res.Coords[plan.Repo]); err != nil {
		return err
	}

	return renderClaim(c, rt, doc)
}

// mintClaim writes the lease from the coordinate the gather already
// resolved — the repository path, its remote and the base — so the
// claim reads them off the one fleet walk rather than a second one. A
// lost race is the one expected non-fatal outcome — another machine got
// there first — so it is carried in the document as a refusal rather
// than surfaced as a command failure; a git fault is returned.
func mintClaim(
	rt *runtime, doc *report.ClaimDoc,
	plan discovery.Plan, branch string, coord fleet.Coord,
) error {
	minted, err := claim.Mint(coord.Path, claim.Options{
		Branch:   branch,
		Base:     coord.Base,
		Remote:   coord.Remote,
		PlanID:   plan.ID,
		PlanFile: plan.Path,
		Lane:     defaultLanePath(coord.Path, plan.ID, branch),
		Host:     hostname(),
	}, rt.git)
	if err != nil {
		if errors.Is(err, claim.ErrLostRace) {
			doc.Refuse("lost the race to another machine")
			return nil
		}
		return err
	}
	doc.Minted(minted.BaseSHA)

	return nil
}

// claimRefusal reports why a plan cannot be claimed, or "" when it can.
// A plan is claimable exactly when it is startable — not begun, held by
// nobody, every dependency done — so membership in the ready set is the
// test, and the reason names why a plan outside it is out.
func claimRefusal(p discovery.Plan, ready []discovery.Plan) string {
	for _, r := range ready {
		if r.Repo == p.Repo && r.ID == p.ID {
			return ""
		}
	}

	switch {
	case p.Held:
		return "already held (" + heldLabel(p.Holds) + ")"
	case p.Done():
		return "already done"
	case p.Superseded():
		return "superseded"
	case p.InProgress():
		return "already in progress"
	default:
		return fmt.Sprintf(
			"blocked by an unfinished dependency; see frit show %d", p.ID)
	}
}

// defaultLanePath is where the lane's worktree lives by convention: a
// sibling of the repository named for it, `<repo>-<slug>`. frit does not
// create it — that is herdr's worktree.create — but the marker records
// it so the lane's history names where the work will live, the same
// path the dispatch ladder hands to herdr.
func defaultLanePath(repoPath string, id int64, branch string) string {
	slug := strings.TrimPrefix(branch, fmt.Sprintf("plan/%d-", id))

	return filepath.Join(filepath.Dir(repoPath),
		filepath.Base(repoPath)+"-"+slug)
}

// renderClaim prints the claim as a table or emits it as JSON.
func renderClaim(c *cli, rt *runtime, doc *report.ClaimDoc) error {
	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printClaim(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printClaim reports the lease that was minted, or why one was not. A
// refusal is not a failure — it is the honest answer that the plan was
// not this run's to take — so the command still exits clean.
func printClaim(out io.Writer, doc *report.ClaimDoc) {
	if doc.Refused != "" {
		_, _ = fmt.Fprintf(out, "refused: plan %d %s\n",
			doc.Plan.ID, doc.Refused)
		return
	}

	_, _ = fmt.Fprintf(out, "claimed plan %d\n  branch: %s\n  base:   %s\n",
		doc.Plan.ID, doc.Branch, doc.Base)
}
