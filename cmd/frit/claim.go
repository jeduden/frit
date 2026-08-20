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
	"github.com/jeduden/frit/internal/herdr"
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

	// The gather withholds a coordinate when two checkouts share the
	// plan's repository name; without one there is no repository to mint
	// the lease in, so refuse rather than guess.
	coord, ok := res.Coords[plan.Repo]
	if !ok {
		doc.Refuse(ambiguousRepo(plan.Repo))
		return renderClaim(c, rt, doc)
	}

	if err := mintClaim(rt, doc, plan, branch, coord); err != nil {
		return err
	}
	if doc.Claimed {
		standUpClaimWorktree(rt, doc, plan, branch, coord)
	}

	return renderClaim(c, rt, doc)
}

// standUpClaimWorktree hands the freshly claimed lane's checkout to
// herdr, so an agent works it in a worktree of its own rather than in
// the shared clone. The lease is already atomic and minted; a herdr that
// cannot stand the worktree up is recorded as a warning, not a failure,
// so a lost checkout never reads as a lost claim. The agent and its
// prompt stay start's rung — claim stands up the checkout only.
func standUpClaimWorktree(
	rt *runtime, doc *report.ClaimDoc,
	plan discovery.Plan, branch string, coord fleet.Coord,
) {
	path := defaultLanePath(coord.Path, plan.ID, branch)
	if _, err := herdr.WorktreeCreate(rt.herdr, herdr.WorktreeSpec{
		CWD:    coord.Path,
		Branch: branch,
		Base:   coord.Base,
		Path:   path,
		Label:  fmt.Sprintf("plan %d", plan.ID),
	}); err != nil {
		doc.Warn(fmt.Sprintf("worktree not stood up: %v", err))
		return
	}
	doc.Stood(path)
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
			doc.Refuse(lostRaceRefusal(err))
			return nil
		}
		return err
	}
	doc.Minted(minted.BaseSHA)

	return nil
}

// lostRaceRefusal names who holds a plan whose claim push lost, from the
// holder facts claim.Mint read off the winning ref's marker. A branch
// that already landed, a claim held on this host, and one held elsewhere
// each read differently; an unread marker falls back to the original
// wording so a missing or malformed body never changes the outcome.
func lostRaceRefusal(err error) string {
	var lost *claim.LostRaceError
	if !errors.As(err, &lost) || !lost.Holder.Known {
		return "lost the race to another machine"
	}

	switch h := lost.Holder; {
	case h.Landed:
		return fmt.Sprintf(
			"the claim branch has already landed; its status is still open, "+
				"so set plan %d to ✅", lost.PlanID)
	case h.ThisHost:
		return fmt.Sprintf("already held on this host (%s)", h.Host)
	case h.Host != "":
		return fmt.Sprintf("lost the race to another machine (%s)", h.Host)
	default:
		// The marker was read but carried no host — a malformed body. Name
		// no machine rather than print an empty pair of parentheses.
		return "lost the race to another machine"
	}
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
	if doc.Worktree != "" {
		_, _ = fmt.Fprintf(out, "  worktree: %s\n", doc.Worktree)
	}
	if doc.Warning != "" {
		_, _ = fmt.Fprintf(out, "  warning: %s\n", doc.Warning)
	}
}
