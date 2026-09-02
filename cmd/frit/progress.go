package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jeduden/frit/internal/fleet"
	"golang.org/x/term"
)

// progressFor picks the reporter a gather emits into. Progress is
// human feedback for an interactive run, so it renders only when
// stderr is a real terminal and the run is not asking for JSON — under
// --json stderr stays empty for the consumer, and a pipe, a file or a
// test buffer gets nothing to keep the JSON contract and the golden
// outputs clean. Every other run walks in silence with a discarding
// reporter, but the gather still emits into it, so the choice is only
// whether to show the events, never whether to produce them.
func progressFor(c *cli, rt *runtime) fleet.Reporter {
	if c.JSON || !isTerminalWriter(rt.stderr) {
		return fleet.DiscardReporter{}
	}

	return newProgress(rt.stderr)
}

// isTerminalWriter reports whether w is an interactive terminal — a nil
// writer, a pipe, a file, or a bytes.Buffer under test all answer
// false, so only a real terminal ever sees progress.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)

	return ok && term.IsTerminal(int(f.Fd()))
}

// progress renders a fleet gather's progress to a writer — stderr in
// production, so a slow walk over many repositories reports which one
// it is on rather than hanging in silence, while stdout stays reserved
// for the command's own table or JSON.
//
// This first cut prints one plain line per repository. Making the line
// transient on a terminal and adding a closing status line is a later
// phase; the reporter seam is here so that refinement changes only this
// file.
type progress struct {
	out io.Writer
}

// newProgress builds the reporter the gather emits into, writing to
// out. It is the one production wiring of fleet.Reporter, so every verb
// that gathers the fleet reports progress by construction.
func newProgress(out io.Writer) *progress {
	return &progress{out: out}
}

// Start names how many repositories the walk will cover.
func (p *progress) Start(repos int) {
	_, _ = fmt.Fprintf(p.out, "gathering %d repositories\n", repos)
}

// Repo names the repository the walk is reading now.
func (p *progress) Repo(name string, index, total int) {
	_, _ = fmt.Fprintf(p.out, "  [%d/%d] %s\n", index, total, name)
}

// Done reports what the walk covered once it closes.
func (p *progress) Done(s fleet.Summary) {
	_, _ = fmt.Fprintf(p.out,
		"gathered %d/%d repositories, %d problem(s), in %s\n",
		s.Read, s.Discovered, s.Problems, s.Elapsed.Round(1e6))
}
