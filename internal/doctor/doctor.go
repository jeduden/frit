// Package doctor scans a repository's plan files for the semantic
// gaps frit now depends on: a missing `## Goal`, a phase with no
// `## Execution` row, and a phase tier that names no known model.
//
// The structural half of that — is a plan even shaped like
// plan/proto.md says — is mdsmith's own job. It already validates a
// plan against that schema and projects front matter through it; this
// package runs that validation through the imported
// github.com/jeduden/mdsmith/pkg/mdsmith library and reports the
// findings, rather than reimplementing a checker. Only the two checks
// mdsmith's schema has no way to see — a table cell's value, and a
// count of rows against another section's headings — are computed
// here, from the same planmeta.Plan every other verb already parses.
package doctor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jeduden/mdsmith/pkg/mdsmith"

	"github.com/jeduden/frit/internal/planmeta"
	"github.com/jeduden/frit/internal/plans"
)

// ErrNoSchema reports a repository whose plan directory carries no
// proto.md. doctor validates against a repository's own schema, so a
// repository with none has nothing to check — not an error, just an
// empty result the caller skips.
var ErrNoSchema = errors.New("no plan/proto.md: nothing to check")

// Finding is one semantic gap doctor found in one plan file.
type Finding struct {
	// ID is the plan's front-matter id.
	ID int64
	// Path is the plan file's path, relative to root.
	Path string
	// Check names which of doctor's checks produced this finding —
	// "goal", "execution-row", "tier", "schema" or "id-sync" —
	// matching the vocabulary the doctor --help text documents.
	Check string
	// Message is what is wrong, in prose. For "goal" and "schema" this
	// is mdsmith's own diagnostic message, carried through rather than
	// reworded.
	Message string
}

// Scan reads every plan file in root/planDir on disk and reports its
// semantic gaps, sorted by plan id then check so the same tree always
// reports in the same order.
func Scan(root, planDir string) ([]Finding, error) {
	protoPath := filepath.Join(root, planDir, planmeta.ProtoName)
	if _, err := os.Stat(protoPath); err != nil {
		return nil, ErrNoSchema
	}

	paths, err := planPaths(root, planDir)
	if err != nil {
		return nil, err
	}

	sess, err := mdsmith.NewSession(mdsmith.SessionOptions{
		Workspace: mdsmith.OSWorkspace{Root: root},
		Config:    mdsmith.ConfigPath(filepath.Join(root, ".mdsmith.yml")),
	})
	if err != nil {
		return nil, err
	}

	var out []Finding
	for _, p := range paths {
		findings, err := scanFile(sess, root, p)
		if err != nil {
			return nil, err
		}
		out = append(out, findings...)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}

		return out[i].Check < out[j].Check
	})

	return out, nil
}

// planPaths lists every plan on disk, flat or folder: a flat
// plan/*.md file, and each subdirectory's one fixed plan.md one level
// deep — the same shape off-refs discovery keeps, so doctor and
// discovery never drift on what counts as a folder plan. The folder
// half stays loose by design: any one-level <dir>/plan.md is a
// candidate, and a folder not named for its id is caught by the
// id-sync check below rather than excluded here.
func planPaths(root, planDir string) ([]string, error) {
	flat, err := filepath.Glob(filepath.Join(root, planDir, "[0-9]*_*.md"))
	if err != nil {
		return nil, err
	}

	folder, err := filepath.Glob(
		filepath.Join(root, planDir, "*", plans.FixedName))
	if err != nil {
		return nil, err
	}

	paths := append(flat, folder...)
	sort.Strings(paths)

	return paths, nil
}

// scanFile checks one plan file: planmeta's own body-derived checks,
// plus whatever of mdsmith's diagnostics doctor cares about. A file
// planmeta cannot parse as a plan is skipped rather than reported —
// that gap belongs to whichever verb reads the fleet index, not to a
// health report scoped to plans it can already read. A directory is
// skipped the same way: filepath.Glob matches directories as well as
// files, and the folder shape makes "plan.md" a name a stray
// directory can plausibly collide with, so one such entry must not
// fail the whole scan and lose every other plan's findings with it.
func scanFile(sess *mdsmith.Session, root, path string) ([]Finding, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}

	if info, err := os.Stat(path); err != nil || info.IsDir() {
		return nil, err
	}

	source, err := os.ReadFile(path) // #nosec G304 -- path comes from Glob under root
	if err != nil {
		return nil, err
	}

	plan, err := planmeta.Parse(source)
	if err != nil {
		return nil, nil
	}

	findings := checkPlan(plan, rel)
	if f := checkIDSync(plan.ID, rel); f != nil {
		findings = append(findings, *f)
	}

	diags, _ := sess.Check(rel, source)
	findings = append(findings, checkDiagnostics(plan.ID, rel, diags)...)

	return findings, nil
}

