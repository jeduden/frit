package main

import (
	"fmt"
	"io"

	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/dispatch"
	"github.com/jeduden/frit/internal/fleet"
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
	plan, err := resolveSelector(rt, o.Selector, res.Plans, true)
	if err != nil {
		return err
	}

	doc := report.NewOpen(c.Root, plan.Repo, plan.ID, plan.Title)
	carryProblems(doc, res.Problems, c.All)

	lane, found, hostProbs, herdrErr := liveLaneFor(c, plan, rt)
	if herdrErr != nil {
		doc.AddProblem("herdr", herdrErr)
	}
	for _, p := range hostProbs {
		doc.AddProblem(p.name, p.err)
	}
	if presenceUnknown(herdrErr, hostProbs) {
		doc.PresenceUnknown()
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

// presenceUnknown decides whether open read live presence at all before
// it names a next action. A herdr it could not reach, or a configured
// host that answered with neither a live read nor a cache, leaves a lane
// possible behind the gap. A host served from stale cache is not that
// case — it still contributed its cached panes to the search, so a
// laneless result there is a real one and the start rung stands. So is a
// clean read that simply found no lane. Kept a pure function of the read
// outcomes so every case drives red/green without the remote read's ssh
// and wall clock.
func presenceUnknown(herdrErr error, hostProbs []hostProblem) bool {
	if herdrErr != nil {
		return true
	}
	for _, p := range hostProbs {
		if p.noPresence {
			return true
		}
	}

	return false
}

// liveLaneFor finds the pane working one of a plan's hold branches, if
// any is live. It reads herdr once and hands back the socket error
// rather than swallowing it, because open needs presence live: a
// missing socket is why there was nothing to open, and the caller
// carries it in the report.
//
// A hold branch name is repo-local — a plan id is only unique within a
// repository, so two repositories can carry the same branch name — so a
// live lane matches only when it sits in the plan's own repository. Its
// repository is resolved from the lane's worktree root, not the pane's
// linked-worktree basename, so a lane in a linked checkout still
// resolves to the repository that owns it. Without that check, an agent
// on an identically named branch elsewhere would be dispatched onto by
// mistake — the one error this whole join exists to prevent.
func liveLaneFor(
	c *cli, p discovery.Plan, rt *runtime,
) (herdr.Lane, bool, []hostProblem, error) {
	panes, probs, err := fleetPresence(c, rt)
	if err != nil {
		return herdr.Lane{}, false, nil, err
	}

	holds := map[string]bool{}
	for _, branch := range p.Holds {
		holds[branch] = true
	}

	for _, lane := range whoLanes(panes, rt.git) {
		if lane.Root == "" || lane.Branch == "" || !holds[lane.Branch] {
			continue
		}
		if fleet.RepoName(lane.Root, gitForHost(rt.git)(lane.Pane.Host)) == p.Repo {
			return lane, true, probs, nil
		}
	}

	return herdr.Lane{}, false, probs, nil
}

// printOpen reports the pane open raised, or that no lane was live to
// raise. A plan with no live lane is not a failure — it is the signal to
// climb a rung, to start rather than open — so it is said plainly and
// the command still exits clean. The message does not claim nobody is
// working the plan, because a herdr frit could not reach leaves that
// unknown; the unreachable socket travels as a problem instead.
func printOpen(out io.Writer, doc *report.OpenDoc) {
	if !doc.Focused {
		_, _ = fmt.Fprintf(out,
			"no live lane for plan %d to open\n", doc.Plan.ID)
		if doc.NextAction != "" {
			_, _ = fmt.Fprintf(out, "  start it with %s\n", doc.NextAction)
		}
		return
	}

	_, _ = fmt.Fprintf(out, "focused %s on plan %d (%s)\n",
		doc.Target, doc.Plan.ID, agentLabel(true, doc.Agent, doc.Status))
}

type nudgeCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
	Phase    string `help:"Phase to dispatch; default is the plan's next open phase."`
	Go       bool   `help:"Send the composed prompt; without it, nudge only prints what it would send."`
}

// Run composes the typed slash command from the plan's phase and, with
// --go, prompts it into the plan's idle lane — rung two. It is dry-run
// by default: without --go it prints the composition and the target and
// sends nothing. A lane that is working, or none live at all, is refused
// rather than interrupted, and the tier is read from the plan, never
// chosen here.
func (n *nudgeCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}
	plan, err := resolveSelector(rt, n.Selector, res.Plans, true)
	if err != nil {
		return err
	}

	phase, ok := dispatch.Phase(plan.Phases, n.Phase)
	if !ok {
		return fmt.Errorf(
			"plan %d has no open phase; pass --phase", plan.ID)
	}
	prompt := dispatch.Command(plan.ID, phase)

	doc := report.NewNudge(c.Root, plan.Repo, plan.ID, plan.Title,
		phase, plan.Model, prompt, n.Go)
	carryProblems(doc, res.Problems, c.All)

	lane, found, hostProbs, herdrErr := liveLaneFor(c, plan, rt)
	for _, p := range hostProbs {
		doc.AddProblem(p.name, p.err)
	}
	switch {
	case herdrErr != nil:
		// A socket frit could not reach is not "nobody is working it": it
		// is presence unknown, so refuse on that rather than on an absent
		// lane frit never actually looked for.
		doc.AddProblem("herdr", herdrErr)
		doc.Refuse("herdr unreachable")
	case presenceUnknown(herdrErr, hostProbs):
		// herdrErr is nil here, so a configured host went unread — no live
		// read and no cache. A lane may be live behind the gap, so refuse
		// on unread presence the way open withholds its action, not on an
		// absent lane.
		doc.Refuse("presence unknown: a configured host went unread")
	default:
		if err := nudgeSend(rt, n, doc, plan, lane, found, prompt); err != nil {
			return err
		}
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printNudge(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// nudgeSend applies the rung-two rules to the lane it found: refuse a
// plan with no live lane, refuse a lane that is not idle, and otherwise
// send only when --go was given. A send that fails is surfaced rather
// than reported as done.
func nudgeSend(
	rt *runtime, n *nudgeCmd, doc *report.NudgeDoc,
	plan discovery.Plan, lane herdr.Lane, found bool, prompt string,
) error {
	switch {
	case !found:
		doc.Refuse(fmt.Sprintf("no live lane for plan %d", plan.ID))
	case lane.Pane.Presence() != herdr.StatusIdle:
		doc.SetTarget(lane.Pane.PaneID)
		doc.Refuse(fmt.Sprintf("lane %s is %s, not idle",
			lane.Branch, lane.Pane.Presence()))
	default:
		doc.SetTarget(lane.Pane.PaneID)
		if n.Go {
			if err := herdr.Prompt(rt.herdr, lane.Pane.PaneID,
				prompt); err != nil {
				return fmt.Errorf("prompt %s: %w", lane.Pane.PaneID, err)
			}
			doc.MarkSent()
		}
	}

	return nil
}

// printNudge reports the composition and its fate: refused, sent, or —
// the default — held back for a --go that was not given. The slash
// command is always shown, because seeing exactly what would go is the
// point of the dry run.
func printNudge(out io.Writer, doc *report.NudgeDoc) {
	if doc.Refused != "" {
		verb := "would refuse"
		if doc.Go {
			verb = "refused"
		}
		_, _ = fmt.Fprintf(out, "%s: %s\n  %s  (%s)\n",
			verb, doc.Refused, doc.Prompt, modelLabel(doc.Tier))
		return
	}
	if doc.Sent {
		_, _ = fmt.Fprintf(out, "sent %s → %s\n", doc.Prompt, doc.Target)
		return
	}
	_, _ = fmt.Fprintf(out,
		"would send %s → %s  (%s)\nrun again with --go to send\n",
		doc.Prompt, doc.Target, modelLabel(doc.Tier))
}
