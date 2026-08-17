package main

import (
	"fmt"
	"io"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/report"
)

// The dispatch verbs climb the ladder above the read-only board: open
// hands a running pane over, and the rungs above it compose a typed
// slash command from the plan and send it. Three rules hold across all
// of them — the tool composes the prompt and the user never writes one,
// it sends then hands over and never reads a reply, and every rung that
// sends is dry-run until --go.

type openCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
}

// Run raises the pane the plan's lane is already running in — rung one,
// the read-only handoff. It sends no text and starts no agent; a plan
// nobody is working has no pane to raise, and open says so plainly
// rather than escalating on its own.
func (o *openCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}
	plan, err := resolveSelector(rt, o.Selector, res.Plans)
	if err != nil {
		return err
	}

	doc := report.NewOpen(c.Root, plan.Repo, plan.ID, plan.Title)

	lane, found, herdrErr := liveLaneFor(plan, rt)
	if herdrErr != nil {
		doc.AddProblem("herdr", herdrErr)
	}
	if found {
		if err := herdr.Focus(rt.herdr, lane.Pane.PaneID); err != nil {
			return fmt.Errorf("focus %s: %w", lane.Pane.PaneID, err)
		}
		doc.Focus(lane)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printOpen(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// liveLaneFor finds the pane working one of a plan's hold branches, if
// any is live. It reads herdr once and hands back the socket error
// rather than swallowing it, because open needs presence live: a
// missing socket is why there was nothing to open, and the caller
// carries it in the report.
func liveLaneFor(
	p discovery.Plan, rt *runtime,
) (herdr.Lane, bool, error) {
	panes, err := herdr.List(rt.herdr)
	if err != nil {
		return herdr.Lane{}, false, err
	}

	live := map[string]herdr.Lane{}
	for _, lane := range whoLanes(panes, rt.git) {
		if lane.Branch != "" {
			live[lane.Branch] = lane
		}
	}
	for _, branch := range p.Holds {
		if lane, ok := live[branch]; ok {
			return lane, true, nil
		}
	}

	return herdr.Lane{}, false, nil
}

// printOpen reports the pane open raised, or that no lane was live to
// raise. A plan with no live lane is not a failure — it is the signal to
// climb a rung, to start rather than open — so it is said plainly and
// the command still exits clean.
func printOpen(out io.Writer, doc *report.OpenDoc) {
	if !doc.Focused {
		_, _ = fmt.Fprintf(out,
			"no live lane for plan %d; nobody is working it\n", doc.Plan.ID)
		return
	}

	_, _ = fmt.Fprintf(out, "focused %s on plan %d (%s)\n",
		doc.Target, doc.Plan.ID, agentLabel(true, doc.Agent, doc.Status))
}
