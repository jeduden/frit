package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
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

	landedByRepo := map[string]map[int64]bool{}
	for _, p := range res.Plans {
		if !p.Unfinished() {
			continue
		}
		coord, ok := res.Coords[p.Repo]
		if !ok {
			continue
		}

		landed, ok := landedByRepo[p.Repo]
		if !ok {
			landed, err = driftLanded(coord.Path, rt.git)
			if err != nil {
				doc.AddProblem(p.Repo, err)
				landed = map[int64]bool{}
			}
			landedByRepo[p.Repo] = landed
		}

		commits, err := driftCommits(coord.Path, p.ID, rt.git)
		if err != nil {
			doc.AddProblem(p.Repo, err)
			continue
		}
		doc.AddRow(p.Repo, p.ID, landed[p.ID], commits)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printDrift(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// driftLanded reports, per plan id, whether a ref matching that
// plan's hold pattern has merged into the repository's default
// branch — the ancestor-merge half of the landed signal. A
// squash-merge leaves no such ref, and is not this function's
// concern.
func driftLanded(path string, git gitwt.Runner) (map[int64]bool, error) {
	cfg, err := repocfg.Load(path)
	if err != nil {
		return nil, err
	}
	holds, err := cfg.Compiled()
	if err != nil {
		return nil, err
	}
	preferred := gitobj.DefaultRef(path, git)
	refs, err := gitobj.Refs(path, git)
	if err != nil {
		return nil, err
	}
	merged, err := gitobj.MergedRefs(path, preferred, git)
	if err != nil {
		return nil, err
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

	return landed, nil
}

// driftCommits lists every commit, across every ref, whose message
// names the given plan id — the evidence a status flip is judged
// against, newest first as git log already orders it.
func driftCommits(
	path string, id int64, git gitwt.Runner,
) ([]report.DriftCommit, error) {
	out, err := git(path, "log", "--all", "--format="+driftLogFormat,
		"--grep="+strconv.FormatInt(id, 10))
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

// printDrift renders one row per not-done plan.
func printDrift(out io.Writer, doc *report.DriftDoc) {
	if len(doc.Rows) == 0 {
		_, _ = fmt.Fprintln(out, "no not-done plans found")
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range doc.Rows {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			r.Repo, r.ID, landedLabel(r.Landed), plural(len(r.Commits), "commit"))
	}
	_ = tw.Flush()
}

// landedLabel is a not-done plan's ancestor-merge signal, worded for a
// person.
func landedLabel(landed bool) string {
	if landed {
		return "landed"
	}

	return "not landed"
}
