# Agent Notes

<!-- Included content comes from CLAUDE.md. Edit that
     file first, then run `mdsmith fix .` to propagate. -->

Instructions for AI coding agents (Codex, Copilot,
Claude).

<?include
file: CLAUDE.md
strip-frontmatter: "true"
?>
# CLAUDE.md

## Project

frit — a command and control CLI over plans, worktrees, hosts and
agents, written in Go.

## Steering Is Local, Coordination Is Origin

The main principle. frit steers agents from their local edits, and it
runs as many instances, on many hosts, at once. The one place they all
coordinate is git origin: the claim that holds a plan, the rescue ref
that parks a lane, and the plan files themselves are refs pushed there,
because a ref push is the only atom every host can see and race on
safely. A fact that never reaches origin — a live pane, an in-flight
merge, a PR open on a pushed branch — is local. frit reads it from the
host that has it (herdr's socket) or asks the agent; it never infers it
from the shared refs, which cannot carry it. So a deserted, unlanded
lane and one whose work is open as a PR read alike to a distant frit —
the difference is local, and so is the resolution. See
[docs/architecture.md](docs/architecture.md).

## Docs

- [Plan template; see PLAN.md for status, plans live in plan/](plan/proto.md)
- [Architecture — the boundaries and the claim](docs/architecture.md)
- [UX principles — how frit behaves, and why](docs/ux-principles.md)
- [Development — build, test, release, mechanics](docs/development.md)
- [How frit and mdsmith fit together](docs/mdsmith-and-frit.md)
- [Why frit exists — the research](docs/research/fleet-index/README.md)
- [Where the name comes from](docs/research/naming.md)

Research notes record how a decision was reached, including the wrong
turns, and are dated rather than kept current. When one has been
overturned it says so inline. This file and PLAN.md are the current
record; the research is the reasoning.

## Development Workflow

- Any change follows Red/Green TDD: failing test, then pass, then commit
- Keep commits small and focused on one change
- Run `mdsmith check .` before committing; all markdown must pass
- Never modify `.mdsmith.yml` (linter configuration) without explicit
  user consent
- Run `mdsmith merge-driver install` once per clone; see
  [docs/development.md](docs/development.md) for why

## Plan Maintenance

When implementing work tracked by `plan/`:

- Update the plan file **as part of implementation**, not a follow-up
- Check off tasks and acceptance criteria as they're verified
- Move front-matter `status`: `🔲` → `🔳` on start, `✅` when done
- If implementation deviates, update plan text to match
- Run `mdsmith fix PLAN.md` after editing front matter

## Reporting

Report in the plan's terms, not the source's.

- Speak of acceptance criteria met, blockers cleared, and what the
  change enables — not the functions, types, or symbols touched. When
  a symbol doubles as an ordinary word, use the ordinary word.
- Reach for a source entity only when the high-level frame cannot
  carry the point. Then lead into it: trace from the criterion down to
  the entity first, so the detail lands with its context — never open
  on the symbol.

## Configuration

frit's own settings — `--root` and friends — resolve from four places,
most specific first:

1. the command line (`--root`)
2. the environment (`FRIT_ROOT`)
3. `.frit.yml` beside the work, or `$FRIT_CONFIG`
4. the user config, `$XDG_CONFIG_HOME/frit/config.yml`

kong's own `env:` tag sits *beneath* its config resolver, which would
let a stale file outrank the environment. `envResolver` in
[cmd/frit/main.go](cmd/frit/main.go) restores the expected order and is
pinned by a precedence test — do not remove it.

Per-repository settings live in a committed `.frit.yml` (`plan-dir`,
the `holds` patterns); `frit init` writes it with every default
present and commented. Why the two kinds differ, and why holds are a
list, is in [docs/ux-principles.md](docs/ux-principles.md).

## The JSON Contract

`--json` is global, so every command answers it, and both renderings
are built from one model in [internal/report](internal/report) — never
two printers kept in step by hand. The contract a consumer relies on:
every key present, a list is `[]` and never null, and a repository
frit could not read is carried in the document. The reasoning and the
deliberate table/document divergences are in
[docs/ux-principles.md](docs/ux-principles.md).

A table is for a person's eyes; `--json` is for a decision. A skill's
example command shows `--json` whenever an agent branches on the
result — a status checked, a held lane told apart from an unheld one —
since a table's glyphs can misread in a way a field never does. A verb
whose plain rendering already is the payload an agent reads directly
(`phase`, `show`, a dry-run composition) stays plain: wrapping that
same prose in a JSON string adds a field, not structure.

## Shipping Skills

Every agent-facing verb ships with the thin skill that fronts it — new
or folded — in the same change that adds the verb. A verb an agent can
only reach through hand-run git or a bare `frit --help` scan is the gap
this rule exists to close. The skills bundle, its `--via` substitution
and the lint that keeps each skill token-cheap are in
[docs/development.md](docs/development.md). A skill's example commands
follow the JSON Contract's `--json` rule above.

## Code Style

- Follow standard Go conventions (gofmt, goimports)
- Keep functions small and focused
- Error messages: lowercase, no trailing punctuation
- Prefer returning errors over panicking
- Every function ships with a dedicated unit test

## Defensive Code

Add a defensive branch only when you can drive it red/green. Write the
failing test first. Then add the code that takes the branch.

## Shelling Out To Git

Git is invoked as a subprocess, never through a library. Two rules
keep that safe:

- Always pass `-C <dir>`; never rely on the process working
  directory, because frit walks many repositories in one run.
- Parse porcelain formats only (`--porcelain`, `-z`, explicit
  `--format`). Human-readable git output is not a stable contract.
<?/include?>
