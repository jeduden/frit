package report

import "github.com/jeduden/frit/internal/herdr"

// The dispatch documents are what the ladder's mutating verbs report.
// Unlike the read-only board, these commands act — they focus a pane,
// send a composed prompt, or stand a lane up — so their documents say
// what was done, or under a dry run what would be, and never carry an
// agent's reply back.

// DispatchPlan names the plan a dispatch verb resolved: enough to
// confirm the right lane was targeted without re-reading the index.
type DispatchPlan struct {
	Repo  string `json:"repo"`
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// OpenDoc is what `frit open` did: the plan it resolved and the pane it
// raised, or the plain fact that no lane was live to raise.
//
// open is rung one, the read-only handoff. It sends no text and starts
// no agent, so the document carries a focus and nothing more.
type OpenDoc struct {
	header
	Root    string       `json:"root"`
	Plan    DispatchPlan `json:"plan"`
	Focused bool         `json:"focused"`
	Target  string       `json:"target"`
	Agent   string       `json:"agent"`
	Status  string       `json:"status"`
	Branch  string       `json:"branch"`
	// Problems carries a herdr frit could not reach. Presence is the one
	// thing open needs live, but a missing socket is reported, not
	// crashed on.
	Problems []Problem `json:"problems"`
}

// NewOpen opens a handoff report for a resolved plan.
func NewOpen(root string, repo string, id int64, title string) *OpenDoc {
	return &OpenDoc{
		header:   newHeader("open"),
		Root:     root,
		Plan:     DispatchPlan{Repo: repo, ID: id, Title: title},
		Problems: []Problem{},
	}
}

// Focus records the pane open raised and the lane it belongs to.
func (d *OpenDoc) Focus(lane herdr.Lane) {
	d.Focused = true
	d.Target = lane.Pane.PaneID
	d.Agent = lane.Pane.Agent
	d.Status = lane.Pane.Presence()
	d.Branch = lane.Branch
}

// AddProblem records a socket frit could not read.
func (d *OpenDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
