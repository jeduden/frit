---
id: 2608230705
title: A fresh claim refuses to clobber local work on its own lease branch
status: "🔳"
summary: >-
  Acquire's fresh path mints a claim marker on opts.Base and, on a
  winning CAS, force-moves the local plan/<id> ref onto it with a bare
  update-ref — silently discarding any commit a local branch of that
  same name already carried but never pushed. Guard the fresh path: if
  the local ref exists and is not an ancestor of the base it is about
  to be rebased onto, refuse instead of clobbering.
model: sonnet
depends-on: []
phases:
  - n: 1
    title: a fresh acquire refuses an unpushed local lease branch
    status: "🔲"
---
# A fresh claim refuses to clobber local work on its own lease branch

## Goal

`frit claim` on a plan whose lease ref is absent on the remote must
refuse, not silently discard history. Refuse when the *local* branch
`plan/<id>` already holds commits not reachable from the base it is
about to acquire on.

## Context

Found live: plan 2608222201 was authored directly on its own lease
branch (`plan/2608222201`, commit `7ac3700`, the plan file itself)
before that commit was ever pushed to `origin`. `frit claim 2608222201`
then ran. `Acquire` in
[internal/claim/lease.go](../internal/claim/lease.go) reads only the
*remote* ref (`remoteHolder`, line 145). Finding it absent, it took the
fresh path (`tip == ""`) straight to `pushClaimMarker(..., parent="",
...)`. There, `par := parent; if par == "" { par = baseSHA }` (lines
639-642) mints the marker as a child of `opts.Base` — `origin/main`'s
tip. It never reads the local branch at all. The remote CAS
(`casPush`, `--force-with-lease=<ref>:<expected>` with `expected=""`)
correctly succeeded, because the *remote* ref really was absent. That
part is the design working as documented: "the remote is the arbiter;
frit never decides holdership from a local view" (lease.go:1-8). But a
win then calls `syncLocalRef` (line 730-732):

```go
func syncLocalRef(repoDir, ref, tip string, run gitwt.Runner) {
    _, _ = run(repoDir, "update-ref", ref, tip)
}
```

an unconditional `update-ref` with no old-value check. It moved the
local `refs/heads/plan/2608222201` from `7ac3700` to the new marker,
orphaning `7ac3700` — recoverable only via reflog, and not at all once
that expires or in a fresh clone.

Reuse searched:

- **`casPush`'s CAS-and-classify shape** — the remote transition
  already refuses on a lost race rather than papering over it
  (`heldError`). The fix mirrors that stance for the local ref instead
  of inventing a new one.
- **`checkedOut`** (lease.go:305-318) — an existing precedent for a
  guard that runs before a ref mutation that could strand something;
  the new guard is sibling code to it, not a rewrite of it.
- No existing local-divergence check: nothing in `internal/claim`
  inspects the local ref before a fresh acquisition — `Acquire`'s
  fresh branch is genuinely blind to local state, confirmed by reading
  every call site of `syncLocalRef` and `pushClaimMarker`.

The guard only has to cover the fresh path (`parent == ""` in
`pushClaimMarker`). The resume path (`parent` is a real
release-marker tip) and `Renew`/`Takeover` already CAS from a tip that
`Acquire` or the caller read off the remote first. The local ref they
move from is never a stray unrelated branch. Only a fresh acquisition
can land on top of a same-named local branch it never looked at.

## Tasks

1. A fresh acquire refuses when the local `plan/<id>` branch holds
   commits not reachable from `opts.Base`, naming the branch, the
   local tip and the recovery in the error instead of overwriting it;
   prove `pick --go` inherits the refusal through the shared path.

## Phase 1: A fresh acquire refuses an unpushed local lease branch

**RED.** In
[internal/claim/lease_test.go](../internal/claim/lease_test.go), add
`TestAcquireRefusesToClobberUnpushedLocalBranch`. Build a fixture with
`originAndClone(t)`: plan id 7, so the lease branch is `plan/7`. Then,
before calling `Acquire`, create that exact local branch ahead of
`origin/main`. Never push it:

