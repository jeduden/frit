package main

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/jeduden/frit/internal/repocfg"
	"github.com/jeduden/frit/internal/report"
)

// driftLogFormat pairs a commit's sha with its subject, joined by a
// unit separator so an ordinary subject's own punctuation can never be
// mistaken for the field boundary — the explicit, porcelain format the
// git rule requires.
const driftLogFormat = "%H\x1f%s"

type driftCmd struct{}

// Run reports, for each not-done plan, whether its work ref has landed
// and which commits name its id — the git evidence a status flip is
// judged against. frit reports the evidence; the flip stays the
// caller's.
func (d *driftCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	doc := report.NewDrift(c.Root)
	carryProblems(doc, res.Problems, c.All)

	ctxByRepo := map[string]driftRepoContext{}
	failedRepo := map[string]bool{}
	for _, p := range res.Plans {
		if !p.Unfinished() {
			continue
		}
		coord, ok := res.Coords[p.Repo]
		if !ok || failedRepo[p.Repo] {
			continue
		}

		ctx, ok := ctxByRepo[p.Repo]
		if !ok {
			var ctxErr error
			ctx, ctxErr = newDriftRepoContext(coord.Path, coord.Base, rt.git)
			if ctxErr != nil {
				doc.AddProblem(p.Repo, ctxErr)
				failedRepo[p.Repo] = true
				continue
			}
			ctxByRepo[p.Repo] = ctx
		}

		commits := ctx.commitsNaming(p.ID)
		landed := ctx.landed(coord.Path, p.ID, p.HoldTip, rt.git)
		lastPhase := namesLastPhase(commits, lastPhaseNumber(p.Phases))
		doc.AddRow(p.Repo, p.ID, landed, lastPhase, commits)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printDrift(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// driftRepoContext is what the landed check needs, read once per
// repository rather than once per plan: the base a fresh tip is
// judged against, the ancestor-merge signal every ref carries, and
// every commit across every ref, bucketed by the id it names — the
// same walk and the same regex match commitsNaming used to repeat
// once per not-done plan.
type driftRepoContext struct {
	preferred      string
	ancestorLanded map[int64]bool
	byID           map[int64][]report.DriftCommit
}

// commitsNaming is a repo's commits naming the given plan id, newest
// first as the walk that built the bucket already ordered them — or
// an empty, never-nil slice for an id no commit names.
func (ctx driftRepoContext) commitsNaming(id int64) []report.DriftCommit {
	if commits, ok := ctx.byID[id]; ok {
		return commits
	}

	return []report.DriftCommit{}
}

// newDriftRepoContext reads the ids whose hold-pattern ref has already
// merged into base, and every commit across every ref — the two halves
// of the landed signal and the evidence a status flip is judged
// against. base is the coordinate's own — the same ref every other
// verb (claim, reap, orphans) judges landed against, honoring a
// repository's `base:` override in .frit.yml rather than re-deriving
// the default branch independently. A squash-merge leaves no merged
// ref, and is covered instead by landed's own content check.
func newDriftRepoContext(
	path, base string, git gitwt.Runner,
) (driftRepoContext, error) {
	cfg, err := repocfg.Load(path)
	if err != nil {
		return driftRepoContext{}, err
	}
	holds, err := cfg.Compiled()
	if err != nil {
		return driftRepoContext{}, err
	}
	refs, err := gitobj.Refs(path, git)
	if err != nil {
		return driftRepoContext{}, err
	}
	merged, err := gitobj.MergedRefs(path, base, git)
	if err != nil {
		return driftRepoContext{}, err
	}
	commits, err := allCommits(path, git)
	if err != nil {
		return driftRepoContext{}, err
	}

	landed := map[int64]bool{}
	for _, ref := range refs {
		if !merged[ref.Name] {
			continue
		}
		branch, ok := ref.Branch()
		if !ok {
			continue
		}
		id, ok := holds.Match(branch)
		if !ok {
			continue
		}
		landed[id] = true
	}

	return driftRepoContext{
		preferred: base, ancestorLanded: landed, byID: bucketByID(commits),
	}, nil
}

// bucketByID groups a repository's commits by every plan id their
// subject names, so a not-done plan's evidence is a map lookup rather
// than a fresh regex compiled and matched against every commit once
// per plan. A subject's id is a maximal run of ASCII digits — the
// same rule commitsNaming's old \b<id>\b match enforced, but decided
// by non-digit boundaries rather than Go's \b, which treats '_' as a
// word character and would miss an id immediately followed by one, as
// in a plan's own <id>_<slug>.md filename quoted in a commit subject.
func bucketByID(commits []report.DriftCommit) map[int64][]report.DriftCommit {
	byID := map[int64][]report.DriftCommit{}
	for _, c := range commits {
		for _, run := range digitRun.FindAllString(c.Subject, -1) {
			id, err := strconv.ParseInt(run, 10, 64)
			if err != nil {
				continue
			}
			byID[id] = append(byID[id], c)
		}
	}

	return byID
}

// digitRun matches a maximal run of ASCII digits — the boundary rule
// bucketByID needs; regexp's greedy match already stops at the first
// non-digit on either side, so no explicit \b is needed.
var digitRun = regexp.MustCompile(`\d+`)

// landed reports whether a plan's work reached the default branch:
// the ancestor-merge signal already read for the whole repository, or
// — when a hold tip exists and was not already caught by it — the
// squash-merge signal that a differing tip's content already matches
// the default branch's own tree. A tip with no real work beyond
// frit's own markers is never called landed on content alone, or a
// bare claim's trivial no-op merge would misread as finished work.
func (ctx driftRepoContext) landed(
	path string, id int64, tip string, git gitwt.Runner,
) bool {
	if ctx.ancestorLanded[id] {
		return true
	}
	if tip == "" || ctx.preferred == "" {
		return false
	}

	return claim.WorkLanded(path, id, ctx.preferred, tip, git)
}

// allCommits lists every commit across every ref, newest first as git
// log already orders it — read once per repository rather than once
// per not-done plan, since --all walks the whole commit graph however
// narrow the eventual filter.
func allCommits(path string, git gitwt.Runner) ([]report.DriftCommit, error) {
	out, err := git(path, "log", "--all", "--format="+driftLogFormat)
	if err != nil {
		return nil, err
	}

	commits := []report.DriftCommit{}
	for line := range strings.SplitSeq(string(out), "\n") {
		if line == "" {
			continue
		}
		sha, subject, ok := strings.Cut(line, "\x1f")
		if !ok {
			continue
		}
		commits = append(commits,
			report.DriftCommit{SHA: sha, Subject: subject})
	}

	return commits, nil
}

// lastPhaseNumber is a plan's own final phase — the highest-numbered
// one, not merely the last entry in the front matter's own phases
// list order, since nothing enforces that a plan's phases: block is
// written in ascending order — or "" for a plan with no ledger. It is
// the number namesLastPhase looks for among a plan's naming commits.
func lastPhaseNumber(phases []planmeta.Phase) string {
	best := ""
	bestN := -1
	for _, p := range phases {
		n, err := strconv.Atoi(string(p.N))
		if err != nil {
			continue
		}
		if n > bestN {
			bestN, best = n, string(p.N)
		}
	}
	if best == "" && len(phases) > 0 {
		// No phase number parsed as a plain integer (an unusual
		// non-numeric phase id) — fall back to the last entry rather
		// than reporting no last phase at all.
		return string(phases[len(phases)-1].N)
	}

	return best
}

// namesLastPhase reports whether some commit's subject names the
// given phase number — the mechanical "a commit for the last phase is
// present" signal the classification ladder looks for. It is not a
// verdict: whether that commit is really the close is still the
// caller's judgment.
func namesLastPhase(commits []report.DriftCommit, phase string) bool {
	if phase == "" {
		return false
	}
	pattern := regexp.MustCompile(`(?i)\bphase\s+` + regexp.QuoteMeta(phase) + `\b`)
	for _, c := range commits {
		if pattern.MatchString(c.Subject) {
			return true
		}
	}

	return false
}

// printDrift renders one row per not-done plan.
func printDrift(out io.Writer, doc *report.DriftDoc) {
	if len(doc.Rows) == 0 {
		_, _ = fmt.Fprintln(out, "no not-done plans found")
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range doc.Rows {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n",
			r.Repo, r.ID, landedLabel(r.Landed),
			lastPhaseLabel(r.LastPhaseCommit), plural(len(r.Commits), "commit"))
	}
	_ = tw.Flush()
}

// landedLabel is a not-done plan's landed signal, worded for a
// person.
func landedLabel(landed bool) string {
	if landed {
		return "landed"
	}

	return "not landed"
}

// lastPhaseLabel is the last-phase-commit flag, worded for a person.
func lastPhaseLabel(present bool) string {
	if present {
		return "last phase named"
	}

	return ""
}
