---
n: 7
title: who on a headless host still lists the configured hosts' lanes
status: "✅"
result: true
summary: >-
  On a host with no herdr, or with herdr turned off, who still reads
  the configured hosts and lists their live lanes, carrying the
  missing herdr as a problem so the list reads as partial. C25 proves
  it.
---
# Phase 7 result

## Handoff

`who` now reads presence past a herdr that is not installed or is
turned off, as start's gates already did. The configured hosts are
asked, their live lanes listed, and the missing local herdr is carried
as a problem. An installed herdr that does not answer still stops the
read before any host is asked.

**Verified by.** Two unit tests, written failing first: a lane live on
`box` is listed past a missing herdr, and an unread host is carried
beside it. C25 runs through godog against the built frit under a
git-only `$PATH`, with a fake ssh answering for `box`. Before the
change it failed with plan 7's lane not listed.
