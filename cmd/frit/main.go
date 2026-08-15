// Command frit indexes plans, worktrees and agents across a fleet.
//
// This binary is read-only by construction. It shells out to git to
// read state and prints what it finds; nothing here writes to a
// repository, claims a plan, or talks to an agent.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"

	"github.com/jeduden/frit/internal/config"
	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/gitobj"
	"github.com/jeduden/frit/internal/gitwt"
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

	Config kong.ConfigFlag `help:"Load configuration from a file." placeholder:"PATH"`

	Repos   reposCmd   `cmd:"" help:"List repositories and their worktrees."`
	Plans   plansCmd   `cmd:"" help:"List plan files found on every ref."`
	Orphans orphansCmd `cmd:"" help:"Report claims and checkouts that no longer add up."`
	Stale   staleCmd   `cmd:"" help:"Report worktrees whose branch has not moved."`
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

	doc := report.NewStale(c.Root, s.Days)
	for _, repo := range repos {
		times, err := gitobj.RefTimes(repo.Path, rt.git)
		if err != nil {
			doc.AddProblem(repo.Name, err)
			continue
		}
		doc.AddRepo(repo.Name,
			lanes.Stale(repo.Worktrees, times, now, cutoff))
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
func printStale(out io.Writer, doc *report.StaleDoc) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	found := false

	for _, repo := range doc.Repos {
		if len(repo.Stale) == 0 {
			continue
		}
		found = true
		_, _ = fmt.Fprintf(tw, "%s\t\t\n", repo.Name)
		for _, aged := range repo.Stale {
			_, _ = fmt.Fprintf(tw, "  %dd\t%s\t%s\n",
				aged.AgeDays, aged.Worktree.Name, aged.Worktree.Branch)
		}
	}
	_ = tw.Flush()

	if !found {
		_, _ = fmt.Fprintf(out,
			"no worktree idle longer than %d days\n", doc.Days)
	}
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
// write, and how to reach git. Both are injected so tests never
// touch the real streams and can fake git.
type runtime struct {
	stdout  io.Writer
	stderr  io.Writer
	git     gitwt.Runner
	gitPipe gitwt.PipeRunner
}

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

	host, err := os.Hostname()
	if err != nil {
		host = "localhost"
	}

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
