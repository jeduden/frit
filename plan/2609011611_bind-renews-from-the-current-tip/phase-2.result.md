---
n: 2
title: bindSession stamps the session instead of self-fencing
status: "✅"
result: true
summary: bindSession renews through claim.RenewToBind, so a dispatch whose agent has already committed on the shared work ref stamps the session instead of reporting this machine as its own fence; a foreign takeover in the same window still warns and names the mover, and the lane stays up either way.
---
## Handoff

The bind is wired to Phase 1's reconcile: `bindSession` in
[cmd/frit/start.go](../../cmd/frit/start.go) calls `claim.RenewToBind`
where it called `claim.Renew`, and passes the same mint tip it always
did. Reconciling that tip is the atom's job now, so the call site
changed by one identifier and a comment saying why the baseline is
expected to be stale here and nowhere else.

**The reported failure is closed at the level a user meets it.**
`TestStartBindsTheSessionOntoARefTheLaneAlreadyAdvanced` in
[cmd/frit/start_test.go](../../cmd/frit/start_test.go) drives the fake
herdr to push an ordinary work commit — no lease marker of its own —
onto the work ref at the moment start reads the agent back, which is
the last call before the bind and therefore the real window. Before the
wiring it reproduced the reported text verbatim: `bind session sess-1
to plan/7: fenced: the work ref for plan 7 was moved by je-framework;
run yield`. After it, the ref's tip is a beat carrying `session:
sess-1`, and that beat is a child of the lane's own commit rather than
of the mint tip — the proof that the renewal read the ref rather than
remembering it.

**The guard has its own pin at this level too.**
`TestStartWarnsWhenAForeignMoveFencesTheBind` seizes the ref in the
same window with box-b's takeover marker. The warning still names
box-b, nothing is stamped over the mover, the exit code is zero and the
lane is still reported running — the warning-not-abort contract is
unchanged, and it now fires only for the case it was written for.

**Nothing else moved.** `claim.Renew` keeps every other caller: the
beat-for-holder and resume paths renew from a tip that is genuinely
theirs, where a lost CAS is a real fence. `frit pick --go` needed no
edit of its own — `pickCmd.start` in
[cmd/frit/main.go](../../cmd/frit/main.go) reaches the same
`buildStart` → `startExecute` → `bindSession` path `frit start --go`
does, so the one line fixes both by construction.

**A gap in the plan folder, closed in passing.** Phase 2 had no
`phase-2.md` when the plan was created — the Execution row and Task 2
named it, but no spec file was written, so `frit phase` reported no
open phase after Phase 1 closed. The file was reconstructed at
execution time from those two plus Phase 1's handoff, and says so
inline. Worth watching for on the next plan created the same way: the
phase catalog is generated from the files on disk, so a missing phase
file is invisible in the rendered plan and only surfaces when the lane
asks for its next phase.

**Gate.** `go test ./...` and `go tool -modfile=tools/go.mod
golangci-lint run` are green; `mdsmith check .` is clean.
