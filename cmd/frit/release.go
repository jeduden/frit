package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/report"
)

type releaseCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
}

// Run ends the calling lane's own lease: a release marker pushed from
// its persisted token, the same proof resume reads (F9, F11, S3, S21).
// A plan nobody holds, or one whose hold already reads as released, is
// a no-op rather than a refusal — there is nothing left to end. A plan
// held live or matured by another lane is refused: only that lane's
// own token can release it; a stranger takes a matured one over
// through claim instead, never through release. A hold whose work
// already landed is scavenged rather than released.
func (rc *releaseCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}
	plan, err := resolveSelector(rt, rc.Selector, res.Plans, true)
	if err != nil {
		return err
	}

	branch := claim.Branch(plan.ID)
	doc := report.NewRelease(c.Root, plan.Repo, plan.ID, plan.Title, branch)
	carryProblems(doc, res.Problems, c.All)

	coord, ok := res.Coords[plan.Repo]
	if !ok {
		doc.Refuse(ambiguousRepo(plan.Repo))

		return renderRelease(c, rt, doc)
	}

	switch {
	case plan.HoldTip == "":
		doc.Nothing("nothing holds it")
	case !plan.Held:
		releaseUnheld(rt, doc, plan, coord)
	default:
		releaseHeld(rt, doc, plan, coord)
	}

	return renderRelease(c, rt, doc)
}

// releaseUnheld handles a work ref the hold filters already read as
// unheld: either its tip is a release marker — a lease that already
// ended, a no-op to release again — or its work already landed on the
// base, which the hold filters read the same way merged evidence
// always does; that state is scavenged, not released.
func releaseUnheld(
	rt *runtime, doc *report.ReleaseDoc, plan discovery.Plan, coord fleet.Coord,
) {
	if claim.Released(coord.Path, plan.HoldTip, plan.ID, rt.git) {
		doc.Nothing("already released")

		return
	}
	scavengeRef(rt, doc, plan, coord, plan.HoldTip)
}

// releaseHeld handles a live hold: this lane's own, proved by its
// persisted token, is released; anything else is refused, worded by
// whether its window has matured — a stranger's matured hold is
// claim's takeover to make, never release's to wait on.
func releaseHeld(
	rt *runtime, doc *report.ReleaseDoc, plan discovery.Plan, coord fleet.Coord,
) {
	cwd, _ := os.Getwd()
	lane, tip, ok := ownToken(rt, plan, coord, cwd)
	if !ok {
		doc.Refuse(foreignHoldRefusal(plan))

		return
	}

	opts := claim.LeaseOptions{
		PlanID: plan.ID, Remote: coord.Remote, Base: coord.Base,
		Holder: hostname(), Lane: lane,
	}
	if _, err := claim.Release(coord.Path, opts, tip, rt.git); err != nil {
		doc.Warn(fmt.Sprintf("release: %v", err))

		return
	}
	doc.MarkReleased()
}

// foreignHoldRefusal names why a hold this lane's own token does not
// match is left standing: a live one names the holder, and a matured
// window or a bound session herdr confirms gone both point at claim's
// takeover instead — release never seizes a lease that is not its
// own, whatever its window or session says.
func foreignHoldRefusal(plan discovery.Plan) string {
	switch {
	case plan.Stale:
		return "hold has matured; run `frit claim` to take it over " +
			"rather than wait on a release that will not come"
	case plan.Dead:
		return "the bound session is confirmed gone; run `frit claim` " +
			"to take it over rather than wait on a release that will not come"
	}

	return "is held live by another lane (" + heldLabel(plan.Holds) +
		"); only its own lane can release it"
}

// renderRelease prints the release as a table or emits it as JSON.
func renderRelease(c *cli, rt *runtime, doc *report.ReleaseDoc) error {
	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printRelease(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printRelease reports what release did, or why nothing changed —
// refused, a no-op, or the landed evidence it scavenged instead.
func printRelease(out io.Writer, doc *report.ReleaseDoc) {
	switch {
	case doc.Refused != "":
		_, _ = fmt.Fprintf(out, "refused: plan %d %s\n",
			doc.Plan.ID, doc.Refused)
	case doc.NoOp != "":
		_, _ = fmt.Fprintf(out, "plan %d: %s\n", doc.Plan.ID, doc.NoOp)
	case doc.Released:
		_, _ = fmt.Fprintf(out, "released plan %d\n  branch: %s\n",
			doc.Plan.ID, doc.Branch)
	case doc.Scavenged != "":
		_, _ = fmt.Fprintf(out,
			"plan %d: hold already landed; scavenged %s\n",
			doc.Plan.ID, doc.Scavenged)
	}
	if doc.Rescue != "" {
		_, _ = fmt.Fprintf(out, "  rescued: %s\n", doc.Rescue)
	}
	if doc.Warning != "" {
		_, _ = fmt.Fprintf(out, "  warning: %s\n", doc.Warning)
	}
}
