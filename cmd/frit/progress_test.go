package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/fleet"
	"github.com/jeduden/frit/internal/textw"
)

// TestProgressRepoRedrawsInPlace asserts each Repo write returns to the
// start of the line and clears it before writing the next
// "[i/total] name", rather than appending a new line, so a fast walk
// over many repositories does not scroll the terminal.
func TestProgressRepoRedrawsInPlace(t *testing.T) {
	var buf bytes.Buffer
	p := newProgress(&buf, 0)

	p.Repo("atlas", 1, 3)
	p.Repo("borealis", 2, 3)

	out := buf.String()
	if strings.Count(out, "\n") != 0 {
		t.Fatalf("Repo must not emit newlines between redraws, got %q", out)
	}
	if !strings.Contains(out, "\r") {
		t.Fatalf("Repo must return to the start of the line with \\r, got %q", out)
	}

	last := out[strings.LastIndex(out, "\r"):]
	if !strings.Contains(last, "borealis") {
		t.Fatalf("last redraw must show the current repository, got %q", out)
	}
	if strings.Contains(last, "atlas") {
		t.Fatalf("last redraw must clear the prior line's tail, got %q", out)
	}
}

// TestProgressDoneClosesTransientLine asserts Done clears the transient
// line and writes a newline-terminated closing status, so the command's
// own output starts clean on the next line.
func TestProgressDoneClosesTransientLine(t *testing.T) {
	var buf bytes.Buffer
	p := newProgress(&buf, 0)

	p.Repo("atlas", 1, 1)
	p.Done(fleet.Summary{
		Discovered: 1,
		Read:       1,
		Problems:   0,
		Elapsed:    2 * time.Millisecond,
	})

	out := buf.String()
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("Done must end the output with a newline, got %q", out)
	}
	visible := out[strings.LastIndex(out, "\r"):]
	if strings.Contains(visible, "atlas") {
		t.Fatalf("Done must clear the transient line before writing its own, not leave it visible, got %q", out)
	}
	if !strings.Contains(visible, "gathered 1/1 repositories") {
		t.Fatalf("Done must keep the closing status counts, got %q", out)
	}
}

// TestProgressRepoFitsWidthToAvoidWrap asserts a transient line is
// capped to the terminal's column count, so a long repository name
// cannot wrap onto a second row that clearLine's erase-to-end-of-line
// leaves behind as stale text.
func TestProgressRepoFitsWidthToAvoidWrap(t *testing.T) {
	var buf bytes.Buffer
	const width = 20
	p := newProgress(&buf, width)

	p.Repo("a-very-long-repository-name-that-would-wrap", 1, 3)

	fitted := strings.TrimPrefix(buf.String(), clearLine)
	if w := textw.Width(fitted); w > width {
		t.Fatalf("Repo line %q is %d cols, exceeds width %d and would wrap", fitted, w, width)
	}
}

// TestProgressStartFitsWidthToAvoidWrap asserts Start's opening line is
// capped to the terminal width for the same reason as Repo's redraws.
func TestProgressStartFitsWidthToAvoidWrap(t *testing.T) {
	var buf bytes.Buffer
	const width = 10
	p := newProgress(&buf, width)

	p.Start(1000000)

	fitted := strings.TrimPrefix(buf.String(), clearLine)
	if w := textw.Width(fitted); w > width {
		t.Fatalf("Start line %q is %d cols, exceeds width %d and would wrap", fitted, w, width)
	}
}

// TestProgressWidthZeroImposesNoLimit asserts a zero width — a pipe, a
// file, a test buffer — leaves the line untruncated, so only a real
// terminal whose width is known ever caps the transient line.
func TestProgressWidthZeroImposesNoLimit(t *testing.T) {
	var buf bytes.Buffer
	p := newProgress(&buf, 0)

	p.Repo("a-very-long-repository-name-that-would-wrap", 1, 1)

	if !strings.Contains(buf.String(), "a-very-long-repository-name-that-would-wrap") {
		t.Fatalf("width 0 must impose no limit, got %q", buf.String())
	}
}

// TestProgressDoneRendersFromGatherStatusLine asserts the closing line
// is drawn from the one report.Gather.StatusLine the report footer uses,
// so the terminal's stderr close and the stdout footer cannot render the
// same coverage two different ways.
func TestProgressDoneRendersFromGatherStatusLine(t *testing.T) {
	var buf bytes.Buffer
	s := fleet.Summary{
		Discovered: 3, Read: 2, Fetched: 1, Problems: 1,
		Elapsed: 2 * time.Millisecond,
	}
	newProgress(&buf, 0).Done(s)

	want := gatherOf(s).StatusLine()
	visible := strings.TrimPrefix(strings.TrimRight(buf.String(), "\n"), clearLine)
	if visible != want {
		t.Fatalf("Done must render coverage from the one StatusLine model:\n got %q\nwant %q",
			visible, want)
	}
}

// TestProgressStartFoldsIntoTransientShape asserts Start does not by
// itself scroll the terminal with a standalone newline-terminated line
// ahead of the redrawn Repo lines.
func TestProgressStartFoldsIntoTransientShape(t *testing.T) {
	var buf bytes.Buffer
	p := newProgress(&buf, 0)

	p.Start(3)
	p.Repo("atlas", 1, 3)

	out := buf.String()
	if strings.Count(out, "\n") != 0 {
		t.Fatalf("Start must not leave a standalone newline-terminated line ahead of the redrawn progress, got %q", out)
	}
}
