package report

import "github.com/jeduden/frit/internal/herdr"

// WhoDoc is what `frit who` found: every lane with a live agent on it,
// resolved back to the plan it sits on.
//
// A pane that resolved to no plan is carried with an empty plan id
// rather than dropped, and an agent whose state frit could not read is
// carried as "unknown" rather than idle — a board that hides either is
// lying about what the fleet is doing.
type WhoDoc struct {
	header
	Root     string    `json:"root"`
	Lanes    []WhoLane `json:"lanes"`
	Problems []Problem `json:"problems"`
}

// WhoLane is one live agent: what it is, how it reads, and where it is
// working.
type WhoLane struct {
	Agent     string `json:"agent"`
	Status    string `json:"status"`
	Repo      string `json:"repo"`
	Root      string `json:"root"`
	Branch    string `json:"branch"`
	PlanID    int64  `json:"plan_id"`
	Workspace string `json:"workspace"`
	Session   string `json:"session"`
	Pane      string `json:"pane"`
	Title     string `json:"title"`
}

// NewWho opens a presence report.
func NewWho(root string) *WhoDoc {
	return &WhoDoc{
		header:   newHeader("who"),
		Root:     root,
		Lanes:    []WhoLane{},
		Problems: []Problem{},
	}
}

// AddLane records one resolved lane.
//
// Status comes from Presence, not the raw field, so the unknown state
// survives into the report and is never quietly rounded to idle.
func (d *WhoDoc) AddLane(l herdr.Lane) {
	d.Lanes = append(d.Lanes, WhoLane{
		Agent:     l.Pane.Agent,
		Status:    l.Pane.Presence(),
		Repo:      l.Repo,
		Root:      l.Root,
		Branch:    l.Branch,
		PlanID:    l.PlanID,
		Workspace: l.Pane.Workspace,
		Session:   l.Pane.Session,
		Pane:      l.Pane.PaneID,
		Title:     l.Pane.Title,
	})
}

// AddProblem records a socket frit could not read. An unreachable
// herdr is not fatal — the git board still answers — so the failure
// travels with the report rather than ending it.
func (d *WhoDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