```go
work := originAndClone(t)
gitCmd(t, work, "checkout", "-q", "-b", "plan/7")
require.NoError(t, os.WriteFile(
    filepath.Join(work, "draft.md"), []byte("x\n"), 0o600))
gitCmd(t, work, "add", "-A")
gitCmd(t, work, "commit", "-q", "-m", "local draft, never pushed")
local := gitCmd(t, work, "rev-parse", "plan/7")
gitCmd(t, work, "checkout", "-q", "main")
```

Call `Acquire(work, leaseOptions("box-a", "/lanes/a"), gitwt.Exec)`.
Assert it returns an error (not a `Lease`), and that
`gitCmd(t, work, "rev-parse", "refs/heads/plan/7")` still equals
`local` — the local branch is untouched, not fast-forwarded past the
draft commit. This fails today: the current code force-moves the
local ref and returns a `Lease` with no error.

Also add `TestAcquireStillWinsWhenLocalBranchIsAncestorOfBase`. Same
setup. Skip creating the local `plan/7` branch entirely — the common
case, no local branch of that name at all. Confirm `Acquire` still
succeeds, exactly as the existing race tests already prove. This is a
regression guard on the fix, restated here so the new refusal and its
counterexample sit side by side.

`pick --go` is the verb whose collision cost the plan file this
incident. It reaches the same fresh path too: `mintOrTakeOver` →
`Acquire` ([cmd/frit/start.go](../cmd/frit/start.go):349-369). One
guard in `pushClaimMarker` covers it for free. Prove that end to end,
not just trust it. In
[cmd/frit/pick_test.go](../cmd/frit/pick_test.go), add
`TestPickGoRefusesADivergingLocalBranch`: reuse `claimableRepo(t, root,
"atlas", 7, "Shader unit")`, then `git checkout -b plan/7` in that repo
and commit an unpushed file, mirroring the fixture above. Run
`run([]string{"pick", "--go", "--root", root}, &out, &errb)`. Assert a
non-zero exit code, that stderr names plan 7's collision, and that
`refs/heads/plan/7` in the repo still points at the local draft
commit — pick surfaces the same refusal `Acquire` returns, not a
swallowed error or a silent clobber.

**GREEN.** In `pushClaimMarker`, before minting on the fresh path
(`parent == ""`), read the local ref's tip (a `run(repoDir,
"rev-parse", "--verify", "--quiet", ref)`, tolerating "ref doesn't
exist" as no divergence to check). If it exists, check `run(repoDir,
"merge-base", "--is-ancestor", localTip, baseSHA)`: a non-zero exit
means the local branch carries commits `baseSHA` does not, so return a
new error (`LocalDivergesError`, mirroring `HeldError`'s shape:
`PlanID`, `Branch`, `LocalTip`) and mint nothing. Its `Error()` names
the branch and the recovery, not a bare refusal — an operator reading
it needs to know what to do next: `"plan %d: local branch %s carries
unpushed work (%s); push it or rename it before claiming"`. An
ancestor (or an absent local ref) proceeds exactly as today.

**Gate.** `go test ./internal/claim -run TestAcquire` green; `go test
./...` green; `go vet ./...` clean; `go tool -modfile=tools/go.mod
golangci-lint run` clean.

## Execution

| Phase | Work                                                       | Tier   |
| ----- | ---------------------------------------------------------- | ------ |
| 1     | Guard the fresh-acquire path against a diverging local ref | sonnet |

## Acceptance Criteria

- [ ] `Acquire` on a plan whose remote lease ref is absent, but whose
      local `plan/<id>` branch already holds commits not reachable
      from `opts.Base`, returns an error and leaves that local branch
      untouched.
- [ ] `Acquire` still wins a genuinely fresh acquisition — no local
      branch of that name, or one that is a clean ancestor of the base
      — exactly as before.
- [ ] `pick --go` on a colliding plan refuses through the same guard,
      naming the branch and the recovery, and leaves the local branch
      untouched.
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
