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
	for _, p := range res.Plans {
		if !p.Unfinished() {
			continue
		}
		coord, ok := res.Coords[p.Repo]
		if !ok {
			continue
		}

		ctx, ok := ctxByRepo[p.Repo]
		if !ok {
			var ctxErr error
			ctx, ctxErr = newDriftRepoContext(coord.Path, coord.Base, rt.git)
			if ctxErr != nil {
				doc.AddProblem(p.Repo, ctxErr)
				ctx = driftRepoContext{ancestorLanded: map[int64]bool{}}
			}
			ctxByRepo[p.Repo] = ctx
		}

		commits := commitsNaming(ctx.commits, p.ID)
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
// every commit across every ref — the same walk driftCommits used to
// repeat once per not-done plan.
type driftRepoContext struct {
	preferred      string
	ancestorLanded map[int64]bool
	commits        []report.DriftCommit
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
		preferred: base, ancestorLanded: landed, commits: commits,
	}, nil
}

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
	work, err := claim.HasWork(path, id, ctx.preferred, tip, git)
	if err != nil || !work {
		return false
	}

	return claim.ContentLanded(path, ctx.preferred, tip, git)
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

// commitsNaming filters a repository's commits down to the ones whose
// message names the given plan id — the evidence a status flip is
// judged against. The id must appear as a whole run of digits, not
// merely a substring of some other number, or plan 500 would read
// evidence out of a commit that only ever mentioned 1500 or 45001.
func commitsNaming(commits []report.DriftCommit, id int64) []report.DriftCommit {
	pattern := regexp.MustCompile(`\b` + strconv.FormatInt(id, 10) + `\b`)
	named := []report.DriftCommit{}
	for _, c := range commits {
		if pattern.MatchString(c.Subject) {
			named = append(named, c)
		}
	}

	return named
}

// lastPhaseNumber is a plan's own final phase, or "" for a plan with
// no ledger — the number namesLastPhase looks for among its naming
// commits.
func lastPhaseNumber(phases []planmeta.Phase) string {
	if len(phases) == 0 {
		return ""
	}

	return string(phases[len(phases)-1].N)
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
