package report

import "github.com/jeduden/frit/internal/discovery"

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
