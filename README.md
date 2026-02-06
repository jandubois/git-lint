# git-lint

Check git repo health and optionally fix violations. Installs as `git-lint`, so git discovers it as `git lint`.

## Install

```sh
go install github.com/jandubois/git-lint@latest
```

Or build from source:

```sh
go build -o git-lint .
cp git-lint /usr/local/bin/
```

## Usage

```
git lint              # report violations
git lint --fix        # fix what it can, warn for the rest
git lint --verbose    # show all checks, including passing ones
```

Exit 0 means all checks pass (warnings are acceptable). Exit 1 means at least one check failed.

## Configuration

Create `~/.config/git-lint/config.json` (or `$XDG_CONFIG_HOME/git-lint/config.json`):

```json
{
  "workOrgs": ["acme", "acme-labs"],
  "identity": {
    "name": "Alice Example",
    "workEmail": "alice@acme.com",
    "personalEmail": "alice@example.com"
  },
  "thresholds": {
    "stashMaxAge": "7d",
    "stashMaxCount": 2,
    "uncommittedMaxAge": "1d",
    "unpushedMaxAge": "2d"
  }
}
```

## Rules

### Repo classification

A repo is **work** if any remote URL contains a configured work org (e.g. `github.com/acme/`). All other repos are **personal**. This classification drives identity and remote checks.

### Identity (all repos)

| Check | Fix |
|-------|-----|
| `user.name` matches configured name | `git config user.name` |
| `user.email` matches work email (work repos) or personal email | `git config user.email` |

### Remote structure (work repos with multiple remotes)

| Check | Fix |
|-------|-----|
| `origin` points to personal fork, not work org | warn only |
| main/master tracks an upstream (non-origin) remote | set tracking branch |
| main/master `pushRemote` = `no_push` | set pushRemote |
| `gh-resolved = base` on upstream remote | set gh-resolved |

### Claude Code attribution (work repos)

| Check | Fix |
|-------|-----|
| `.claude/settings.local.json` has empty attribution | create/update file |

### Staleness (all repos)

| Check | Fix |
|-------|-----|
| No stash entries older than threshold | warn only |
| Stash count within threshold | warn only |
| No uncommitted changes older than threshold | warn only |
| No unpushed commits older than threshold | warn only |

### Submodules (repos with `.gitmodules`)

| Check | Fix |
|-------|-----|
| Submodule commit matches parent record | warn only |
| No uncommitted changes in submodule | warn only |
| No untracked files in submodule | warn only |
| No unpushed commits in submodule | warn only |

## License

Apache License 2.0. See [LICENSE](LICENSE).
