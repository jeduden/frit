package report

import "github.com/jeduden/frit/internal/lanes"

// StaleDoc is what `frit stale` found: worktrees whose branch tip has
// not moved for longer than the cutoff. The cutoff travels with the
// report, because "41 days idle" only means abandoned against the
// threshold it was measured with.
type StaleDoc struct {
	header
	Root     string      `json:"root"`
	Days     int         `json:"days"`
	Repos    []StaleRepo `json:"repos"`
	Problems []Problem   `json:"problems"`
}

// StaleRepo is one repository's idle checkouts.
type StaleRepo struct {
	Name  string `json:"name"`
	Stale []Aged `json:"stale"`
}

// Aged is one worktree and how long its branch has stood still.
//
// The age is stated twice on purpose: whole days are what a person
// reads, and seconds are what a consumer applies its own threshold to
// without having to guess how the days were rounded.
type Aged struct {
	Worktree   Worktree `json:"worktree"`
	AgeDays    int      `json:"age_days"`
	AgeSeconds int64    `json:"age_seconds"`
}

// NewStale opens a staleness report measured against a cutoff in days.
func NewStale(root string, days int) *StaleDoc {
	return &StaleDoc{
		header:   newHeader("stale"),
		Root:     root,
		Days:     days,
		Repos:    []StaleRepo{},
		Problems: []Problem{},
	}
}

// AddRepo records one repository's idle checkouts, fresh or not.
func (d *StaleDoc) AddRepo(name string, aged []lanes.Aged) {
	repo := StaleRepo{Name: name, Stale: make([]Aged, 0, len(aged))}

	for _, a := range aged {
		repo.Stale = append(repo.Stale, Aged{
			Worktree:   worktreeOf(a.Worktree),
			AgeDays:    int(a.Age.Hours() / 24),
			AgeSeconds: int64(a.Age.Seconds()),
		})
	}

	d.Repos = append(d.Repos, repo)
}

// AddProblem records a repository whose ref times could not be read.
func (d *StaleDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
