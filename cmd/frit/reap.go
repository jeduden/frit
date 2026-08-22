package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/jeduden/frit/internal/reap"
	"github.com/jeduden/frit/internal/report"
)

type reapCmd struct {
	Go bool `help:"Remove a landed checkout and delete its branch; without it, reap only prints what it would do."`
}

// Run tears down the kinds of orphan `frit orphans` already reports.
// This phase reaps a stranded lane: a checkout whose branch has
// landed, still standing. It is a dry-run by default and acts only on
// --go, exactly like nudge and start.
func (rc *reapCmd) Run(c *cli, rt *runtime) error {
	repos, err := discover.Repos(c.Root, rt.git)
	if err != nil {
		return err
	}

	doc := report.NewReap(c.Root, rc.Go)
	for _, repo := range repos {
		built, evidence, err := repoLanes(repo, rt)
		if err != nil {
			doc.AddProblem(repo.Name, err)
			continue
		}
		found := lanes.Find(built, repo.Worktrees)

		reaped, refused, err := reapStranded(rt, repo, found.Stranded, evidence, rc.Go)
		if err != nil {
			doc.AddProblem(repo.Name, err)
			continue
		}
		doc.AddRepo(repo.Name, reaped, refused)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printReap(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// reapStranded classifies and, under doGo, tears down every worktree
// of a repository's stranded lanes. The landed check reap deletes on
// is the same evidence lanes.Build already joined the claims against —
// re-checked here per worktree's own branch rather than trusted from
// the lane's stranded classification alone.
func reapStranded(
	rt *runtime, repo discover.Repo, stranded []lanes.Lane,
	evidence landedEvidence, doGo bool,
) ([]report.ReapedLane, []report.RefusedLane, error) {
	reaped := []report.ReapedLane{}
	refused := []report.RefusedLane{}

	for _, lane := range stranded {
		landed := func(branch string) bool {
			return evidence.Merged["refs/heads/"+branch] ||
				evidence.ByPlanID[lane.PlanID]
		}

		for _, d := range reap.Decide(lane.PlanID, lane.Worktrees, landed) {
			if d.Refused != "" {
				refused = append(refused, report.RefusedLane{
					PlanID: d.PlanID, Worktree: report.WorktreeOf(d.Worktree),
					Branch: d.Branch, Reason: d.Refused,
				})
				continue
			}

			if doGo {
				if err := tearDownWorktree(rt, repo, d); err != nil {
					return reaped, refused, err
				}
			}
			reaped = append(reaped, report.ReapedLane{
				PlanID: d.PlanID, Worktree: report.WorktreeOf(d.Worktree),
				Branch: d.Branch,
			})
		}
	}

	return reaped, refused, nil
}

// tearDownWorktree removes a landed checkout and deletes the branch it
// stood on, in that order: git refuses to delete a branch any
// worktree still has checked out, so the checkout must go first.
func tearDownWorktree(rt *runtime, repo discover.Repo, d reap.Decision) error {
	if _, err := rt.git(repo.Path,
		"worktree", "remove", d.Worktree.Path); err != nil {
		return err
	}
	if _, err := rt.git(repo.Path, "branch", "-D", d.Branch); err != nil {
		return err
	}

	return nil
}

// printReap writes a block per repository with something reaped or
// refused, worded for whether --go actually ran or reap only reports
// what it would do.
func printReap(out io.Writer, doc *report.ReapDoc) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	found := false
	verb := "would reap"
	if doc.Go {
		verb = "reaped"
	}

	for _, repo := range doc.Repos {
		if !repo.Any() {
			continue
		}
		found = true
		_, _ = fmt.Fprintf(tw, "%s\t\t\n", repo.Name)

		for _, lane := range repo.Reaped {
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\n",
				verb, lane.Worktree.Name, lane.Branch)
		}
		for _, lane := range repo.Refused {
			_, _ = fmt.Fprintf(tw, "  refused\t%s\t%s (%s)\n",
				lane.Worktree.Name, lane.Branch, lane.Reason)
		}
	}
	_ = tw.Flush()

	if !found {
		_, _ = fmt.Fprintln(out, "nothing to reap")
	}
}
