// Command frit indexes plans, worktrees and agents across a fleet.
//
// This binary is read-only by construction. It shells out to git to
// read state and prints what it finds; nothing here writes to a
// repository, claims a plan, or talks to an agent.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"

	"github.com/jeduden/frit/internal/config"
	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/discovery"
	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/herdr"
	"github.com/jeduden/frit/internal/index"
	"github.com/jeduden/frit/internal/lanes"
	"github.com/jeduden/frit/internal/planmeta"
	"github.com/jeduden/frit/internal/plans"
	"github.com/jeduden/frit/internal/repocfg"
	"github.com/jeduden/frit/internal/report"
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

	// JSON is global rather than per command because every command
	// answers it, and an agent should not have to remember which ones.
	JSON bool `help:"Emit the report as JSON instead of a table."`

	// All un-hides what the default view holds back: satisfied
	// dependencies in show, and files in a plan directory that carry no
	// front matter and so are not plans. It is global because more than
	// one command hides something, and --deps is kept as its name for
	// show, where "show the dependencies" is how it reads.
	All bool `aliases:"deps" help:"Show what is hidden by default: satisfied deps, and files that are not plans."`

	Config kong.ConfigFlag `help:"Load configuration from a file." placeholder:"PATH"`

	Repos   reposCmd   `cmd:"" help:"List repositories and their worktrees."`
	Plans   plansCmd   `cmd:"" help:"List plan files found on every ref."`
	Ready   readyCmd   `cmd:"" help:"List plans startable now: deps done, nobody holds."`
	Pick    pickCmd    `cmd:"" help:"Rank startable plans by how much each unblocks."`
	Next    nextCmd    `cmd:"" help:"Report the first phase of a plan not yet done."`
	Show    showCmd    `cmd:"" help:"Show a plan and everything that blocks it."`
	Find    findCmd    `cmd:"" help:"Search plan titles and summaries across every ref."`
	Orphans orphansCmd `cmd:"" help:"Report claims and checkouts that no longer add up."`
	Stale   staleCmd   `cmd:"" help:"Report worktrees whose branch has not moved."`
	Who     whoCmd     `cmd:"" help:"Report which lane has a live agent on it."`
	Init    initCmd    `cmd:"" help:"Write a .frit.yml with frit's defaults."`
	Version versionCmd `cmd:"" help:"Print the build version."`
}

// repoLanes joins one repository's claims to its checkouts, reading
// that repository's own hold patterns.
func repoLanes(
	repo discover.Repo, rt *runtime,
) ([]lanes.Lane, error) {
	cfg, err := repocfg.Load(repo.Path)
	if err != nil {
		return nil, err
	}
	holds, err := cfg.Compiled()
	if err != nil {
		return nil, err
	}

	refs, err := gitobj.Refs(repo.Path, rt.git)
	if err != nil {
		return nil, err
	}
	merged, err := gitobj.MergedRefs(repo.Path,
		gitobj.DefaultRef(repo.Path, rt.git), rt.git)
	if err != nil {
		return nil, err
	}

	return lanes.Build(repo.Worktrees, refs, merged, holds), nil
}

type orphansCmd struct{}

