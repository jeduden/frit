package herdr

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// The dispatch surface: the mutating herdr calls frit composes but does
// not own. Each one hands a pane back to herdr and steps away — frit
// focuses, starts and prompts, but never reads an agent back. There is
// deliberately no wrapper here for `agent read`: the one call that would
// turn a board into a chat client has no home in this package.

// Focus raises the pane a lane is already running in. It sends no text
// and starts no agent — the read-only handoff, rung one of the ladder.
func Focus(runner Runner, target string) error {
	_, err := runner("agent", "focus", target)

	return err
}

// Prompt submits the composed slash command to the agent on a pane. The
// text is a single argument, so a command carrying spaces reaches herdr
// whole rather than split into flags. It sends and returns — it never
// waits for or reads the reply.
func Prompt(runner Runner, target, text string) error {
	_, err := runner("agent", "prompt", target, text)

	return err
}

// WorktreeSpec is the checkout to stand up: the branch to check out, the
// base to date it against, and where to put it.
type WorktreeSpec struct {
	CWD    string
	Branch string
	Base   string
	Path   string
	Label  string
}

// WorktreeCreate asks herdr to check a branch out into a worktree and
// returns the pane it opened — the pane an agent is then started in. It
// never steals focus: the escalation focuses the pane deliberately at the
// end, not as a side effect of creating it.
func WorktreeCreate(runner Runner, spec WorktreeSpec) (string, error) {
	return worktreePane(runner, "create", spec,
		"--branch", spec.Branch, "--base", spec.Base, "--path", spec.Path)
}

// WorktreeOpen asks herdr to put a checkout that already exists back on
// screen and returns the pane it came up in. It is worktree.create's
// counterpart for a lane frit is reattaching to rather than standing up:
// create refuses a path a worktree already occupies, which is every
// resume. No base is sent — nothing is being dated against anything —
// and, like create, it never steals focus.
func WorktreeOpen(runner Runner, spec WorktreeSpec) (string, error) {
	return worktreePane(runner, "open", spec, "--path", spec.Path)
}

// worktreePane runs one worktree verb — create or open — that answers
// with the pane it put on screen: the verb's own arguments after the
// working directory, then the label, no focus, and the JSON envelope
// both share, read by one parser told which verb to name.
func worktreePane(
	runner Runner, verb string, spec WorktreeSpec, verbArgs ...string,
) (string, error) {
	args := append([]string{"worktree", verb, "--cwd", spec.CWD}, verbArgs...)
	if spec.Label != "" {
		args = append(args, "--label", spec.Label)
	}
	args = append(args, "--no-focus", "--json")

	out, err := runner(args...)
	if err != nil {
		return "", err
	}

	return parseWorktreePane(out, "worktree "+verb)
}

// parseWorktreePane reads the opened pane's id out of a worktree.create
// or worktree.open response — one wire shape, so one reader, told which
// call to name in its error. A response with no pane is an error rather
// than an empty target an agent would be started into.
func parseWorktreePane(data []byte, verb string) (string, error) {
	var env struct {
		Result struct {
			RootPane struct {
				PaneID string `json:"pane_id"`
			} `json:"root_pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", fmt.Errorf("herdr %s: %w", verb, err)
	}
	if env.Result.RootPane.PaneID == "" {
		return "", fmt.Errorf("herdr %s: no pane in response", verb)
	}

	return env.Result.RootPane.PaneID, nil
}

// CurrentPane reads the pane a command is itself running in: metadata
// herdr already tracks about its own terminal, not an agent read. Yield
// uses it to find the workspace its own fenced lane is running in,
// since worktree.remove takes only that handle.
func CurrentPane(runner Runner) (Pane, error) {
	out, err := runner("pane", "current")
	if err != nil {
		return Pane{}, err
	}

	return parseCurrentPane(out)
}

// parseCurrentPane reads the pane out of a pane.current response,
// reusing the same narrowing agent list already does — the wire shape
// is the one rawPane record either way.
func parseCurrentPane(data []byte) (Pane, error) {
	var env struct {
		Result struct {
			Pane rawPane `json:"pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return Pane{}, fmt.Errorf("herdr pane current: %w", err)
	}

	return env.Result.Pane.pane(), nil
}

// WorktreeRemove asks herdr to tear a worktree checkout down by the
// workspace it is open in — the only handle worktree.remove takes, so
// a caller resolves the workspace (CurrentPane, for the lane it is
// itself running in) before calling this.
func WorktreeRemove(runner Runner, workspaceID string) error {
	_, err := runner("worktree", "remove", "--workspace", workspaceID, "--json")

	return err
}

// AgentSpec is the agent to start: its kind, the pane to start it in, the
// tier the plan declares, and how long to wait for it to come up.
type AgentSpec struct {
	Name      string
	Kind      string
	Pane      string
	Model     string
	TimeoutMS int
}

// AgentStart launches an agent in a pane herdr already opened. The tier
// is passed as a `--model` argument to the agent itself, after the `--`
// separator, so a plan that asks for opus gets opus. A plan declaring no
// tier starts the agent at its own default rather than passing an empty
// model. This waits on the agent coming up, not on a reply — it is not
// the forbidden read.
func AgentStart(runner Runner, spec AgentSpec) error {
	args := []string{
		"agent", "start", spec.Name,
		"--kind", spec.Kind, "--pane", spec.Pane,
	}
	if spec.TimeoutMS > 0 {
		args = append(args, "--timeout", strconv.Itoa(spec.TimeoutMS))
	}
	if spec.Model != "" {
		args = append(args, "--", "--model", spec.Model)
	}

	_, err := runner(args...)

	return err
}
