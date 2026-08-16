---
id: 2608161811
title: Multi-host — read the sockets, not the socket
status: "🔲"
summary: >-
  Grow presence from one host to many by SSH fan-out, because git
  already carries the durable half. The plan key already holds the
  host dimension, so this swaps one socket read for many and renders an
  unreachable host stale rather than dropping it.
model: sonnet
depends-on: [2608161808]
---
# Multi-host

## Goal

Answer the fleet's questions across more than one machine. The durable
half — which plans exist, who holds what, what merged — already
crosses hosts through git. Only the ephemeral half, which agent is
live in which pane, needs fan-out. This plan adds it.

## Context

The repositories on the reference machine point at two forges, and on
one repository the self-hosted remote carries 192 branches against
GitHub's 121. So the private forge is where coordination actually
happens, and git is already the bus.

Host state splits in two, and only one half needs anything new:

- **Durable state** is already in git and readable from any host with
  one `ls-remote`. It needs no work when the fleet grows.
- **Ephemeral state** lives only in each host's herdr socket, and is
  the one piece needing fan-out.

The design was written down so growth is additive rather than a
migration. The host dimension is already in the plan key, and the
presence source is already one function. This plan swaps that function
from "read socket" to "read sockets", and nothing upstream changes.

## Phase 1: fan out to many sockets

Read presence from a configured list of hosts, not just the local
socket. For a remote host the call is `ssh <host> herdr agent list`,
run concurrently across hosts, because a serial walk pays every host's
latency in turn.

herdr already supports `--remote <ssh-target>`. So attaching to a pane
spotted elsewhere stays a one-liner. Rung 1 of the dispatch ladder
needs no change.

## Phase 2: an unreachable host is stale, not gone

A dead SSH target must never block the board. An unreachable host
renders its last-known presence with an explicit staleness age, rather
than dropping its lanes or failing the whole command. A host that is
merely slow is bounded by a timeout and treated the same.

The cache has a short TTL so a live fleet is not re-probed on every
invocation. It is keyed on the host, so one slow machine does not
stale the others.

## Execution

Tier is per phase, set by the most demanding ingredient.

| Phase                  | Design | Implement | Gate that catches a wrong answer                              |
| ---------------------- | ------ | --------- | ------------------------------------------------------------- |
| 1 fan out              | sonnet | sonnet    | test that hosts are probed concurrently, not in series        |
| 2 unreachable is stale | opus   | sonnet    | test that a dead host renders stale and never fails the board |

## Non-goals

- No daemon and no central service. git is the durable bus and SSH is
  the ephemeral one; neither needs a resident process.
- No change to the durable half. Holds and plans already cross hosts
  through `ls-remote`; this plan touches only presence.
- No dispatch beyond attach. Starting a lane on a remote host is a
  later escalation of the dispatch ladder, not this plan.

## Tasks

1. Read presence from a configured list of hosts
2. Run the per-host `herdr agent list` calls concurrently
3. Render an unreachable host as stale with an age, never dropped
4. Cache per host with a short TTL keyed on the host
5. Extend the presence `--json` form with the host and its staleness

## Acceptance Criteria

- [ ] Presence is read from every configured host, not just the local one
- [ ] Remote hosts are probed concurrently
- [ ] An unreachable host renders its last-known state with a staleness age
- [ ] A dead or slow host never blocks or fails the board
- [ ] Presence is cached per host with a short TTL
- [ ] The presence `--json` form carries the host and its staleness
- [ ] All tests pass: `go test ./...`
- [ ] `go tool -modfile=tools/go.mod golangci-lint run` is clean
