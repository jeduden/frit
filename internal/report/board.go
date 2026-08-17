package report

import (
	"strings"

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
	Root     string      `json:"root"`
	Presence bool        `json:"presence"`
	Plans    []BoardPlan `json:"plans"`
	Problems []Problem   `json:"problems"`
}

// BoardPlan is one outstanding plan: its status, who holds it, the
// machine it lives on, and the agent working it now if there is one.
type BoardPlan struct {
	Key         string   `json:"key"`
	Host        string   `json:"host"`
	Repo        string   `json:"repo"`
	ID          int64    `json:"id"`
	Status      string   `json:"status"`
	Title       string   `json:"title"`
	Model       string   `json:"model"`
	Held        bool     `json:"held"`
	Holds       []string `json:"holds"`
	Agent       string   `json:"agent"`
	AgentStatus string   `json:"agent_status"`
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
// empty when none is live on its lane.
func (d *BoardDoc) AddPlan(p discovery.Plan, agent, status string) {
	d.Plans = append(d.Plans, BoardPlan{
		Key:         p.Key,
		Host:        hostOf(p.Key),
		Repo:        p.Repo,
		ID:          p.ID,
		Status:      p.Status,
		Title:       p.Title,
		Model:       p.Model,
		Held:        p.Held,
		Holds:       refsOf(p.Holds),
		Agent:       agent,
		AgentStatus: status,
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
