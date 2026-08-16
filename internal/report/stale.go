package report

import "github.com/jeduden/frit/internal/lanes"

// StaleDoc is what `frit stale` found: worktrees whose branch tip has
// not moved for longer than the cutoff. The cutoff travels with the
// report, because "41 days idle" only means abandoned against the
// threshold it was measured with.
//
// Presence records whether herdr was reachable. When it is, an idle
// branch with an agent still on it is told apart from one truly
// abandoned; when it is not, presence is unknown and no lane is called
// abandoned on a guess — the git answer stands on its own.
type StaleDoc struct {
	header
	Root     string      `json:"root"`
	Days     int         `json:"days"`
	Presence bool        `json:"presence"`
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
//
// HasAgent is meaningful only when the document's Presence is true. A
// stale branch with an agent on it is not abandoned, which is the
// distinction the whole report turns on once presence is known.
type Aged struct {
	Worktree   Worktree `json:"worktree"`
	AgeDays    int      `json:"age_days"`
	AgeSeconds int64    `json:"age_seconds"`
	HasAgent   bool     `json:"has_agent"`
}

// NewStale opens a staleness report measured against a cutoff in days.
// presence says whether live agent state was readable; when it is not,
// every lane's HasAgent stays false and the renderer says presence is
// unknown rather than calling anything abandoned.
func NewStale(root string, days int, presence bool) *StaleDoc {
	return &StaleDoc{
		header:   newHeader("stale"),
		Root:     root,
		Days:     days,
		Presence: presence,
		Repos:    []StaleRepo{},
		Problems: []Problem{},
	}
}

// AddRepo records one repository's idle checkouts, fresh or not. live
// is the set of worktree roots with an agent on them; a checkout whose
// path is in it is worked, not abandoned.
func (d *StaleDoc) AddRepo(
	name string, aged []lanes.Aged, live map[string]bool,
) {
	repo := StaleRepo{Name: name, Stale: make([]Aged, 0, len(aged))}

	for _, a := range aged {
		repo.Stale = append(repo.Stale, Aged{
			Worktree:   worktreeOf(a.Worktree),
			AgeDays:    int(a.Age.Hours() / 24),
			AgeSeconds: int64(a.Age.Seconds()),
			HasAgent:   live[a.Worktree.Path],
		})
	}

	d.Repos = append(d.Repos, repo)
}

// AddProblem records a repository whose ref times could not be read.
func (d *StaleDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
