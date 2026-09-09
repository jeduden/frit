---
n: 1
title: Resume after normal dispatch and pushed work
status: "✅"
result: false
---
Prove issue #186 with a real local origin and worktree, using the
existing herdr fake. Start the plan through the CLI so acquisition,
binding and persistence produce the token. Push two ordinary work
commits, keep the tree clean, and make herdr report the old session
gone with no other agent on the lane. Run from inside the worktree.

RED: add a command regression beside the existing resume tests. Assert
the token is a valid beat and ancestor of origin before exercising
`start <id> --go --json`. Cover plain subjects and `plan <id>: ...`
subjects. Record which lane-resolution, marker or token-proof check
causes the refusal. If the minimal issue shape already passes, preserve
it as a control and isolate the missing condition before changing code.

GREEN: repair the failing path using the shared ownership proof and
resume transition. Do not replace proof with ancestry alone or holder
text. If work subjects mask markers, find the governing valid marker
without crossing a real takeover or release. Add a failing unit test
for each changed branch, then pass it before widening the fix.

BDD coverage: implement the reserved `@S94` in
[cross-layer.feature](../../features/cross-layer.feature), then remove
`@pending` and the matrix row's pending label. Reuse the section's
existing steps and world; add bindings in its own section test file.
S76, S77 and S86 remain regression guards; this phase changes lease
resume, so a command-only scenario would not cover its contract.

Gate: the resume beat parents the current work tip, retains the epoch,
and persists the new token. The same checkout and file contents remain,
with exactly one fresh agent and one dispatched prompt. Assert JSON
resume/pane fields and a dry-run with no ref or agent mutation. Exercise
the built binary in the isolated fixture, with fake herdr, as well as
the command tests. Preserve live-agent, unknown-presence, tokenless,
foreign-epoch and unpushed-work guards. Run S94 without skipping, the
full Go tests, lint and markdown checks; cite S94 in the resume docs.
