package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jeduden/frit/internal/fleet"
)

// TestProgressRepoRedrawsInPlace asserts each Repo write returns to the
// start of the line and clears it before writing the next
// "[i/total] name", rather than appending a new line, so a fast walk
// over many repositories does not scroll the terminal.
func TestProgressRepoRedrawsInPlace(t *testing.T) {
	var buf bytes.Buffer
	p := newProgress(&buf)

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
	p := newProgress(&buf)

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
	if strings.Contains(out, "atlas") {
		t.Fatalf("Done must clear the transient line, not leave it behind, got %q", out)
	}
	if !strings.Contains(out, "gathered 1/1 repositories") {
		t.Fatalf("Done must keep the closing status counts, got %q", out)
	}
}

// TestProgressStartFoldsIntoTransientShape asserts Start does not by
// itself scroll the terminal with a standalone newline-terminated line
// ahead of the redrawn Repo lines.
func TestProgressStartFoldsIntoTransientShape(t *testing.T) {
	var buf bytes.Buffer
	p := newProgress(&buf)

	p.Start(3)
	p.Repo("atlas", 1, 3)

	out := buf.String()
	if strings.Count(out, "\n") != 0 {
		t.Fatalf("Start must not leave a standalone newline-terminated line ahead of the redrawn progress, got %q", out)
	}
}
