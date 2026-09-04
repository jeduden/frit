package report

import (
	"strings"
	"time"

	"github.com/jeduden/frit/internal/discovery"
)

// BoardDoc is what `frit board` found: the outstanding plans, each with
// the lane that holds it and the agent live on it, if any.
//
// Presence reports whether herdr answered. With it false the agent
// column is unknown rather than empty — an unreachable socket is not
// the same as no agent, and the board says so instead of guessing.
type BoardDoc struct {
	header
	gathered
	Root     string      `json:"root"`
	Presence bool        `json:"presence"`
	Plans    []BoardPlan `json:"plans"`
	Problems []Problem   `json:"problems"`
}

// BoardPlan is one outstanding plan: its status, who holds it, the
// machine it lives on, and the agent working it now if there is one.
type BoardPlan struct {
	Key    string   `json:"key"`
	Host   string   `json:"host"`
	Repo   string   `json:"repo"`
	ID     int64    `json:"id"`
	Status string   `json:"status"`
	Title  string   `json:"title"`
	Model  string   `json:"model"`
	Held   bool     `json:"held"`
	Holds  []string `json:"holds"`
	// Stale marks a held plan whose takeover window matured: a
	// takeover candidate, with the observed unchanged span in seconds
	// so a consumer applies its own threshold without re-deriving it.
	Stale        bool  `json:"stale"`
	StaleSeconds int64 `json:"stale_seconds"`
	// Dead marks a held plan whose bound session herdr positively
	// confirms is gone AND whose branch no live pane attends: a
	// takeover candidate at once, whether or not Stale has also
	// matured. A pane still working or idling on the branch clears it
	// — "dead" reads to a person as "nobody is here", which a live
	// pane, working or idle, disproves. Agent/AgentStatus carry that
	// pane's own fact either way.
	Dead        bool   `json:"dead"`
	Agent       string `json:"agent"`
	AgentStatus string `json:"agent_status"`
	// Ask is the `frit message` invocation that asks the lane's agent
	// its status, set only when the bound session is confirmed gone
	// and an agent is live on the branch all the same — the lane git
	// cannot classify, whose work may be open as a PR. Empty for a
	// lane with no agent, and for one whose bound session is live.
	Ask string `json:"ask"`
}

// NewBoard opens a status board. presence carries whether herdr was
// reachable, so the agent column can tell unknown from absent.
func NewBoard(root string, presence bool) *BoardDoc {
	return &BoardDoc{
		header:   newHeader("board"),
		Root:     root,
		Presence: presence,
		Plans:    []BoardPlan{},
		Problems: []Problem{},
	}
}

// AddPlan records one outstanding plan, with the agent joined to it or
// empty when none is live on its lane. p.Dead is the identity fact —
// the bound session herdr confirms gone — but a live pane on the lane
// still working or idling means someone is there, so the rendered
// Dead is cleared whenever agent is non-empty rather than copied
// straight through. That same pairing — session gone, agent live — is
// the lane git cannot classify, so it is the one that carries Ask.
func (d *BoardDoc) AddPlan(p discovery.Plan, agent, status string) {
	d.Plans = append(d.Plans, BoardPlan{
		Key:          p.Key,
		Host:         hostOf(p.Key),
		Repo:         p.Repo,
		ID:           p.ID,
		Status:       p.Status,
		Title:        p.Title,
		Model:        p.Model,
		Held:         p.Held,
		Holds:        refsOf(p.Holds),
		Stale:        p.Stale,
		StaleSeconds: int64(p.StaleFor / time.Second),
		Dead:         p.Dead && agent == "",
		Agent:        agent,
		AgentStatus:  status,
		Ask:          askOf(p, agent != ""),
	})
}

// AddProblem records a repository whose plans could not be read.
func (d *BoardDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// hostOf pulls the machine out of a host:repo:id key, so each row names
// the machine its plan lives on even before multi-host fans the board
// across more than one.
func hostOf(key string) string {
	if i := strings.Index(key, ":"); i >= 0 {
		return key[:i]
	}

	return ""
}
