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

frit's own settings belong to the host running it, not to a
repository. Each is a global flag with an environment variable:

| Flag              | Environment          | What it sets                                    |
| ----------------- | -------------------- | ----------------------------------------------- |
| `--root`          | `FRIT_ROOT`          | the directory walked for repositories           |
| `--hosts a,b`     | `FRIT_HOSTS`         | other hosts whose live agents are read over ssh |
| `--no-fetch`      | `FRIT_FETCH=false`   | skip refreshing remote-tracking refs            |
| `--no-herdr`      | `FRIT_HERDR=false`   | never run herdr, even where it is installed     |
| `--git-timeout`   | `FRIT_GIT_TIMEOUT`   | fail a stalled git call after this long         |
| `--herdr-timeout` | `FRIT_HERDR_TIMEOUT` | fail a stalled herdr call after this long       |

A config file sets one under its flag's name, such as `herdr: false`.
They resolve most specific first: the flag, the environment, then
`.frit.yml` or `$FRIT_CONFIG`, then `$XDG_CONFIG_HOME/frit/config.yml`.
[CLAUDE.md](../CLAUDE.md#configuration) pins that order with a test.
[ux-principles.md](ux-principles.md#two-kinds-of-setting) explains why
the two kinds of setting resolve differently.
