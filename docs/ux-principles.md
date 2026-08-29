# UX principles

How frit behaves toward the people and programs that drive it, and why
it behaves that way. The rules an agent must obey live in
[CLAUDE.md](../CLAUDE.md); this page is the reasoning behind the shapes
those rules point at.

## Naming a plan

The discovery verbs — `ready`, `pick`, `next`, `show`, `find` — each
take a plan three ways: an exact id, a slug fragment matched against
titles and branches, or nothing at all. The empty form is inferred
from the worktree the command runs in, so a verb run inside a lane
needs no argument.

## The JSON contract

`--json` is global, so every command answers it. Both renderings are
built from one model in [internal/report](../internal/report), never
from two printers kept in step by hand: a command gathers what it
found into a document and then prints it as a table or encodes it as
JSON.

Three rules make that JSON something a consumer can be written
against, and `internal/report/testdata` pins them:

- every key is always present, so a field is indexed without first
  testing for it
- a list is `[]` and never null
- a repository frit could not read is carried in the document. Under
  `--json` nothing goes to stderr, because stdout is then the whole
  report

Two divergences from the table are deliberate. `--detail` decides how
much of the plan index a person is shown, while the document always
carries all of it; and the table drops a repository with nothing to
report, while the document keeps it with empty sets.

## Two kinds of setting

Per-repository settings and frit's own settings are deliberately
different. Per-repository settings live in a `.frit.yml` committed
inside each repository frit indexes — `plan-dir` and the `holds`
patterns. They describe that project's conventions, so they travel
with it rather than living on one machine. A repository with no file
gets the defaults, so the canonical convention needs no config at all.

Hold patterns are a **list**, because conventions decorate the plan id
freely and one pattern would see only a fraction of a repository's
lanes. A ref matching no pattern is simply not a hold; that is the
honest answer, not a gap.

frit's own settings resolve most-specific first, so the environment
can always override a stale committed file. The exact order and the
`envResolver` that pins it are in [CLAUDE.md](../CLAUDE.md).

## Skills are read under pressure

A skill is read under time pressure and loaded into a working session,
so it must stay skimmable and token-cheap. Every byte an agent loads
to work a plan is paid for once per session, which is why the `skill`
lint kind caps length, tokens and filler. The enforcement lives in
[development.md](development.md).
