---
id: 2608191812
title: Name who holds a claim when the race is lost
status: "🔲"
summary: >-
  When a claim push loses, frit reports "lost the race to another
  machine" whoever holds the ref — including your own branch that merged
  but kept a stale status, where no machine is racing. The marker records
  the host that took it. Read the holder's marker and name it, so a lost
  race tells the truth about who holds the plan.
model: sonnet
depends-on: [2608171835]
---
# Name who holds a claim when the race is lost

## Goal

When a claim loses the push, read the holder's marker and name it. The
refusal should say who really holds the plan: this host, or another
machine. Today it always blames another machine.

## Context

A claim is minted in [internal/claim](../internal/claim/claim.go). When
the push fails and the hold ref on the remote holds a commit that is not
this run's marker, frit reports the plan lost to another machine. That is
right for a genuine race. It is wrong for a plan whose branch merged but
whose status was never set to ✅: the old branch is still on the remote,
the push is rejected, and the message blames a machine that is not there.
Retrying after a partial success reads the same way — the ref holds your
own earlier marker, under a different commit.

The marker already records the host that took the claim, on its `host:`
line ([internal/claim](../internal/claim/claim.go)). The ref that beat us
is named by `ls-remote`; its commit body carries the holder's host, base
and plan file. Reading that body turns a guess into a fact: a claim held
on this host reads differently from one held elsewhere, and a holder
whose ref has merged can be named as landed work with a stale status.

The read is one extra plumbing step on the already-slow failure path, so
it costs nothing on the winning path. If the holder's marker cannot be
read, the message falls back to today's wording rather than failing.

## Tasks

1. Read the holder's marker body for the ref that won, keyed off the sha
   `ls-remote` already returns, driven by a failing test first
2. Report the holder's host in the lost-race message, distinguishing this
   host from another machine
3. When the holder's ref is merged into the default branch, name it as a
   landed claim with a stale status, and point at fixing the status
4. Fall back to the current wording when the marker cannot be read, so a
   missing or malformed body never fails the command

## Acceptance Criteria

- [ ] A lost race to a claim held on this host names this host
- [ ] A lost race to a claim held elsewhere still names another machine
- [ ] A push rejected by a branch that merged names it as a landed claim
      with a stale status, not a live competitor
- [ ] An unreadable holder marker falls back to today's message
- [ ] The winning path runs no extra git call
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
