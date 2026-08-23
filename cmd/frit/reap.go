package main

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/discovery"
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
	// Abandonment evidence for a hold — a matured staleness window, a
	// bound session herdr confirms dead — lives in the observation fold
	// the fleet gather runs, not in lanes.Find's git-ref sweep, so reap
	// gathers beside the walk exactly as orphans does.
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	// The JSON contract keeps stdout the whole report, so progress is
	// silenced there rather than tested for at every write site.
	var progress = io.Writer(rt.stderr)
	if c.JSON {
		progress = io.Discard
	}

	doc := report.NewReap(c.Root, rc.Go)
	for _, repo := range repos {
		built, evidence, err := repoLanes(repo, rt)
		if err != nil {
			doc.AddProblem(repo.Name, err)
			continue
		}
		// The stranded pass parks before it deletes and the unstaffed
		// pass scavenges, so both need the repository's remote and
		// base. Loading them cannot practically fail once repoLanes
		// has read the same config, so a failure here is a genuine
		// problem worth skipping the repository over.
		remote, base, err := repoRemoteBase(repo, rt)
		if err != nil {
			doc.AddProblem(repo.Name, err)
			continue
		}
		found := lanes.Find(built, repo.Worktrees)
		window, _ := staleClock(&res, repo.Name)

		reaped, refused := reapStranded(
			rt, repo, found.Stranded, evidence, remote, base, rc.Go, progress)
		pruned, refusedPruned := reapPruned(
			rt, repo, found.Prunable, found.Empty, rc.Go, progress)
		dropped, refusedHolds := reapUnstaffed(
			rt, repo, found.Unstaffed, res.Plans, remote, base, window,
			rc.Go, progress)

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
//
// The delete honors the park-before-delete rule. Ordinary-merge
// evidence is tied to the tip — an ancestor of the base loses nothing
// to branch -D — but the squash-merge glyph is not: a follow-up commit
// the squash never carried would be destroyed. So the branch tip's
// unlanded work is parked to the plan's rescue ref first, and a park
// that cannot happen refuses the whole teardown, worktree and branch
// both left standing.
func reapStranded(
	rt *runtime, repo discover.Repo, stranded []lanes.Lane,
	evidence landedEvidence, remote, base string, doGo bool,
	progress io.Writer,
) ([]report.ReapedLane, []report.RefusedLane) {
	reaped := []report.ReapedLane{}
	refused := []report.RefusedLane{}

	for _, lane := range stranded {
		landed := func(branch string) bool {
			return evidence.Merged["refs/heads/"+branch] ||
				evidence.ByPlanID[lane.PlanID]
		}
		opts := claim.LeaseOptions{
			PlanID: lane.PlanID, Remote: remote, Base: base,
			Holder: hostname(),
		}

		for _, d := range reap.Decide(
			lane.PlanID, withCommits(lane.Worktrees), landed) {
			wt := report.WorktreeOf(d.Worktree)
			if d.Refused != "" {
				refused = append(refused, report.RefusedLane{
					PlanID: d.PlanID, Worktree: wt,
					Branch: d.Branch, Reason: d.Refused,
				})
				progressRefused(progress, wt.Name, d.Branch, d.Refused)
				continue
			}

			rescue, err := parkBranch(rt, repo, opts, d.Branch, doGo)
			if err != nil {
				reason := "park: " + err.Error()
				refused = append(refused, report.RefusedLane{
					PlanID: d.PlanID, Worktree: wt,
					Branch: d.Branch, Reason: reason,
				})
				progressRefused(progress, wt.Name, d.Branch, reason)
				continue
			}
			if doGo {
				if err := tearDownWorktree(rt, repo, d); err != nil {
					refused = append(refused, report.RefusedLane{
						PlanID: d.PlanID, Worktree: wt,
						Branch: d.Branch, Reason: err.Error(),
					})
					progressRefused(progress, wt.Name, d.Branch, err.Error())
					continue
				}
			}
			reaped = append(reaped, report.ReapedLane{
				PlanID: d.PlanID, Worktree: wt,
				Branch: d.Branch, Rescue: rescue,
			})
			progressReaped(progress, verbFor(doGo, "reaped", "would reap"),
				wt.Name, d.Branch)
		}
	}

	return reaped, refused
}

// progressRefused writes one refused progress line, worded like
// printReap's own refused row for the same lane.
func progressRefused(progress io.Writer, name, branch, reason string) {
	_, _ = fmt.Fprintf(progress, "refused\t%s\t%s (%s)\n", name, branch, reason)
}

// progressReaped writes one reaped (or would-reap) progress line,
// worded like printReap's own row for the same lane.
func progressReaped(progress io.Writer, verb, name, branch string) {
	_, _ = fmt.Fprintf(progress, "%s\t%s\t%s\n", verb, name, branch)
}

// parkBranch parks a branch tip's unlanded work ahead of its delete,
// or under a dry run only names the rescue ref the park would write.
// The tip is read off the branch itself — that is what branch -D
// deletes — and an absent tip parks nothing, leaving the delete to
// speak for itself. The dry-run preview is best-effort: a chain that
// cannot be read previews no rescue rather than failing the report.
func parkBranch(
	rt *runtime, repo discover.Repo, opts claim.LeaseOptions,
	branch string, doGo bool,
) (string, error) {
	tip, err := localRef(rt, repo.Path, branch)
	if err != nil || tip == "" {
		return "", err
	}

	if !doGo {
		if unlanded, err := claim.HasUnlanded(
			repo.Path, opts, tip, rt.git); err == nil && unlanded {
			return claim.RescueRef(opts.PlanID, opts.Holder), nil
		}

		return "", nil
	}

	sc, err := claim.ParkUnlanded(repo.Path, opts, tip, rt.git)
	if err != nil {
		return "", err
	}

	return sc.Rescue, nil
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

// reapUnstaffed drops an unstaffed lane's canonical hold only on the
// lease protocol's own abandonment evidence. "Claimed, no local
// checkout" alone proves nothing — the checkout may be another
// machine's, or the claim seconds old with its stand-up still pending;
// lanes.Build has already filtered landed refs, so an unstaffed hold
// is by construction a live-looking, un-landed lease. What earns the
// drop is a matured staleness window or a bound session herdr
// confirms dead — the same gate discovery.Ready and takeover honor —
// and the scavenge then CASes on the observed tip, so a holder that
// renewed since fences it (A2).
//
// Only the lease protocol's own id-only ref (claim.Branch) is
// Scavenge's to CAS against; a lane held only on a decorated legacy
// branch is refused with a migrate-first reason rather than silently
// doing nothing useful. Any hold left standing is a refusal with the
// reason read, never a hard command failure: one plan's trouble must
// not stop the rest from reaping.
func reapUnstaffed(
	rt *runtime, repo discover.Repo, unstaffed []lanes.Lane,
	plans []discovery.Plan, remote, base string, window time.Duration,
	doGo bool, progress io.Writer,
) ([]report.DroppedHold, []report.RefusedHold) {
	dropped := []report.DroppedHold{}
	refused := []report.RefusedHold{}
	dropVerb := verbFor(doGo, "dropped", "would drop")

	for _, lane := range unstaffed {
		canonical := claim.Branch(lane.PlanID)
		id := fmt.Sprintf("plan %d", lane.PlanID)
		if !holds(lane, canonical) {
			// Every decorated hold this lane carries needs the same
			// migration, not just the first one a lane claimed twice
			// happens to list.
			for _, h := range lane.Holds {
				refused = append(refused,
					refuseDecoratedHold(progress, lane.PlanID, canonical, h.Branch))
			}
			continue
		}

		p, ok := planFor(plans, repo.Name, lane.PlanID)
		if reason := holdRefusal(p, ok, window); reason != "" {
			refused = append(refused,
				refuseHold(progress, lane.PlanID, canonical, reason))
			continue
		}

		// The holder's objects may never have been fetched here — the
		// lease could be another machine's — so bring the ref in once
		// ahead of the park's push. Best-effort: without it the
		// scavenge fails safe into a refusal.
		_, _ = rt.git(repo.Path, "fetch", "--quiet",
			remote, "refs/heads/"+canonical)

		opts := claim.LeaseOptions{
			PlanID: lane.PlanID, Remote: remote, Base: base,
			Holder: hostname(),
		}
		if !doGo {
			rescue := ""
			if unlanded, err := claim.HasUnlanded(
				repo.Path, opts, p.HoldTip, rt.git); err == nil && unlanded {
				rescue = claim.RescueRef(lane.PlanID, hostname())
			}
			dropped = append(dropped, report.DroppedHold{
				PlanID: lane.PlanID, Branch: canonical, Rescue: rescue,
			})
			progressReaped(progress, dropVerb, id, canonical)
			continue
		}

		sc, err := claim.Scavenge(repo.Path, opts, p.HoldTip, rt.git)
		if err != nil {
			refused = append(refused,
				refuseHold(progress, lane.PlanID, canonical, err.Error()))
			continue
		}
		dropped = append(dropped, report.DroppedHold{
			PlanID: lane.PlanID, Branch: canonical, Rescue: sc.Rescue,
		})
		progressReaped(progress, dropVerb, id, canonical)
	}

	return dropped, refused
}

// refuseDecoratedHold builds and streams one refusal for a legacy
// decorated hold — not the lease protocol's own id-only ref, so
// Scavenge has nothing to CAS against.
func refuseDecoratedHold(
	progress io.Writer, id int64, canonical, branch string,
) report.RefusedHold {
	reason := "decorated hold; migrate to " + canonical + " first"
	progressRefused(progress, fmt.Sprintf("plan %d", id), branch, reason)

	return report.RefusedHold{PlanID: id, Branch: branch, Reason: reason}
}

// refuseHold builds and streams one refusal on a lane's canonical hold.
func refuseHold(
	progress io.Writer, id int64, canonical, reason string,
) report.RefusedHold {
	progressRefused(progress, fmt.Sprintf("plan %d", id), canonical, reason)

	return report.RefusedHold{PlanID: id, Branch: canonical, Reason: reason}
}

// holdRefusal is the abandonment gate: why an unstaffed hold is not
// reap's to drop, "" when the evidence clears it. ok says whether the
// gathered fleet view carried the plan at all. window is the repo's
// configured takeover window, named in the not-matured case so the
// operator can judge how close the hold is to being takeable.
func holdRefusal(p discovery.Plan, ok bool, window time.Duration) string {
	switch {
	case !ok || p.HoldTip == "":
		return "no observed lease state to judge abandonment by"
	case !p.Stale && !p.Dead:
		return fmt.Sprintf(
			"held by a live lease, seen unchanged for %s of the %s "+
				"takeover window; not takeable until the window matures "+
				"— its own lane releases it, or claim takes it over "+
				"once the window matures",
			p.StaleFor.Round(time.Second), window.Round(time.Second))
	}

	return ""
}

// planFor finds one repository's plan in the gathered fleet view, the
// carrier of the staleness and dead-session facts the drop gates on.
func planFor(
	plans []discovery.Plan, repo string, id int64,
) (discovery.Plan, bool) {
	for _, p := range plans {
		if p.Repo == repo && p.ID == id {
			return p, true
		}
	}

	return discovery.Plan{}, false
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
	prunable, empty []gitwt.Worktree, doGo bool, progress io.Writer,
) ([]report.PrunedWorktree, []report.RefusedWorktree) {
	pruned := []report.PrunedWorktree{}
	refused := []report.RefusedWorktree{}
	verb := verbFor(doGo, "reaped", "would reap")

	classify := func(worktrees []gitwt.Worktree, kind string) {
		for _, wt := range worktrees {
			name := report.WorktreeOf(wt)
			if doGo {
				if _, err := rt.git(repo.Path,
					"worktree", "remove", wt.Path); err != nil {
					refused = append(refused, report.RefusedWorktree{
						Worktree: name, Kind: kind, Reason: err.Error(),
					})
					progressRefused(progress, name.Name, kind, err.Error())
					continue
				}
			}
			pruned = append(pruned, report.PrunedWorktree{
				Worktree: name, Kind: kind,
			})
			progressReaped(progress, verb, name.Name, kind)
		}
	}
	classify(prunable, "prunable")
	classify(empty, "empty")

	return pruned, refused
}

// printReap writes a block per repository with something reaped or
// refused, worded for whether --go actually ran or reap only reports
// what it would do.
func printReap(out io.Writer, doc *report.ReapDoc) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	found := false
	verb := verbFor(doc.Go, "reaped", "would reap")
	dropVerb := verbFor(doc.Go, "dropped", "would drop")
	parkNote := func(rescue string) string {
		if rescue == "" {
			return ""
		}

		return fmt.Sprintf(" (%s: %s)",
			verbFor(doc.Go, "parked", "would park"), rescue)
	}

	for _, repo := range doc.Repos {
		if !repo.Any() {
			continue
		}
		found = true
		_, _ = fmt.Fprintf(tw, "%s\t\t\n", repo.Name)

		for _, lane := range repo.Reaped {
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s%s\n",
				verb, lane.Worktree.Name, lane.Branch, parkNote(lane.Rescue))
		}
		for _, lane := range repo.Refused {
			_, _ = fmt.Fprintf(tw, "  refused\t%s\t%s (%s)\n",
				lane.Worktree.Name, lane.Branch, lane.Reason)
		}
		for _, h := range repo.Dropped {
			_, _ = fmt.Fprintf(tw, "  %s\tplan %d\t%s%s\n",
				dropVerb, h.PlanID, h.Branch, parkNote(h.Rescue))
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
