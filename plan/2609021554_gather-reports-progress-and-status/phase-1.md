---
n: 1
title: A required reporter and a returned status on Gather
status: "🔲"
result: false
---
Make progress and status structural to `Gather` in
[internal/fleet/gather.go](../../internal/fleet/gather.go): a required
reporter it emits into as it walks, and a status `Summary` it returns
on every `Result`. Wire the one production call site so a real run
shows progress. Drive it red/green with a recording reporter.

**Assumes.** `Gather` loops over the repositories `discover.Repos`
returns, calling `gatherRepo` per repository, appending plans and
problems to `Result`. The only production caller is `gatherFleetOpts`
in [cmd/frit/main.go](../../cmd/frit/main.go); every other call is a
test in
[internal/fleet/gather_test.go](../../internal/fleet/gather_test.go).
The runtime carries `stderr`. The counts a summary needs are already in
hand on the walk — the repository list, the appended problems — so the
summary tallies them rather than re-deriving anything.

**RED.** Add a recording `Reporter` to the fleet tests — one that
appends each call to a slice — over a small multi-repository fixture
built with the existing helpers.

1. Order and coverage: `Gather` calls `Start` once with the repository
   count, then `Repo` once per repository with a rising index, then
   `Done` once — asserted as an ordered sequence.
2. Summary counts: the `Summary` on the returned `Result`, and the one
   handed to `Done`, agree and count the repositories discovered, the
   repositories read, the problems met, and a non-negative elapsed
   span. A fixture with one unreadable repository shows read short of
   discovered by one and the problem tallied.

**GREEN.** Add `internal/fleet/progress.go`. It holds a `Reporter`
interface (`Start(repos int)`, `Repo(name string, index, total int)`,
`Done(Summary)`), a `Summary` struct (repositories discovered, read,
fetched, problems, elapsed), and a `DiscardReporter` no-op. Give
`Gather` a trailing `rep Reporter` parameter. Emit `Start` before the
loop, `Repo` inside it, and `Done` after. Set `res.Summary` before
returning. Update the ~22 existing test call sites to pass
`DiscardReporter{}`. Then, in
[cmd/frit/main.go](../../cmd/frit/main.go), have `gatherFleetOpts`
build a reporter that writes one line per repository to `rt.stderr`,
and pass it in. The transient, terminal-aware rendering and the final
status line are a later phase.

**Gate.** `go test ./internal/fleet` red first, then green. Then build
`frit` and run a fleet-reading verb against a root of several
repositories: per-repository progress appears on stderr, and stdout
still carries only the command's own table or JSON. Then the full suite
and lint.
