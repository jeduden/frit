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
	"sort"
	"text/tabwriter"

	"github.com/alecthomas/kong"
	kongyaml "github.com/alecthomas/kong-yaml"

	"github.com/jeduden/frit/internal/config"
	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/gitwt"
	"github.com/jeduden/frit/internal/plans"
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

	Config kong.ConfigFlag `help:"Load configuration from a file." placeholder:"PATH"`

	Repos   reposCmd   `cmd:"" help:"List repositories and their worktrees."`
	Plans   plansCmd   `cmd:"" help:"List plan files found on every ref."`
	Version versionCmd `cmd:"" help:"Print the build version."`
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
	printRepos(rt.stdout, repos)

	return nil
}

type plansCmd struct {
	Dir    string `help:"Directory holding plan files." default:"plan" env:"FRIT_PLAN_DIR"`
	Detail bool   `help:"List every plan file, not just a count." short:"d"`
}

// Run reads plan files off every ref of every repository under the
// root. Nothing is checked out.
func (p *plansCmd) Run(c *cli, rt *runtime) error {
	repos, err := discover.Repos(c.Root, rt.git)
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(rt.stdout, 0, 0, 2, ' ', 0)
	for _, repo := range repos {
		files, err := plans.Collect(repo.Path, p.Dir,
			rt.git, rt.gitPipe)
		if err != nil {
			// One unreadable repository must not blind the rest.
			_, _ = fmt.Fprintf(rt.stderr, "frit: %s: %v\n",
				repo.Name, err)
			continue
		}
		printPlans(tw, repo.Name, files, p.Detail)
	}
	_ = tw.Flush()

	return nil
}

// printPlans writes one repository's plan summary, and optionally
// every distinct plan file with how many refs carry it.
func printPlans(
	tw io.Writer, name string, files []plans.File, detail bool,
) {
	byPath := map[string]int{}
	refs := map[string]bool{}
	for _, f := range files {
		byPath[f.Path]++
		refs[f.Ref] = true
	}

	_, _ = fmt.Fprintf(tw, "%s\t%s\ton %s\n", name,
		plural(len(byPath), "plan"), plural(len(refs), "ref"))
	if !detail {
		return
	}

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		_, _ = fmt.Fprintf(tw, "  %s\t%s\t\n",
			path, plural(byPath[path], "ref"))
	}
}

type versionCmd struct{}

// Run prints the build version.
func (v *versionCmd) Run(rt *runtime) error {
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
func printRepos(out io.Writer, repos []discover.Repo) {
	if len(repos) == 0 {
		_, _ = fmt.Fprintln(out, "no git repositories found")
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, repo := range repos {
		_, _ = fmt.Fprintf(tw, "%s\t\t%s\n",
			repo.Name, plural(len(repo.Worktrees), "worktree"))
		for _, wt := range repo.Worktrees {
			_, _ = fmt.Fprintf(tw, "  %s\t%s\t%s\n",
				wt.Name(), ref(wt), note(wt))
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
func ref(wt gitwt.Worktree) string {
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
func note(wt gitwt.Worktree) string {
	switch {
	case wt.Bare:
		return ""
	case !wt.HasCommit():
		return "no commit"
	case wt.Prunable:
		return "prunable"
	case wt.Locked:
		return "locked"
	default:
		return ""
	}
}
