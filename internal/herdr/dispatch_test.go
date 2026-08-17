package herdr

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFocusTargetsThePane sends the pane straight to `agent focus` and
// nothing else — no text, no agent start. It is the read-only handoff.
func TestFocusTargetsThePane(t *testing.T) {
	var got []string
	runner := func(args ...string) ([]byte, error) {
		got = args

		return nil, nil
	}

	require.NoError(t, Focus(runner, "wC:p1"))
	assert.Equal(t, []string{"agent", "focus", "wC:p1"}, got)
}

// TestFocusReturnsTheRunnerError hands a failed focus straight up, so
// the command can report it rather than pretending the pane was raised.
func TestFocusReturnsTheRunnerError(t *testing.T) {
	want := errors.New("no such pane")
	err := Focus(func(...string) ([]byte, error) { return nil, want }, "gone")
	assert.ErrorIs(t, err, want)
}

// TestPromptSendsTheTextToTheTarget hands the composed slash command to
// the pane and nothing else. The text is one argument, so a command with
// spaces stays whole rather than splitting into flags.
func TestPromptSendsTheTextToTheTarget(t *testing.T) {
	var got []string
	runner := func(args ...string) ([]byte, error) {
		got = args

		return nil, nil
	}

	require.NoError(t, Prompt(runner, "wC:p1", "/plan-phase 7 2"))
	assert.Equal(t, []string{"agent", "prompt", "wC:p1", "/plan-phase 7 2"},
		got)
}

// TestPromptReturnsTheRunnerError surfaces a failed send rather than
// reporting text that never landed.
func TestPromptReturnsTheRunnerError(t *testing.T) {
	want := errors.New("agent busy")
	err := Prompt(func(...string) ([]byte, error) {
		return nil, want
	}, "wC:p1", "/plan-phase 7 2")
	assert.ErrorIs(t, err, want)
}

// TestWorktreeCreateReturnsTheRootPane reads the pane herdr opened out of
// the response — the pane a lane's agent is then started in — and asks
// for the checkout without stealing focus.
func TestWorktreeCreateReturnsTheRootPane(t *testing.T) {
	var got []string
	runner := func(args ...string) ([]byte, error) {
		got = args

		return []byte(`{"result":{"root_pane":{"pane_id":"wZ:p1"}}}`), nil
	}

	pane, err := WorktreeCreate(runner, WorktreeSpec{
		CWD: "/repo", Branch: "plan/7-x", Base: "origin/main",
		Path: "/repo-x", Label: "plan 7",
	})
	require.NoError(t, err)
	assert.Equal(t, "wZ:p1", pane)
	assert.Equal(t, []string{
		"worktree", "create", "--cwd", "/repo",
		"--branch", "plan/7-x", "--base", "origin/main",
		"--path", "/repo-x", "--label", "plan 7", "--no-focus", "--json",
	}, got)
}

// TestWorktreeCreateReportsAMissingPane treats a response with no pane as
// an error rather than starting an agent in an empty target.
func TestWorktreeCreateReportsAMissingPane(t *testing.T) {
	_, err := WorktreeCreate(func(...string) ([]byte, error) {
		return []byte(`{"result":{}}`), nil
	}, WorktreeSpec{CWD: "/repo", Branch: "b", Base: "main", Path: "/p"})
	assert.Error(t, err)
}

// TestAgentStartPassesTheTierAsAnArg: the tier the plan declares maps to
// a --model arg handed to the agent after the `--` separator, so dispatch
// is typed.
func TestAgentStartPassesTheTierAsAnArg(t *testing.T) {
	var got []string
	runner := func(args ...string) ([]byte, error) {
		got = args

		return nil, nil
	}

	require.NoError(t, AgentStart(runner, AgentSpec{
		Name: "plan-7", Kind: "claude", Pane: "wZ:p1",
		Model: "opus", TimeoutMS: 120000,
	}))
	assert.Equal(t, []string{
		"agent", "start", "plan-7", "--kind", "claude", "--pane", "wZ:p1",
		"--timeout", "120000", "--", "--model", "opus",
	}, got)
}

// TestAgentStartOmitsModelWhenEmpty: a plan that declares no tier starts
// the agent at its default rather than passing an empty --model.
func TestAgentStartOmitsModelWhenEmpty(t *testing.T) {
	var got []string
	runner := func(args ...string) ([]byte, error) {
		got = args

		return nil, nil
	}

	require.NoError(t, AgentStart(runner, AgentSpec{
		Name: "plan-7", Kind: "claude", Pane: "wZ:p1",
	}))
	assert.NotContains(t, got, "--model")
	assert.NotContains(t, got, "--")
}
