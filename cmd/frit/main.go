// Command frit indexes plans, worktrees and agents across a fleet, and
// mints the one mutation it owns: the claim.
//
// Almost everything here is read-only — it shells out to git to read
// state and prints what it finds. The exception is the claim, an atomic
// hold frit leases with a force-with-lease push, because a hold has to
// be atomic and a ref push is the only atom git offers. Beyond that,
// nothing writes to a repository, spawns an agent it does not hand
// straight back, or reads a transcript.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"
	"golang.org/x/term"

	"github.com/jeduden/frit/internal/claim"
	"github.com/jeduden/frit/internal/config"
	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/discovery"
	doctorpkg "github.com/jeduden/frit/internal/doctor"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/index"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/jeduden/frit/internal/observe"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/jeduden/frit/internal/plans"
	"github.com/jeduden/frit/internal/presence"
	"github.com/jeduden/frit/internal/repocfg"
	"github.com/jeduden/frit/internal/report"
	"github.com/jeduden/frit/internal/scaffold"
	"github.com/jeduden/frit/internal/skills"
	"github.com/jeduden/frit/internal/textw"
)

// version is stamped at build time with -ldflags.
var version = "dev"

const description = `A register of plans, worktrees, hosts and agents.

Settings resolve flag first, then $FRIT_* in the environment, then
.frit.yml beside the work, then the user config file.`

// cli is the whole command surface. Every flag here is readable from
// three places — the command line, the environment, and a config
// file — because a fleet root is typed once and then wanted by every
// invocation.
type cli struct {
	Root string `help:"Directory to walk for repositories." env:"FRIT_ROOT" default:"." type:"path"`

	// Hosts is the fleet's other machines. Presence — which agent is
	// live in which pane — is the one half of the board that does not
	// cross hosts through git, so it is read from each host over ssh.
	// The list is frit's own setting, not a per-repo convention: a
	// machine roster belongs to whoever runs frit, not to a repository,
	// so it resolves from the flag, $FRIT_HOSTS, or the user config like
	// --root does. Empty, the default, reads only the local socket.
	Hosts []string `help:"Other hosts to read live agent presence from, over ssh." env:"FRIT_HOSTS"`

	// JSON is global rather than per command because every command
	// answers it, and an agent should not have to remember which ones.
	JSON bool `help:"Emit the report as JSON instead of a table."`

	// All un-hides what the default view holds back: satisfied
	// dependencies in show, and files in a plan directory that carry no
	// front matter and so are not plans. It is global because more than
	// one command hides something, and --deps is kept as its name for
	// show, where "show the dependencies" is how it reads.
	All bool `aliases:"deps" help:"Show what is hidden by default: satisfied deps, and files that are not plans."`

	// Width overrides the detected terminal width, for fitting a table
	// where there is no terminal to measure — piped into a pager, or run
	// under a harness that indents the output. Zero, the default, means
	// detect; a positive value is used as given.
	Width int `help:"Fit tables to this many columns (0 auto-detects the terminal)."`

	Config kong.ConfigFlag `help:"Load configuration from a file." placeholder:"PATH"`

	Repos   reposCmd   `cmd:"" help:"List repositories and their worktrees."`
	Plans   plansCmd   `cmd:"" help:"List plan files found on every ref."`
	Ready   readyCmd   `cmd:"" help:"List plans startable now: deps done, nobody holds."`
	Pick    pickCmd    `cmd:"" help:"Rank startable plans by how much each unblocks; --go claims and starts the top."`
	Next    nextCmd    `cmd:"" help:"Report the first phase of a plan not yet done."`
	Show    showCmd    `cmd:"" help:"Show a plan and everything that blocks it."`
	Find    findCmd    `cmd:"" help:"Search plan titles and summaries across every ref."`
	Board   boardCmd   `cmd:"" help:"Outstanding plans: status, who holds each, and the agent on it."`
	Open    openCmd    `cmd:"" help:"Focus the pane a plan's lane is running in; sends no text."`
	Nudge   nudgeCmd   `cmd:"" help:"Prompt a plan's phase into its idle lane; dry-run unless --go."`
	Claim   claimCmd   `cmd:"" help:"Mint frit's own atomic hold on a startable plan."`
	Release releaseCmd `cmd:"" help:"End this lane's own lease with a release marker."`
	Yield   yieldCmd   `cmd:"" help:"End a fenced lane: park its divergence to a rescue ref and tear it down."`
	Start   startCmd   `cmd:"" help:"Compose the full escalation for a plan; dry-run unless --go."`
	Orphans orphansCmd `cmd:"" help:"Report claims and checkouts that no longer add up."`
	Reap    reapCmd    `cmd:"" help:"Tear down what orphans reports; dry-run unless --go."`
	Stale   staleCmd   `cmd:"" help:"Report worktrees whose branch has not moved."`
	Doctor  doctorCmd  `cmd:"" help:"Report plans with a semantic gap: missing Goal, tier, Execution row."`
	Who     whoCmd     `cmd:"" help:"Report which lane has a live agent on it."`
	Init    initCmd    `cmd:"" help:"Write a .frit.yml with frit's defaults."`
	Skills  skillsCmd  `cmd:"" help:"Install the bundled agent skills into .claude/skills."`
	Version versionCmd `cmd:"" help:"Print the build version."`
}

// landedEvidence is the two facts a landed check for a ref or a plan
// rides on: Merged is an ordinary merge's ancestry, keyed by full ref
// name; ByPlanID is the default branch's own plan status, the signal
// that closes the squash-merge gap ancestry cannot see.
type landedEvidence struct {
	Merged   map[string]bool
	ByPlanID map[int64]bool
}

// repoLanes joins one repository's claims to its checkouts, reading
// that repository's own hold patterns, alongside the landed evidence
// that joined them — reap's own delete gate re-checks a stranded
// lane's branch against this same evidence rather than trust the
// classification alone.
func repoLanes(
	repo discover.Repo, rt *runtime,
) ([]lanes.Lane, landedEvidence, error) {
	cfg, err := repocfg.Load(repo.Path)
	if err != nil {
		return nil, landedEvidence{}, err
	}
	holds, err := cfg.Compiled()
	if err != nil {
		return nil, landedEvidence{}, err
	}

	refs, err := gitobj.Refs(repo.Path, rt.git)
	if err != nil {
		return nil, landedEvidence{}, err
	}
	preferred := gitobj.DefaultRef(repo.Path, rt.git)
	merged, err := gitobj.MergedRefs(repo.Path, preferred, rt.git)
	if err != nil {
		return nil, landedEvidence{}, err
	}

	// A squash-merge lands a plan without leaving its branch an ancestor
	// of the default branch, so the merged filter cannot see it. The
	// default-branch status can: a plan done there is landed work, and
	// its claim ref is not a live hold. Reading the index costs the
	// orphan report the same plan walk the fleet already runs.
	files, err := plans.Collect(repo.Path, cfg.PlanDir, rt.git, rt.gitPipe)
	if err != nil {
		return nil, landedEvidence{}, err
	}
	entries, _ := index.Build("", repo.Name, preferred, files)
	landed := index.LandedIDs(entries, preferred)
	evidence := landedEvidence{Merged: merged, ByPlanID: landed}

	return lanes.Build(repo.Worktrees, refs, merged, landed, holds), evidence, nil
}

type orphansCmd struct{}

