package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/dispatch"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/herdr"
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
	plan, err := resolveSelector(rt, s.Selector, res.Plans, true)
	if err != nil {
		return err
	}

	return startResolved(c, rt, res, plan, s.Phase, s.Note, s.Edit, s.Go)
}

// startResolved composes and, under doGo, runs the escalation for a plan
// already chosen — whether start resolved it from a selector or pick
// ranked it to the top — and renders the result. A lost race is rendered
// as the refusal it is; only pick --go retries past it.
func startResolved(
	c *cli, rt *runtime, res fleet.Result, plan discovery.Plan,
	phaseSel, note string, edit, doGo bool,
) error {
	doc, _, err := buildStart(c, rt, res, plan, phaseSel, note, edit, doGo)
	if err != nil {
		return err
	}

	return renderStart(c, rt, doc)
}

// buildStart composes the escalation doc for a plan already chosen and,
// under doGo, runs start's claim-and-stand-up path. It refuses an
// unstartable plan and an ambiguous repository the same way for both
// verbs, so they cannot drift on what "startable" or "started" means.
// The bool is true when execution lost the claim's race — the one
// refusal pick --go retries past rather than reports.
func buildStart(
	c *cli, rt *runtime, res fleet.Result, plan discovery.Plan,
	phaseSel, note string, edit, doGo bool,
) (*report.StartDoc, bool, error) {
	phase, ok := dispatch.Phase(plan.Phases, phaseSel)
	if !ok {
		if len(plan.Phases) == 0 {
			return nil, false, fmt.Errorf(
				"plan %d carries no phase ledger; pass --phase", plan.ID)
		}

		return nil, false, fmt.Errorf(
			"plan %d has no open phase; pass --phase", plan.ID)
	}

	// The gather withholds a coordinate when two checkouts share the
	// plan's repository name; without one there is no repository to stand
	// the lane in, so a resume cannot be checked either.
	coord, coordOK := res.Coords[plan.Repo]
	resumeTip := startResumeTip(rt, plan, coord, coordOK)

	// Refuse before reading the repository off disk: a plan already held
	// or blocked needs no base, worktree path or git subprocess. A
	// resumable own lease skips this refusal — it is startable by
	// definition, whether or not its window has matured.
	if resumeTip == "" {
		if reason := claimRefusal(plan, discovery.Ready(res.Plans)); reason != "" {
			return refusedStart(c, res, plan, phase, doGo, reason), false, nil
		}
	}

	if !coordOK {
		return refusedStart(
			c, res, plan, phase, doGo, ambiguousRepo(plan.Repo)), false, nil
	}

	sc := startContextOf(coord)
	sp := composeStart(plan, phase, note, sc)
	doc := report.NewStart(c.Root, plan.Repo, plan.ID, plan.Title, sp, doGo)
	carryProblems(doc, res.Problems, c.All)
	if resumeTip != "" {
		doc.MarkResumed()
	}

	if doGo {
		if err := startExecute(rt, doc, plan, sp, sc, edit, resumeTip); err != nil {
			if errors.Is(err, claim.ErrLostRace) {
				doc.Refuse(lostRaceRefusal(err))

				return doc, true, nil
			}

			return nil, false, err
		}
	}

	return doc, false, nil
}

// startResumeTip resolves the lane's own lease from its persisted
// token, when start is run from that exact lane — ahead of the
// "already held" refusal, exactly as claim orders it (F9, F11, S3,
// S21). "" when the plan carries no matching coordinate, or none of
// the resume conditions hold; start's ordinary claim path is then the
// arbiter.
func startResumeTip(
	rt *runtime, plan discovery.Plan, coord fleet.Coord, coordOK bool,
) string {
	if !coordOK {
		return ""
	}
	cwd, _ := os.Getwd()
	_, tip, ok := resumeToken(rt, plan, coord, cwd)
	if !ok {
		return ""
	}

	return tip
}

// refusedStart composes the escalation doc for a plan buildStart is
// refusing before it reaches the repository — an unstartable plan, or
// one whose repository name is ambiguous across checkouts.
func refusedStart(
	c *cli, res fleet.Result, plan discovery.Plan,
	phase string, doGo bool, reason string,
) *report.StartDoc {
	doc := report.NewStart(c.Root, plan.Repo, plan.ID, plan.Title,
		report.StartPlan{Phase: phase, Tier: plan.Model, Kind: "claude"}, doGo)
	carryProblems(doc, res.Problems, c.All)
	doc.Refuse(reason)

	return doc
}

