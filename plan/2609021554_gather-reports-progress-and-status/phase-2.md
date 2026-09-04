---
n: 2
title: the terminal progress is transient, with a closing status line
status: "✅"
result: false
---
Make the terminal progress rendering transient: a single line the walk
redraws in place as it advances, cleared and replaced by a closing
status line when it ends. The reporter seam Phase 1 landed means this
changes only [cmd/frit/progress.go](../../cmd/frit/progress.go).

**Assumes.** Phase 1's `progress` struct writes one plain line per
repository — a `Start` line, a `Repo` line each, a `Done` line —
through `newProgress(rt.stderr)`, and `progressFor` already hands every
non-terminal, non-interactive or `--json` run a `DiscardReporter`. So
only a real terminal ever sees these writes, and this phase changes what
that terminal sees, nothing else.

**Value.** One line per repository scrolls a dozen lines up the terminal
for a walk that is over in a second, burying the command's own output
that follows on stdout. A transient line — redrawn in place — shows
which repository the walk is on while it runs, then collapses to a
single closing status line, so a fast walk leaves one line behind and a
slow one still shows progress live.

**RED.** Drive the `progress` struct directly with a buffer, since it
writes to any `io.Writer`:

- assert `Repo` redraws in place rather than appending a new line: each
  `Repo` write returns to the start of the line (a carriage return) and
  clears it before writing the new "[i/total] name", so the buffer does
  not accumulate one newline per repository.
- assert `Done` finishes the transient line — clears it — and writes the
  closing status line terminated by a newline, so the command's own
  output starts clean on the next line.
- keep a `Start` assertion consistent with the transient shape (it may
  open the line or be folded into the first `Repo` redraw; the handoff
  records which).

Each fails against Phase 1's newline-per-repository writer. Commit the
red.

**GREEN.** Render the transient line in
[cmd/frit/progress.go](../../cmd/frit/progress.go): write the current
repository with a leading carriage return and a clear-to-end-of-line, no
trailing newline, so the next `Repo` overwrites it; on `Done`, clear the
line and print the closing status with a newline. Keep the writes to
`p.out` so the buffer test drives them. `DiscardReporter` and
`progressFor`'s terminal gate are unchanged — a pipe, a file and a test
still see nothing.

**Guard the edges.** stdout is untouched: progress stays on stderr. A
`--json` or piped run still gets `DiscardReporter`, so the JSON contract
and the golden outputs do not move. The closing status line keeps the
counts Phase 1's `Done` reported. Every changed function keeps its unit
test.

**Gate.** The new `progress` unit tests pass — a redrawn line, a cleared
close, a final status line; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are clean; the built `frit`
over a multi-repository root shows a single redrawing line on a terminal
with stdout still carrying only the command's output.

Write the handoff to `phase-2.result.md`. Record the exact control
sequence used and how `Start` folded into the transient shape, so Phase
3 leaves the stderr rendering alone and works only on the report model.
