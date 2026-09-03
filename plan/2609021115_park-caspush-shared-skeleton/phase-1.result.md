---
n: 1
title: casPush, park and Scavenge's delete share one classify-on-failure helper
status: "✅"
result: true
summary: >-
  One push-then-confirm helper now carries the skeleton casPush, park
  and Scavenge's delete confirmation each wrote independently: run the
  push, and only on failure ask the remote what holds the ref, with a
  read fault kept apart from a confirmed-absent ref. Each caller keeps
  its own success side effect and its own failure-shape switch inline.
  No typed error, message wording or signature changed, and the suite
  passed unchanged before and after.
---
## Handoff

The boundary was drawn from the three real call sites rather than
guessed from two, which is what waiting for plan 2609021114 bought.
All three turned out narrower than the phase sketched: each pushes to
one ref, takes its lease on that same ref, and re-reads that same ref.
So the helper takes the ref, the expected old value, and the source to
push — not a pre-built lease clause and refspec — and an empty source
is exactly the delete refspec Scavenge needs. That is the one
deviation from the phase's design note, and it is a narrowing the
third call site made safe to make.

The helper returns the push's own error alongside the read's answer,
so nothing about the callers' shapes had to be forced into a common
mould. A successful push costs no read and returns immediately; each
caller then runs its own side effect — the local-ref sync, nothing,
the local-ref cleanup. On a failure each keeps its own switch inline
and its own error types: the `(lost, tip, error)` triple, the
create-only park's three-way answer, the delete's gone-is-a-win test.
The widening the phase left open was not needed; the switches share
the read and nothing more.

The step worth having in one place is the one that must never fold: an
unreadable remote is not an absent ref. It is now written once. Its
own test pins it, alongside three more covering the success path
skipping the read, the delete refspec, and the failure carrying both
answers back.

Every existing test on `casPush`, `park` and `Scavenge` passes
untouched — no test file changed, only the new one was added, which is
the evidence the gate asked for that this was a refactor and not a
behavior change.

`go test ./...`, `go tool -modfile=tools/go.mod golangci-lint run` and
`mdsmith check .` are clean.
