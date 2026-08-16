// Package herdr reads live pane state from a herdr server.
//
// herdr owns panes, worktrees and prompts; frit only reads its socket
// to learn which lane has an agent on it right now. This package is
// the parser for that read and nothing more — it never sends a prompt,
// and it never reads an agent back.
package herdr

import (
	"encoding/json"
	"fmt"
)

// The agent_status values herdr reports. Working and idle are the
// ordinary states of an integrated agent; unknown is the honest third
// state for a pane whose agent frit cannot read, and it must never be
// collapsed into idle — a false idle invites dispatch onto an occupied
// lane.
const (
	StatusWorking = "working"
	StatusIdle    = "idle"
	StatusUnknown = "unknown"
)

// Pane is one record of `herdr agent list`: the agent on a pane, if
// any, and where that pane sits.
//
// A pane with no agent is kept rather than dropped. It is a real pane
// a person may be working in, and a board that hides it is lying about
// what the machine is doing.
type Pane struct {
	// Agent is the agent kind, e.g. "claude"; empty for a bare pane.
	Agent string
	// Status is agent_status: working, idle, unknown, or another value
	// a newer herdr introduces, carried through verbatim.
	Status string
	// CWD is the pane's shell directory. It drifts below the worktree
	// root as the shell cds around, so resolving a lane from it means
	// walking back up with git rather than trusting it directly.
	CWD string
	// ForegroundCWD is the working directory of the foreground process.
	ForegroundCWD string
	// Session is the agent session id, empty when there is no agent.
	Session string
	// PaneID identifies the pane within its server.
	PaneID string
	// Workspace is the workspace the pane belongs to.
	Workspace string
	// Title is the human label for the pane, with any animation glyph
	// already stripped.
	Title string
}

// HasAgent reports whether an agent is on the pane. A bare pane
// answers false, which is what separates a staffed lane from an empty
// terminal.
func (p Pane) HasAgent() bool {
	return p.Agent != ""
}

// envelope is the socket response wrapping the agent list.
type envelope struct {
	Result struct {
		Agents []rawPane `json:"agents"`
	} `json:"result"`
}

// rawPane is one wire record, decoded before it is narrowed to Pane.
type rawPane struct {
	Agent                 string `json:"agent"`
	AgentStatus           string `json:"agent_status"`
	CWD                   string `json:"cwd"`
	ForegroundCWD         string `json:"foreground_cwd"`
	PaneID                string `json:"pane_id"`
	WorkspaceID           string `json:"workspace_id"`
	TerminalTitle         string `json:"terminal_title"`
	TerminalTitleStripped string `json:"terminal_title_stripped"`
	AgentSession          *struct {
		Value string `json:"value"`
	} `json:"agent_session"`
}

// ParseAgentList turns a `herdr agent list` response into typed panes.
//
// Invalid JSON is an error the caller must be able to tell apart from
// an empty list: a garbled socket is a fault, an empty one is a
// running server with no panes.
func ParseAgentList(data []byte) ([]Pane, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("herdr agent list: %w", err)
	}

	panes := make([]Pane, 0, len(env.Result.Agents))
	for _, raw := range env.Result.Agents {
		panes = append(panes, raw.pane())
	}

	return panes, nil
}

// pane narrows a wire record to what frit reports.
func (r rawPane) pane() Pane {
	session := ""
	if r.AgentSession != nil {
		session = r.AgentSession.Value
	}

	return Pane{
		Agent:         r.Agent,
		Status:        r.AgentStatus,
		CWD:           r.CWD,
		ForegroundCWD: r.ForegroundCWD,
		Session:       session,
		PaneID:        r.PaneID,
		Workspace:     r.WorkspaceID,
		Title:         r.title(),
	}
}

// title prefers the stripped label, so the marker glyph a terminal
// paints for animation stays out of a board's output. It falls back
// to the raw title when no stripped form was sent.
func (r rawPane) title() string {
	if r.TerminalTitleStripped != "" {
		return r.TerminalTitleStripped
	}

	return r.TerminalTitle
}