// startContext is the repository state the escalation reads once: where
// the repository lives, the remote a claim is pushed to, and the base a
// lease is dated against.
type startContext struct {
	repoPath string
	remote   string
	base     string
}

// startContextOf reads the escalation's inputs off the coordinate the
// gather already resolved — the repository path, its remote and the
// base — so start dates a lease from the one fleet walk rather than a
// second one. It reads the same coordinate claim mints from, so the two
// never disagree on where a lease is dated from.
func startContextOf(coord fleet.Coord) startContext {
	return startContext{
		repoPath: coord.Path,
		remote:   coord.Remote,
		base:     coord.Base,
	}
}

// composeStart builds the escalation from the plan: the claim branch and
// its base, the worktree path herdr would use, the agent kind and tier,
// and the typed prompt with any note folded in. The composition is the
// same whether or not --go follows.
func composeStart(
	plan discovery.Plan, phase, note string, sc startContext,
) report.StartPlan {
	return report.StartPlan{
		Phase:  phase,
		Tier:   plan.Model,
		Kind:   "claude",
		Branch: claim.Branch(plan.ID),
		Base:   sc.base,
		Lane:   defaultLanePath(sc.repoPath, plan.Path),
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
	sp report.StartPlan, sc startContext, edit bool, resumeTip string,
) error {
	// Amend the prompt before minting anything: an editor that fails to
	// launch, or a prompt left empty, must abort with no claim pushed and
	// no lane half-built. git aborts an empty commit message the same way.
	text := sp.Prompt
	if edit {
		edited, err := openEditor(text)
		if err != nil {
			return err
		}
		text = edited
		if strings.TrimSpace(text) == "" {
			return errors.New("the edited prompt is empty; nothing was started")
		}
		doc.Prompt = text
	}

	lease, err := startAcquire(rt, plan, sc, sp, resumeTip)
	if err != nil {
		// A lost race, or a veto, is returned, not swallowed: buildStart
		// records it as a refusal for start, and pick --go retries past it
		// to the next candidate. Every other error is a real fault.
		return err
	}

	pane, session, err := standUpLane(rt, plan, sp, sc.repoPath, text)
	if err != nil {
		// The lease is minted but nothing answers behind it. Release it —
		// a pushed marker, never a delete, so the next acquire reads epoch
		// E+1 — and name what the dead handoff left standing. If the
		// release itself fails the lease is still on the remote, so that is
		// reported alongside the handoff error rather than swallowed into a
		// silent orphan.
		err = handoffError(sp.Lane, pane, err)
		if relErr := releaseLease(rt, sc, plan, sp, lease.Tip); relErr != nil {
			return errors.Join(err, relErr)
		}

		return err
	}
	bindSession(rt, doc, plan, sp, sc, lease.Tip, session)
	doc.MarkStarted(pane)

	return nil
}

// startAcquire runs the transition start's escalation calls for: the
// self-resume on a matching persisted token when one was found, or
// otherwise the same mint-or-takeover claim already runs — a live
// bound session vetoes a stale hold, a matured one is seized, and a
// fresh plan is acquired outright. start meets the same lease protocol
// claim does; it just goes on to stand the lane up afterward.
func startAcquire(
	rt *runtime, plan discovery.Plan, sc startContext,
	sp report.StartPlan, resumeTip string,
) (claim.Lease, error) {
	opts := claim.LeaseOptions{
		PlanID: plan.ID,
		Remote: sc.remote,
		Base:   sp.Base,
		Holder: hostname(),
		Lane:   sp.Lane,
	}
	if resumeTip != "" {
		opts.Session = currentSession(rt)

		return claim.Resume(sc.repoPath, opts, resumeTip, rt.git)
	}

	coord := fleet.Coord{Path: sc.repoPath, Remote: sc.remote, Base: sc.base}

	return mintOrTakeOver(rt, plan, coord, opts)
}

// bindSession records the started agent's herdr session on the lease:
// a beat CASed from the tip the acquire just minted, carrying the
// session trailer, so a later takeover can ask herdr whether this
// lease's holder is still alive (F3, S61).
//
// A failed bind is a warning, never an abort: the lane is up and
// working, the lease is valid on the remote, and an unbound lease only
// forgoes the veto and falls back to the staleness window. Tearing a
// healthy lane down over a decoration would be the worse failure.
func bindSession(
	rt *runtime, doc *report.StartDoc, plan discovery.Plan,
	sp report.StartPlan, sc startContext, tip, session string,
) {
	if session == "" {
		return
	}
	if _, err := claim.Renew(sc.repoPath, claim.LeaseOptions{
		PlanID:  plan.ID,
		Remote:  sc.remote,
		Base:    sp.Base,
		Holder:  hostname(),
		Lane:    sp.Lane,
		Session: session,
	}, tip, rt.git); err != nil {
		doc.AddProblem(plan.Repo, fmt.Errorf(
			"bind session %s to %s: %w", session, sp.Branch, err))
	}
}

// handoffError names what a failed handoff left behind: the worktree
// and pane that stood up before the failure, so they can be found and
// torn down rather than guessed at. A handoff that died before herdr
// opened a pane left nothing standing and reports the cause alone.
func handoffError(lane, pane string, err error) error {
	if pane == "" {
		return err
	}

	return fmt.Errorf(
		"worktree %s and pane %s were stood up and are left behind: %w",
		lane, pane, err)
}

// releaseLease unwinds a lease minted before a handoff that then
// failed: the release transition, a pushed marker rather than a
// delete, so the plan frees while the history stays for the next
// acquire to CAS on. It returns nil when the release lands, and an
// error naming the still-held ref when it did not take — a failed
// unwind is surfaced and can be found, not left as a silent orphan for
// the next run to trip over.
func releaseLease(
	rt *runtime, sc startContext, plan discovery.Plan,
	sp report.StartPlan, tip string,
) error {
	if _, err := claim.Release(sc.repoPath, claim.LeaseOptions{
		PlanID: plan.ID,
		Remote: sc.remote,
		Base:   sp.Base,
		Holder: hostname(),
		Lane:   sp.Lane,
	}, tip, rt.git); err != nil {
		return fmt.Errorf(
			"lease %s could not be released and is left on the remote; "+
				"run frit orphans to find it: %w", sp.Branch, err)
	}

	return nil
}

// standUpLane hands the checkout, the agent, the prompt and the focus to
// herdr in turn and returns the pane it opened, and the herdr session
// the started agent was given — on failure too, once a pane exists, so
// the unwind can name what stood up. Every call here is herdr's — frit
// spawns nothing it does not hand straight over — and `agent read` is
// deliberately never among them.
func standUpLane(
	rt *runtime, plan discovery.Plan, sp report.StartPlan,
	repoPath, text string,
) (string, string, error) {
	pane, err := herdr.WorktreeCreate(rt.herdr, herdr.WorktreeSpec{
		CWD:    repoPath,
		Branch: sp.Branch,
		Base:   sp.Base,
		Path:   sp.Lane,
		Label:  fmt.Sprintf("plan %d", plan.ID),
	})
	if err != nil {
		return "", "", fmt.Errorf("worktree create: %w", err)
	}

	if err := herdr.AgentStart(rt.herdr, herdr.AgentSpec{
		Name:      fmt.Sprintf("plan-%d", plan.ID),
		Kind:      sp.Kind,
		Pane:      pane,
		Model:     sp.Tier,
		TimeoutMS: startTimeoutMS,
	}); err != nil {
		return pane, "", fmt.Errorf("agent start: %w", err)
	}
	// The first moment a session exists: herdr assigns one when the
	// agent starts, and neither this call nor worktree.create answers
	// with it, so it is read back off the same agent list every other
	// read uses. Best-effort — a lookup that fails costs the lease
	// only its herdr veto, not the lane.
	session := herdr.PaneSession(rt.herdr, pane)

	if err := herdr.Prompt(rt.herdr, pane, text); err != nil {
		return pane, session, fmt.Errorf("prompt: %w", err)
	}
	if err := herdr.Focus(rt.herdr, pane); err != nil {
		return pane, session, fmt.Errorf("focus: %w", err)
	}

	return pane, session, nil
}

// openEditor is the seam for --edit: it hands the composed prompt to
// $EDITOR and reads back what the human left. It is a package variable so
// a test can amend the prompt without a real editor on the machine.
var openEditor = editInEditor

// editInEditor writes the prompt to a temp file, opens it in $EDITOR, and
// returns the edited contents. It is the git-commit-message pattern: a
// prefilled template to amend, not an empty box.
func editInEditor(prompt string) (string, error) {
	// $VISUAL wins over $EDITOR, the shell convention, and both may carry
	// flags — "code --wait", "emacsclient -c" — so the value is split into
	// a command and its arguments rather than treated as one binary name.
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return "", errors.New("no editor set")
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

	args := append(fields[1:], f.Name())
	cmd := exec.Command(fields[0], args...)
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
	switch {
	case doc.Started && doc.Resumed:
		head = "resumed plan %d — %s\n"
	case doc.Started:
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
