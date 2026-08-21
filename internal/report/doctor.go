package report

import "github.com/jeduden/frit/internal/doctor"

// DoctorDoc is what `frit doctor` found: the plans across the fleet
// carrying a semantic gap. A repository walked but clean, or one with
// no plan/proto.md to check against, contributes no findings —
// doctor's table already omits a clean repository, the same
// convention orphans and stale use, so the document does too rather
// than padding it with empty entries.
type DoctorDoc struct {
	header
	Root     string          `json:"root"`
	Findings []DoctorFinding `json:"findings"`
	Problems []Problem       `json:"problems"`
}

// DoctorFinding is one semantic gap doctor found in one plan.
type DoctorFinding struct {
	Repo string `json:"repo"`
	ID   int64  `json:"id"`
	Path string `json:"path"`
	// Check names which of doctor's checks produced this finding —
	// see the doctor command's own --help for what each one means and
	// where it comes from.
	Check   string `json:"check"`
	Message string `json:"message"`
}

// NewDoctor opens a doctor report.
func NewDoctor(root string) *DoctorDoc {
	return &DoctorDoc{
		header:   newHeader("doctor"),
		Root:     root,
		Findings: []DoctorFinding{},
		Problems: []Problem{},
	}
}

// AddFindings records what one repository's plans turned up.
func (d *DoctorDoc) AddFindings(repo string, found []doctor.Finding) {
	for _, f := range found {
		d.Findings = append(d.Findings, DoctorFinding{
			Repo: repo, ID: f.ID, Path: f.Path,
			Check: f.Check, Message: f.Message,
		})
	}
}

// AddProblem records a repository doctor could not scan.
func (d *DoctorDoc) AddProblem(repo string, err error) {
	d.Problems = append(d.Problems, problemOf(repo, err))
}
