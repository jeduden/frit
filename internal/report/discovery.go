package report

import (
	"fmt"
	"time"

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
	// Stale marks a held plan whose takeover window matured: a takeover
	// candidate, with the observed unchanged span in seconds so a
	// consumer applies its own threshold without re-deriving it.
	Stale        bool  `json:"stale"`
	StaleSeconds int64 `json:"stale_seconds"`
	// Dead marks a held plan whose bound session herdr positively
	// confirms is gone: a takeover candidate at once, whether or not
	// Stale has also matured.
	Dead bool `json:"dead"`
}

// cardOf projects a discovery plan into its wire shape.
func cardOf(p discovery.Plan) PlanCard {
	return PlanCard{
		Key:          p.Key,
		Repo:         p.Repo,
		ID:           p.ID,
		Status:       p.Status,
		Title:        p.Title,
		Summary:      p.Summary,
		Model:        p.Model,
		DependsOn:    idsOf(p.DependsOn),
		Path:         p.Path,
		Held:         p.Held,
		Stale:        p.Stale,
		StaleSeconds: int64(p.StaleFor / time.Second),
		Dead:         p.Dead,
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
	gathered
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
	gathered
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
	gathered
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

// PhaseCard is one phase in the wire form: its number, title, own
// status, what its `## Execution` row names, and its own section's
// prose. The number is a string, since a phase may be 3b as well as
// 3. Tier and Gate are empty for a phase whose Execution row is
// missing — see NextDoc's Problems for that gap, rather than reading
// an empty Tier as "no tier asked for". Status is empty for a phase
// derived from a `## Phase N` heading rather than a front-matter
// ledger — section state carries no status, so none is invented.
type PhaseCard struct {
	N      string `json:"n"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Tier   string `json:"tier"`
	Gate   string `json:"gate"`
	Body   string `json:"body"`
}

// SourceDefaultBranch and SourceLane name where a next or show
// document's plan came from: the fleet's default-branch copy, the
// shared truth every other verb reads, or the resolved plan's own held
// lane before its work has merged into it.
const (
	SourceDefaultBranch = "default-branch"
	SourceLane          = "lane"
)

// NextDoc is what `frit next` found: a plan and the first phase of it
// not yet done — the phase an executor would pick up.
//
// HasPhase distinguishes "the next phase is this" from "there is no
// open phase": a plan already done, or one with no phase ledger at all,
// carries HasPhase false rather than a made-up phase zero.
type NextDoc struct {
	header
	gathered
	Root string `json:"root"`
	// Source names where Plan and Phase came from: SourceDefaultBranch
	// unless the cwd stood in the plan's own held lane, in which case
	// SourceLane.
	Source   string    `json:"source"`
	Plan     PlanCard  `json:"plan"`
	Phase    PhaseCard `json:"phase"`
	HasPhase bool      `json:"has_phase"`
	// Rescue lists the plan's rescue refs — where a scavenge or a
	// yield parked work that never landed — so stranded commits are
	// found again. Empty when nothing has ever been parked.
	Rescue   []string  `json:"rescue"`
	Problems []Problem `json:"problems"`
}

// NewNext opens a next-phase report for one resolved plan.
//
// A phase with no `## Execution` row is reported through Problems
// rather than rendered with a blank tier and gate as if the plan had
// asked for nothing: frit never fails over the gap, but it says so.
func NewNext(root string, plan discovery.Plan) *NextDoc {
	doc := &NextDoc{
		header:   newHeader("next"),
		Root:     root,
		Source:   SourceDefaultBranch,
		Plan:     cardOf(plan),
		Phase:    PhaseCard{},
		Rescue:   []string{},
		Problems: []Problem{},
	}
	if phase, ok := plan.NextPhase(); ok {
		doc.Phase = phaseCard(phase)
		doc.HasPhase = true
		if !phase.HasExecutionRow {
			doc.Problems = append(doc.Problems, Problem{
				Repo: plan.Repo,
				Message: fmt.Sprintf(
					"plan %d phase %s has no Execution row: no tier, no gate",
					plan.ID, phase.N),
			})
		}
	}

	return doc
}

// AddProblem records a repository whose plans could not be read.
func (d *NextDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// SetRescue records the plan's rescue refs.
func (d *NextDoc) SetRescue(refs []string) { d.Rescue = refs }

// SetSource records where the reported plan and phase came from.
func (d *NextDoc) SetSource(source string) { d.Source = source }

// phaseCard projects a phase into its wire shape.
func phaseCard(p planmeta.Phase) PhaseCard {
	return PhaseCard{
		N: string(p.N), Title: p.Title, Status: p.Status,
		Tier: p.Tier, Gate: p.Gate, Body: p.Body,
	}
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
	gathered
	Root string `json:"root"`
	// Source names where Goal and Tree's root plan came from:
	// SourceDefaultBranch unless the cwd stood in the shown plan's own
	// held lane, in which case SourceLane.
	Source string `json:"source"`
	// Goal is the shown plan's `## Goal`, read from its body. It is a
	// document-level fact because show is about one plan; the tree
	// underneath it is what blocks that plan. Empty when the plan
	// carries no Goal section.
	Goal string  `json:"goal"`
	Tree DepCard `json:"tree"`
	// Rescue lists the shown plan's rescue refs — where a scavenge or
	// a yield parked work that never landed — so stranded commits are
	// found again. Like Goal, it is a fact about the shown plan, not
	// the dependency tree beneath it.
	Rescue   []string  `json:"rescue"`
	Problems []Problem `json:"problems"`
}

// NewShow opens a dependency-walk report from a resolved tree.
func NewShow(root string, tree discovery.DepNode) *ShowDoc {
	return &ShowDoc{
		header:   newHeader("show"),
		Root:     root,
		Source:   SourceDefaultBranch,
		Goal:     tree.Plan.Goal,
		Tree:     depCard(tree),
		Rescue:   []string{},
		Problems: []Problem{},
	}
}

// AddProblem records a repository whose plans could not be read.
func (d *ShowDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// SetRescue records the shown plan's rescue refs.
func (d *ShowDoc) SetRescue(refs []string) { d.Rescue = refs }

// SetSource records where the reported Goal and Tree came from.
func (d *ShowDoc) SetSource(source string) { d.Source = source }

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
