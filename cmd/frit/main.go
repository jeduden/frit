// Command frit indexes plans, worktrees and agents across a fleet.
//
// This binary is read-only by construction. It shells out to git to
// read state and prints what it finds; nothing here writes to a
// repository, claims a plan, or talks to an agent.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/jeduden/frit/internal/discover"
	"github.com/jeduden/frit/internal/gitwt"
)

// version is stamped at build time with -ldflags.
var version = "dev"

const usage = `frit — a register of plans, worktrees, hosts and agents

usage:
  frit repos [--root <dir>]   list repositories and their worktrees
  frit version                print the build version
  frit help                   show this message
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point. It returns the process exit code:
// 0 on success, 1 on a runtime failure, 2 on a usage error.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "repos":
		return cmdRepos(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "frit: unknown command %q\n\n", args[0])
		fmt.Fprint(stderr, usage)
		return 2
	}
}

// cmdRepos walks a root and prints every repository it finds with the
// worktrees attached to it.
func cmdRepos(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("repos", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "directory to walk for repositories")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintf(stderr, "frit: %v\n", err)
		return 1
	}

	repos, err := discover.Repos(abs, gitwt.Exec)
	if err != nil {
		fmt.Fprintf(stderr, "frit: %v\n", err)
		return 1
	}

	printRepos(stdout, repos)

	return 0
}

// printRepos renders the repository listing as an aligned table.
func printRepos(out io.Writer, repos []discover.Repo) {
	if len(repos) == 0 {
		fmt.Fprintln(out, "no git repositories found")
		return
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, repo := range repos {
		fmt.Fprintf(tw, "%s\t\t%s\n",
			repo.Name, plural(len(repo.Worktrees), "worktree"))
		for _, wt := range repo.Worktrees {
			fmt.Fprintf(tw, "  %s\t%s\t%s\n",
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
