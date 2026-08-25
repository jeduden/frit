package report

// DriftDoc is what `frit drift` found: for each not-done plan, the
// first-order git evidence a status flip is judged against — whether
// its work landed, and which commits name its id.
type DriftDoc struct {
	header
	Root     string     `json:"root"`
	Rows     []DriftRow `json:"rows"`
	Problems []Problem  `json:"problems"`
}

// DriftRow is one not-done plan's drift evidence.
type DriftRow struct {
	Repo   string `json:"repo"`
	ID     int64  `json:"id"`
	Landed bool   `json:"landed"`
	// Commits are the commits whose message names this plan's id,
	// newest first — the evidence a status flip is judged against, not
	// a verdict. frit reports it; the flip stays the caller's.
	Commits []DriftCommit `json:"commits"`
}

// DriftCommit is one commit whose message names a plan's id.
type DriftCommit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

// NewDrift opens a drift report.
func NewDrift(root string) *DriftDoc {
	return &DriftDoc{
		header:   newHeader("drift"),
		Root:     root,
		Rows:     []DriftRow{},
		Problems: []Problem{},
	}
}

// AddRow records one not-done plan's drift evidence.
func (d *DriftDoc) AddRow(
	repo string, id int64, landed bool, commits []DriftCommit,
) {
	if commits == nil {
		commits = []DriftCommit{}
	}
	d.Rows = append(d.Rows, DriftRow{
		Repo: repo, ID: id, Landed: landed, Commits: commits,
	})
}

// AddProblem records a repository drift could not read.
func (d *DriftDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