// Run reports what is claimed but unstaffed, prepared but unstarted,
// or already gone.
func (o *orphansCmd) Run(c *cli, rt *runtime) error {
	repos, err := discover.Repos(c.Root, rt.git)
	if err != nil {
		return err
	}

	doc := report.NewOrphans(c.Root)
	for _, repo := range repos {
		built, err := repoLanes(repo, rt)
		if err != nil {
			// One unreadable repository must not blind the rest.
			doc.AddProblem(repo.Name, err)
			continue
		}
		doc.AddRepo(repo.Name, lanes.Find(built, repo.Worktrees))
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printOrphans(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printOrphans writes a block per repository with something wrong.
// The three kinds stay labelled rather than merged into a count,
// because each calls for a different response. A repository in good
// order is left out entirely; --json lists it with empty sets.
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
		for _, wt := range repo.Empty {
			_, _ = fmt.Fprintf(tw, "  never started\t%s\t%s\n",
				wt.Name, wt.Branch)
		}
		for _, wt := range repo.Prunable {
			_, _ = fmt.Fprintf(tw, "  prunable\t%s\t%s\n",
				wt.Name, wt.PruneReason)
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
	live, presence := livePresence(rt)

	doc := report.NewStale(c.Root, s.Days, presence)
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

// livePresence reads the fleet's live agent roots from herdr. A failing
// or missing socket yields an empty set and false, which every reader
// treats as "presence unknown" rather than "no agents".
func livePresence(rt *runtime) (map[string]bool, bool) {
	panes, err := herdr.List(rt.herdr)
	if err != nil {
		return nil, false
	}

	return herdr.LiveRoots(panes, rt.git), true
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

	panes, err := herdr.List(rt.herdr)
	if err != nil {
		doc.AddProblem("herdr", err)
	} else {
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
// then plan, then pane.
func whoLanes(panes []herdr.Pane, git gitwt.Runner) []herdr.Lane {
	staffed := make([]herdr.Pane, 0, len(panes))
	for _, p := range panes {
		if p.HasAgent() {
			staffed = append(staffed, p)
		}
	}

	lanes := herdr.Join(staffed, git, holdsForRoot)
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
	Dir   string `arg:"" optional:"" default:"." type:"path" help:"Repository to write .frit.yml into."`
	Force bool   `short:"f" help:"Overwrite an existing .frit.yml."`
}

// Run writes a per-repository config carrying frit's defaults.
func (i *initCmd) Run(c *cli, rt *runtime) error {
	path, err := repocfg.Init(i.Dir, i.Force)
	if err != nil {
		return err
	}

	if c.JSON {
		return report.WriteJSON(rt.stdout, report.Init(path))
	}
	_, _ = fmt.Fprintf(rt.stdout, "wrote %s\n", path)

	return nil
}

// runtime carries what commands need to do their work: where to
// write, how to reach git, and how to reach herdr. All are injected so
// tests never touch the real streams and can fake either subprocess.
type runtime struct {
	stdout  io.Writer
	stderr  io.Writer
	git     gitwt.Runner
	gitPipe gitwt.PipeRunner
	herdr   herdr.Runner
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
	return fleet.Gather(c.Root, hostname(), rt.git, rt.gitPipe)
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
func resolveSelector(
	rt *runtime, selector string, plans []discovery.Plan,
) (discovery.Plan, error) {
	if selector != "" {
		return discovery.Resolve(selector, plans)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return discovery.Plan{}, err
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

type readyCmd struct{}

// Run lists every plan startable now: not begun, held by nobody, and
// with every dependency done, across all repositories and refs.
func (r *readyCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	doc := report.NewReady(c.Root, hostname())
	carryProblems(doc, res.Problems, c.All)
	doc.SetPlans(discovery.Ready(res.Plans))

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printReady(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

type pickCmd struct {
	N int `short:"n" default:"5" help:"How many candidates to list; 0 for all."`
}

// Run lists the startable plans ranked by how much each unblocks,
// trimmed to the number asked for.
func (pc *pickCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	doc := report.NewPick(c.Root, hostname())
	carryProblems(doc, res.Problems, c.All)
	doc.SetPlans(discovery.Pick(res.Plans, pc.N))

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printReady(rt.stdout, readyView(doc))
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// readyView lets pick reuse the ready table: both print a ranked list
// of startable plans, so the rendering is shared rather than copied.
func readyView(doc *report.PickDoc) *report.ReadyDoc {
	return &report.ReadyDoc{Plans: doc.Plans}
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
	plan, err := resolveSelector(rt, n.Selector, res.Plans)
	if err != nil {
		return err
	}

	doc := report.NewNext(c.Root, plan)
	carryProblems(doc, res.Problems, c.All)

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
	plan, err := resolveSelector(rt, s.Selector, res.Plans)
	if err != nil {
		return err
	}

	doc := report.NewShow(c.Root, discovery.Dependencies(plan, res.Plans))
	carryProblems(doc, res.Problems, c.All)

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printShow(rt.stdout, doc, c.All)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

type findCmd struct {
	Query string `arg:"" help:"Text to match in plan titles and summaries."`
}

// Run searches plan titles and summaries across every repository and
// ref for a query, for when the topic is remembered but not the id.
func (f *findCmd) Run(c *cli, rt *runtime) error {
	res, err := gatherFleet(c, rt)
	if err != nil {
		return err
	}

	doc := report.NewFind(c.Root, hostname(), f.Query)
	carryProblems(doc, res.Problems, c.All)
	doc.SetPlans(discovery.Find(f.Query, res.Plans))

	if c.JSON {
		return report.WriteJSON(rt.stdout, doc)
	}
	printFind(rt.stdout, doc)
	printProblems(rt.stderr, doc.Problems)

	return nil
}

// printFind writes one line per match, carrying the status because find
// answers with plans in any state, not only startable ones. A search
// that matched nothing says so with the query, so an empty result is
// never mistaken for a broken command.
func printFind(out io.Writer, doc *report.FindDoc) {
	if len(doc.Plans) == 0 {
		_, _ = fmt.Fprintf(out, "no plan matches %q\n", doc.Query)
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, p := range doc.Plans {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			p.Repo, p.ID, statusLabel(p.Status), p.Title)
	}
	_ = tw.Flush()
}

// printNext writes the plan and the phase to pick up, the seed a
// dispatch verb will one day type for you: a plan id, a phase number,
// and the tier the plan asks for. A plan with no open phase says why —
// done, or carrying no phase ledger at all.
func printNext(out io.Writer, doc *report.NextDoc) {
	p := doc.Plan
	if !doc.HasPhase {
		if p.Status == planmeta.StatusDone {
			_, _ = fmt.Fprintf(out, "plan %d is done\n", p.ID)
			return
		}
		_, _ = fmt.Fprintf(out, "%s %d  %s  (no phase ledger)\n",
			p.Repo, p.ID, p.Title)
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(tw, "%s\t%d\tphase %s\t%s\t%s\n",
		p.Repo, p.ID, doc.Phase.N, doc.Phase.Title, modelLabel(p.Model))
	_ = tw.Flush()
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
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	printDep(tw, doc.Tree, 0, all)
	if visibleDeps(doc.Tree, all) == 0 {
		_, _ = fmt.Fprintln(tw, "  "+emptyDepsNote(all))
	}
	_ = tw.Flush()
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
func printReady(out io.Writer, doc *report.ReadyDoc) {
	if len(doc.Plans) == 0 {
		_, _ = fmt.Fprintln(out, "nothing startable")
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, p := range doc.Plans {
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			p.Repo, p.ID, modelLabel(p.Model), p.Title)
	}
	_ = tw.Flush()
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