// Run reports what is claimed but unstaffed, prepared but unstarted,
// held past its takeover window, or already gone.
func (o *orphansCmd) Run(c *cli, rt *runtime) error {
	repos, err := discover.Repos(c.Root, rt.git)
	if err != nil {
		return err
	}
	// The held-stale cell of the verb-state table reads the same
	// observation fold board and claim use, not lanes.Find's git-ref
	// sweep, so it needs its own gather beside the lanes walk below.
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	doc := report.NewOrphans(c.Root)
	for _, repo := range repos {
		built, _, err := repoLanes(repo, rt)
		if err != nil {
			// One unreadable repository must not blind the rest.
			doc.AddProblem(repo.Name, err)
			continue
		}
		doc.AddRepo(repo.Name, lanes.Find(built, repo.Worktrees))
		doc.AddStale(repo.Name, staleHeld(res.Plans, repo.Name))
		// Without a coordinate there is no origin to compare a token
		// against, so a repository the gather could not place — the
		// ambiguous-name case — contributes no deserted candidates
		// rather than guessing at one.
		if coord, ok := res.Coords[repo.Name]; ok {
			doc.AddDeserted(repo.Name, desertedHeld(
				rt, res.Plans, repo.Name, repo.Worktrees, coord))
		}
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printOrphans(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// staleHeld filters one repository's held plans whose takeover window
// has matured — a candidate nobody has acted on yet. A bound session
// herdr confirms is gone, ahead of that window, is desertedHeld's own
// cell instead; the two never collide.
func staleHeld(plans []discovery.Plan, repo string) []discovery.Plan {
	out := make([]discovery.Plan, 0)
	for _, p := range plans {
		if p.Repo == repo && p.Held && p.Stale {
			out = append(out, p)
		}
	}

	return out
}

// desertedHeld filters one repository's held plans that are a dead
// end: herdr confirms the bound session gone, the takeover window has
// not matured (a matured hold is staleHeld's instead, S76), and no
// worktree of this repository holds a token that still matches
// origin's tip — the resume shortcut ownToken already checks, reused
// here rather than reimplemented (F9, F11, S3, S21). The caller skips
// this call entirely when the gather withheld a coordinate for an
// ambiguous repository, so there is no coordOK left for this function
// itself to check.
func desertedHeld(
	rt *runtime, plans []discovery.Plan, repo string,
	worktrees []gitwt.Worktree, coord fleet.Coord,
) []discovery.Plan {
	out := make([]discovery.Plan, 0)
	for _, p := range plans {
		if p.Repo != repo || !p.Held || !p.Dead || p.Stale {
			continue
		}
		if resumableFromAnyLane(rt, p, worktrees, coord) {
			continue
		}
		out = append(out, p)
	}

	return out
}

// resumableFromAnyLane reports whether some worktree of this
// repository carries its own persisted token still matching origin's
// tip for the plan — the same proof ownToken reads from a single
// lane's cwd, tried here against every worktree on this host rather
// than one directory, since orphans is not run from inside any one
// lane.
func resumableFromAnyLane(
	rt *runtime, p discovery.Plan, worktrees []gitwt.Worktree, coord fleet.Coord,
) bool {
	for _, wt := range worktrees {
		if _, _, ok := ownToken(rt, p, coord, wt.Path); ok {
			return true
		}
	}

	return false
}

// printOrphans writes a block per repository with something wrong.
// The kinds stay labelled rather than merged into a count, because
// each calls for a different response. A repository in good order is
// left out entirely; --json lists it with empty sets.
func printOrphans(out io.Writer, doc *report.OrphansDoc) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	found := false

	for _, repo := range doc.Repos {
		if !repo.Any() {
			continue
		}
		found = true
		_, _ = fmt.Fprintf(tw, "%s\t\t\n", repo.Name)

		for _, lane := range repo.Unstaffed {
			_, _ = fmt.Fprintf(tw, "  claimed, no checkout\tplan %d\t%s\n",
				lane.PlanID, lane.Holds[0].Branch)
		}
		for _, lane := range repo.Stranded {
			for _, wt := range lane.Worktrees {
				_, _ = fmt.Fprintf(tw,
					"  landed, still checked out\t%s\t%s\n",
					wt.Name, wt.Branch)
			}
		}
		for _, wt := range repo.Empty {
			_, _ = fmt.Fprintf(tw, "  never started\t%s\t%s\n",
				wt.Name, wt.Branch)
		}
		for _, wt := range repo.Prunable {
			_, _ = fmt.Fprintf(tw, "  prunable\t%s\t%s\n",
				wt.Name, wt.PruneReason)
		}
		for _, m := range repo.Migratable {
			_, _ = fmt.Fprintf(tw, "  decorated hold, migrate\tplan %d\t%s → %s\n",
				m.PlanID, m.From, m.To)
		}
		for _, s := range repo.StaleHolds {
			age := (time.Duration(s.StaleSeconds) * time.Second).Round(time.Minute)
			_, _ = fmt.Fprintf(tw, "  stale, takeover candidate\tplan %d\t%s (%s)\n",
				s.PlanID, s.Branch, age)
		}
		for _, d := range repo.Deserted {
			_, _ = fmt.Fprintf(tw, "  deserted, session gone\tplan %d\t%s\n",
				d.PlanID, d.Branch)
		}
	}
	_ = tw.Flush()

	if !found {
		_, _ = fmt.Fprintln(out, "no orphaned lanes")
	}
}

type staleCmd struct {
	Days int `default:"30" help:"Report worktrees untouched for this many days."`
}

// Run reports worktrees whose branch tip has not moved for a while.
func (s *staleCmd) Run(c *cli, rt *runtime) error {
	repos, err := discover.Repos(c.Root, rt.git)
	if err != nil {
		return err
	}

	cutoff := time.Duration(s.Days) * 24 * time.Hour
	now := time.Now()

	// Live presence sharpens staleness: a branch that has not moved but
	// still has an agent in its worktree is being worked, not abandoned.
	// It is read once for the whole fleet, and an unreachable socket
	// leaves the git answer standing rather than failing the report.
	live, present, hostProbs := livePresence(c, rt)

	doc := report.NewStale(c.Root, s.Days, present)
	for _, p := range hostProbs {
		doc.AddProblem(p.name, p.err)
	}
	for _, repo := range repos {
		times, err := gitobj.RefTimes(repo.Path, rt.git)
		if err != nil {
			doc.AddProblem(repo.Name, err)
			continue
		}
		doc.AddRepo(repo.Name,
			lanes.Stale(repo.Worktrees, times, now, cutoff), live)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printStale(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printStale writes the idle checkouts, oldest first within each
// repository, and says so plainly when there are none.
//
// With live presence known, each lane is marked abandoned or live, so
// a branch that has not moved but still has an agent is not mistaken
// for dropped work. With presence unknown the column is left blank
// rather than guessing.
func printStale(out io.Writer, doc *report.StaleDoc) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	found := false

	for _, repo := range doc.Repos {
		if len(repo.Stale) == 0 {
			continue
		}
		found = true
		_, _ = fmt.Fprintf(tw, "%s\t\t\t\n", repo.Name)
		for _, aged := range repo.Stale {
			_, _ = fmt.Fprintf(tw, "  %dd\t%s\t%s\t%s\n",
				aged.AgeDays, aged.Worktree.Name, aged.Worktree.Branch,
				staleState(doc.Presence, aged.HasAgent))
		}
	}
	_ = tw.Flush()

	if !found {
		_, _ = fmt.Fprintf(out,
			"no worktree idle longer than %d days\n", doc.Days)
	}
}

// staleState labels an idle checkout once presence is known: an agent
// on it means live, none means abandoned. With presence unknown the
// label is empty, because calling a lane abandoned on a socket we
// never reached would be a false negative dressed as a fact.
func staleState(presence, hasAgent bool) string {
	switch {
	case !presence:
		return ""
	case hasAgent:
		return "live"
	default:
		return "abandoned"
	}
}

type doctorCmd struct{}

// Help documents doctor's checks and their provenance, so a reader
// learns what a finding means and where it came from without opening
// the source — the contract this verb promises to catch, and nothing
// beyond it.
func (d *doctorCmd) Help() string {
	return `doctor scans every plan on disk and lists these gaps:

  goal            a "## Goal" section with no meaningful body content
  schema          a front-matter field plan/proto.md's schema rejects
                  (today, only the model tier)
  execution-row   a phase with no matching row in its "## Execution"
                  table
  tier            an Execution row naming a tier that is not haiku,
                  sonnet or opus

goal and schema are mdsmith's own findings: doctor runs mdsmith as an
imported library (github.com/jeduden/mdsmith/pkg/mdsmith) against each
repository's own plan/proto.md, rather than reimplementing a checker.
execution-row and tier read the body data frit already parses for
next and show — mdsmith's schema has no way to see inside a markdown
table's cells, or cross-reference a table's rows against another
section's headings.

A repository with no plan/proto.md has nothing to check.`
}

// Run scans every repository's plan directory for the semantic gaps
// frit now depends on, read-only — see Help.
func (d *doctorCmd) Run(c *cli, rt *runtime) error {
	repos, err := discover.Repos(c.Root, rt.git)
	if err != nil {
		return err
	}

	doc := report.NewDoctor(c.Root)
	for _, repo := range repos {
		cfg, err := repocfg.Load(repo.Path)
		if err != nil {
			doc.AddProblem(repo.Name, err)
			continue
		}
		findings, err := doctorpkg.Scan(repo.Path, cfg.PlanDir)
		if err != nil {
			if errors.Is(err, doctorpkg.ErrNoSchema) {
				continue
			}
			doc.AddProblem(repo.Name, err)
			continue
		}
		doc.AddFindings(repo.Name, findings)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printDoctor(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printDoctor writes one row per finding. A repository with nothing
// wrong contributes no rows, the same convention orphans and stale
// use to stay short.
func printDoctor(out io.Writer, doc *report.DoctorDoc) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, f := range doc.Findings {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n",
			f.Repo, f.ID, f.Path, f.Check, f.Message)
	}
	_ = tw.Flush()

	if len(doc.Findings) == 0 {
		_, _ = fmt.Fprintln(out, "no semantic gaps found")
	}
}

// hostReadTTL serves a remote host's presence from cache for this long
// before a re-probe, so an interactive board does not ssh the whole
// fleet every time it is run. hostReadTimeout bounds each host so a slow
// machine renders stale rather than hanging the board on it.
const (
	hostReadTTL     = 15 * time.Second
	hostReadTimeout = 3 * time.Second
)

// hostProblem names a host frit could not read live, or could only read
// stale from cache, so a command can carry it in its report.
type hostProblem struct {
	name string
	err  error
}

// fleetPresence reads live panes across the fleet. The local socket is
// read first and its failure is returned as err — so a caller still
// tells "presence unknown" from "nobody live", exactly as the
// single-socket read did. With no hosts configured that is all it does.
// With hosts, the remotes are read best-effort over ssh and reconciled
// against the cache: a dead or slow one renders its last-known panes and
// travels back as a hostProblem rather than failing the read, so one
// unreachable machine never blocks the board.
func fleetPresence(
	c *cli, rt *runtime,
) (panes []herdr.Pane, probs []hostProblem, err error) {
	local, err := herdr.List(rt.herdr)
	if err != nil {
		return nil, nil, err
	}
	if len(c.Hosts) == 0 {
		return local, nil, nil
	}

	path, perr := presence.CachePath()
	if perr != nil {
		// With nowhere to cache last-known presence, the local socket
		// still stands rather than nothing; the remotes wait for a cache.
		return local, nil, nil
	}

	exec := func(name string, args ...string) ([]byte, error) {
		return herdr.Run(name, args...)
	}
	opt := presence.Options{TTL: hostReadTTL, Timeout: hostReadTimeout}
	remotePanes, statuses := presence.Read(
		toHosts(c.Hosts), exec, path, opt, time.Now())

	return append(local, remotePanes...), hostProblems(statuses), nil
}

// hostProblems turns the reconciled per-host statuses into the problems
// a report carries: a host never reached is unreachable, and one served
// from cache is flagged with how stale its presence is. A fresh read is
// no problem and is left silent.
func hostProblems(statuses []presence.Status) []hostProblem {
	var probs []hostProblem
	for _, s := range statuses {
		switch {
		case !s.Seen:
			probs = append(probs, hostProblem{
				name: "host " + string(s.Host),
				err:  errors.New("unreachable, no cached presence"),
			})
		case !s.Fresh:
			probs = append(probs, hostProblem{
				name: "host " + string(s.Host),
				err: fmt.Errorf(
					"unreachable; showing presence %s stale",
					s.Age.Round(time.Second)),
			})
		}
	}

	return probs
}

// toHosts is the configured remote roster as herdr hosts. The local
// host is not in it: fleetPresence reads the local socket directly, so
// the fan-out is remotes only.
func toHosts(hosts []string) []herdr.Host {
	out := make([]herdr.Host, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, herdr.Host(h))
	}

	return out
}

// gitForHost selects the git runner that reaches a pane's host: the
// local runner for the empty Host, and one that shells `ssh <host> git
// -C <dir> …` for a remote one. A remote pane's cwd is a path on the
// remote, so only the remote's git can resolve it to a lane.
func gitForHost(local gitwt.Runner) func(herdr.Host) gitwt.Runner {
	return func(host herdr.Host) gitwt.Runner {
		if host == "" {
			return local
		}

		return remoteGit(host)
	}
}

// remoteGit runs a git command on a remote host over ssh, standing in
// for gitwt.Exec's local `git -C`. The cwd and args are handed to the
// remote shell as `ssh <host> git -C <dir> <args…>`.
func remoteGit(host herdr.Host) gitwt.Runner {
	return func(dir string, args ...string) ([]byte, error) {
		full := append([]string{string(host), "git", "-C", dir}, args...)

		return herdr.Run("ssh", full...)
	}
}

// livePresence reads the fleet's live agent roots from herdr. A failing
// or missing socket yields an empty set and false, which every reader
// treats as "presence unknown" rather than "no agents". Per-host
// problems travel back so a caller can carry an unreachable or stale
// host in its report.
func livePresence(
	c *cli, rt *runtime,
) (map[string]bool, bool, []hostProblem) {
	panes, probs, err := fleetPresence(c, rt)
	if err != nil {
		return nil, false, nil
	}

	return herdr.LiveRoots(panes, rt.git), true, probs
}

type whoCmd struct{}

// Run reads herdr's live panes and reports every lane with an agent on
// it, resolved back to the plan it sits on.
//
// An unreachable socket is not fatal. This is the one command that
// needs a live server, but the rest of the board is answered from git,
// so a missing herdr travels in the document and the command still
// exits clean rather than failing the whole read.
func (w *whoCmd) Run(c *cli, rt *runtime) error {
	doc := report.NewWho(c.Root)

	panes, hostProbs, err := fleetPresence(c, rt)
	if err != nil {
		doc.AddProblem("herdr", err)
	} else {
		for _, p := range hostProbs {
			doc.AddProblem(p.name, p.err)
		}
		for _, lane := range whoLanes(panes, rt.git) {
			doc.AddLane(lane)
		}
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printWho(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// whoLanes keeps the panes with an agent, resolves each to its lane,
// and orders them so the board reads the same way twice: by repository,
// then plan, then pane. Each pane is resolved against its own host's
// git, so a pane read from another machine lands on the right lane
// rather than being lost or misresolved by the local git.
func whoLanes(panes []herdr.Pane, git gitwt.Runner) []herdr.Lane {
	staffed := make([]herdr.Pane, 0, len(panes))
	for _, p := range panes {
		if p.HasAgent() {
			staffed = append(staffed, p)
		}
	}

	lanes := herdr.Join(staffed, gitForHost(git), holdsForRoot)
	sort.Slice(lanes, func(i, j int) bool {
		if lanes[i].Repo != lanes[j].Repo {
			return lanes[i].Repo < lanes[j].Repo
		}
		if lanes[i].PlanID != lanes[j].PlanID {
			return lanes[i].PlanID < lanes[j].PlanID
		}

		return lanes[i].Pane.PaneID < lanes[j].Pane.PaneID
	})

	return lanes
}

// holdsForRoot reads a worktree root's hold patterns. A root with a
// broken or absent config yields no patterns, so its lanes resolve to
// no plan rather than failing the whole board — the same tolerance the
// rest of frit gives a repository it does not own.
func holdsForRoot(root string) repocfg.Holds {
	cfg, err := repocfg.Load(root)
	if err != nil {
		return nil
	}
	holds, err := cfg.Compiled()
	if err != nil {
		return nil
	}

	return holds
}

// printWho writes one line per live agent, and says so plainly when
// there are none. A lane that resolved to no plan or no repository is
// still listed, marked with a dash rather than hidden.
func printWho(out io.Writer, doc *report.WhoDoc) {
	if len(doc.Lanes) == 0 {
		_, _ = fmt.Fprintln(out, "no live agents")
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, lane := range doc.Lanes {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			repoLabel(lane.Repo), planLabel(lane.PlanID),
			lane.Agent, lane.Status, lane.Title)
	}
	_ = tw.Flush()
}

// repoLabel names the repository a lane sits in, or says plainly that
// it sits in none.
func repoLabel(repo string) string {
	if repo == "" {
		return "(no repo)"
	}

	return repo
}

// planLabel names the plan a lane claims, or a dash when the branch
// claims none.
func planLabel(id int64) string {
	if id == 0 {
		return "-"
	}

	return strconv.FormatInt(id, 10)
}

type initCmd struct {
	Dir     string `arg:"" optional:"" default:"." type:"path" help:"Repository to write .frit.yml into."`
	Force   bool   `short:"f" help:"Overwrite existing files."`
	Mdsmith bool   `help:"Also scaffold the mdsmith machinery: plan/proto.md and PLAN.md."`
}

// Run writes a per-repository config carrying frit's defaults. With
// --mdsmith it also lays down the plan machinery frit's workflow
// assumes: a default .mdsmith.yml, the plan/proto.md schema in the
// configured plan directory, and a PLAN.md catalog seed. That machinery
// is gated because it depends on mdsmith to be of value — without the
// config proto.md does not even lint — so a plain init never seeds a
// file the repo cannot keep correct without mdsmith. Every file is a
// shipped default, editable after, and none is rewritten over an edit
// without --force.
func (i *initCmd) Run(c *cli, rt *runtime) error {
	cfgPath, err := repocfg.Init(i.Dir, i.Force)
	if err != nil {
		return err
	}
	paths := []string{cfgPath}

	if i.Mdsmith {
		cfg, err := repocfg.Load(i.Dir)
		if err != nil {
			return err
		}
		mdsmithPath, err := scaffold.WriteMdsmithConfig(i.Dir, i.Force)
		if err != nil {
			return err
		}
		protoPath, err := scaffold.WriteProto(
			filepath.Join(i.Dir, cfg.PlanDir), i.Force)
		if err != nil {
			return err
		}
		indexPath, err := scaffold.WritePlanIndex(i.Dir, i.Force)
		if err != nil {
			return err
		}
		paths = append(paths, mdsmithPath, protoPath, indexPath)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, report.Init(paths))
	}
	for _, p := range paths {
		_, _ = fmt.Fprintf(rt.stdout, "wrote %s\n", p)
	}

	return nil
}

type skillsCmd struct {
	Dir   string `arg:"" optional:"" default:"." type:"path" help:"Repository to install the skills into."`
	Force bool   `short:"f" help:"Overwrite existing skill files."`
	Via   string `help:"How the installed skills invoke frit, e.g. \"mise exec -- frit\". Default: bare frit."`
}

// Run lays frit's bundled agent skills into the repository's
// .claude/skills, so the repo carries the instructions for driving
// frit, not just the tool.
func (s *skillsCmd) Run(c *cli, rt *runtime) error {
	paths, err := skills.Install(s.Dir, s.Force, s.Via)
	if err != nil {
		return err
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, report.Skills(paths))
	}
	for _, p := range paths {
		_, _ = fmt.Fprintf(rt.stdout, "wrote %s\n", p)
	}

	return nil
}

// runtime carries what commands need to do their work: where to
// write, how to reach git, and how to reach herdr. All are injected so
// tests never touch the real streams and can fake either subprocess.
//
// width is the terminal's column count when stdout is one, and zero
// otherwise — piped, redirected, or under test. A table that trims to
// fit reads it; zero means impose no limit, so a pipe and a golden get
// the full, stable output rather than a width-dependent one.
type runtime struct {
	stdout  io.Writer
	stderr  io.Writer
	git     gitwt.Runner
	gitPipe gitwt.PipeRunner
	herdr   herdr.Runner
	width   int
}

// herdrRunner is how commands reach a herdr server. It is a package
// variable rather than wired straight to herdr.Exec so a test can
// install a fake socket without a herdr on the machine — git commands
// fake with a real temporary repository, but there is no throwaway
// herdr server to stand up the same way.
var herdrRunner = herdr.Exec

// exitCode unwinds kong's os.Exit call so run can return it instead.
// kong exits the process on --help and on a usage error, which would
// take the test binary with it.
type exitCode int

type reposCmd struct{}

// Run lists every repository under the configured root.
func (r *reposCmd) Run(c *cli, rt *runtime) error {
	repos, err := discover.Repos(c.Root, rt.git)
	if err != nil {
		return err
	}

	doc := report.Repos(c.Root, repos)
	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printRepos(rt.stdout, doc)

	return nil
}

type plansCmd struct {
	Dir    string `help:"Override the plan directory; the default is each repository's .frit.yml." env:"FRIT_PLAN_DIR"`
	Detail bool   `help:"List every plan file, not just a count." short:"d"`
}

// planDir answers where one repository keeps its plans: the override
// when given, otherwise whatever that repository declares for itself.
func (p *plansCmd) planDir(repoPath string) (string, error) {
	if p.Dir != "" {
		return p.Dir, nil
	}

	cfg, err := repocfg.Load(repoPath)
	if err != nil {
		return "", err
	}

	return cfg.PlanDir, nil
}

// Run reads plan files off every ref of every repository under the
// root and indexes them. Nothing is checked out.
func (p *plansCmd) Run(c *cli, rt *runtime) error {
	repos, err := discover.Repos(c.Root, rt.git)
	if err != nil {
		return err
	}

	host := hostname()

	doc := report.NewPlans(c.Root, host)
	for _, repo := range repos {
		dir, err := p.planDir(repo.Path)
		if err != nil {
			doc.AddProblem(repo.Name, err)
			continue
		}

		files, err := plans.Collect(repo.Path, dir,
			rt.git, rt.gitPipe)
		if err != nil {
			// One unreadable repository must not blind the rest.
			doc.AddProblem(repo.Name, err)
			continue
		}

		entries, problems := index.Build(host, repo.Name,
			gitobj.DefaultRef(repo.Path, rt.git), files)
		for _, problem := range problems {
			// A file with no front matter is not a plan, only noise on a
			// board that keeps notes beside its plans; hold it back unless
			// everything was asked for.
			if !c.All && errors.Is(problem, planmeta.ErrNoFrontMatter) {
				continue
			}
			doc.AddProblem(repo.Name, problem)
		}
		doc.AddRepo(repo.Name, entries)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printPlans(rt.stdout, doc, p.Detail)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printPlans writes one line per repository, and with detail every
// plan under it. --json is not affected by --detail: a person is
// shown a summary by default, a consumer always gets the whole index.
func printPlans(out io.Writer, doc *report.PlansDoc, detail bool) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	for _, repo := range doc.Repos {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", repo.Name,
			plural(len(repo.Plans), "plan"), statusBar(repo.Counts()))
		if !detail {
			continue
		}
		for _, p := range repo.Plans {
			_, _ = fmt.Fprintf(tw, "  %d %s\t%s\t%s\n",
				p.ID, p.Status, p.Title, plural(p.RefCount, "ref"))
		}
	}
	_ = tw.Flush()
}

// hostname names the machine this run reads, falling back to a stable
// label so a plan key is well formed even when the hostname is
// unreadable.
func hostname() string {
	host, err := os.Hostname()
	if err != nil {
		return "localhost"
	}

	return host
}

// gatherFleet reads every repository's plans and holds into the view
// the discovery verbs share.
func gatherFleet(c *cli, rt *runtime) (fleet.Result, error) {
	res, err := fleet.Gather(c.Root, hostname(), rt.git, rt.gitPipe)
	if err != nil {
		return res, err
	}
	observeHolds(&res, rt, time.Now())

	return res, nil
}

// observeHolds folds this run's view of every held work ref into the
// per-host observation store and marks the plans whose takeover window
// has matured. Observation piggybacks on every fleet-reading verb —
// the one side effect a read verb owns is this local state file, never
// a ref — and is best-effort: an unwritable store only delays a
// takeover, so it fails quiet rather than failing the verb.
//
// The window and sample gap are the repository's own — read off the
// coordinate the gather already resolved, so a repo declaring its own
// clock in .frit.yml is watched on it rather than the discovery
// defaults (F12) — and the threshold backs off by k·T, k the takeover
// markers already in the chain, so oscillation between two live but
// quiet agents damps out (F3).
func observeHolds(res *fleet.Result, rt *runtime, now time.Time) {
	path, err := observe.Path()
	if err != nil {
		return
	}
	state := observe.Load(path)
	var panes []herdr.Pane
	panesRead, panesOK := false, false
	for i := range res.Plans {
		p := &res.Plans[i]
		key := observe.Key(p.Repo, p.ID)
		if p.HoldTip == "" {
			// No work ref, no window; dropping the key keeps the state
			// to what this host actually watches. A ref that exists is
			// observed whether or not it counts as a hold — glyph
			// evidence needs a matured window on a ref the hold
			// filters already dropped.
			delete(state, key)
			continue
		}
		window, sampleGap := staleClock(res, p.Repo)
		w := discovery.Observe(state[key], p.HoldTip, now, sampleGap)
		state[key] = w
		threshold := window
		if coord, ok := res.Coords[p.Repo]; ok {
			k := claim.TakeoverCount(coord.Path, p.ID, coord.Base, p.HoldTip, rt.git)
			threshold = time.Duration(k+1) * window
			if p.Held {
				// One List call serves every held plan in the fleet
				// rather than one herdr round-trip per plan. An
				// unreachable herdr is unknown, not dead, exactly as
				// a per-plan SessionDead call would have read it —
				// so a failed read is never retried as a live pane
				// list with nothing in it.
				if !panesRead {
					var listErr error
					panes, listErr = herdr.List(rt.herdr)
					panesRead, panesOK = true, listErr == nil
				}
				if panesOK {
					p.Dead = deadSession(rt, coord, *p, panes)
				}
			}
		}
		p.Stale = discovery.StaleHold(w, now, threshold, sampleGap)
		p.StaleFor = w.Span()
	}
	_ = observe.Save(path, state)
}

// deadSession reports whether a held plan's bound session herdr
// positively confirms is gone (S76): the marker at the tip the fleet
// already observed names who to ask, and only a herdr that actually
// answered may say so — see herdr.SessionDeadIn. panes is the one
// List read observeHolds already made for the whole fleet; an unheld
// plan or one whose marker cannot be read answers false, falling back
// to the staleness window exactly as before this signal existed.
func deadSession(
	rt *runtime, coord fleet.Coord, p discovery.Plan, panes []herdr.Pane,
) bool {
	if !p.Held {
		return false
	}
	m, ok := claim.ReadMarker(coord.Path,
		claim.LeaseOptions{PlanID: p.ID, Remote: coord.Remote}, p.HoldTip, rt.git)
	if !ok {
		return false
	}

	return herdr.SessionDeadIn(panes, m.Session)
}

// staleClock is the staleness window and sample gap to watch a
// repository's holds on: its own coordinate when the gather resolved
// one, the discovery defaults otherwise — an ambiguous repository
// withholds a coordinate entirely, and repocfg.Default already fills
// a repository declaring nothing with the same defaults, so this only
// ever falls back for a repository the gather could not place.
func staleClock(res *fleet.Result, repo string) (time.Duration, time.Duration) {
	coord, ok := res.Coords[repo]
	if !ok || coord.TakeoverWindow == 0 {
		return discovery.DefaultTakeoverWindow, discovery.DefaultSampleGap
	}

	return coord.TakeoverWindow, coord.SampleGap
}

// ambiguousRepo is the refusal a mutating verb reports when a plan's
// repository name is shared by another checkout under the root. The
// fleet keys plans by basename, so frit cannot tell which checkout to
// mint the lease in — the gather withholds the coordinate rather than
// pick whichever the walk reached last, and this names why. Renaming
// one repository resolves it.
func ambiguousRepo(name string) string {
	return fmt.Sprintf(
		"its repository name %q is shared by another checkout under the "+
			"root; rename one so the claim lands in the right repository",
		name)
}

// problemAdder is the AddProblem every discovery document carries. The
// commands share one loop to move a gather's problems onto whichever
// document they are building.
type problemAdder interface {
	AddProblem(repo string, err error)
}

// carryProblems moves a gather's per-repository failures onto a
// document, so a single broken checkout travels in the report rather
// than blinding the board. A benign not-a-plan file is held back unless
// all is set, since a plan directory routinely holds a PLAN.md and
// notes that would otherwise drown the real failures.
func carryProblems(doc problemAdder, problems []fleet.Problem, all bool) {
	for _, p := range problems {
		if p.NotPlan && !all {
			continue
		}
		doc.AddProblem(p.Repo, p.Err)
	}
}

// resolveSelector turns a command's optional selector into one plan.
//
// A selector given on the command line is resolved by id or slug; an
// empty one is inferred from the current directory, the cwd join run
// backwards. An ambiguous or unknown selector returns the error
// discovery raised, which the command surfaces and exits non-zero on.
//
// When guardForeign is set — for a verb that acts on the lane: claim,
// start, nudge or open — it refuses an empty selector inferred from a
// checkout another host holds. A read-only report passes false: refusing
// a status query hands out no lane and only blocks a harmless read.
//
// yield passes false too, but for a third reason, not the read-only
// one: it acts on the lane, but a foreign hold is exactly the state
// yield exists to act on — refusing it here would refuse the plan it
// was built for. Its own StillHeldError check stands in for the guard,
// refusing the one case that actually needs it: this lane still holding
// the live lease.
func resolveSelector(
	rt *runtime, selector string, plans []discovery.Plan, guardForeign bool,
) (discovery.Plan, error) {
	if selector != "" {
		return discovery.Resolve(selector, plans)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return discovery.Plan{}, err
	}

	// Preflight the shared checkout: inferring a plan from the cwd must
	// not hand this session the lane another host holds. A claim minted
	// on one machine and worked in a clone a second agent shares would
	// otherwise read as that agent's own current plan. Only a verb that
	// acts on the lane guards; a read-only report has nothing to refuse.
	if guardForeign {
		if host, foreign := fleet.ForeignHold(
			cwd, hostname(), rt.git, holdsForRoot); foreign {
			return discovery.Plan{}, fmt.Errorf(
				"the current worktree stands on a claim held by %s; "+
					"pass a plan explicitly to work another", host)
		}
	}

	repo, id, ok := fleet.CurrentPlanID(cwd, rt.git, holdsForRoot)
	if !ok {
		return discovery.Plan{}, errors.New(
			"no plan given and none inferred from the current directory")
	}

	// Both halves of the key are known here, so resolve the exact plan
	// rather than matching the id fleet-wide, where another repository's
	// same id would read as ambiguous.
	return discovery.ByRepoID(repo, id, plans)
}

// sortFlags is the shared ordering control the list commands embed, so
// --sort and --reverse read the same on board, ready, pick and find.
type sortFlags struct {
	Sort    string `help:"Order by status, repo, id or held (default: the command's own order)."`
	Reverse bool   `help:"Reverse the order."`
}

// order applies a command's sort choice to the plans it gathered. An
// empty key keeps the command's own order, which --reverse alone still
// turns end to end; an unrecognised key is a usage error rather than a
// silent no-op.
func (s sortFlags) order(plans []discovery.Plan) ([]discovery.Plan, error) {
	if s.Sort == "" {
		if s.Reverse {
			discovery.Reverse(plans)
		}

		return plans, nil
	}

	key, ok := discovery.ParseSortKey(s.Sort)
	if !ok {
		return nil, fmt.Errorf(
			"unknown sort %q: want status, repo, id or held", s.Sort)
	}
	discovery.Sort(plans, key, s.Reverse)

	return plans, nil
}

type readyCmd struct {
	sortFlags
}

// Run lists every plan startable now: not begun, held by nobody, and
// with every dependency done, across all repositories and refs.
func (r *readyCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	list, err := r.order(discovery.Ready(res.Plans))
	if err != nil {
		return err
	}

	doc := report.NewReady(c.Root, hostname())
	carryProblems(doc, res.Problems, c.All)
	doc.SetPlans(list)

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printReady(rt.stdout, doc, rt.width)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

type pickCmd struct {
	N     int    `short:"n" default:"5" help:"How many candidates to list; 0 for all."`
	Go    bool   `help:"Claim and start the top plan; resume an unheld in-progress one, skip a lost race to the next."`
	Phase string `help:"Phase to dispatch under --go; default is the plan's next open phase."`
	sortFlags
}

// Run lists the startable plans ranked by how much each unblocks,
// trimmed to the number asked for. With --go it stops listing and starts
// the top candidate outright — the selection the skill used to make by
// hand — running start's own claim-and-stand-up path on it.
func (pc *pickCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	if pc.Go {
		return pc.start(c, rt, res)
	}

	list, err := pc.order(discovery.Pick(res.Plans, pc.N))
	if err != nil {
		return err
	}

	doc := report.NewPick(c.Root, hostname())
	carryProblems(doc, res.Problems, c.All)
	doc.SetPlans(list)

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printReady(rt.stdout, readyView(doc), rt.width)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// readyView lets pick reuse the ready table: both print a ranked list
// of startable plans, so the rendering is shared rather than copied.
func readyView(doc *report.PickDoc) *report.ReadyDoc {
	return &report.ReadyDoc{Plans: doc.Plans}
}

// start runs pick --go: walk the ranked candidates and run start's own
// claim-and-stand-up path on the first that takes. A candidate whose
// claim loses its race is skipped for the next — the retry the skill
// used to spell out by hand — so a live hold on the top pick does not
// stall the fleet. When nothing is startable, or every candidate loses
// its race, it prints the same empty answer a bare pick gives.
func (pc *pickCmd) start(c *cli, rt *runtime, res fleet.Result) error {
	for _, plan := range discovery.Candidates(res.Plans) {
		doc, lost, err := buildStart(c, rt, res, plan, pc.Phase, "", false, true)
		if err != nil {
			return err
		}
		if lost {
			continue
		}

		return renderStart(c, rt, doc)
	}

	return pc.emptyStart(c, rt, res)
}

// emptyStart prints the ranked-list report with no candidate — the same
// "nothing startable" answer a bare pick gives — for when pick --go
// finds nothing to start or loses every race.
func (pc *pickCmd) emptyStart(c *cli, rt *runtime, res fleet.Result) error {
	doc := report.NewPick(c.Root, hostname())
	carryProblems(doc, res.Problems, c.All)

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printReady(rt.stdout, readyView(doc), rt.width)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// rescueRefsFor lists a plan's rescue refs, so next and show surface
// stranded commits alongside the phase or the dependency tree. A plan
// whose repository coordinate the gather withheld — the ambiguous-name
// case — has nowhere to read a rescue ref from, so it reads as none
// rather than guessing a repository.
func rescueRefsFor(rt *runtime, res fleet.Result, plan discovery.Plan) []string {
	coord, ok := res.Coords[plan.Repo]
	if !ok {
		return []string{}
	}

	return claim.RescueRefs(coord.Path, coord.Remote, plan.ID, rt.git)
}

type nextCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
}

// Run reports the first phase of a plan not yet done — the phase an
// executor would pick up — for the plan named, or the one the current
// worktree is on.
func (n *nextCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}
	plan, err := resolveSelector(rt, n.Selector, res.Plans, false)
	if err != nil {
		return err
	}

	doc := report.NewNext(c.Root, plan)
	carryProblems(doc, res.Problems, c.All)
	doc.SetRescue(rescueRefsFor(rt, res, plan))

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printNext(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

type showCmd struct {
	Selector string `arg:"" optional:"" help:"Plan id or slug; empty infers from the cwd."`
}

// Run shows a plan and its upstream dependencies, so "what blocks this"
// has a direct answer. By default only the blockers are shown — the
// upstreams not yet done — because a finished dependency blocks
// nothing. --all shows the whole dependency tree, done edges included.
func (s *showCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}
	plan, err := resolveSelector(rt, s.Selector, res.Plans, false)
	if err != nil {
		return err
	}

	doc := report.NewShow(c.Root, discovery.Dependencies(plan, res.Plans))
	carryProblems(doc, res.Problems, c.All)
	doc.SetRescue(rescueRefsFor(rt, res, plan))

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printShow(rt.stdout, doc, c.All)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

type boardCmd struct {
	Wip     bool   `help:"Only plans in progress, not those merely not started."`
	Columns string `help:"Comma-separated columns to show: host, repo, id, status, held, agent, title."`
	sortFlags
}

// Run shows the board of outstanding work: every unfinished plan, the
// lane that holds it, the machine it lives on, and the agent live on it
// now — in-progress first. --wip narrows it to what is actually under
// way.
//
// The agent half needs herdr; a missing socket leaves the git board
// standing with the agent column marked unknown rather than failing.
func (b *boardCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	cols, err := selectBoardColumns(b.Columns)
	if err != nil {
		return err
	}
	list, err := b.order(discovery.Board(res.Plans, b.Wip))
	if err != nil {
		return err
	}

	live, present, hostProbs := liveByBranch(c, rt)
	doc := report.NewBoard(c.Root, present)
	carryProblems(doc, res.Problems, c.All)
	for _, p := range hostProbs {
		doc.AddProblem(p.name, p.err)
	}
	for _, p := range list {
		agent, status := agentFor(p, live)
		doc.AddPlan(p, agent, status)
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printBoard(rt.stdout, doc, rt.width, cols)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// liveByBranch keys every staffed lane by the branch its worktree is
// on, so a plan can find the agent working one of its hold branches. A
// missing socket yields no map and false, which the board reads as
// "presence unknown".
func liveByBranch(
	c *cli, rt *runtime,
) (map[string]herdr.Lane, bool, []hostProblem) {
	panes, probs, err := fleetPresence(c, rt)
	if err != nil {
		return nil, false, nil
	}

	live := map[string]herdr.Lane{}
	for _, lane := range whoLanes(panes, rt.git) {
		if lane.Branch != "" {
			live[lane.Branch] = lane
		}
	}

	return live, true, probs
}

// agentFor finds the agent working one of a plan's hold branches, if
// any is live. A plan nobody holds has no lane to be worked on, so it
// reports none.
func agentFor(
	p discovery.Plan, live map[string]herdr.Lane,
) (agent, status string) {
	for _, branch := range p.Holds {
		if lane, ok := live[branch]; ok {
			return lane.Pane.Agent, lane.Pane.Presence()
		}
	}

	return "", ""
}

// boardRow is one board line's cells, computed once so the title can be
// trimmed against the width the other columns take.
// boardCols is the board's full set of columns, in default order. The
// two flexible ones — held and title — have no natural bound and are
// trimmed to fit; the rest set their width from their content.
var boardCols = []string{"host", "repo", "id", "status", "held", "agent", "title"}

// boardColAliases lets a person name a column by the word they think in
// — description for the title, lane for who holds it.
var boardColAliases = map[string]string{
	"description": "title",
	"desc":        "title",
	"lane":        "held",
	"machine":     "host",
}

// selectBoardColumns resolves a --columns value to an ordered column
// list, defaulting to all of them. An unknown name is a usage error, so
// a typo is caught rather than silently dropping a column.
func selectBoardColumns(spec string) ([]string, error) {
	if strings.TrimSpace(spec) == "" {
		return boardCols, nil
	}

	var out []string
	for _, raw := range strings.Split(spec, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if canon, ok := boardColAliases[name]; ok {
			name = canon
		}
		if !slices.Contains(boardCols, name) {
			return nil, fmt.Errorf("unknown column %q: want %s",
				name, strings.Join(boardCols, ", "))
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, errors.New("no columns selected")
	}

	return out, nil
}

// boardCell renders one plan's value for a named column.
func boardCell(name string, doc *report.BoardDoc, p report.BoardPlan) string {
	switch name {
	case "host":
		return p.Host
	case "repo":
		return p.Repo
	case "id":
		return strconv.FormatInt(p.ID, 10)
	case "status":
		return p.Status
	case "held":
		return heldCell(p)
	case "agent":
		return agentLabel(doc.Presence, p.Agent, p.AgentStatus)
	default: // title
		return p.Title
	}
}

// printBoard writes one line per outstanding plan, over the chosen
// columns. A clear board says so plainly.
//
// When width is positive — stdout is a terminal — the flexible columns
// are trimmed so each row fits. A width of zero, which is what a pipe or
// a redirect gives, imposes no limit: the full text travels so a
// downstream reader gets everything.
func printBoard(
	out io.Writer, doc *report.BoardDoc, width int, cols []string,
) {
	if len(doc.Plans) == 0 {
		_, _ = fmt.Fprintln(out, "nothing outstanding")
		return
	}

	rows := make([][]string, len(doc.Plans))
	for i, p := range doc.Plans {
		row := make([]string, len(cols))
		for c, name := range cols {
			row[c] = boardCell(name, doc, p)
		}
		rows[i] = row
	}

	if width > 0 {
		fitBoard(width, rows, cols)
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		_, _ = fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	_ = tw.Flush()
}

// fitBoard trims the flexible columns present — the lane and the title —
// so the widest row fits the terminal. The fixed columns and the gaps
// are subtracted first; what remains goes to the title, with the lane
// kept whole until doing so would crowd the title below a readable
// minimum, at which point the lane gives up its tail first.
func fitBoard(width int, rows [][]string, cols []string) {
	maxw := make([]int, len(cols))
	for _, r := range rows {
		for c := range r {
			if w := textw.Width(r[c]); w > maxw[c] {
				maxw[c] = w
			}
		}
	}

	fixed, heldIdx, titleIdx := 0, -1, -1
	for c, name := range cols {
		switch name {
		case "held":
			heldIdx = c
		case "title":
			titleIdx = c
		default:
			fixed += maxw[c]
		}
	}
	if heldIdx < 0 && titleIdx < 0 {
		return
	}

	budget := width - fixed - (len(cols)-1)*2
	if budget < 1 {
		budget = 1
	}

	heldW, titleW := allocateFlex(budget, maxw, heldIdx, titleIdx)
	if heldIdx >= 0 {
		trimColumn(rows, heldIdx, heldW)
	}
	if titleIdx >= 0 {
		trimColumn(rows, titleIdx, titleW)
	}
}

// allocateFlex splits a budget between the lane and the title. With
// both, the title is favoured — the lane keeps its natural width until
// that would leave the title below a minimum. With only one, it takes
// the whole budget.
func allocateFlex(budget int, maxw []int, heldIdx, titleIdx int) (held, title int) {
	const minTitle = 12
	switch {
	case heldIdx >= 0 && titleIdx >= 0:
		held = maxw[heldIdx]
		title = budget - held
		if title < minTitle {
			if budget <= minTitle {
				held, title = 0, budget
			} else {
				held, title = budget-minTitle, minTitle
			}
		}
		if held < 0 {
			held = 0
		}
	case titleIdx >= 0:
		title = budget
	default:
		held = budget
	}

	return held, title
}

// trimColumn caps every row's cell in one column to a width.
func trimColumn(rows [][]string, col, width int) {
	for _, r := range rows {
		r[col] = textw.Truncate(r[col], width)
	}
}

// laneShorts drops the redundant plan/<id>- prefix a hold branch carries,
// since the id is already its own column. A branch on a different
// convention is left whole rather than mangled.
func laneShorts(holds []string, id int64) []string {
	prefix := fmt.Sprintf("plan/%d-", id)
	out := make([]string, len(holds))
	for i, h := range holds {
		out[i] = strings.TrimPrefix(h, prefix)
	}

	return out
}

// heldCell renders the board's held column: the lane names, with a
// stale marker and its age appended once the takeover window has
// matured, or a dead marker once herdr confirms the bound session is
// gone — the held-stale cell of the verb-state table, told apart from
// a live hold at a glance rather than by a second read.
func heldCell(p report.BoardPlan) string {
	label := heldLabel(laneShorts(p.Holds, p.ID))
	switch {
	case p.Stale:
		age := time.Duration(p.StaleSeconds) * time.Second
		return fmt.Sprintf("%s (stale %s)", label, age.Round(time.Minute))
	case p.Dead:
		return fmt.Sprintf("%s (dead)", label)
	}

	return label
}

// heldLabel names the lane holding a plan, or a dash when nobody does.
// Several holds are joined, so a plan claimed on two machines' branches
// reads as both rather than one.
func heldLabel(holds []string) string {
	if len(holds) == 0 {
		return "-"
	}

	return strings.Join(holds, ",")
}

// terminalWidth reports the column count when the writer is a terminal,
// and zero otherwise. A bytes.Buffer under test, a pipe, or a file all
// answer zero, which the tables read as "no limit" — so only an
// interactive terminal ever trims output, and a golden never shifts
// with a window.
func terminalWidth(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	if !term.IsTerminal(int(f.Fd())) {
		return 0
	}
	width, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}

	return width
}

// agentLabel names the agent on a lane and how it reads. With herdr
// unreachable the column is unknown rather than empty, since a missing
// socket is not the same as no agent.
func agentLabel(presence bool, agent, status string) string {
	switch {
	case !presence:
		return "?"
	case agent == "":
		return "-"
	case status == "":
		return agent
	default:
		return agent + " (" + status + ")"
	}
}

type findCmd struct {
	Query string `arg:"" help:"Text to match in plan titles and summaries."`
	sortFlags
}

// Run searches plan titles and summaries across every repository and
// ref for a query, for when the topic is remembered but not the id.
func (f *findCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	list, err := f.order(discovery.Find(f.Query, res.Plans))
	if err != nil {
		return err
	}

	doc := report.NewFind(c.Root, hostname(), f.Query)
	carryProblems(doc, res.Problems, c.All)
	doc.SetPlans(list)

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printFind(rt.stdout, doc, rt.width)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printFind writes one line per match, carrying the status because find
// answers with plans in any state, not only startable ones. A search
// that matched nothing says so with the query, so an empty result is
// never mistaken for a broken command.
func printFind(out io.Writer, doc *report.FindDoc, width int) {
	if len(doc.Plans) == 0 {
		_, _ = fmt.Fprintf(out, "no plan matches %q\n", doc.Query)
		return
	}

	rows := make([][]string, 0, len(doc.Plans))
	for _, p := range doc.Plans {
		rows = append(rows, []string{
			p.Repo, strconv.FormatInt(p.ID, 10), statusLabel(p.Status), p.Title,
		})
	}
	fitTable(out, width, rows)
}

// printNext writes the plan and the phase to pick up, the seed a
// dispatch verb will one day type for you: a plan id, a phase number,
// the tier and gate its Execution row names, and its own section's
// prose — the phase an executor reads, not just its title. A phase
// with no Execution row prints a dash rather than the plan's own
// tier — the gap is said explicitly, in doc.Problems. A plan with no
// open phase says why — done, or carrying no phase ledger at all.
func printNext(out io.Writer, doc *report.NextDoc) {
	p := doc.Plan
	if !doc.HasPhase {
		if p.Status == planmeta.StatusDone {
			_, _ = fmt.Fprintf(out, "plan %d is done\n", p.ID)
			printRescue(out, doc.Rescue)
			return
		}
		_, _ = fmt.Fprintf(out, "%s %d  %s  (no phase ledger)\n",
			p.Repo, p.ID, p.Title)
		printRescue(out, doc.Rescue)
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "%s\t%d\tphase %s\t%s\t%s\t%s\n",
		p.Repo, p.ID, doc.Phase.N, doc.Phase.Title,
		modelLabel(doc.Phase.Tier), orDash(doc.Phase.Gate))
	_ = tw.Flush()
	if doc.Phase.Body != "" {
		_, _ = fmt.Fprintf(out, "\n%s\n", doc.Phase.Body)
	}
	printRescue(out, doc.Rescue)
}

// printRescue lists a plan's rescue refs, so stranded commits a
// scavenge or a yield parked are found again. Silent when there are
// none, the same convention orphans and stale use for a clean report.
func printRescue(out io.Writer, refs []string) {
	if len(refs) == 0 {
		return
	}
	_, _ = fmt.Fprintln(out, "\nrescue refs:")
	for _, ref := range refs {
		_, _ = fmt.Fprintf(out, "  %s\n", ref)
	}
}

// orDash names an empty string as a dash, so a blank cell reads as
// "nothing here" rather than looking like a column the renderer
// dropped.
func orDash(s string) string {
	if s == "" {
		return "-"
	}

	return s
}

// printShow writes the plan and its upstream dependencies, one plan per
// line, indented by depth so the walk reads top to bottom. By default
// only the blockers are shown; with all, every dependency is. When the
// view has nothing under the root, that is said plainly rather than
// left as a bare line.
//
// The document always carries the whole tree — all decides how much a
// person is shown, never what a consumer receives, the same split as
// plans --detail.
func printShow(out io.Writer, doc *report.ShowDoc, all bool) {
	if doc.Goal != "" {
		_, _ = fmt.Fprintf(out, "Goal: %s\n\n", doc.Goal)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	printDep(tw, doc.Tree, 0, all)
	if visibleDeps(doc.Tree, all) == 0 {
		_, _ = fmt.Fprintln(tw, "  "+emptyDepsNote(all))
	}
	_ = tw.Flush()
	printRescue(out, doc.Rescue)
}

// printDep writes one dependency node and recurses into its upstreams.
// In the default view a satisfied upstream is pruned with its whole
// subtree, because a done dependency blocks nothing; all keeps them.
func printDep(out io.Writer, node report.DepCard, depth int, all bool) {
	indent := strings.Repeat("  ", depth)
	if !node.Found {
		_, _ = fmt.Fprintf(out, "%s?\t%d\t(unknown plan)\n", indent, node.ID)
		return
	}
	_, _ = fmt.Fprintf(out, "%s%s\t%d\t%s\n",
		indent, statusLabel(node.Status), node.ID, node.Title)
	for _, child := range node.Deps {
		if !all && satisfied(child) {
			continue
		}
		printDep(out, child, depth+1, all)
	}
}

// satisfied reports whether an edge is done and so blocks nothing. An
// unresolved edge is never satisfied: one frit cannot confirm done is
// treated as a blocker.
func satisfied(node report.DepCard) bool {
	return node.Found && node.Status == planmeta.StatusDone
}

// visibleDeps counts the root's dependencies the current view will
// print, so an empty walk can be labelled honestly.
func visibleDeps(root report.DepCard, all bool) int {
	n := 0
	for _, child := range root.Deps {
		if all || !satisfied(child) {
			n++
		}
	}

	return n
}

// emptyDepsNote explains an empty walk. The default view is about
// blockers, so an empty one means nothing blocks the plan — whether it
// has no dependencies or every one is done. --all is about the edges
// themselves, so an empty one means there are none.
func emptyDepsNote(all bool) string {
	if all {
		return "(no dependencies)"
	}

	return "(nothing blocks it)"
}

// statusLabel renders a plan's status glyph, or a dash when it carries
// none, so the column stays aligned.
func statusLabel(status string) string {
	if status == "" {
		return "-"
	}

	return status
}

// printReady writes one line per startable plan, carrying the model
// tier because it is what a person reaches for the plan to dispatch it.
// A fleet with nothing startable says so plainly.
func printReady(out io.Writer, doc *report.ReadyDoc, width int) {
	if len(doc.Plans) == 0 {
		_, _ = fmt.Fprintln(out, "nothing startable")
		return
	}

	rows := make([][]string, 0, len(doc.Plans))
	for _, p := range doc.Plans {
		rows = append(rows, []string{
			p.Repo, strconv.FormatInt(p.ID, 10), modelLabel(p.Model), p.Title,
		})
	}
	fitTable(out, width, rows)
}

// fitTable renders rows as an aligned table, trimming each row's final
// cell so the widest line fits the terminal. The last column is the
// flexible one — a title, the text with no natural bound — and the
// others take their own width from their content. A width of zero, a
// pipe or a test, imposes no limit and prints in full.
func fitTable(out io.Writer, width int, rows [][]string) {
	if width > 0 {
		fitLastColumn(width, rows)
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		_, _ = fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	_ = tw.Flush()
}

// fitLastColumn caps each row's final cell to the columns left after the
// fixed columns and the gaps between them, so no rendered line spills
// past width. The cap never falls below a readable minimum: a narrow
// terminal trims hard rather than to nothing.
func fitLastColumn(width int, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	last := len(rows[0]) - 1
	if last < 1 {
		return
	}

	fixed := 0
	for c := 0; c < last; c++ {
		m := 0
		for _, r := range rows {
			if w := textw.Width(r[c]); w > m {
				m = w
			}
		}
		fixed += m
	}

	const minCol = 12
	budget := width - fixed - last*2
	if budget < minCol {
		budget = minCol
	}
	for _, r := range rows {
		r[last] = textw.Truncate(r[last], budget)
	}
}

// modelLabel names the tier a plan asks for, or a dash when it names
// none, so the column stays aligned.
func modelLabel(model string) string {
	if model == "" {
		return "-"
	}

	return model
}

// printProblems reports the repositories that could not be read.
//
// They are written after the table rather than interleaved with it,
// because stdout is what a pipe keeps and a failure must not land in
// the middle of it. Under --json they are not printed at all: the
// document carries them, and it is then the whole report.
func printProblems(errw io.Writer, problems []report.Problem) {
	for _, p := range problems {
		_, _ = fmt.Fprintf(errw, "frit: %s: %s\n", p.Repo, p.Message)
	}
}

// statusBar renders the lifecycle breakdown in a fixed order, so two
// repositories' lines stay comparable at a glance.
func statusBar(counts map[string]int) string {
	order := []string{
		planmeta.StatusInProgress,
		planmeta.StatusNotStarted,
		planmeta.StatusDone,
		planmeta.StatusSuperseded,
	}

	parts := make([]string, 0, len(order))
	for _, status := range order {
		if counts[status] > 0 {
			parts = append(parts,
				fmt.Sprintf("%s %d", status, counts[status]))
		}
	}

	return strings.Join(parts, "  ")
}

type versionCmd struct{}

// Run prints the build version.
func (v *versionCmd) Run(c *cli, rt *runtime) error {
	if c.JSON {
		return report.WriteJSON(rt.stdout, report.Version(version))
	}
	_, _ = fmt.Fprintln(rt.stdout, version)

	return nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point. It returns the process exit code:
// 0 on success, 1 on a runtime failure, 2 on a usage error.
func run(args []string, stdout, stderr io.Writer) (code int) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if c, ok := r.(exitCode); ok {
			code = int(c)
			return
		}
		panic(r)
	}()

	var c cli
	rt := &runtime{
		stdout:  stdout,
		stderr:  stderr,
		git:     gitwt.Exec,
		gitPipe: gitwt.ExecPipe,
		herdr:   herdrRunner,
		width:   terminalWidth(stdout),
	}

	parser, err := newParser(&c, stdout, stderr)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "frit: %v\n", err)
		return 2
	}

	ctx, err := parser.Parse(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "frit: %v\n", err)
		return 2
	}

	// An explicit --width overrides detection, so a table can be fit to
	// a terminal frit cannot see — behind a pipe, or under a harness
	// that indents the output.
	if c.Width > 0 {
		rt.width = c.Width
	}

	if err := ctx.Run(&c, rt); err != nil {
		_, _ = fmt.Fprintf(stderr, "frit: %v\n", err)
		return 1
	}

	return 0
}

// newParser builds the kong parser with configuration layered in.
func newParser(c *cli, stdout, stderr io.Writer) (*kong.Kong, error) {
	workdir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	return kong.New(c,
		kong.Name("frit"),
		kong.Description(description),
		kong.Writers(stdout, stderr),
		kong.UsageOnError(),
		// Turn kong's process exit into a panic run recovers, so the
		// exit code survives without killing a test binary.
		kong.Exit(func(code int) { panic(exitCode(code)) }),
		kong.Configuration(kongyaml.Loader,
			config.Paths(os.Getenv, workdir)...),
		// kong's own `env:` tag is applied beneath its config
		// resolver, so a config file would silently outrank the
		// environment. This resolver restores the order operators
		// expect — environment over file — and is registered after
		// Configuration because a later resolver wins.
		kong.Resolvers(envResolver(os.Getenv)),
	)
}

// envResolver reads a flag's `env:` names directly, so the
// environment outranks any configuration file.
func envResolver(getenv func(string) string) kong.Resolver {
	return kong.ResolverFunc(func(
		_ *kong.Context, _ *kong.Path, flag *kong.Flag,
	) (any, error) {
		for _, name := range flag.Envs {
			if v := getenv(name); v != "" {
				return v, nil
			}
		}

		return nil, nil
	})
}

// printRepos renders the repository listing as an aligned table.
func printRepos(out io.Writer, doc report.ReposDoc) {
	if len(doc.Repos) == 0 {
		_, _ = fmt.Fprintln(out, "no git repositories found")
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, repo := range doc.Repos {
		_, _ = fmt.Fprintf(tw, "%s\t\t%s\n",
			repo.Name, plural(len(repo.Worktrees), "worktree"))
		for _, wt := range repo.Worktrees {
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\n",
				wt.Name, ref(wt), note(wt))
		}
	}
	_ = tw.Flush()
}

// plural renders a count with its noun, pluralised by the only rule
// this tool needs.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}

	return fmt.Sprintf("%d %ss", n, noun)
}

// ref is what the worktree is on, in the form a person recognises.
func ref(wt report.Worktree) string {
	switch {
	case wt.Bare:
		return "(bare)"
	case wt.Detached:
		return "(detached)"
	case wt.Branch == "":
		return "(unknown)"
	default:
		return wt.Branch
	}
}

// note flags the states that make a lane worth a second look. An
// empty note is the ordinary case.
func note(wt report.Worktree) string {
	switch {
	case wt.Bare:
		return ""
	case !wt.HasCommit:
		return "no commit"
	case wt.Prunable:
		return "prunable"
	case wt.Locked:
		return "locked"
	default:
		return ""
	}
}
