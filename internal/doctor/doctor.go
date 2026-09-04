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
	// "goal", "execution-row", "tier", "schema", "id-sync",
	// "phase-n-sync" or "handoff" — matching the vocabulary the doctor
	// --help text documents.
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

	return scanPaths(root, paths)
}

// ScanID re-checks the single plan whose on-disk name leads with id,
// read from root's own working copy — the seam frit doctor uses to
// re-read the one plan whose lane the cwd stands in, so a gap fixed in
// the lane clears before the branch merges, the way next, show and
// phase already read the lane. Every other plan is left to the fleet's
// default-branch scan. A repository with no proto.md reports ErrNoSchema
// exactly as Scan does.
func ScanID(root, planDir string, id int64) ([]Finding, error) {
	protoPath := filepath.Join(root, planDir, planmeta.ProtoName)
	if _, err := os.Stat(protoPath); err != nil {
		return nil, ErrNoSchema
	}

	paths, err := planPaths(root, planDir)
	if err != nil {
		return nil, err
	}

	want := strconv.FormatInt(id, 10)
	kept := paths[:0]
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil, err
		}
		if leadingIDToken(rel) == want {
			kept = append(kept, p)
		}
	}

	return scanPaths(root, kept)
}

// scanPaths opens root's mdsmith session once and scans each plan path
// through it, sorted by plan id then check so the same tree always
// reports in the same order — the shared body of Scan and ScanID.
func scanPaths(root string, paths []string) ([]Finding, error) {
	sess, err := openSession(root)
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

// openSession opens an mdsmith session against root's own .mdsmith.yml,
// the same rule config every other check in the repository runs
// against. A repository with no such file gets mdsmith's own built-in
// defaults rather than failing to open at all: mdsmith itself runs on
// those defaults with no config file present, and a repository frit
// indexes is not required to ship one.
func openSession(root string) (*mdsmith.Session, error) {
	cfg := mdsmith.ConfigSource(mdsmith.ConfigYAML(""))
	if _, err := os.Stat(filepath.Join(root, ".mdsmith.yml")); err == nil {
		cfg = mdsmith.ConfigPath(filepath.Join(root, ".mdsmith.yml"))
	}

	return mdsmith.NewSession(mdsmith.SessionOptions{
		Workspace: mdsmith.OSWorkspace{Root: root},
		Config:    cfg,
	})
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

	// A ledger-free folder plan carries its phases as separate
	// phase-*.md files, invisible to Parse, which sees only plan.md's
	// bytes. Assemble them from that directory so its Execution rows are
	// validated too. The ledger wins where present: this fills in only a
	// folder plan whose plan.md left Plan.Phases empty.
	if len(plan.Phases) == 0 && plans.IsFolderPlanFile(rel) {
		phases, err := planmeta.PhasesFromDir(filepath.Dir(path), source)
		if err != nil {
			return nil, err
		}
		plan.Phases = phases
	}

	findings := checkPlan(plan, rel)
	if f := checkIDSync(plan.ID, rel); f != nil {
		findings = append(findings, *f)
	}
	if plans.IsFolderPlanFile(rel) {
		nSync, err := checkPhaseNumberSync(plan.ID, rel, filepath.Dir(path))
		if err != nil {
			return nil, err
		}
		findings = append(findings, nSync...)
	}

	hs, err := checkHandoff(plan, rel, filepath.Dir(path), source, plans.IsFolderPlanFile(rel))
	if err != nil {
		return nil, err
	}
	findings = append(findings, hs...)

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

// checkPhaseNumberSync mirrors checkIDSync for a folder plan's own
// phase files: frit derives a phase's number from its phase-N.md (or
// phase-N.result.md) filename, while a generated `## Phases` catalog
// renders from front-matter `n` on both. One finding per divergent
// file, each pointing at the skewed file itself rather than plan.md,
// so a reader lands on the file to fix.
func checkPhaseNumberSync(id int64, planRel, dir string) ([]Finding, error) {
	mismatches, err := planmeta.PhaseFilenameMismatches(dir)
	if err != nil {
		return nil, err
	}

	out := make([]Finding, 0, len(mismatches))
	for _, m := range mismatches {
		name := planmeta.SpecFileName(m.FileToken)
		if m.Result {
			name = planmeta.ResultFileName(m.FileToken)
		}
		out = append(out, Finding{
			ID:    id,
			Path:  filepath.Join(filepath.Dir(planRel), name),
			Check: "phase-n-sync",
			Message: fmt.Sprintf(
				"%s front-matter n %q does not match its filename",
				name, m.FrontMatterN),
		})
	}

	return out, nil
}

// checkHandoff reports a phase recorded done whose handoff frit can
// find no readable trace of — the record plan-handoff writes inside
// the phase's own closing commit, and Resume reads back as the next
// phase's inherited context. A plan already `✅` or `⛔` needs no
// lingering handoff, since nothing resumes into a finished plan, so
// this only runs on one still in progress.
//
// A folder plan's handoff lives in its own phase-N.result.md, checked
// per phase since each phase owns its own file. A single-file plan's
// lives in one shared plan.md `## Handoff` heading, overwritten on
// every close, so one finding covers every done phase once that
// heading is missing rather than one per phase re-reporting the same
// gap.
func checkHandoff(p planmeta.Plan, planRel, dir string, planBody []byte, isFolder bool) ([]Finding, error) {
	if p.Status == planmeta.StatusDone || p.Status == planmeta.StatusSuperseded {
		return nil, nil
	}

	if isFolder {
		return checkFolderHandoff(p, planRel, dir)
	}

	var lastDone planmeta.PhaseNumber
	anyDone := false
	for _, ph := range p.Phases {
		if ph.Status == planmeta.StatusDone {
			anyDone, lastDone = true, ph.N
		}
	}
	if !anyDone {
		return nil, nil
	}
	if _, ok := planmeta.HandoffOf(planBody); ok {
		return nil, nil
	}

	return []Finding{{
		ID: p.ID, Path: planRel, Check: "handoff",
		Message: fmt.Sprintf(
			"phase %s recorded done has no readable ## Handoff in %s",
			lastDone, filepath.Base(planRel)),
	}}, nil
}

// checkFolderHandoff is checkHandoff's folder-plan half: one finding
// per done phase whose own phase-N.result.md carries no readable `##
// Handoff` — a missing file counts the same as an unreadable one,
// since a done phase's result file is expected to exist.
func checkFolderHandoff(p planmeta.Plan, planRel, dir string) ([]Finding, error) {
	var out []Finding
	for _, ph := range p.Phases {
		if ph.Status != planmeta.StatusDone {
			continue
		}
		name := planmeta.ResultFileName(string(ph.N))
		// #nosec G304 -- name comes from the plan's own phase ledger
		result, err := os.ReadFile(filepath.Join(dir, name))
		hasHandoff := false
		switch {
		case err == nil:
			_, hasHandoff = planmeta.HandoffOf(result)
		case !os.IsNotExist(err):
			return nil, err
		}
		if hasHandoff {
			continue
		}
		out = append(out, Finding{
			ID:    p.ID,
			Path:  filepath.Join(filepath.Dir(planRel), name),
			Check: "handoff",
			Message: fmt.Sprintf(
				"phase %s recorded done has no readable ## Handoff in %s",
				ph.N, name),
		})
	}

	return out, nil
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
