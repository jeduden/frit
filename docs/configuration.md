# Configuration

Per-repository settings travel with the project in a committed
`.frit.yml`. `frit init` writes every key with its default and a
comment. A repository with no file gets the defaults.

```yaml
plan-dir: plan          # where plan files live
holds:                  # ref names that count as a claim; {id} is the plan id
  - "plan/{id}"
  - "plan/{id}-*"
remote: origin          # where the lease is pushed
takeover-window: 2h     # how long a lease sits unchanged before it reads stale
sample-gap: 30m         # a gap between looks wider than this restarts the window
# base: origin/main     # pin the ref a lease is dated against
```

frit's own settings, such as `--root`, resolve most specific first.
[CLAUDE.md](../CLAUDE.md#configuration) pins the exact order with a
test. [ux-principles.md](ux-principles.md#two-kinds-of-setting)
explains why the two kinds of setting resolve differently.
