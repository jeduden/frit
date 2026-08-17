package main

import (
	"fmt"
	"io"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/dispatch"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/repocfg"
	"github.com/jeduden/frit/internal/report"
)

type startCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
	Phase    string `help:"Phase to dispatch; default is the plan's next open phase."`
	Note     string `help:"A rider folded into the composed prompt before it is sent."`
	Edit     bool   `help:"Open the composed prompt in $EDITOR before sending it."`
	Go       bool   `help:"Run the escalation; without it, start only prints what it would do."`
}

// Run composes the full escalation — rung three — and, with --go, runs
// it: mint the claim, stand the worktree and agent up through herdr,
// prompt the agent, and focus the pane. It is dry-run by default: without
// --go it prints the whole plan and spawns nothing, so the escalation
// stays auditable before anything runs. A plan that is not startable is
// refused, and the tier is read from the plan, never chosen here.
//
// Every mutation is delegated where it is not frit's: frit mints the
// claim, and hands the worktree, the agent and the pane to herdr. It
// never reads a reply.
func (s *startCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}
	plan, err := resolveSelector(rt, s.Selector, res.Plans)
	if err != nil {
		return err
	}

	phase, ok := dispatch.Phase(plan.Phases, s.Phase)
	if !ok {
		if len(plan.Phases) == 0 {
			return fmt.Errorf(
				"plan %d carries no phase ledger; pass --phase", plan.ID)
		}

		return fmt.Errorf("plan %d has no open phase; pass --phase", plan.ID)
	}

	sp, err := composeStart(c, rt, plan, phase, s.Note)
	if err != nil {
		return err
	}
	doc := report.NewStart(c.Root, plan.Repo, plan.ID, plan.Title, sp, s.Go)
	carryProblems(doc, res.Problems, c.All)

	if reason := claimRefusal(plan, discovery.Ready(res.Plans)); reason != "" {
		doc.Refuse(reason)
	}

	return renderStart(c, rt, doc)
}

// composeStart builds the escalation from the plan: the claim branch and
// its base, the worktree path herdr would use, the agent kind and tier,
// and the typed prompt with any note folded in. It reads git and config
// but writes nothing — the composition is the same whether or not --go
// follows.
func composeStart(
	c *cli, rt *runtime, plan discovery.Plan, phase, note string,
) (report.StartPlan, error) {
	repoPath, err := repoPathFor(c.Root, plan.Repo, rt.git)
	if err != nil {
		return report.StartPlan{}, err
	}
	cfg, err := repocfg.Load(repoPath)
	if err != nil {
		return report.StartPlan{}, err
	}
	base := cfg.Base
	if base == "" {
		base = gitobj.DefaultRef(repoPath, rt.git)
	}

	branch := claim.Branch(plan.ID, plan.Path)

	return report.StartPlan{
		Phase:  phase,
		Tier:   plan.Model,
		Kind:   "claude",
		Branch: branch,
		Base:   base,
		Lane:   defaultLanePath(repoPath, plan.ID, branch),
		Prompt: withNote(dispatch.Command(plan.ID, phase), note),
	}, nil
}

// withNote folds a rider into the composed prompt as its own paragraph.
// The subject stays the tool's — a slash command naming a plan and a
// phase — and the note rides beneath it. An empty note changes nothing.
func withNote(prompt, note string) string {
	if note == "" {
		return prompt
	}

	return prompt + "\n\n" + note
}

// renderStart prints the escalation as a table or emits it as JSON.
func renderStart(c *cli, rt *runtime, doc *report.StartDoc) error {
	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printStart(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printStart writes the escalation plan: the claim, worktree, agent,
// prompt and focus start would run, or the reason it was refused. The
// whole plan is shown before anything is spawned, because seeing the
// escalation is the point of the dry run.
//
// The --go path that executes the plan is wired against a live herdr
// separately; until then --go prints the plan and spawns nothing, so the
// surface is honest about what it does today.
func printStart(out io.Writer, doc *report.StartDoc) {
	if doc.Refused != "" {
		_, _ = fmt.Fprintf(out, "refused: plan %d %s\n",
			doc.Plan.ID, doc.Refused)
		return
	}

	_, _ = fmt.Fprintf(out, "start plan %d — %s%s\n",
		doc.Plan.ID, doc.Plan.Title, dryRunTag(doc))
	_, _ = fmt.Fprintf(out, "  claim:    %s  (base %s)\n", doc.Branch, doc.Base)
	_, _ = fmt.Fprintf(out, "  worktree: %s\n", doc.Lane)
	_, _ = fmt.Fprintf(out, "  agent:    %s --model %s\n",
		doc.Kind, modelLabel(doc.Tier))
	_, _ = fmt.Fprintf(out, "  prompt:   %s\n", doc.Prompt)
	_, _ = fmt.Fprintln(out, "  focus:    the new pane")

	if doc.Go && !doc.Started {
		_, _ = fmt.Fprintln(out,
			"note: --go execution is being wired against herdr; nothing spawned")
		return
	}
	if !doc.Go {
		_, _ = fmt.Fprintln(out, "run again with --go to execute")
	}
}

// dryRunTag marks the header when nothing will be spawned.
func dryRunTag(doc *report.StartDoc) string {
	if doc.Started {
		return ""
	}

	return "  (dry run)"
}