// checkPlan reports the two semantic gaps only frit can see: a phase
// whose Execution row is missing, and a phase whose Execution row
// names a tier frit does not recognize. A phase with no row already
// has no tier to check, so the two never double-report the same gap.
//
// The check reads Design and Implement directly rather than the
// collapsed Tier: mostDemandingTier picks whichever of the two ranks
// higher, so an unrecognized value in one column is invisible in Tier
// whenever the other column names a real model — a typo would hide
// behind a valid neighbor if this checked Tier alone.
func checkPlan(p planmeta.Plan, path string) []Finding {
	var out []Finding
	for _, ph := range p.Phases {
		if !ph.HasExecutionRow {
			out = append(out, Finding{
				ID: p.ID, Path: path, Check: "execution-row",
				Message: fmt.Sprintf(
					"phase %s has no Execution row: no tier, no gate", ph.N),
			})

			continue
		}
		if bad := badTier(ph.Design, ph.Implement); bad != "" {
			out = append(out, Finding{
				ID: p.ID, Path: path, Check: "tier",
				Message: fmt.Sprintf(
					"phase %s tier %q names no known model", ph.N, bad),
			})
		}
	}

	return out
}

// checkIDSync reports a plan whose on-disk name disagrees with its
// front-matter id: a flat file's stem, or a folder plan's folder name,
// carries the same <id>_<slug> convention and the same latent skew, so
// one check covers both shapes. Names are compared as strings, never
// parsed, so a non-numeric or missing prefix is simply a mismatch —
// never a crash, never a skip.
func checkIDSync(id int64, rel string) *Finding {
	token := leadingIDToken(rel)
	want := strconv.FormatInt(id, 10)
	if token == want {
		return nil
	}

	return &Finding{
		ID: id, Path: rel, Check: "id-sync",
		Message: fmt.Sprintf(
			"name %q does not match front-matter id %d", token, id),
	}
}

// leadingIDToken takes the id token a plan's on-disk name carries: a
// folder plan's folder name, or a flat plan's own file stem, up to its
// first underscore.
func leadingIDToken(rel string) string {
	name := filepath.Base(rel)
	if plans.IsFolderPlanFile(rel) {
		name = filepath.Base(filepath.Dir(rel))
	} else {
		name = strings.TrimSuffix(name, ".md")
	}

	if i := strings.IndexByte(name, '_'); i >= 0 {
		name = name[:i]
	}

	return name
}

// badTier returns the first of design or implement that names no
// known model, or "" when both do.
func badTier(design, implement string) string {
	if !planmeta.KnownTier(design) {
		return design
	}
	if !planmeta.KnownTier(implement) {
		return implement
	}

	return ""
}

// checkDiagnostics filters mdsmith's own findings to the two doctor
// promises: an empty `## Goal` (mdsmith's empty-section-body rule) and
// an invalid front-matter field against plan/proto.md's schema
// (mdsmith's required-structure rule, today only the model tier).
// Every other diagnostic — line length, heading style, and the rest of
// the repository's own .mdsmith.yml — is mdsmith's file-level job, not
// doctor's fleet-wide one.
func checkDiagnostics(id int64, path string, diags []mdsmith.Diagnostic) []Finding {
	var out []Finding
	for _, d := range diags {
		switch {
		case d.Name == "empty-section-body" &&
			strings.Contains(d.Message, `"## Goal"`):
			out = append(out,
				Finding{ID: id, Path: path, Check: "goal", Message: d.Message})
		case d.Name == "required-structure" &&
			strings.HasPrefix(d.Message, "model:"):
			out = append(out,
				Finding{ID: id, Path: path, Check: "schema", Message: d.Message})
		}
	}

	return out
}
