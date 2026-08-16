package report

import (
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/planmeta"
)

// PlanCard is one plan as the discovery listings show it. ready, pick
// and find all answer with a list of these, so a consumer written
// against one reads them all.
type PlanCard struct {
	Key       string  `json:"key"`
	Repo      string  `json:"repo"`
	ID        int64   `json:"id"`
	Status    string  `json:"status"`
	Title     string  `json:"title"`
	Summary   string  `json:"summary"`
	Model     string  `json:"model"`
	DependsOn []int64 `json:"depends_on"`
	Path      string  `json:"path"`
	Held      bool    `json:"held"`
}

// cardOf projects a discovery plan into its wire shape.
func cardOf(p discovery.Plan) PlanCard {
	return PlanCard{
		Key:       p.Key,
		Repo:      p.Repo,
		ID:        p.ID,
		Status:    p.Status,
		Title:     p.Title,
		Summary:   p.Summary,
		Model:     p.Model,
		DependsOn: idsOf(p.DependsOn),
		Path:      p.Path,
		Held:      p.Held,
	}
}

// cardsOf projects a list, returning [] rather than nil so the encoded
// form is a list and never null.
func cardsOf(plans []discovery.Plan) []PlanCard {
	out := make([]PlanCard, 0, len(plans))
	for _, p := range plans {
		out = append(out, cardOf(p))
	}

	return out
}

// ReadyDoc is what `frit ready` found: the plans startable now, across
// every repository and ref.
type ReadyDoc struct {
	header
	Root     string     `json:"root"`
	Host     string     `json:"host"`
	Plans    []PlanCard `json:"plans"`
	Problems []Problem  `json:"problems"`
}

// NewReady opens a readiness report.
func NewReady(root, host string) *ReadyDoc {
	return &ReadyDoc{
		header:   newHeader("ready"),
		Root:     root,
		Host:     host,
		Plans:    []PlanCard{},
		Problems: []Problem{},
	}
}

// SetPlans records the startable plans, in the order discovery ranked
// them.
func (d *ReadyDoc) SetPlans(plans []discovery.Plan) {
	d.Plans = cardsOf(plans)
}

// AddProblem records a repository whose plans could not be read.
func (d *ReadyDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// PickDoc is what `frit pick` found: the startable plans, ranked, and
// trimmed to the number asked for.
type PickDoc struct {
	header
	Root     string     `json:"root"`
	Host     string     `json:"host"`
	Plans    []PlanCard `json:"plans"`
	Problems []Problem  `json:"problems"`
}

// NewPick opens a ranked candidate report.
func NewPick(root, host string) *PickDoc {
	return &PickDoc{
		header:   newHeader("pick"),
		Root:     root,
		Host:     host,
		Plans:    []PlanCard{},
		Problems: []Problem{},
	}
}

// SetPlans records the ranked candidates, in the order discovery gave.
func (d *PickDoc) SetPlans(plans []discovery.Plan) {
	d.Plans = cardsOf(plans)
}

// AddProblem records a repository whose plans could not be read.
func (d *PickDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// FindDoc is what `frit find` found: the plans whose title or summary
// matched the query, across every repository and ref. The query is
// echoed so a consumer holding the document out of context knows what
// was asked.
type FindDoc struct {
	header
	Root     string     `json:"root"`
	Host     string     `json:"host"`
	Query    string     `json:"query"`
	Plans    []PlanCard `json:"plans"`
	Problems []Problem  `json:"problems"`
}

// NewFind opens a search report for one query.
func NewFind(root, host, query string) *FindDoc {
	return &FindDoc{
		header:   newHeader("find"),
		Root:     root,
		Host:     host,
		Query:    query,
		Plans:    []PlanCard{},
		Problems: []Problem{},
	}
}

// SetPlans records the matches, in the order discovery gave them.
func (d *FindDoc) SetPlans(plans []discovery.Plan) {
	d.Plans = cardsOf(plans)
}

// AddProblem records a repository whose plans could not be read.
func (d *FindDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// PhaseCard is one phase in the wire form: its number, title and its
// own status.
type PhaseCard struct {
	N      int    `json:"n"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// NextDoc is what `frit next` found: a plan and the first phase of it
// not yet done — the phase an executor would pick up.
//
// HasPhase distinguishes "the next phase is this" from "there is no
// open phase": a plan already done, or one with no phase ledger at all,
// carries HasPhase false rather than a made-up phase zero.
type NextDoc struct {
	header
	Root     string    `json:"root"`
	Plan     PlanCard  `json:"plan"`
	Phase    PhaseCard `json:"phase"`
	HasPhase bool      `json:"has_phase"`
	Problems []Problem `json:"problems"`
}

// NewNext opens a next-phase report for one resolved plan.
func NewNext(root string, plan discovery.Plan) *NextDoc {
	doc := &NextDoc{
		header:   newHeader("next"),
		Root:     root,
		Plan:     cardOf(plan),
		Phase:    PhaseCard{},
		Problems: []Problem{},
	}
	if phase, ok := plan.NextPhase(); ok {
		doc.Phase = phaseCard(phase)
		doc.HasPhase = true
	}

	return doc
}

// AddProblem records a repository whose plans could not be read.
func (d *NextDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// phaseCard projects a phase into its wire shape.
func phaseCard(p planmeta.Phase) PhaseCard {
	return PhaseCard{N: p.N, Title: p.Title, Status: p.Status}
}

// DepCard is one plan in the dependency walk. Found is false for an
// edge that resolved to no known plan, whose only sure fields are its
// id and repository.
type DepCard struct {
	Key    string    `json:"key"`
	Repo   string    `json:"repo"`
	ID     int64     `json:"id"`
	Status string    `json:"status"`
	Title  string    `json:"title"`
	Found  bool      `json:"found"`
	Deps   []DepCard `json:"deps"`
}

// ShowDoc is what `frit show --deps` found: the upstream dependency
// tree of one plan, so "what blocks this" has a direct answer.
type ShowDoc struct {
	header
	Root     string    `json:"root"`
	Tree     DepCard   `json:"tree"`
	Problems []Problem `json:"problems"`
}

// NewShow opens a dependency-walk report from a resolved tree.
func NewShow(root string, tree discovery.DepNode) *ShowDoc {
	return &ShowDoc{
		header:   newHeader("show"),
		Root:     root,
		Tree:     depCard(tree),
		Problems: []Problem{},
	}
}

// AddProblem records a repository whose plans could not be read.
func (d *ShowDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// depCard projects a dependency node and its subtree into wire shape,
// keeping the list empty rather than null at every level.
func depCard(n discovery.DepNode) DepCard {
	card := DepCard{
		Key:    n.Plan.Key,
		Repo:   n.Plan.Repo,
		ID:     n.Plan.ID,
		Status: n.Plan.Status,
		Title:  n.Plan.Title,
		Found:  n.Found,
		Deps:   make([]DepCard, 0, len(n.Deps)),
	}
	for _, child := range n.Deps {
		card.Deps = append(card.Deps, depCard(child))
	}

	return card
}
