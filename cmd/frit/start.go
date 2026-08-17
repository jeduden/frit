package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/dispatch"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/repocfg"
	"github.com/jeduden/frit/internal/report"
)

// startTimeoutMS bounds how long herdr waits for a started agent to come
// up before failing. It is a wait on the agent's readiness, not on a
// reply — the escalation prompts only after the agent answers ready.
const startTimeoutMS = 120000

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

	sc, err := startResolve(c, rt, plan)
	if err != nil {
		return err
	}
	sp := composeStart(plan, phase, s.Note, sc)
	doc := report.NewStart(c.Root, plan.Repo, plan.ID, plan.Title, sp, s.Go)
	carryProblems(doc, res.Problems, c.All)

	if reason := claimRefusal(plan, discovery.Ready(res.Plans)); reason != "" {
		doc.Refuse(reason)
		return renderStart(c, rt, doc)
	}

	if s.Go {
		if err := startExecute(rt, doc, plan, sp, sc, s.Edit); err != nil {
			return err
		}
	}

	return renderStart(c, rt, doc)
}

// startContext is the repository state the escalation reads once: where
// the repository lives, its config, and the base a lease is dated
// against.
type startContext struct {
	repoPath string
	cfg      repocfg.Config
	base     string
}

// startResolve reads the repository path, its config, and the base ref —
// the inputs both the composition and the execution need, read once and
// shared.
func startResolve(
	c *cli, rt *runtime, plan discovery.Plan,
) (startContext, error) {
	repoPath, err := repoPathFor(c.Root, plan.Repo, rt.git)
	if err != nil {
		return startContext{}, err
	}
	cfg, err := repocfg.Load(repoPath)
	if err != nil {
		return startContext{}, err
	}
	base := cfg.Base
	if base == "" {
		base = gitobj.DefaultRef(repoPath, rt.git)
	}

	return startContext{repoPath: repoPath, cfg: cfg, base: base}, nil
}

// composeStart builds the escalation from the plan: the claim branch and
// its base, the worktree path herdr would use, the agent kind and tier,
// and the typed prompt with any note folded in. The composition is the
// same whether or not --go follows.
func composeStart(
	plan discovery.Plan, phase, note string, sc startContext,
) report.StartPlan {
	branch := claim.Branch(plan.ID, plan.Path)

	return report.StartPlan{
		Phase:  phase,
		Tier:   plan.Model,
		Kind:   "claude",
		Branch: branch,
		Base:   sc.base,
		Lane:   defaultLanePath(sc.repoPath, plan.ID, branch),
		Prompt: withNote(dispatch.Command(plan.ID, phase), note),
	}
}

// startExecute runs the escalation the composition describes: mint the
// claim, then hand the checkout, the agent and the pane to herdr in turn,
// and prompt it. Each mutation frit does not own is delegated — the
// worktree and the pane are herdr's — and no reply is ever read. A claim
// lost to another machine is carried as a refusal, not a fault.
func startExecute(
	rt *runtime, doc *report.StartDoc, plan discovery.Plan,
	sp report.StartPlan, sc startContext, edit bool,
) error {
	if _, err := claim.Mint(sc.repoPath, claim.Options{
		Branch:   sp.Branch,
		Base:     sp.Base,
		Remote:   sc.cfg.Remote,
		PlanID:   plan.ID,
		PlanFile: plan.Path,
		Lane:     sp.Lane,
		Host:     hostname(),
	}, rt.git); err != nil {
		if errors.Is(err, claim.ErrLostRace) {
			doc.Refuse("lost the race to another machine")
			return nil
		}

		return err
	}

	text := sp.Prompt
	if edit {
		edited, err := openEditor(text)
		if err != nil {
			return err
		}
		text = edited
		doc.Prompt = text
	}

	pane, err := standUpLane(rt, plan, sp, sc.repoPath, text)
	if err != nil {
		return err
	}
	doc.MarkStarted(pane)

	return nil
}

// standUpLane hands the checkout, the agent, the prompt and the focus to
// herdr in turn and returns the pane it opened. Every call here is
// herdr's — frit spawns nothing it does not hand straight over — and
// `agent read` is deliberately never among them.
func standUpLane(
	rt *runtime, plan discovery.Plan, sp report.StartPlan,
	repoPath, text string,
) (string, error) {
	pane, err := herdr.WorktreeCreate(rt.herdr, herdr.WorktreeSpec{
		CWD:    repoPath,
		Branch: sp.Branch,
		Base:   sp.Base,
		Path:   sp.Lane,
		Label:  fmt.Sprintf("plan %d", plan.ID),
	})
	if err != nil {
		return "", fmt.Errorf("worktree create: %w", err)
	}

	if err := herdr.AgentStart(rt.herdr, herdr.AgentSpec{
		Name:      fmt.Sprintf("plan-%d", plan.ID),
		Kind:      sp.Kind,
		Pane:      pane,
		Model:     sp.Tier,
		TimeoutMS: startTimeoutMS,
	}); err != nil {
		return "", fmt.Errorf("agent start: %w", err)
	}

	if err := herdr.Prompt(rt.herdr, pane, text); err != nil {
		return "", fmt.Errorf("prompt: %w", err)
	}
	if err := herdr.Focus(rt.herdr, pane); err != nil {
		return "", fmt.Errorf("focus: %w", err)
	}

	return pane, nil
}

// openEditor is the seam for --edit: it hands the composed prompt to
// $EDITOR and reads back what the human left. It is a package variable so
// a test can amend the prompt without a real editor on the machine.
var openEditor = editInEditor

// editInEditor writes the prompt to a temp file, opens it in $EDITOR, and
// returns the edited contents. It is the git-commit-message pattern: a
// prefilled template to amend, not an empty box.
func editInEditor(prompt string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	f, err := os.CreateTemp("", "frit-prompt-*.md")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.WriteString(prompt); err != nil {
		_ = f.Close()

		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	cmd := exec.Command(editor, f.Name())
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor: %w", err)
	}

	edited, err := os.ReadFile(f.Name())
	if err != nil {
		return "", err
	}

	return string(edited), nil
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

// printStart writes the escalation: the claim, worktree, agent, prompt
// and focus start ran or would run, or the reason it was refused. The
// whole plan is shown either way, because seeing the escalation is the
// point — under a dry run before it happens, under --go as the record of
// what did.
func printStart(out io.Writer, doc *report.StartDoc) {
	if doc.Refused != "" {
		_, _ = fmt.Fprintf(out, "refused: plan %d %s\n",
			doc.Plan.ID, doc.Refused)
		return
	}

	head := "start plan %d — %s  (dry run)\n"
	if doc.Started {
		head = "started plan %d — %s\n"
	}
	_, _ = fmt.Fprintf(out, head, doc.Plan.ID, doc.Plan.Title)
	_, _ = fmt.Fprintf(out, "  claim:    %s  (base %s)\n", doc.Branch, doc.Base)
	_, _ = fmt.Fprintf(out, "  worktree: %s\n", doc.Lane)
	_, _ = fmt.Fprintf(out, "  agent:    %s --model %s\n",
		doc.Kind, modelLabel(doc.Tier))
	_, _ = fmt.Fprintf(out, "  prompt:   %s\n", doc.Prompt)
	if doc.Started {
		_, _ = fmt.Fprintf(out, "  focus:    %s\n", doc.Pane)
		return
	}
	_, _ = fmt.Fprintln(out, "  focus:    the new pane")
	_, _ = fmt.Fprintln(out, "run again with --go to execute")
}
