package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/jeduden/frit/internal/reap"
	"github.com/jeduden/frit/internal/repocfg"
	"github.com/jeduden/frit/internal/report"
)

type reapCmd struct {
	Go bool `help:"Tear a stranded or unstaffed lane down; without it, reap only prints what it would do."`
}

// Run tears down every kind of orphan `frit orphans` already reports:
// a stranded checkout, an unstaffed hold, a prunable or never-started
// worktree. It is a dry-run by default and acts only on --go, exactly
// like nudge and start.
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

		// Stranded and pruned teardown need nothing but what repoLanes
		// and the worktree list already gave us, so neither waits on
		// remote/base — only reapUnstaffed's Scavenge call does — and
		// neither aborts the repo on the other's failure: each kind is
		// reported on its own, the same rule prunedEntry already
		// follows for a single worktree.
		reaped, refused := reapStranded(rt, repo, found.Stranded, evidence, rc.Go)
		pruned, refusedPruned := reapPruned(
			rt, repo, found.Prunable, found.Empty, rc.Go)

		dropped := []report.DroppedHold{}
		refusedHolds := []report.RefusedHold{}
		if remote, base, err := repoRemoteBase(repo, rt); err != nil {
			doc.AddProblem(repo.Name, err)
		} else {
			dropped, refusedHolds = reapUnstaffed(
				rt, repo, found.Unstaffed, remote, base, rc.Go)
		}

		doc.AddRepo(repo.Name, reaped, refused,
			dropped, refusedHolds, pruned, refusedPruned)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printReap(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// repoRemoteBase reads the remote a claim lease is pushed to and the
// ref it is dated against — the two facts claim.Scavenge needs, read
// directly here since reap walks every repository rather than
// resolving one plan's own fleet.Coord.
func repoRemoteBase(repo discover.Repo, rt *runtime) (string, string, error) {
	cfg, err := repocfg.Load(repo.Path)
	if err != nil {
		return "", "", err
	}
	base := cfg.Base
	if base == "" {
		base = gitobj.DefaultRef(repo.Path, rt.git)
	}

	return cfg.Remote, base, nil
}

// reapStranded classifies and, under doGo, tears down every worktree
// of a repository's stranded lanes that actually carries a commit. The
// landed check reap deletes on is the same evidence lanes.Build
// already joined the claims against — re-checked here per worktree's
// own branch rather than trusted from the lane's stranded
// classification alone.
//
// A zero-commit worktree is deliberately left out: it holds nothing a
// landed check exists to protect, lanes.Find's independent empty/
// prunable pass already reports it, and judging it here too would
// produce a "not landed" refusal that pass's own unconditional
// teardown immediately contradicts (S79 and the never-started case
// alike — see reapPruned).
//
// A teardown failure refuses that one worktree rather than aborting
// the rest: a leftover untracked file in one landed checkout must not
// hide every other lane this repository could otherwise reap.
func reapStranded(
	rt *runtime, repo discover.Repo, stranded []lanes.Lane,
	evidence landedEvidence, doGo bool,
) ([]report.ReapedLane, []report.RefusedLane) {
	reaped := []report.ReapedLane{}
	refused := []report.RefusedLane{}

	for _, lane := range stranded {
		landed := func(branch string) bool {
			return evidence.Merged["refs/heads/"+branch] ||
				evidence.ByPlanID[lane.PlanID]
		}

		for _, d := range reap.Decide(
			lane.PlanID, withCommits(lane.Worktrees), landed) {
			if d.Refused != "" {
				refused = append(refused, report.RefusedLane{
					PlanID: d.PlanID, Worktree: report.WorktreeOf(d.Worktree),
					Branch: d.Branch, Reason: d.Refused,
				})
				continue
			}

			if doGo {
				if err := tearDownWorktree(rt, repo, d); err != nil {
					refused = append(refused, report.RefusedLane{
						PlanID: d.PlanID, Worktree: report.WorktreeOf(d.Worktree),
						Branch: d.Branch, Reason: err.Error(),
					})
					continue
				}
			}
			reaped = append(reaped, report.ReapedLane{
				PlanID: d.PlanID, Worktree: report.WorktreeOf(d.Worktree),
				Branch: d.Branch,
			})
		}
	}

	return reaped, refused
}

// withCommits keeps only the worktrees that actually carry a commit.
func withCommits(worktrees []gitwt.Worktree) []gitwt.Worktree {
	out := make([]gitwt.Worktree, 0, len(worktrees))
	for _, wt := range worktrees {
		if wt.HasCommit() {
			out = append(out, wt)
		}
	}

	return out
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

// reapUnstaffed classifies and, under doGo, drops every unstaffed
// lane's canonical hold through claim.Scavenge, parking any unlanded
// work to a rescue ref first. Only the lease protocol's own id-only
// ref (claim.Branch) is Scavenge's to CAS against; a lane held only on
// a decorated legacy branch is refused with a migrate-first reason
// rather than silently doing nothing useful. A hold Scavenge cannot
// drop — fenced by another machine since observed, or already gone —
// is refused with the reason it read, never a hard command failure:
// one plan's scavenge trouble must not stop the rest from reaping.
func reapUnstaffed(
	rt *runtime, repo discover.Repo, unstaffed []lanes.Lane,
	remote, base string, doGo bool,
) ([]report.DroppedHold, []report.RefusedHold) {
	dropped := []report.DroppedHold{}
	refused := []report.RefusedHold{}

	for _, lane := range unstaffed {
		canonical := claim.Branch(lane.PlanID)
		if !holds(lane, canonical) {
			// Every decorated hold this lane carries needs the same
			// migration, not just the first one a lane claimed twice
			// happens to list.
			for _, h := range lane.Holds {
				refused = append(refused, report.RefusedHold{
					PlanID: lane.PlanID, Branch: h.Branch,
					Reason: "decorated hold; migrate to " + canonical + " first",
				})
			}
			continue
		}

		tip := claim.RemoteTip(repo.Path, remote, lane.PlanID, rt.git)
		if tip == "" {
			refused = append(refused, report.RefusedHold{
				PlanID: lane.PlanID, Branch: canonical,
				Reason: "hold ref already gone",
			})
			continue
		}

		if !doGo {
			dropped = append(dropped, report.DroppedHold{
				PlanID: lane.PlanID, Branch: canonical,
			})
			continue
		}

		sc, err := claim.Scavenge(repo.Path, claim.LeaseOptions{
			PlanID: lane.PlanID, Remote: remote, Base: base,
			Holder: hostname(),
		}, tip, rt.git)
		if err != nil {
			refused = append(refused, report.RefusedHold{
				PlanID: lane.PlanID, Branch: canonical, Reason: err.Error(),
			})
			continue
		}
		dropped = append(dropped, report.DroppedHold{
			PlanID: lane.PlanID, Branch: canonical, Rescue: sc.Rescue,
		})
	}

	return dropped, refused
}

// holds reports whether a lane carries a hold on exactly this branch.
func holds(lane lanes.Lane, branch string) bool {
	for _, h := range lane.Holds {
		if h.Branch == branch {
			return true
		}
	}

	return false
}

// reapPruned tears down every prunable and never-started worktree the
// same way a stranded checkout is — git worktree remove — but leaves
// any branch it stands on alone: unlike a landed lane's, a prunable or
// empty checkout's branch may still be live work under another name.
//
// lanes.Find's two loops are independent, so a worktree whose branch
// ref vanished without landing (S79) reads a zero-commit HEAD exactly
// like one that never started, and can surface here as well as
// stranded. git itself is the arbiter rather than a second
// classification: `worktree remove` refuses a directory that still
// holds real content it cannot reconcile against a resolvable commit,
// and that refusal is reported rather than treated as a command
// failure, so one ambiguous worktree never stops the rest from being
// reaped.
func reapPruned(
	rt *runtime, repo discover.Repo,
	prunable, empty []gitwt.Worktree, doGo bool,
) ([]report.PrunedWorktree, []report.RefusedWorktree) {
	pruned := []report.PrunedWorktree{}
	refused := []report.RefusedWorktree{}

	classify := func(worktrees []gitwt.Worktree, kind string) {
		for _, wt := range worktrees {
			p, r, ok := prunedEntry(rt, repo, wt, kind, doGo)
			if ok {
				pruned = append(pruned, p)
			} else {
				refused = append(refused, r)
			}
		}
	}
	classify(prunable, "prunable")
	classify(empty, "empty")

	return pruned, refused
}

// prunedEntry removes one worktree under doGo and reports it either
// way, labelled by which kind it was found as. ok is false when git
// itself refused the removal, carried back as a RefusedWorktree rather
// than a command failure.
func prunedEntry(
	rt *runtime, repo discover.Repo, wt gitwt.Worktree, kind string, doGo bool,
) (report.PrunedWorktree, report.RefusedWorktree, bool) {
	if doGo {
		if _, err := rt.git(repo.Path, "worktree", "remove", wt.Path); err != nil {
			return report.PrunedWorktree{}, report.RefusedWorktree{
				Worktree: report.WorktreeOf(wt), Kind: kind,
				Reason: err.Error(),
			}, false
		}
	}

	return report.PrunedWorktree{Worktree: report.WorktreeOf(wt), Kind: kind},
		report.RefusedWorktree{}, true
}

// printReap writes a block per repository with something reaped or
// refused, worded for whether --go actually ran or reap only reports
// what it would do.
func printReap(out io.Writer, doc *report.ReapDoc) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	found := false
	verb := verbFor(doc.Go, "reaped", "would reap")
	dropVerb := verbFor(doc.Go, "dropped", "would drop")

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
		for _, h := range repo.Dropped {
			if h.Rescue != "" {
				_, _ = fmt.Fprintf(tw, "  %s\tplan %d\t%s (parked: %s)\n",
					dropVerb, h.PlanID, h.Branch, h.Rescue)
				continue
			}
			_, _ = fmt.Fprintf(tw, "  %s\tplan %d\t%s\n",
				dropVerb, h.PlanID, h.Branch)
		}
		for _, h := range repo.RefusedHolds {
			_, _ = fmt.Fprintf(tw, "  refused\tplan %d\t%s (%s)\n",
				h.PlanID, h.Branch, h.Reason)
		}
		for _, wt := range repo.Pruned {
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\n",
				verb, wt.Worktree.Name, wt.Kind)
		}
		for _, wt := range repo.RefusedPruned {
			_, _ = fmt.Fprintf(tw, "  refused\t%s\t%s (%s)\n",
				wt.Worktree.Name, wt.Kind, wt.Reason)
		}
	}
	_ = tw.Flush()

	if !found {
		_, _ = fmt.Fprintln(out, "nothing to reap")
	}
}

// verbFor words a mutation's fate for whether --go actually ran or
// reap only reports what it would do.
func verbFor(goFlag bool, done, pending string) string {
	if goFlag {
		return done
	}

	return pending
}
