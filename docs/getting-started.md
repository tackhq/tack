# Getting Started

## Installation

### Homebrew (macOS/Linux)

```bash
brew install tackhq/tap/tack
```

### Download Binary

Download from the [releases page](https://github.com/tackhq/tack/releases).

### Go Install

```bash
go install github.com/tackhq/tack/cmd/tack@latest
```

### From Source

```bash
git clone https://github.com/tackhq/tack.git
cd tack && make build
# Binary at ./bin/tack
```

## Verify Installation

```bash
tack --version
tack modules
```

## Your First Playbook

Create `hello.yaml`:

```yaml
name: Hello Tack
hosts: localhost
connection: local
gather_facts: true

tasks:
  - name: Show system info
    command:
      cmd: echo "Hello from {{ facts.os_type }} on {{ facts.architecture }}"

  - name: Create a test file
    copy:
      dest: /tmp/tack-hello
      content: |
        Tack was here!
        OS: {{ facts.os_type }}
        User: {{ facts.user }}
```

Run it:

```bash
tack run hello.yaml             # plan + approve + apply
tack run hello.yaml --check     # preview only, no changes
tack run hello.yaml --debug     # detailed output
```

## CLI Overview

```
tack run <playbook|role>    Run a playbook or role directory
tack validate <playbook>    Check playbook syntax without executing
tack test <playbook|role>   Test in an ephemeral Docker container
tack generate               Capture live system state as a playbook
tack scaffold <name>        Create a new role with sample files
tack module <name>          Show module documentation
tack modules                List available modules
tack vault init <file>      Create a new encrypted vault file
tack vault edit <file>      Edit an existing encrypted vault file
```

### Key Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--check` / `--dry-run` | `-n` | Preview without applying |
| `--debug` | `-d` | Detailed task output |
| `--verbose` | `-v` | Full diffs in plan |
| `--auto-approve` | | Skip confirmation prompt |
| `--forks N` | `-f` | Parallel host execution (default: 1) |
| `--output json` | | Machine-readable JSON output |
| `--no-color` | | Disable colored output |
| `--inventory` | `-i` | Inventory source (YAML, executable, or plugin config; repeatable) |
| `--inventory-timeout` | | Timeout in seconds for dynamic inventory plugins (default: 30) |
| `--extra-vars` | `-e` | Extra variables (key=value) |
| `--connection` | `-c` | Connection URI (e.g. `ssh://user@host`) |

Run `tack run --help` for the full flag reference.

## Next Steps

- [Playbook Structure](playbooks.md) - tasks, handlers, loops, conditionals
- [Modules Reference](modules.md) - apt, brew, yum, file, copy, command, systemd, template
- [Variables & Facts](variables.md) - interpolation, filters, system facts
- [Connectors](connectors.md) - local, Docker, SSH, SSM
- [Roles](roles.md) - reusable role structure
