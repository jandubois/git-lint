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
git lint                    # report violations with detail lines
git lint --quiet            # report violations without detail lines
git lint --verbose          # show all checks with full details
git lint --fix              # fix what it can, warn for the rest
git-lint -C ~/git -R        # check every git repo under ~/git
git-lint -C ~/git -R --fix  # fix across all repos
```

Use `git -C <path> lint` or `git-lint -C <path>` to run in a different directory. `-R` (`--recursive`) scans immediate subdirectories for git repos and checks each one.

Exit 0 means all checks pass (warnings are acceptable). Exit 1 means at least one check failed.

Warnings and failures include detail lines (filenames, commit subjects, etc.). By default, each result shows up to `detailLines` lines of detail; `--quiet` suppresses them; `--verbose` shows all.

## Configuration

Create `~/.config/git-lint/config.json` (or `$XDG_CONFIG_HOME/git-lint/config.json`):

```json
{
  "protocol": "ssh",
  "detailLines": 10,
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

### Remote protocol (all repos, when `protocol` is set)

| Check | Fix |
|-------|-----|
| All remotes use configured protocol (`ssh` or `https`) | `git remote set-url` (GitHub only) |

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
| No untracked files older than threshold | warn only |
| No unpushed commits older than threshold | warn only |

### Branch cleanup (all repos)

| Check | Fix |
|-------|-----|
| No branches with deleted upstream (`[gone]`) | `git branch -D` |
| No branches fully merged into main | `git branch -D` |
| No stale `gh pr checkout` branches (PR merged or updated since checkout) | `git branch -D` |
| No orphan branches by other authors (no upstream, tip by someone else) | `git branch -D` |

The current branch is never deleted; switch to another branch first. Fixable warnings display in cyan on TTY output.

### Submodules (repos with `.gitmodules`)

| Check | Fix |
|-------|-----|
| Submodule initialized | `git submodule update --init --recursive` |
| Submodule commit matches parent record | warn only |
| No uncommitted changes in submodule | warn only |
| No untracked files in submodule | warn only |
| No unpushed commits in submodule | warn only |

## License

Apache License 2.0. See [LICENSE](LICENSE).
