# aim

CLI to list, resume, and migrate local AI agent sessions tied to directories.

## Supported Agents

- Claude Code (`claude`)
- Google Antigravity CLI (`agy`)
- Gemini CLI (`gemini`)
- Oh-My-Pi (`omp`)
- Pi Coding Agent (`pi`)

## Commands

### `aim ls`
List sessions tied to a folder (default: current working directory).

```bash
aim ls
aim ls /path/to/project
aim ls -a claude
aim ls --json
```

### `aim c`
Fuzzy-pick (`fzf` with numbered fallback) and resume a session interactively. Alias: `aim continue`.

```bash
aim c
aim c /path/to/project
aim c -a claude
```

### `aim mv`
Rebind session paths across agent state directories when moving or renaming a repository folder.

By default, shows planned changes and prompts for confirmation (`[y/N]`).

```bash
# Preview and prompt before applying
aim mv /path/to/old /path/to/new

# Also move directory on filesystem (os.Rename)
aim mv --fs /path/to/old /path/to/new

# Also update trusted path lists in agent settings
aim mv --trust /path/to/old /path/to/new

# Non-interactive / dry run
aim mv -f /path/to/old /path/to/new
aim mv -n /path/to/old /path/to/new
```

## Shell Aliases

Optional shortcuts for your shell configuration (`.zshrc`, `.bashrc`, `config.fish`):

```bash
alias ails="aim ls"
alias aic="aim c"
alias aimv="aim mv"
```

## Installation

Builds and installs `aim` to `~/.local/bin`:

```bash
make
```

Custom installation prefix:

```bash
make PREFIX=/usr/local
```
