package main

import (
	"fmt"
	"io"
	"os"

	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/textw"
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

	return newProgress(rt.stderr, terminalWidth(rt.stderr))
}

// isTerminalWriter reports whether w is an interactive terminal — a nil
// writer, a pipe, a file, or a bytes.Buffer under test all answer
// false, so only a real terminal ever sees progress.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)

	return ok && term.IsTerminal(int(f.Fd()))
}

// clearLine returns to the start of the current line and erases it, so
// the next write redraws in place rather than appending a new line.
const clearLine = "\r\x1b[K"

// progress renders a fleet gather's progress to a writer — stderr in
// production, so a slow walk over many repositories reports which one
// it is on rather than hanging in silence, while stdout stays reserved
// for the command's own table or JSON.
//
// The line is transient: each Repo redraws it in place, and Done clears
// it and writes a single closing status line, so a fast walk leaves one
// line behind and a slow one still shows live progress.
type progress struct {
	out   io.Writer
	width int
}

// newProgress builds the reporter the gather emits into, writing to
// out. It is the one production wiring of fleet.Reporter, so every verb
// that gathers the fleet reports progress by construction. width is the
// terminal's column count, so the transient line can be capped to fit
// on one row; a zero width — a pipe, a file, a test buffer — imposes no
// limit.
func newProgress(out io.Writer, width int) *progress {
	return &progress{out: out, width: width}
}

// transient redraws the current line in place: it returns to the start
// and clears it, then writes s capped to the terminal width — no
// trailing newline, so the next transient call overwrites it. Capping
// to the width keeps s on one row, so clearLine's erase-to-end-of-line,
// which reaches only the cursor's own row, always clears the whole
// line; an unbounded s could wrap and strand its first row.
func (p *progress) transient(s string) {
	if p.width > 0 {
		s = textw.Truncate(s, p.width)
	}
	_, _ = fmt.Fprintf(p.out, "%s%s", clearLine, s)
}

// Start folds into the transient shape: it opens the line without a
// trailing newline, so it is redrawn rather than scrolled by the first
// Repo.
func (p *progress) Start(repos int) {
	p.transient(fmt.Sprintf("gathering %d repositories", repos))
}

// Repo redraws the transient line in place with the repository the walk
// is reading now.
func (p *progress) Repo(name string, index, total int) {
	p.transient(fmt.Sprintf("[%d/%d] %s", index, total, name))
}

// Done clears the transient line and writes the closing status,
// terminated by a newline so the command's own output starts clean. The
// status is rendered from report.Gather.StatusLine, the one renderer a
// verb's report footer also uses, so the terminal's close and the
// report cannot show the same coverage two different ways.
func (p *progress) Done(s fleet.Summary) {
	_, _ = fmt.Fprintf(p.out, "%s%s\n", clearLine, gatherOf(s).StatusLine())
}
