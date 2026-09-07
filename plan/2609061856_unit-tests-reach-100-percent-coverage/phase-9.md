---
n: 9
title: cmd/frit/yield.go reaches 100% of its reachable lines, and check-coverage.sh gains exclusions
status: "✅"
result: false
---
Drive [cmd/frit/yield.go](../../cmd/frit/yield.go) to 100% line
coverage, minus one line that sits on a real boundary. It is at 90.0%
today. Ten of its eleven gaps are plainly reachable; the eleventh is a
raw-git fault path that cannot be provoked without corrupting a live
repository's object store mid-run. This is the first `cmd/frit` file to
need the exclusion list the plan's own context anticipated, so this
phase also teaches
[scripts/check-coverage.sh](../../scripts/check-coverage.sh) to accept
one, verified in isolation before the real entry is added.

**BDD coverage.** None applies. This phase adds unit tests and extends
the coverage gate's own logic; no lease-protocol behavior changes. No
`@S<n>` scenario is needed per
[docs/development.md](../../docs/development.md)'s matrix.

**Assumes.** `yield.go:46` unconditionally overwrites `rt.git` right
after `gatherFleet`, with the real `gitwt.WithDeadline(gitwt.ExecContext,
...)` — the same shape `release.go`'s `Run` uses. So nothing past that
point in a full-CLI test can inject a fake git runner. `localRef`'s own
error return (its line 179) is still reachable directly, by calling the
unexported function with a stub runner. The outer call site in `Run`
(65-67) is not: it only surfaces a *real* git fault, an object gone
missing after the ref was already confirmed to exist. No
`--git-timeout` value can exhaust the shared per-run deadline before
this call without also breaking `gatherFleet` itself, since both draw
from the same clock.

**Value.** Third of five `cmd/frit` files. It also builds the exclusion
mechanism the plan's context named from the start. `terminalWidth`,
`progressFor` and `run`'s panic guard all need that mechanism in
[phase 12](phase-12.md), so building and proving it here, against one
well-understood line, is cheaper than discovering it mid-way through
the largest remaining file.

**RED, the tests.** `go test ./cmd/frit -coverprofile`, then `go tool
cover -func` filtered to `yield.go`:

- `Run` (37-39): `gatherFleet`'s error branch uncovered — no test
  points `--root` at an unreadable path.
- `Run` (49-51): `resolveSelector`'s error branch uncovered — no test
  omits the selector from a non-lane cwd.
- `Run` (59-62): the ambiguous-repo `!ok` branch uncovered — no test
  puts two same-named repos under root.
- `Run` (65-67) / `localRef` (179, the outer call site only —
  `localRef`'s own line closes below): a real git fault after the ref
  is confirmed present. Not closeable without a production seam or a
  fragile git-corruption trick timed against `fleet.Gather`'s internal
  call sequence.
- `localRef` (179, direct): the "real fault" branch, in isolation —
  reachable by calling the function directly with a stub runner.
- `tearDownLane` (203-206): `herdr.CurrentPane`'s error branch
  uncovered.
- `tearDownLane` (218-221): `herdr.WorktreeRemove`'s error branch
  uncovered.
- `renderYield` (227-229): the `c.JSON` branch uncovered — no test
  emits yield's JSON form.

**RED, the gate.** `scripts/check-coverage.sh` requires a package's raw
`go test -cover` output to read exactly `100.0%`; it has no way to
accept a package with one declared, justified gap. Prove this before
changing it: point the script at a throwaway scratch package carrying
one excluded-by-intent line (e.g. a temp package with a function whose
body is `panic("boundary")`, never called) and confirm the script
still fails it even after adding an exclusion entry naming that exact
line — the RED that the GREEN below removes.

**GREEN, the tests.** One test per gap:

- `Run`/gatherFleet: `run([]string{"yield", "7", "--root",
  filepath.Join(root, "does-not-exist")}, ...)`; assert exit code 1.
- `Run`/resolveSelector: no selector, `t.Chdir` into a plain non-repo
  temp dir; assert the "no plan given" error.
- `Run`/ambiguous repo: mirror `TestClaimRefusesAnAmbiguousRepoName`'s
  fixture — two repos named `frontend` under different parents — then
  `run(["yield","7","--root",root])`; assert `"shared by another
  checkout"`.
- `localRef` (direct): `localRef(&runtime{git: func(string,
  ...string) ([]byte, error) { return nil, errors.New("boom") }},
  "/repo", "plan/7")`; assert a plain error passes through.
- `tearDownLane`/CurrentPane: call `tearDownLane(rt, doc)` directly
  with a herdr runner that errors only on `agent`/`current`; assert
  `doc.Warning` contains `"pane current"`.
- `tearDownLane`/WorktreeRemove: same shape, erroring only on
  `worktree`/`remove`, against a real checked-out `plan/7` worktree;
  assert `doc.Warning` contains `"worktree remove"` and
  `!doc.TornDown`.
- `renderYield`/JSON: the existing `emit(t, &doc, "yield", "7",
  "--root", root)` helper against the no-op success case; unmarshal
  into `report.YieldDoc`, assert `Command == "yield"`.

**GREEN, the gate.** Extend `scripts/check-coverage.sh` to accept a
declared exclusion list, e.g. `scripts/coverage-exclude.txt`, one entry
per excluded statement range as `path/to/file.go:startLine-endLine  #
reason`. For a package with entries, the script filters the raw
coverage profile (`go test $pkg -coverprofile=<tmp>`) to drop any
statement block whose file and line range fall inside a declared
entry, then computes the percentage from the filtered profile's own
statement/count totals instead of parsing `go test`'s printed summary
line. A package with no entries behaves exactly as before — the
existing six packages' gate does not change shape. Verify GREEN against
the same throwaway scratch package the RED step used, then remove the
scratch fixture.

Add the real entry once the mechanism passes its own test:

```text
cmd/frit/yield.go:65-67  # localRef's real git-fault call site in Run: the object is confirmed present moments earlier in the same run, so a real failure here needs a corrupted object store mid-run, not an input a unit test can construct
```

Re-run `go test ./cmd/frit -coverprofile` and `go tool cover -func`
filtered to `yield.go` until every line but 65-67 reads 100.0%.

**Guard the edges.** One exclusion, one line, one-line reason, per the
plan's own bar: a pure fault of the git subprocess after this run's own
prior check already passed, not a place hiding untested logic. No
seam. Branch coverage stays out of scope. `./cmd/frit` is not yet added
to `scripts/check-coverage.sh`'s CI call — `start.go` and
`main.go`/`progress.go` still carry gaps, and the whole-package check
only makes sense once every file in it is closed.

**Gate.** `go test ./cmd/frit -coverprofile=cover.out && go tool cover
-func=cover.out | grep yield.go` shows every line but 65-67 at 100.0%;
the new exclusion logic in `scripts/check-coverage.sh` is proven
against a scratch fixture (RED then GREEN) and left in place with the
real `yield.go` entry; `go test ./...` and `go tool
-modfile=tools/go.mod golangci-lint run` are green.
