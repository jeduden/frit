---
n: 2
title: the terminal progress is transient, with a closing status line
status: "✅"
result: true
summary: >-
  The terminal progress line is transient: Repo redraws it in place
  with \r + ANSI erase-to-end-of-line, and Done clears it before
  writing a newline-terminated closing status. DiscardReporter and
  progressFor's terminal gate are unchanged, so a pipe, a file, a
  test buffer or a --json run still see nothing.
---
`cmd/frit/progress.go`'s `progress` struct no longer writes one plain
line per repository. Each write now leads with `clearLine`
(`"\r\x1b[K"` — carriage return, then ANSI erase-to-end-of-line) and
drops its trailing newline, so the next write redraws the same
terminal line rather than scrolling past it:

- `Start` opens the transient line with "gathering N repositories",
  folded into the same redraw shape as `Repo` rather than a
  standalone newline-terminated line ahead of it.
- `Repo` redraws the line in place with `[i/total] name`.
- `Done` clears the line and writes the closing status
  ("gathered R/D repositories, P problem(s), in E"), this time
  terminated by `\n` so the command's own output starts clean on the
  next line.

`DiscardReporter` and `progressFor`'s terminal gate are untouched — a
pipe, a file, a test buffer, and a `--json` run still get no writes at
all, so the JSON contract and golden outputs do not move.

**RED/GREEN.** `cmd/frit/progress_test.go` drives `progress` directly
with a `bytes.Buffer`: a redraw test asserts no newlines accumulate
across two `Repo` calls and the segment after the last `\r` shows only
the current repository; a close test asserts the output ends in `\n`
and the segment after the last `\r` shows the closing status, not the
stale repository name; a `Start` test asserts it does not leave a
standalone newline ahead of the first redraw. All three failed against
Phase 1's newline-per-repository writer, then passed against the
`clearLine`-prefixed rewrite.

**Verified live.** Built `frit ready` over a three-repository fleet
under a real pty (`script`): the raw stream shows
`\r\x1b[K` before each of "gathering 3 repositories", "[1/3] atlas",
"[2/3] borealis", "[3/3] zephyr", and the closing "gathered 3/3
repositories, 0 problem(s), in 72ms", then a bare `\n` before the
command's own "nothing startable". A piped run and a `--json` run both
leave stderr empty — no escape sequences, no progress text.

## Handoff

The transient line and its control sequence
(`"\r\x1b[K"`, no trailing newline until `Done`) are settled inside
`cmd/frit/progress.go` alone; `Start` folds into the same redraw shape
as `Repo` rather than opening a separate scrolled line. Phase 3 leaves
this file alone and works only on the report model: it projects
`Result.Summary` (already populated by Phase 1) into
`internal/report` so `frit <verb> --json` and the table both surface
the gather's status.
