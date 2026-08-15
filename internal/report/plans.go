package report

import "github.com/jeduden/frit/internal/index"

// PlansDoc is what `frit plans` found: the plan index, one entry per
// plan per repository.
//
// The host is stated once at the top rather than repeated in every
// key, because a single run reads a single machine. It is still part
// of each plan's key, since plan ids only collide across repositories
// and hosts.
type PlansDoc struct {
	header
	Root     string     `json:"root"`
	Host     string     `json:"host"`
	Repos    []PlanRepo `json:"repos"`
	Problems []Problem  `json:"problems"`
}

// PlanRepo is one repository's plans.
type PlanRepo struct {
	Name  string `json:"name"`
	Plans []Plan `json:"plans"`
}

// Plan is one plan, read from its authoritative version.
//
// A plan can exist in several versions at once — the copy being edited
// on a branch, and the copy every other ref still carries. The fields
// here are the version the index ranked first, which is the copy on
// the default branch when there is one. RefCount and Versions describe
// the whole entry, so a consumer can see that a plan is contested
// without being handed every version of it.
type Plan struct {
	Key       string   `json:"key"`
	ID        int64    `json:"id"`
	Status    string   `json:"status"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Model     string   `json:"model"`
	DependsOn []int64  `json:"depends_on"`
	Path      string   `json:"path"`
	Refs      []string `json:"refs"`
	RefCount  int      `json:"ref_count"`
	Versions  int      `json:"versions"`
}

// Counts breaks a repository's plans down by status, for a renderer
// that summarises rather than lists.
func (r PlanRepo) Counts() map[string]int {
	counts := map[string]int{}
	for _, p := range r.Plans {
		counts[p.Status]++
	}

	return counts
}

// NewPlans opens a plan index for one host.
func NewPlans(root, host string) *PlansDoc {
	return &PlansDoc{
		header:   newHeader("plans"),
		Root:     root,
		Host:     host,
		Repos:    []PlanRepo{},
		Problems: []Problem{},
	}
}

// AddRepo records one repository's plans.
//
// Every plan is carried, whatever the table was asked to print:
// --detail decides how much a person is shown, while a consumer is
// always given the whole index. A repository with no plans is still
// recorded, so "walked and found nothing" stays distinguishable from
// "never walked".
func (d *PlansDoc) AddRepo(name string, entries []index.Entry) {
	repo := PlanRepo{Name: name, Plans: make([]Plan, 0, len(entries))}

	for _, e := range entries {
		v := e.Primary()
		repo.Plans = append(repo.Plans, Plan{
			Key:       e.Key.String(),
			ID:        e.Key.ID,
			Status:    v.Plan.Status,
			Title:     v.Plan.Title,
			Summary:   v.Plan.Summary,
			Model:     v.Plan.Model,
			DependsOn: idsOf(v.Plan.DependsOn),
			Path:      v.Path,
			Refs:      refsOf(v.Refs),
			RefCount:  e.RefCount(),
			Versions:  len(e.Versions),
		})
	}

	d.Repos = append(d.Repos, repo)
}

// AddProblem records a repository whose plans could not be read.
func (d *PlansDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}

// idsOf returns a list that encodes as [] rather than null.
func idsOf(in []int64) []int64 {
	if in == nil {
		return []int64{}
	}

	return in
}

// refsOf returns a list that encodes as [] rather than null.
func refsOf(in []string) []string {
	if in == nil {
		return []string{}
	}

	return in
}
