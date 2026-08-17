package herdr

import (
	"encoding/json"
	"errors"
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
	args := []string{
		"worktree", "create", "--cwd", spec.CWD,
		"--branch", spec.Branch, "--base", spec.Base,
		"--path", spec.Path,
	}
	if spec.Label != "" {
		args = append(args, "--label", spec.Label)
	}
	args = append(args, "--no-focus", "--json")

	out, err := runner(args...)
	if err != nil {
		return "", err
	}

	return parseWorktreePane(out)
}

// parseWorktreePane reads the opened pane's id out of a worktree.create
// response. A response with no pane is an error rather than an empty
// target an agent would be started into.
func parseWorktreePane(data []byte) (string, error) {
	var env struct {
		Result struct {
			RootPane struct {
				PaneID string `json:"pane_id"`
			} `json:"root_pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", fmt.Errorf("herdr worktree create: %w", err)
	}
	if env.Result.RootPane.PaneID == "" {
		return "", errors.New("herdr worktree create: no pane in response")
	}

	return env.Result.RootPane.PaneID, nil
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
